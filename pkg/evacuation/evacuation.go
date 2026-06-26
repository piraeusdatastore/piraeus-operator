package evacuation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	affinity "github.com/piraeusdatastore/linstor-affinity-controller/pkg/version"
	linstorcsidriver "github.com/piraeusdatastore/linstor-csi/pkg/driver"
	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	schedulingcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/clusterapi"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

const (
	VolumeEvacuatingEvent                   = "VolumeEvacuating"
	VolumeTemporarilyMadeReschedulableEvent = "VolumeTemporarilyMadeReschedulable"
	VolumeDeletedForEvacuationEvent         = "VolumeDeletedForEvacuation"
	VolumeNotAttachedForEvacuationEvent     = "VolumeNotAttachedForEvacuation"
	VolumeAttachedForEvacuationEvent        = "VolumeAttachedForEvacuation"
)

// SatelliteEvacuatedCondition tracks the progress of evacuating volumes off a Satellite.
const SatelliteEvacuatedCondition = "SatelliteEvacuated"

// admissionMu serializes evacuation admission decisions so concurrent reconciles compute a consistent view
// of how many Satellites are currently evacuating.
var admissionMu sync.Mutex

// EvacuateSatellite ensures all LINSTOR Resources have been moved to other nodes, honoring the cluster-wide
// evacuation concurrency limit, and records the progress on the SatelliteEvacuatedCondition.
//
// Returns a message indicating the current progress and a bool indicating if evacuation is complete. The
// function should be called repeatedly until completion of the evacuation.
func EvacuateSatellite(ctx context.Context, cl client.Client, lclient *lapi.Client, recorder record.EventRecorder, node *lapi.Node, machineClient *clusterapi.Client, machine *clusterapi.Machine, evacuationStrategy *piraeusiov1.EvacuationStrategy, conds conditions.Conditions, maxConcurrent int) (bool, error) {
	// Gate evacuation progress on the cluster-wide concurrency limit.
	admitted, position, total, err := admitEvacuation(ctx, cl, node.Name, maxConcurrent)
	if err != nil {
		conds.AddError(SatelliteEvacuatedCondition, err)
		return false, err
	}

	if !admitted {
		msg := fmt.Sprintf("Waiting for a free evacuation slot (position %d of %d candidates, limit %d)", position+1, total, maxConcurrent)
		conds.AddWaiting(SatelliteEvacuatedCondition, msg)
		return false, nil
	}

	msg, done, err := evacuateSatellite(ctx, cl, lclient, recorder, node, machineClient, machine, evacuationStrategy)
	switch {
	case err != nil:
		conds.AddError(SatelliteEvacuatedCondition, err)
		return false, err
	case !done:
		conds.AddInProgress(SatelliteEvacuatedCondition, msg)
		return false, nil
	default:
		conds.AddCompleted(SatelliteEvacuatedCondition, "LINSTOR Satellite evacuated")
		return true, nil
	}
}

// admitEvacuation decides whether the Satellite with the given name may start (or continue) evacuating,
// honoring the cluster-wide maxConcurrent budget.
//
// Admission is derived from the cluster state on every call: all Satellites currently evacuating or waiting
// for a slot are ordered deterministically - already in-progress evacuations first (so started work is never
// interrupted), then waiting Satellites by the time they started waiting, then by name - and the first
// maxConcurrent are admitted. Because every call computes the same order, Satellites converge on the same
// admitted set without a central lock. A limit of 0 means unlimited.
//
// Returns whether the Satellite is admitted, its zero-based position in the ordering, and the total number
// of candidates.
func admitEvacuation(ctx context.Context, cl client.Client, name string, maxConcurrent int) (bool, int, int, error) {
	if maxConcurrent <= 0 {
		return true, 0, 0, nil
	}

	admissionMu.Lock()
	defer admissionMu.Unlock()

	var satellites piraeusiov1.LinstorSatelliteList
	err := cl.List(ctx, &satellites)
	if err != nil {
		return false, 0, 0, err
	}

	type candidate struct {
		name       string
		inProgress bool
		since      time.Time
	}

	now := time.Now()
	var candidates []candidate
	seenSelf := false
	for i := range satellites.Items {
		sat := &satellites.Items[i]

		cond := meta.FindStatusCondition(sat.Status.Conditions, SatelliteEvacuatedCondition)
		inProgress := cond != nil && cond.Reason == string(conditions.ReasonInProgress)
		waiting := cond != nil && cond.Reason == string(conditions.ReasonWaiting)

		// Only Satellites that are actively evacuating or already waiting for a slot consume budget. The
		// Satellite being evacuated is always included, even before it has recorded a condition.
		if !inProgress && !waiting && sat.Name != name {
			continue
		}

		since := now
		if cond != nil {
			since = cond.LastTransitionTime.Time
		}

		candidates = append(candidates, candidate{name: sat.Name, inProgress: inProgress, since: since})
		if sat.Name == name {
			seenSelf = true
		}
	}

	if !seenSelf {
		candidates = append(candidates, candidate{name: name, since: now})
	}

	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.inProgress != b.inProgress {
			if a.inProgress {
				return -1
			}
			return 1
		}
		if !a.since.Equal(b.since) {
			return a.since.Compare(b.since)
		}
		return strings.Compare(a.name, b.name)
	})

	position := slices.IndexFunc(candidates, func(c candidate) bool {
		return c.name == name
	})

	return position < maxConcurrent, position, len(candidates), nil
}

