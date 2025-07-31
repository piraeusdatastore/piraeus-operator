package evacuation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	affinity "github.com/piraeusdatastore/linstor-affinity-controller/pkg/version"
	linstorcsidriver "github.com/piraeusdatastore/linstor-csi/pkg/driver"
	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	schedulingcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/clusterapi"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// EvacuateSatellite by ensuring all LINSTOR Resource have been moved to other nodes.
//
// Returns a message indicating the current progress and a bool indicating if evacuation is complete.
// The function should be called repeatedly until completion of the evacuation.
func EvacuateSatellite(ctx context.Context, cl client.Client, lclient *lapi.Client, node *lapi.Node, machineClient *clusterapi.Client, machine *clusterapi.Machine) (string, bool, error) {
	// Step 1: ensure PVs can be rescheduled to other nodes
	msg, done, err := preparePVsforEvacuation(ctx, cl, node.Name)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 2: wait for all Satellites to be online, so rescheduled workloads can attach the volumes
	msg, done, err = waitForConfiguredSatellites(ctx, cl, node.Name)
	if err != nil || !done {
		return msg, done, err
	}

	// Step 3: Node is now ready to be drained
	err = machineClient.AllowMachineDrain(ctx, machine)
	if err != nil {
		return "", false, err
	}

	// Step 4: Wait for PVs that have been active on this node to be reattached
	msg, done, err = waitForReattach(ctx, cl, node.Name)
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
	err = machineClient.AllowMachineTermination(ctx, machine)
	if err != nil {
		return "", false, err
	}

	return "", true, nil
}

// preparePVsforEvacuation is the first step of node evacuation.
//
// It checks for PVs attached to the current node and:
// * marks them with annotations so later steps can find them.
// * ensures that "local" PVs are temporarily reschedulable during evacuation.
func preparePVsforEvacuation(ctx context.Context, cl client.Client, nodeName string) (string, bool, error) {
	var volumeAttachmentList storagev1.VolumeAttachmentList
	err := cl.List(ctx, &volumeAttachmentList)
	if err != nil {
		return "", false, err
	}

	var nodeList corev1.NodeList
	err = cl.List(ctx, &nodeList)
	if err != nil {
		return "", false, err
	}

	attachedPVs := getAttachedPVs(volumeAttachmentList.Items, nodeName)
	var unschedulablePVs []string
	var errs []error
	for _, pvName := range attachedPVs {
		var pv corev1.PersistentVolume
		err := cl.Get(ctx, client.ObjectKey{Name: pvName}, &pv)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}
			return "", false, fmt.Errorf("failed to get PV %s: %w", pvName, err)
		}

		toUpdate := false

		if pv.Annotations == nil {
			pv.Annotations = make(map[string]string)
		}

		_, ok := pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName]
		if !ok {
			toUpdate = true
			pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName] = "true"
		}

		// Check if the PV is "local access only". This can either be because of a local-only access policy, or some
		// more complicated affinity settings. In both cases, we need to temporarily lift the affinity requirements
		// on the PV, so that the workloads can be rescheduled.
		if hasLocalOnlyAccessPolicy(&pv, nodeName) || !isAttachableOnOtherNode(&pv, nodeName, nodeList.Items) {
			// This annotation is used by the Affinity Controller to override the normal Node Affinity of the PV.
			// We set it to "true", meaning "allow access from anywhere".
			_, ok := pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName]
			if !ok {
				toUpdate = true
				pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName] = "true"
			}

			unschedulablePVs = append(unschedulablePVs, pv.Name)
		}

		if toUpdate {
			errs = append(errs, cl.Update(ctx, &pv))
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

func waitForReattach(ctx context.Context, cl client.Client, nodeName string) (string, bool, error) {
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

	var unattachedPVs []string
	for _, pv := range pvs.Items {
		if pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName] != "true" {
			continue
		}

		attachedNodes := pvAttachementMap[pv.Name]

		// We don't want the volume to still be attached to evacuating Node.
		if slices.Contains(attachedNodes, nodeName) {
			unattachedPVs = append(unattachedPVs, pv.Name)
		}

		if len(attachedNodes) == 0 {
			unattachedPVs = append(unattachedPVs, pv.Name)
		}
	}

	if len(unattachedPVs) > 0 {
		return fmt.Sprintf("Waiting for PVs to reattach on other nodes: %s", strings.Join(unattachedPVs, ", ")), false, nil
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
		err := lclient.Nodes.Evacuate(ctx, node.Name)
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
		_, hasReattachAnnotation := pv.Annotations[vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName]
		_, hasEvacuateAnnotation := pv.Annotations[affinity.OverrideAnnotationPrefix+"/"+nodeName]

		if hasEvacuateAnnotation || hasReattachAnnotation {
			delete(pv.Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/"+nodeName)
			delete(pv.Annotations, affinity.OverrideAnnotationPrefix+"/"+nodeName)
			errs = append(errs, cl.Update(ctx, &pv))
		}
	}

	return errors.Join(errs...)
}

func getAttachedPVs(attachments []storagev1.VolumeAttachment, nodeName string) []string {
	var attachedPVs []string
	for _, attachment := range attachments {
		if attachment.Spec.NodeName != nodeName {
			continue
		}

		if attachment.Spec.Attacher != linstorcsi.DriverName {
			continue
		}

		if attachment.Spec.Source.PersistentVolumeName == nil {
			continue
		}

		attachedPVs = append(attachedPVs, *attachment.Spec.Source.PersistentVolumeName)
	}

	return attachedPVs
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
