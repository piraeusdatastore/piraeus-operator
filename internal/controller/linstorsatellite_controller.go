/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/barepodpatch"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/clusterapi"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/evacuation"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/imageversions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/satellite"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// LinstorSatelliteReconciler reconciles a LinstorSatellite object
type LinstorSatelliteReconciler struct {
	client.Client
	MachineClient      *clusterapi.Client
	Scheme             *runtime.Scheme
	Namespace          string
	ImageConfigMapName string
	RequeueInterval    time.Duration
	LinstorClientOpts  []lapi.Option
	Kustomizer         *resources.Kustomizer
	log                logr.Logger
}

//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LinstorSatelliteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	lsatellite := &piraeusiov1.LinstorSatellite{}
	err := r.Get(ctx, req.NamespacedName, lsatellite)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	var node corev1.Node
	err = r.Get(ctx, req.NamespacedName, &node)
	if err != nil && !k8serrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	conds := conditions.New()

	var applyErr error
	if node.Name != "" {
		applyErr = r.reconcileAppliedResource(ctx, lsatellite, &node)
		if applyErr != nil {
			conds.AddError(conditions.Applied, applyErr)
		} else {
			conds.AddSuccess(conditions.Applied, "Resources applied")
		}
	}

	var status piraeusiov1.LinstorSatelliteStatus
	var deleteErr, stateErr error
	if lsatellite.GetDeletionTimestamp() != nil {
		msg, done, err := r.deleteSatellite(ctx, lsatellite, &node, conds)
		if err != nil {
			conds.AddError("SatelliteDeleted", err)
			deleteErr = err
		} else if !done {
			conds.AddInProgress("SatelliteDeleted", msg)
		} else {
			conds.AddCompleted("SatelliteDeleted", fmt.Sprintf("deletion using '%s' policy complete", lsatellite.Spec.DeletionPolicy))
		}
	} else {
		if controllerutil.AddFinalizer(lsatellite, vars.SatelliteFinalizer) {
			deleteErr = r.Client.Update(ctx, lsatellite)
		}

		stateErr = r.reconcileLinstorSatelliteState(ctx, lsatellite, &node, conds, &status)
	}

	_, condErr := controllerutil.CreateOrPatch(ctx, r.Client, lsatellite, func() error {
		status.DeepCopyInto(&lsatellite.Status)

		for _, cond := range conds.ToConditions(lsatellite.Generation) {
			meta.SetStatusCondition(&lsatellite.Status.Conditions, cond)
		}

		if status.FreeCapacityBytes != nil && status.TotalCapacityBytes != nil {
			// Report used/total capacity. Assume "used" is total - free. Always report in GiB.
			lsatellite.Status.Capacity = fmt.Sprintf("%d/%dGiB",
				resource.NewQuantity(*status.TotalCapacityBytes-*status.FreeCapacityBytes, resource.BinarySI).ScaledValue(resource.Giga),
				resource.NewQuantity(*status.TotalCapacityBytes, resource.BinarySI).ScaledValue(resource.Giga),
			)
		}

		slices.Sort(lsatellite.Status.StorageProviders)
		slices.Sort(lsatellite.Status.DeviceLayers)

		return nil
	})

	return utils.AnyResult(ctrl.Result{RequeueAfter: r.RequeueInterval}, applyErr, stateErr, deleteErr, condErr)
}