// evacuateSatellite runs the evacuation steps, ensuring all LINSTOR Resources have been moved to other nodes.
//
// Returns a message indicating the current progress and a bool indicating if evacuation is complete.
// The function should be called repeatedly until completion of the evacuation.
func evacuateSatellite(ctx context.Context, cl client.Client, lclient *lapi.Client, recorder record.EventRecorder, node *lapi.Node, machineClient *clusterapi.Client, machine *clusterapi.Machine, evacuationStrategy *piraeusiov1.EvacuationStrategy) (string, bool, error) {
	// Step 1: ensure PVs can be rescheduled to other nodes
	msg, done, err := preparePVsforEvacuation(ctx, cl, lclient, recorder, node.Name)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 2: wait for all Satellites to be online, so rescheduled workloads can attach the volumes
	msg, done, err = waitForConfiguredSatellites(ctx, cl, node.Name)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 3: Node is now ready to be drained
	err = machineClient.AllowMachineDrain(ctx, recorder, machine)
	if err != nil {
		return "", false, err
	}

	// Step 4: Wait for PVs that have been active on this node to be reattached
	msg, done, err = waitForReattach(ctx, cl, recorder, node.Name, evacuationStrategy)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 5: Start evacuating the Satellite
	err = evacuateNode(ctx, lclient, node)
	if err != nil {
		return "", false, err
	}

	// Step 6: Wait for Satellite to be completely evacuated
	msg, done, err = waitForEvacuation(ctx, lclient, node.Name)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 7: Remove temporary PV annotations
	err = cleanPVsAfterEvacuation(ctx, cl, node.Name)
	if err != nil {
		return "", false, err
	}

	// Step 8: Allow Machine to be terminated
	err = machineClient.AllowMachineTermination(ctx, recorder, machine)
	if err != nil {
		return "", false, err
	}

	return "", true, nil
}

// preparePVsforEvacuation is the first step of node evacuation.
//
// It checks for PVs that are either only accessible from the local node or are currently attached and:
// * marks them with annotations so later steps can find them.
// * ensures that "local" PVs are temporarily reschedulable during evacuation.
func preparePVsforEvacuation(ctx context.Context, cl client.Client, lclient *lapi.Client, recorder record.EventRecorder, nodeName string) (string, bool, error) {
	var volumeAttachmentList storagev1.VolumeAttachmentList
	err := cl.List(ctx, &volumeAttachmentList)
	if err != nil {
		return "", false, err
	}

	attachments := AttachmentsByPV(volumeAttachmentList.Items)

	var nodeList corev1.NodeList
	err = cl.List(ctx, &nodeList)
	if err != nil {
		return "", false, err
	}

	var persistentVolumeList corev1.PersistentVolumeList
	err = cl.List(ctx, &persistentVolumeList)
	if err != nil {
		return "", false, err
	}

	pvs := PVsByLinstorResource(persistentVolumeList.Items)

	rgSlice, err := lclient.ResourceGroups.GetAll(ctx)
	if err != nil {
		return "", false, err
	}
	rgs := make(map[string]lapi.ResourceGroup)
	for _, rg := range rgSlice {
		rgs[rg.Name] = rg
	}

	rdSlice, err := lclient.ResourceDefinitions.GetAll(ctx, lapi.RDGetAllRequest{})
	if err != nil {
		return "", false, err
	}
	rds := make(map[string]lapi.ResourceDefinition)
	for _, rd := range rdSlice {
		rds[rd.Name] = rd.ResourceDefinition
	}

	resourceList, err := getDiskfulResourcesOnNode(ctx, lclient, nodeName)
	if err != nil {
		return "", false, err
	}

	var unschedulablePVs []string
	var errs []error
	for _, rdName := range resourceList {
		pv, ok := pvs[rdName]
		if !ok {
			// No PV -> not managed by Kubernetes, so there is nothing to prepare
			continue
		}

		toUpdate := false

		if pv.Annotations == nil {
			pv.Annotations = make(map[string]string)
		}

		_, ok = pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName]
		if !ok {
			toUpdate = true

			attachment := attachments[pv.Name]
			if attachment != nil {
				pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName] = "true"
			} else {
				pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName] = "false"
			}

			PVCOrPVEventf(recorder, pv, corev1.EventTypeNormal, VolumeEvacuatingEvent, "Volume is being evacuated from Node '%s'", nodeName)
		}

		switch evacuationActionForPV(pv, rds, rgs, nodeName, nodeList.Items) {
		case PVEvacuationActionAffinityOverride:
			// This annotation is used by the Affinity Controller to override the normal Node Affinity of the PV.
			// We set it to "true", meaning "allow access from anywhere".
			_, ok := pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName]
			if !ok {
				toUpdate = true
				pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName] = "true"

				PVCOrPVEventf(recorder, pv, corev1.EventTypeNormal, VolumeTemporarilyMadeReschedulableEvent, "Volume is made reschedulable to attach on replacement node")
			}

			unschedulablePVs = append(unschedulablePVs, pv.Name)
		case PVEvacuationActionDelete:
			// Delete the PVC and the PV, so that they can get recreated on new nodes.
			if pv.Spec.ClaimRef != nil {
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pv.Spec.ClaimRef.Name,
						Namespace: pv.Spec.ClaimRef.Namespace,
					},
				}

				PVCOrPVEventf(recorder, pv, corev1.EventTypeNormal, VolumeDeletedForEvacuationEvent, "Volume deleted on node according to evacuation action")

				if err := cl.Delete(ctx, pvc, client.Preconditions(*metav1.NewUIDPreconditions(string(pv.Spec.ClaimRef.UID)))); err != nil && !k8serrors.IsNotFound(err) {
					errs = append(errs, err)
				}
			}

			if pv.DeletionTimestamp == nil {
				if err := cl.Delete(ctx, pv, client.Preconditions(*metav1.NewUIDPreconditions(string(pv.UID)))); err != nil && !k8serrors.IsNotFound(err) {
					errs = append(errs, err)
				}
			}
		case PVEvacuationActionNone:
			// Nothing to do
		}

		if toUpdate {
			if err := cl.Update(ctx, pv); err != nil && !k8serrors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return "", false, err
	}

	if len(unschedulablePVs) > 0 {
		return fmt.Sprintf("Waiting for PVs to be schedulable on other nodes: %s", strings.Join(unschedulablePVs, ", ")), false, nil
	}

	return "", true, nil
}

type PVEvacuationAction string

const (
	// PVEvacuationActionNone indicates the PV is ready for evacuation as-is.
	PVEvacuationActionNone PVEvacuationAction = "None"
	// PVEvacuationActionDelete indicates the PV and PVC should be deleted so that it gets recreated on draining the node.
	PVEvacuationActionDelete PVEvacuationAction = "Delete"
	// PVEvacuationActionAffinityOverride indicates the PV should be prepared for evacuation by setting a less strict affinity.
	PVEvacuationActionAffinityOverride PVEvacuationAction = "AffinityOverride"
)