func (r *LinstorSatelliteReconciler) reconcileAppliedResource(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) error {
	resMap, err := r.kustomizeNodeResources(ctx, lsatellite, node)
	if err != nil {
		return err
	}

	if lsatellite.GetDeletionTimestamp() != nil && lsatellite.Spec.DeletionPolicy == piraeusiov1.DeletionPolicyDelete {
		r.log.Info("Forcing deletion of Satellite resources because of deletion policy 'Delete'")
		resMap = resmap.New()
	}

	for _, res := range resMap.Resources() {
		raw, err := res.Map()
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{Object: raw}
		err = controllerutil.SetControllerReference(lsatellite, u, r.Scheme)
		if err != nil {
			return err
		}

		err = r.Client.Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner(vars.FieldOwner))
		if err != nil {
			return err
		}
	}

	err = utils.PruneResources(ctx, r.Client, lsatellite, r.Namespace, resMap,
		&appsv1.DaemonSet{},
		&corev1.Pod{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&certmanagerv1.Certificate{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *LinstorSatelliteReconciler) kustomizeNodeResources(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) (resmap.ResMap, error) {
	resourceDirs := []string{"satellite"}

	patches, err := SatelliteCommonNodePatch(lsatellite.Name)
	if err != nil {
		return nil, err
	}

	if lsatellite.Spec.InternalTLS != nil {
		secretName := lsatellite.Spec.InternalTLS.SecretName
		if secretName == "" {
			secretName = lsatellite.Name + "-tls"
		}

		p, err := SatelliteLinstorInternalTLSPatch(secretName, lsatellite.Spec.InternalTLS.CAReference)
		if err != nil {
			return nil, err
		}

		patches = append(patches, p...)

		if lsatellite.Spec.InternalTLS.CertManager != nil {
			resourceDirs = append(resourceDirs, "satellite/cert-manager")

			p, err := SatelliteLinstorInternalTLSCertManagerPatch(secretName, lsatellite.Spec.InternalTLS.CertManager)
			if err != nil {
				return nil, err
			}

			patches = append(patches, p...)
		}

		if lsatellite.Spec.InternalTLS.TLSHandshakeDaemon {
			p, err := SatelliteLinstorHandshakeDaemonPatch()
			if err != nil {
				return nil, err
			}

			patches = append(patches, p...)
		}
	}

	var bindMountPaths []string
	for i := range lsatellite.Spec.StoragePools {
		pool := &lsatellite.Spec.StoragePools[i]

		if pool.FilePool == nil && pool.FileThinPool == nil {
			continue
		}

		path := pool.PoolName()
		bindMountPaths = append(bindMountPaths, path)

		// Use an index-based name, as volume names are restricted to [0-9a-z-], so we can't use the storage pool name.
		volName := fmt.Sprintf("file-pool-%d", i)

		p, err := SatelliteHostPathVolumePatch(volName, path)
		if err != nil {
			return nil, err
		}

		patches = append(patches, p...)
	}

	if len(bindMountPaths) > 0 {
		p, err := SatelliteHostPathVolumeEnvPatch(bindMountPaths)
		if err != nil {
			return nil, err
		}

		patches = append(patches, p...)
	}

	cfg, err := imageversions.FromConfigMap(ctx, r.Client, types.NamespacedName{Name: r.ImageConfigMapName, Namespace: r.Namespace})
	if err != nil {
		return nil, err
	}

	if lsatellite.Spec.ClusterRef.ExternalController != nil {
		lc, err := linstorhelper.NewClientForCluster(
			ctx,
			r.Client,
			r.Namespace,
			&lsatellite.Spec.ClusterRef,
			r.LinstorClientOpts...,
		)
		if err != nil {
			return nil, err
		}

		err = imageversions.SetFromExternalCluster(ctx, lc.Client, cfg)
		if err != nil {
			return nil, err
		}
	}

	imgs, precompiled := cfg.GetVersions(lsatellite.Spec.Repository, node.Status.NodeInfo.OSImage)

	if precompiled {
		// Module is precompiled, so we can skip bind-mounting and add the LB_HOW variable
		p, err := SatellitePrecompiledModulePatch()
		if err != nil {
			return nil, err
		}

		patches = append(patches, p...)
	}

	userPatches, err := barepodpatch.ConvertBarePodPatch(lsatellite.Spec.Patches...)
	if err != nil {
		return nil, err
	}

	k := &kusttypes.Kustomization{
		Namespace:    r.Namespace,
		Labels:       r.kustomLabels(lsatellite.UID, lsatellite.Spec.ClusterRef.Name),
		Resources:    resourceDirs,
		Images:       imgs,
		Replacements: SatelliteNameReplacements,
		NameSuffix:   fmt.Sprintf(".%s", lsatellite.Name),
		Patches:      append(patches, utils.MakeKustPatches(userPatches...)...),
	}

	return r.Kustomizer.Kustomize(k)
}

func (r *LinstorSatelliteReconciler) reconcileLinstorSatelliteState(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node, conds conditions.Conditions, status *piraeusiov1.LinstorSatelliteStatus) error {
	// machine might be nil if
	// * the MachineClient is nil (integration disabled)
	// * the cluster is not using ClusterAPI
	// * the machine could not be found
	// this is all expected, all functions are expected to deal with that.
	machine, err := r.MachineClient.GetMachineForNode(ctx, node)
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "failed to get ClusterAPI Machine")
		return err
	}

	if lsatellite.Spec.DeletionPolicy == piraeusiov1.DeletionPolicyEvacuate {
		err = r.MachineClient.PreventMachineDeletion(ctx, machine)
		if err != nil {
			conds.AddError(conditions.Available, err)
			conds.AddUnknown(conditions.Configured, "failed to update ClusterAPI Machine")
			return err
		}
	} else {
		err = errors.Join(
			r.MachineClient.AllowMachineDrain(ctx, machine),
			r.MachineClient.AllowMachineTermination(ctx, machine),
		)
		if err != nil {
			conds.AddError(conditions.Available, err)
			conds.AddUnknown(conditions.Configured, "failed to update ClusterAPI Machine")
			return err
		}
	}

	lc, err := linstorhelper.NewClientForCluster(
		ctx,
		r.Client,
		r.Namespace,
		&lsatellite.Spec.ClusterRef,
		r.LinstorClientOpts...,
	)
	if err != nil || lc == nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Controller unreachable")
		return err
	}

	pod, err := r.getReadyPod(ctx, lsatellite)
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Pod not ready")
		return err
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = lc.Controller.GetVersion(connectCtx)
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Controller unreachable")
		return err
	}

	props, err := utils.ResolveNodeProperties(node, lsatellite.Spec.Properties...)
	if err != nil {
		conds.AddError(conditions.Configured, err)
		return err
	}

	if clusterapi.ShouldEvacuateNode(machine) {
		props[linstor.KeyAutoplaceAllowTarget] = "false"
	}

	var netIfs []lapi.NetInterface
	for _, podIP := range pod.Status.PodIPs {
		ip := net.ParseIP(podIP.IP)

		var family piraeusiov1.IPFamily
		var name string
		switch {
		case ip.To4() != nil:
			name = "default-ipv4"
			family = piraeusiov1.IPFamily(corev1.IPv4Protocol)
		case ip.To16() != nil:
			name = "default-ipv6"
			family = piraeusiov1.IPFamily(corev1.IPv6Protocol)
		default:
			conds.AddError(conditions.Available, fmt.Errorf("unrecognized address format: %s", ip.String()))
			conds.AddUnknown(conditions.Configured, "Node registration not up to date")
			return nil
		}

		if len(lsatellite.Spec.IPFamilies) > 0 && !slices.Contains(lsatellite.Spec.IPFamilies, family) {
			continue
		}

		encryptType := linstor.ValNetcomTypePlain
		port := linstor.DfltStltPortPlain
		if lsatellite.Spec.InternalTLS != nil {
			encryptType = linstor.ValNetcomTypeSsl
			port = linstor.DfltStltPortSsl
		}

		netIfs = append(netIfs, lapi.NetInterface{
			Name:                    name,
			Address:                 ip,
			SatellitePort:           int32(port),
			SatelliteEncryptionType: encryptType,
		})
	}

	lnode, err := lc.CreateOrUpdateNode(ctx, lapi.Node{
		Name:          pod.Spec.NodeName,
		Type:          linstor.ValNodeTypeStlt,
		Props:         props,
		NetInterfaces: netIfs,
	})
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Node registration not up to date")
		return err
	}

	if lnode.ConnectionStatus == "ONLINE" {
		conds.AddSuccess(conditions.Available, lnode.ConnectionStatus)

		err := r.reconcileStoragePools(ctx, lc, lsatellite, node)
		if err != nil {
			conds.AddError(conditions.Configured, err)
		} else {
			conds.AddSuccess(conditions.Configured, "Pools configured")
		}

		// Add additional status information
		sp, err := lc.Nodes.GetStoragePools(ctx, node.Name)
		if err == nil {
			var total, free int64
			for _, pool := range sp {
				if pool.TotalCapacity == math.MaxInt64 {
					// Skip all the "diskless" pools, they always report max capacity
					continue
				}

				// LINSTOR reports KiB
				total += pool.TotalCapacity * 1024
				free += pool.FreeCapacity * 1024
			}

			status.TotalCapacityBytes = &total
			status.FreeCapacityBytes = &free
		}

		rs, err := lc.Resources.GetResourceView(ctx, &lapi.ListOpts{Node: []string{node.Name}})
		if err == nil {
			n := int32(len(rs))
			status.NumberOfVolumes = &n
		}

		snaps, err := lc.Resources.GetSnapshotView(ctx, &lapi.ListOpts{Node: []string{node.Name}})
		if err == nil {
			n := int32(len(snaps))
			status.NumberOfSnapshots = &n
		}

		for _, s := range lnode.StorageProviders {
			status.StorageProviders = append(status.StorageProviders, string(s))
		}

		for _, l := range lnode.ResourceLayers {
			status.DeviceLayers = append(status.DeviceLayers, string(l))
		}

		if clusterapi.ShouldEvacuateNode(machine) && lsatellite.Spec.DeletionPolicy == piraeusiov1.DeletionPolicyEvacuate {
			r.log.Info("Request to evacuate node from ClusterAPI")
			msg, done, err := evacuation.EvacuateSatellite(ctx, r.Client, lc.Client, lnode, r.MachineClient, machine)
			if err != nil {
				conds.AddError("SatelliteEvacuated", err)
			} else if !done {
				conds.AddInProgress("SatelliteEvacuated", msg)
			} else {
				conds.AddCompleted("SatelliteEvacuated", "LINSTOR Satellite evacuated")
			}
		} else if slices.Contains(lnode.Flags, linstor.FlagEvacuate) || slices.Contains(lnode.Flags, linstor.FlagEvicted) {
			err := lc.Nodes.Restore(ctx, lnode.Name, lapi.NodeRestore{})
			if err != nil {
				conds.AddError(conditions.Configured, err)
			}
		}
	} else {
		conds.AddError(conditions.Available, fmt.Errorf("%s", lnode.ConnectionStatus))
	}

	return nil
}