// Determine the evacuation action for this PV.
//
// It first searches the PV annotations for vars.EvacuationActionAnnotation.
// Then, it searches first on the ResourceDefinition and then the ResourceGroup properties.
// As fallback, it returns PVEvacuationActionAffinityOverride for volumes which are local accessible only.
func evacuationActionForPV(pv *corev1.PersistentVolume, rds map[string]lapi.ResourceDefinition, rgs map[string]lapi.ResourceGroup, nodeName string, nodes []corev1.Node) PVEvacuationAction {
	if v, ok := pv.Annotations[vars.EvacuationActionAnnotation]; ok {
		return PVEvacuationAction(v)
	}

	if pv.Spec.CSI != nil {
		rd := rds[pv.Spec.CSI.VolumeHandle]
		if v, ok := rd.Props[linstor.NamespcAuxiliary+"/"+vars.EvacuationActionAnnotation]; ok {
			return PVEvacuationAction(v)
		}

		rg := rgs[rd.ResourceGroupName]
		if v, ok := rg.Props[linstor.NamespcAuxiliary+"/"+vars.EvacuationActionAnnotation]; ok {
			return PVEvacuationAction(v)
		}
	}

	// Check if the PV is "local access only". This can either be because of a local-only access policy, or some
	// more complicated affinity settings. In both cases, we need to temporarily lift the affinity requirements
	// on the PV, so that the workloads can be rescheduled.
	if hasLocalOnlyAccessPolicy(pv, nodeName) && !isAttachableOnOtherNode(pv, nodeName, nodes) {
		return PVEvacuationActionAffinityOverride
	}

	return PVEvacuationActionNone
}

func waitForConfiguredSatellites(ctx context.Context, cl client.Client, nodeName string) (string, bool, error) {
	var satellites piraeusiov1.LinstorSatelliteList
	err := cl.List(ctx, &satellites)
	if err != nil {
		return "", false, err
	}

	var unreadySatellites []string
	for _, satellite := range satellites.Items {
		// Ignore own satellite, during evacuation it might not report the full status.
		if satellite.Name == nodeName {
			continue
		}

		// Ignore all satellites with a deletion timestamp set, they are probably being removed just like this one.
		if satellite.DeletionTimestamp != nil {
			continue
		}

		cond := meta.FindStatusCondition(satellite.Status.Conditions, string(conditions.Configured))
		if cond == nil {
			unreadySatellites = append(unreadySatellites, satellite.Name)
			continue
		}

		if cond.ObservedGeneration != satellite.Generation {
			unreadySatellites = append(unreadySatellites, satellite.Name)
			continue
		}

		if cond.Status != metav1.ConditionTrue {
			unreadySatellites = append(unreadySatellites, satellite.Name)
			continue
		}
	}

	if len(unreadySatellites) > 0 {
		return fmt.Sprintf("Waiting for LinstorSatellites to become ready: %s", strings.Join(unreadySatellites, ", ")), false, nil
	}

	return "", true, nil
}

func waitForReattach(ctx context.Context, cl client.Client, recorder record.EventRecorder, nodeName string, evacuationStrategy *piraeusiov1.EvacuationStrategy) (string, bool, error) {
	now := time.Now()

	var pvs corev1.PersistentVolumeList
	err := cl.List(ctx, &pvs)
	if err != nil {
		return "", false, err
	}

	var volumeAttachmentList storagev1.VolumeAttachmentList
	err = cl.List(ctx, &volumeAttachmentList)
	if err != nil {
		return "", false, err
	}

	pvAttachementMap := toAttachedNodes(volumeAttachmentList.Items)

	var unattachedPVs []*corev1.PersistentVolume
	for _, pv := range pvs.Items {
		waitedForReattachSince := now
		if val, ok := pv.Annotations[vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName]; ok {
			waitedForReattachSince, err = time.Parse(time.RFC3339, val)
			if err != nil {
				return "", false, err
			}
		}

		switch pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName] {
		case "true":
			if now.Sub(waitedForReattachSince) >= evacuationStrategy.AttachedVolumeReattachTimeout.Duration {
				PVCOrPVEventf(recorder, &pv, corev1.EventTypeNormal, VolumeNotAttachedForEvacuationEvent, "Timed out waiting for Volume to reattach (limit: %s), proceeding with evacuation", evacuationStrategy.AttachedVolumeReattachTimeout.Duration)
				continue
			}
		case "false":
			if now.Sub(waitedForReattachSince) >= evacuationStrategy.UnattachedVolumeAttachTimeout.Duration {
				PVCOrPVEventf(recorder, &pv, corev1.EventTypeNormal, VolumeNotAttachedForEvacuationEvent, "Timed out waiting for Volume to attach (limit: %s), proceeding with evacuation", evacuationStrategy.UnattachedVolumeAttachTimeout.Duration)
				continue
			}
		default:
			continue
		}

		attachedNodes := pvAttachementMap[pv.Name]

		// We don't want the volume to still be attached to evacuating Node.
		if len(attachedNodes) == 0 || slices.Contains(attachedNodes, nodeName) {
			unattachedPVs = append(unattachedPVs, &pv)
		} else {
			// PV attached on another node, we can remove it from our PVs to wait on.
			delete(pv.Annotations, vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName)
			delete(pv.Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName)

			if err := cl.Update(ctx, &pv); err != nil {
				return "", false, err
			}

			PVCOrPVEventf(recorder, &pv, corev1.EventTypeNormal, VolumeAttachedForEvacuationEvent, "Volume successfully attached on Node(s) [%s], proceeding with evacuation", strings.Join(attachedNodes, ", "))
		}
	}

	if len(unattachedPVs) > 0 {
		unattachedPVNames := make([]string, 0, len(unattachedPVs))
		for _, pv := range unattachedPVs {
			if _, ok := pv.Annotations[vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName]; !ok {
				// NB: pv.Annotations cannot be null, as only PVs with the "wait-for-reattach" annotation are considered
				pv.Annotations[vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName] = now.Format(time.RFC3339)

				if err := cl.Update(ctx, pv); err != nil {
					return "", false, err
				}
			}

			unattachedPVNames = append(unattachedPVNames, pv.Name)
		}

		return fmt.Sprintf("Waiting for PVs to reattach on other nodes: %s", strings.Join(unattachedPVNames, ", ")), false, nil
	}

	return "", true, nil
}

func evacuateNode(ctx context.Context, lclient *lapi.Client, node *lapi.Node) error {
	ress, err := lclient.Resources.GetResourceView(ctx, &lapi.ListOpts{Node: []string{node.Name}})
	if err != nil && !errors.Is(err, lapi.NotFoundError) {
		return err
	}

	// Workaround for current LINSTOR issues:
	// * We need to call evacuate repeatedly, as a restart of the LINSTOR Controller causes LINSTOR to not properly
	//   clean up the remaining resources on the evacuated node.
	// * However, if we call evacuate while the replacement resource is syncing, LINSTOR will create a new replica
	//   because it does not consider the syncing replica a valid replacement (yet).
	// So we want to call evacuate only when:
	// * Once to initiate evacuation.
	// * All current resource are fully synced, but there are still resource remaining on the node.
	evacuationInProgress := false
	for _, resource := range ress {
		// This key is set by LINSTOR to indicate that the resource should be cleaned up after syncing. Use this as an
		// indication that the resource is being evacuated.
		if resource.Props[linstor.KeyRscMigrateFrom] != "" {
			status, err := lclient.ResourceDefinitions.SyncStatus(ctx, resource.Name)
			if err != nil {
				return fmt.Errorf("failed to check sync status of '%s': %w", resource.Name, err)
			}

			if !status.SyncedOnAll {
				evacuationInProgress = true
			}
		}
	}

	if !slices.Contains(node.Flags, linstor.FlagEvacuate) || !evacuationInProgress {
		err := lclient.Nodes.Evacuate(ctx, node.Name, lapi.NodeEvacuate{})
		if err != nil && !errors.Is(err, lapi.NotFoundError) {
			return err
		}
	}

	return nil
}

func waitForEvacuation(ctx context.Context, lclient *lapi.Client, nodeName string) (string, bool, error) {
	ress, err := lclient.Resources.GetResourceView(ctx, &lapi.ListOpts{Node: []string{nodeName}})
	if err != nil && !errors.Is(err, lapi.NotFoundError) {
		return "", false, err
	}

	snaps, err := lclient.Resources.GetSnapshotView(ctx, &lapi.ListOpts{Node: []string{nodeName}})
	if err != nil && !errors.Is(err, lapi.NotFoundError) {
		return "", false, err
	}

	var remainingResources []string
	for _, res := range ress {
		remainingResources = append(remainingResources, res.Name)
	}

	var remainingSnapshots []string
	for _, snap := range snaps {
		remainingSnapshots = append(remainingSnapshots, snap.Name)
	}

	if len(remainingResources)+len(remainingSnapshots) > 0 {
		return fmt.Sprintf("Waiting on remaining resources and snapshots: resources: [%s] snapshots: [%s]", strings.Join(remainingResources, ", "), strings.Join(remainingSnapshots, ", ")), false, nil
	}

	return "", true, nil
}