func (r *LinstorSatelliteReconciler) reconcileStoragePools(ctx context.Context, lc *linstorhelper.Client, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) error {
	cached := true
	expectedPools := make(map[string]struct{})

	currentPools, err := lc.Nodes.GetStoragePools(ctx, lsatellite.Name, &lapi.ListOpts{Cached: &cached})
	if err != nil {
		return err
	}

	for i := range lsatellite.Spec.StoragePools {
		pool := &lsatellite.Spec.StoragePools[i]
		expectedPools[pool.Name] = struct{}{}

		expectedProperties, err := utils.ResolveNodeProperties(node, pool.Properties...)
		if err != nil {
			return err
		}

		expectedProperties[linstorhelper.ManagedByProperty] = vars.OperatorName
		expectedProperties[linstor.NamespcStorageDriver+"/"+linstor.KeyStorPoolName] = pool.PoolName()

		var existingPool *lapi.StoragePool
		for j := range currentPools {
			if currentPools[j].StoragePoolName == pool.Name {
				existingPool = &currentPools[j]
			}
		}

		if existingPool == nil && pool.Source != nil && len(pool.Source.HostDevices) > 0 {
			err := lc.Nodes.CreateDevicePool(ctx, lsatellite.Name, lapi.PhysicalStorageCreate{
				ProviderKind: pool.ProviderKind(),
				PoolName:     pool.PoolName(),
				DevicePaths:  pool.Source.HostDevices,
				WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
					Name:  pool.Name,
					Props: linstorhelper.UpdateLastApplyProperty(expectedProperties),
				},
			})
			if err != nil {
				r.log.Error(err, "failed to create device pool", "pool", pool)
			}

			p, err := lc.Nodes.GetStoragePool(ctx, lsatellite.Name, pool.Name, &lapi.ListOpts{Cached: &cached})
			if err == nil {
				existingPool = &p
			}
		}

		if existingPool == nil {
			err := lc.Nodes.CreateStoragePool(ctx, lsatellite.Name, lapi.StoragePool{
				StoragePoolName: pool.Name,
				ProviderKind:    pool.ProviderKind(),
				Props:           linstorhelper.UpdateLastApplyProperty(expectedProperties),
			})
			if err != nil {
				return err
			}

			p, err := lc.Nodes.GetStoragePool(ctx, lsatellite.Name, pool.Name, &lapi.ListOpts{Cached: &cached})
			if err != nil {
				return err
			}

			existingPool = &p
		}

		modification := linstorhelper.MakePropertiesModification(existingPool.Props, expectedProperties)
		if modification != nil {
			err := lc.Nodes.ModifyStoragePool(ctx, existingPool.NodeName, existingPool.StoragePoolName, *modification)
			if err != nil {
				return err
			}
		}
	}

	for i := range currentPools {
		pool := &currentPools[i]
		if pool.Props[linstorhelper.ManagedByProperty] != vars.OperatorName {
			continue
		}

		_, ok := expectedPools[currentPools[i].StoragePoolName]
		if !ok {
			err := lc.Nodes.DeleteStoragePool(ctx, lsatellite.Name, pool.StoragePoolName)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteSatellite tries to reconcile deletion of the satellites.
//
// Because this might take some time, and needs several attempts, this method returns
// * A human-readable message of what is currently preventing satellite removal.
// * true if the satellite was deleted, false otherwise.
// * Any errors that prevented further progress.
func (r *LinstorSatelliteReconciler) deleteSatellite(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node, conds conditions.Conditions) (string, bool, error) {
	if !controllerutil.ContainsFinalizer(lsatellite, vars.SatelliteFinalizer) {
		return "", true, nil
	}

	machine, err := r.MachineClient.GetMachineForNode(ctx, node)
	if err != nil {
		return "", false, err
	}

	lc, err := linstorhelper.NewClientForCluster(
		ctx,
		r.Client,
		r.Namespace,
		&lsatellite.Spec.ClusterRef,
		r.LinstorClientOpts...,
	)
	if err != nil {
		return "", false, err
	}

	if lc == nil {
		r.log.Info("Allow Machine to drain for Satellite without cluster")
		err := r.MachineClient.AllowMachineDrain(ctx, machine)
		if err != nil {
			return "", false, err
		}

		r.log.Info("Allow Machine to terminate for Satellite without cluster")
		err = r.MachineClient.AllowMachineTermination(ctx, machine)
		if err != nil {
			return "", false, err
		}

		r.log.Info("Removing finalizer from Satellite without cluster")
		controllerutil.RemoveFinalizer(lsatellite, vars.SatelliteFinalizer)
		err = r.Client.Update(ctx, lsatellite)
		if err != nil {
			return "", false, err
		}

		return "", true, nil
	}

	r.log.Info("Deleting Satellite", "Policy", lsatellite.Spec.DeletionPolicy)
	switch lsatellite.Spec.DeletionPolicy {
	case piraeusiov1.DeletionPolicyEvacuate:
		lnode, err := lc.Nodes.Get(ctx, lsatellite.Name)
		if err != nil {
			if errors.Is(err, lapi.NotFoundError) {
				// If the node is already remove, skip everything
				break
			}

			return "", false, err
		}

		msg, done, err := evacuation.EvacuateSatellite(ctx, r.Client, lc.Client, &lnode, r.MachineClient, machine)
		if err != nil {
			conds.AddError("SatelliteEvacuated", err)
			return "", false, err
		} else if !done {
			conds.AddInProgress("SatelliteEvacuated", msg)
			return msg, done, nil
		} else {
			err = lc.Nodes.Delete(ctx, lsatellite.Name)
			if err != nil && !errors.Is(err, lapi.NotFoundError) {
				return "", false, err
			}

			conds.AddCompleted("SatelliteEvacuated", "LINSTOR Satellite evacuated")
		}
	case piraeusiov1.DeletionPolicyDelete:
		err := lc.Nodes.Lost(ctx, lsatellite.Name)
		if err != nil && !errors.Is(err, lapi.NotFoundError) {
			return "", false, err
		}
	case "", piraeusiov1.DeletionPolicyRetain:
		r.log.Info("Nothing to do for deletion of satellite with 'Retain' deletion policy")
	}

	controllerutil.RemoveFinalizer(lsatellite, vars.SatelliteFinalizer)
	err = r.Client.Update(ctx, lsatellite)
	if err != nil {
		return "", false, err
	}

	return "", true, nil
}

func (r *LinstorSatelliteReconciler) kustomLabels(uuid types.UID, instance string) []kusttypes.Label {
	return []kusttypes.Label{
		{
			Pairs: map[string]string{
				"app.kubernetes.io/name":     vars.ProjectName,
				"app.kubernetes.io/instance": instance,
				vars.SatelliteNodeLabel:      string(uuid),
			},
			IncludeSelectors: true,
			IncludeTemplates: true,
		},
		{
			Pairs: vars.ExtraLabels,
		},
	}
}

func (r *LinstorSatelliteReconciler) getReadyPod(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite) (*corev1.Pod, error) {
	var pods corev1.PodList

	err := r.Client.List(ctx, &pods, client.MatchingLabels{vars.SatelliteNodeLabel: string(lsatellite.UID)})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Pods with label '%s=%s': %w", vars.SatelliteNodeLabel, lsatellite.UID, err)
	}

	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("expected one Pod, got %d with label '%s=%s'", len(pods.Items), vars.SatelliteNodeLabel, lsatellite.UID)
	}
	pod := &pods.Items[0]

	if len(pod.Status.PodIPs) == 0 {
		return nil, fmt.Errorf("no assinged IP address for Pod '%s'", pod.Name)
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return pod, nil
		}
	}

	return nil, fmt.Errorf("'%s' not ready", pod.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinstorSatelliteReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	kustomizer, err := resources.NewKustomizer(&satellite.Resources, krusty.MakeDefaultOptions())
	if err != nil {
		return err
	}
	r.Kustomizer = kustomizer

	if opts.RateLimiter == nil {
		opts.RateLimiter = DefaultRateLimiter[reconcile.Request]()
	}

	r.log = mgr.GetLogger().WithName("LinstorSatelliteReconciler")

	return ctrl.NewControllerManagedBy(mgr).
		For(&piraeusiov1.LinstorSatellite{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(predicate.Or[client.Object](predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Owns(&corev1.Secret{}, builder.WithPredicates(predicate.Or[client.Object](predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(func(_ context.Context, object client.Object) []reconcile.Request {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: object.GetName()}}}
			}),
			builder.WithPredicates(predicate.Or[client.Object](predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.allSatelliteRequests),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
				return object.GetName() == r.ImageConfigMapName && object.GetNamespace() == r.Namespace
			})),
		).
		WithOptions(opts).
		Complete(r)
}

func (r *LinstorSatelliteReconciler) allSatelliteRequests(ctx context.Context, _ client.Object) []reconcile.Request {
	satellites := piraeusiov1.LinstorSatelliteList{}
	_ = r.Client.List(ctx, &satellites)
	requests := make([]reconcile.Request, 0, len(satellites.Items))

	for i := range satellites.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: satellites.Items[i].Name},
		})
	}

	return requests
}

// SatelliteNameReplacements are the kustomize replacements for renaming resources for a single satellite.
var SatelliteNameReplacements = []kusttypes.ReplacementField{
	{Replacement: kusttypes.Replacement{
		Source: &kusttypes.SourceSelector{
			ResId: resid.NewResId(resid.NewGvk("apps", "v1", "DaemonSet"), "linstor-satellite"),
			// Selects the name of the node we expected to be running on.
			FieldPath: "spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms.0.matchFields.0.values.0",
		},
		Targets: []*kusttypes.TargetSelector{
			{
				// Sets the domain name of the issued certificate to "<node-name>"
				Select:     &kusttypes.Selector{ResId: resid.NewResId(resid.NewGvk("cert-manager.io", "v1", "Certificate"), "linstor-satellite")},
				FieldPaths: []string{"spec.dnsNames.0"},
			},
		},
	}},
}