func cleanPVsAfterEvacuation(ctx context.Context, cl client.Client, nodeName string) error {
	var pvs corev1.PersistentVolumeList
	err := cl.List(ctx, &pvs)
	if err != nil {
		return err
	}

	var errs []error
	for _, pv := range pvs.Items {
		_, hasReattachSinceAnnotation := pv.Annotations[vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName]
		_, hasReattachAnnotation := pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName]
		_, hasEvacuateAnnotation := pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName]

		if hasReattachSinceAnnotation || hasEvacuateAnnotation || hasReattachAnnotation {
			delete(pv.Annotations, vars.PersistentVolumeWaitForReattachSinceAnnotationPrefix+"/"+nodeName)
			delete(pv.Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName)
			delete(pv.Annotations, affinity.OverrideAnnotationPrefix+"/"+nodeName)
			errs = append(errs, cl.Update(ctx, &pv))
		}
	}

	return errors.Join(errs...)
}

func getDiskfulResourcesOnNode(ctx context.Context, lclient *lapi.Client, nodeName string) ([]string, error) {
	ress, err := lclient.Resources.GetResourceView(ctx, &lapi.ListOpts{Node: []string{nodeName}})
	if err != nil && !errors.Is(err, lapi.NotFoundError) {
		return nil, err
	}

	var resources []string
	for _, res := range ress {
		if !slices.Contains(res.Flags, linstor.FlagDiskless) {
			resources = append(resources, res.Name)
		}
	}

	return resources, nil
}

// PVsByLinstorResource converts a list of PersistentVolumes to a map of LINSTOR Resource Names -> Persistent Volumes
func PVsByLinstorResource(pvs []corev1.PersistentVolume) map[string]*corev1.PersistentVolume {
	result := make(map[string]*corev1.PersistentVolume)
	for i := range pvs {
		pv := &pvs[i]

		if pv.Spec.CSI == nil {
			continue
		}

		if pv.Spec.CSI.Driver != linstorcsi.DriverName {
			continue
		}

		result[pv.Spec.CSI.VolumeHandle] = pv
	}

	return result
}

func AttachmentsByPV(attachments []storagev1.VolumeAttachment) map[string]*storagev1.VolumeAttachment {
	result := make(map[string]*storagev1.VolumeAttachment)
	for i := range attachments {
		if attachments[i].Spec.Source.PersistentVolumeName != nil {
			result[*attachments[i].Spec.Source.PersistentVolumeName] = &attachments[i]
		}
	}

	return result
}

// toAttachedNodes converts the attachments to a map of PV name -> attached node names
func toAttachedNodes(attachments []storagev1.VolumeAttachment) map[string][]string {
	result := make(map[string][]string)
	for _, attachment := range attachments {
		if attachment.Spec.Source.PersistentVolumeName != nil {
			result[*attachment.Spec.Source.PersistentVolumeName] = append(result[*attachment.Spec.Source.PersistentVolumeName], attachment.Spec.NodeName)
		}
	}

	return result
}

func hasLocalOnlyAccessPolicy(pv *corev1.PersistentVolume, currentNodeName string) bool {
	if pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+currentNodeName] == "true" {
		return false
	}

	if pv.Spec.CSI == nil {
		return false
	}

	if pv.Spec.CSI.VolumeAttributes[linstorcsidriver.VolumeContextMarker] != "true" {
		return false
	}

	return pv.Spec.CSI.VolumeAttributes[linstorcsidriver.RemoteAccessPolicyOpts] == "false"
}

func isAttachableOnOtherNode(pv *corev1.PersistentVolume, currentNodeName string, nodes []corev1.Node) bool {
	if pv.Spec.NodeAffinity == nil {
		return true
	}

	if pv.Spec.NodeAffinity.Required == nil {
		return true
	}

	for i := range nodes {
		if nodes[i].Name == currentNodeName {
			continue
		}

		if ok, _ := schedulingcorev1.MatchNodeSelectorTerms(&nodes[i], pv.Spec.NodeAffinity.Required); ok {
			return true
		}
	}

	return false
}

// PVCOrPVEventf records an event, either for the PVC or for the PV, if no PVC is currently bound.
func PVCOrPVEventf(recorder record.EventRecorder, pv *corev1.PersistentVolume, eventtype, reason, message string, args ...interface{}) {
	if pv.Spec.ClaimRef != nil {
		recorder.Eventf(pv.Spec.ClaimRef, eventtype, reason, message, args...)
	} else {
		recorder.Eventf(pv, eventtype, reason, message, args...)
	}
}
