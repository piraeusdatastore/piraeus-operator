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

package controllers

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	linstor "github.com/LINBIT/golinstor"
	lclient "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/imageversions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/podpatcher"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/satellite"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// LinstorSatelliteReconciler reconciles a LinstorSatellite object
type LinstorSatelliteReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Namespace     string
	ImageVersions *imageversions.Config
	Kustomizer    *resources.Kustomizer
	log           logr.Logger
}

//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods;configmaps;secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LinstorSatelliteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	lsatellite := &piraeusiov1.LinstorSatellite{}
	err := r.Get(ctx, req.NamespacedName, lsatellite)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	var node corev1.Node
	err = r.Get(ctx, req.NamespacedName, &node)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	conds := conditions.New()

	var applyErr, stateErr error
	if node.Name != "" {
		applyErr = r.reconcileAppliedResource(ctx, lsatellite, &node)
		if applyErr != nil {
			conds.AddError(conditions.Applied, err)
		} else {
			conds.AddSuccess(conditions.Applied, "Resources applied")
		}

		stateErr = r.reconcileLinstorSatelliteState(ctx, lsatellite, &node, conds)
	}

	var deleteErr error
	if lsatellite.GetDeletionTimestamp() != nil {
		deleteErr = r.deleteSatellite(ctx, lsatellite)
		if deleteErr != nil {
			conds.AddError("EvacuationCompleted", deleteErr)
		} else {
			conds.AddSuccess("EvacuationCompleted", "evacuation complete")
		}
	} else {
		if controllerutil.AddFinalizer(lsatellite, vars.SatelliteFinalizer) {
			deleteErr = r.Client.Update(ctx, lsatellite)
		}
	}

	_, condErr := controllerutil.CreateOrPatch(ctx, r.Client, lsatellite, func() error {
		for _, cond := range conds.ToConditions(lsatellite.Generation) {
			meta.SetStatusCondition(&lsatellite.Status.Conditions, cond)
		}

		return nil
	})

	return ctrl.Result{}, utils.AnyError(applyErr, stateErr, deleteErr, condErr)
}

func (r *LinstorSatelliteReconciler) reconcileAppliedResource(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) error {
	resMap, err := r.kustomizeNodeResources(lsatellite, node)
	if err != nil {
		return err
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

		if u.GetKind() == "Pod" {
			err = podpatcher.Patch(ctx, r.Client, u, client.Apply, client.ForceOwnership, client.FieldOwner(vars.FieldOwner))
		} else {
			err = r.Client.Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner(vars.FieldOwner))
		}
		if err != nil {
			return err
		}
	}

	err = utils.PruneResources(ctx, r.Client, lsatellite, r.Namespace, resMap,
		&corev1.Pod{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *LinstorSatelliteReconciler) kustomizeNodeResources(lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) (resmap.ResMap, error) {
	nodeAffinityPatch, err := utils.ToEncodedPatch(&kusttypes.Selector{
		ResId: resid.NewResId(resid.NewGvk("", "v1", "Pod"), "satellite"),
	}, applycorev1.Pod("satellite", "").
		WithSpec(applycorev1.PodSpec().
			WithAffinity(applycorev1.Affinity().
				WithNodeAffinity(applycorev1.NodeAffinity().
					WithRequiredDuringSchedulingIgnoredDuringExecution(applycorev1.NodeSelector().
						WithNodeSelectorTerms(applycorev1.NodeSelectorTerm().
							WithMatchFields(applycorev1.NodeSelectorRequirement().
								WithKey("metadata.name").
								WithOperator(corev1.NodeSelectorOpIn).
								WithValues(node.Name),
							),
						),
					),
				),
			),
		),
	)
	if err != nil {
		return nil, err
	}

	nodeNamePatch, err := utils.ToEncodedPatch(&kusttypes.Selector{
		ResId: resid.NewResId(resid.NewGvk("", "v1", "Pod"), "satellite"),
	}, []utils.JsonPatch{{
		Op:    utils.Replace,
		Path:  "/metadata/name",
		Value: lsatellite.Name,
	}})
	if err != nil {
		return nil, err
	}

	satelliteConfigPatch, err := utils.ToEncodedPatch(&kusttypes.Selector{
		ResId: resid.NewResId(resid.NewGvk("", "v1", "ConfigMap"), "satellite-config"),
	}, []utils.JsonPatch{{
		Op:    utils.Replace,
		Path:  "/metadata/name",
		Value: lsatellite.Name + "-satellite-config",
	}})
	if err != nil {
		return nil, err
	}

	reactorConfigPatch, err := utils.ToEncodedPatch(&kusttypes.Selector{
		ResId: resid.NewResId(resid.NewGvk("", "v1", "ConfigMap"), "reactor-config"),
	}, []utils.JsonPatch{{
		Op:    utils.Replace,
		Path:  "/metadata/name",
		Value: lsatellite.Name + "-reactor-config",
	}})
	if err != nil {
		return nil, err
	}

	imgs, err := r.ImageVersions.GetVersions(lsatellite.Spec.Repository, node.Status.NodeInfo.OSImage)
	if err != nil {
		return nil, err
	}

	k := &kusttypes.Kustomization{
		Namespace: r.Namespace,
		Labels:    r.kustomLabels(lsatellite.Spec.ClusterRef.Name),
		Resources: []string{"pod"},
		Images:    imgs,
		Patches:   append(utils.MakeKustPatches(lsatellite.Spec.Patches...), *nodeAffinityPatch, *nodeNamePatch, *satelliteConfigPatch, *reactorConfigPatch),
	}

	return r.Kustomizer.Kustomize(k)
}

func (r *LinstorSatelliteReconciler) reconcileLinstorSatelliteState(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node, conds conditions.Conditions) error {
	lc, err := linstorhelper.NewClientForCluster(
		ctx,
		r.Client,
		lsatellite.Spec.ClusterRef.Name,
		linstorhelper.Logr(log.FromContext(ctx)),
	)
	if err != nil || lc == nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Controller unreachable")
		return err
	}

	pod := &corev1.Pod{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: lsatellite.Name, Namespace: r.Namespace}, pod)
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Missing Pod")
		return err
	}

	if len(pod.Status.PodIPs) == 0 {
		conds.AddError(conditions.Available, fmt.Errorf("missing IP address on pod"))
		conds.AddUnknown(conditions.Configured, "missing IP address on pod")
		return nil
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

	var netIfs []lclient.NetInterface
	for _, podIP := range pod.Status.PodIPs {
		ip := net.ParseIP(podIP.IP)

		var name string
		switch {
		case ip.To4() != nil:
			name = "default-ipv4"
		case ip.To16() != nil:
			name = "default-ipv6"
		default:
			conds.AddError(conditions.Available, fmt.Errorf("unrecognized address format: %s", ip.String()))
			conds.AddUnknown(conditions.Configured, "Node registration not up to date")
			return nil
		}

		netIfs = append(netIfs, lclient.NetInterface{
			Name:                    name,
			Address:                 ip,
			SatellitePort:           3366,
			SatelliteEncryptionType: linstor.ValNetcomTypePlain,
		})
	}

	lnode, err := lc.CreateOrUpdateNode(ctx, lclient.Node{
		Name:          pod.Name,
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
		conds.AddSuccess(conditions.Available, "satellite online")

		err := r.reconcileStoragePools(ctx, lc, lsatellite, node)
		if err != nil {
			conds.AddError(conditions.Configured, err)
		} else {
			conds.AddSuccess(conditions.Configured, "Pools configured")
		}
	} else {
		conds.AddError(conditions.Available, fmt.Errorf("satellite not online"))
	}

	return nil
}

func (r *LinstorSatelliteReconciler) reconcileStoragePools(ctx context.Context, lc *linstorhelper.Client, lsatellite *piraeusiov1.LinstorSatellite, node *corev1.Node) error {
	cached := true
	expectedPools := make(map[string]struct{})

	currentPools, err := lc.Nodes.GetStoragePools(ctx, lsatellite.Name, &lclient.ListOpts{Cached: &cached})
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

		var existingPool *lclient.StoragePool
		for j := range currentPools {
			if currentPools[j].StoragePoolName == pool.Name {
				existingPool = &currentPools[j]
			}
		}

		if existingPool == nil && pool.Source != nil && len(pool.Source.HostDevices) > 0 {
			err := lc.Nodes.CreateDevicePool(ctx, lsatellite.Name, lclient.PhysicalStorageCreate{
				ProviderKind: pool.ProviderKind(),
				PoolName:     pool.PoolName(),
				DevicePaths:  pool.Source.HostDevices,
				WithStoragePool: lclient.PhysicalStorageStoragePoolCreate{
					Name:  pool.Name,
					Props: linstorhelper.UpdateLastApplyProperty(expectedProperties),
				},
			})
			if err != nil {
				r.log.Error(err, "failed to create device pool", "pool", pool)
			}

			p, err := lc.Nodes.GetStoragePool(ctx, lsatellite.Name, pool.Name, &lclient.ListOpts{Cached: &cached})
			if err == nil {
				existingPool = &p
			}
		}

		if existingPool == nil {
			err := lc.Nodes.CreateStoragePool(ctx, lsatellite.Name, lclient.StoragePool{
				StoragePoolName: pool.Name,
				ProviderKind:    pool.ProviderKind(),
				Props:           linstorhelper.UpdateLastApplyProperty(expectedProperties),
			})
			if err != nil {
				return err
			}

			p, err := lc.Nodes.GetStoragePool(ctx, lsatellite.Name, pool.Name, &lclient.ListOpts{Cached: &cached})
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

func (r *LinstorSatelliteReconciler) deleteSatellite(ctx context.Context, lsatellite *piraeusiov1.LinstorSatellite) error {
	if !controllerutil.ContainsFinalizer(lsatellite, vars.SatelliteFinalizer) {
		return nil
	}

	lc, err := linstorhelper.NewClientForCluster(
		ctx,
		r.Client,
		lsatellite.Spec.ClusterRef.Name,
		linstorhelper.Logr(log.FromContext(ctx)),
	)
	if err != nil {
		return err
	}

	if lc == nil {
		r.log.Info("Removing finalizer from resource without cluster")
		controllerutil.RemoveFinalizer(lsatellite, vars.SatelliteFinalizer)
		return r.Client.Update(ctx, lsatellite)
	}

	err = lc.Nodes.Evacuate(ctx, lsatellite.Name)
	if err != nil && err != lclient.NotFoundError {
		return err
	}

	ress, err := lc.Resources.GetResourceView(ctx, &lclient.ListOpts{Node: []string{lsatellite.Name}})
	if err != nil {
		return err
	}

	if len(ress) > 0 {
		resNames := make([]string, 0, len(ress))
		for _, r := range ress {
			resNames = append(resNames, r.Name)
		}

		return fmt.Errorf("remaining resources: %s", strings.Join(resNames, ", "))
	}

	err = lc.Nodes.Delete(ctx, lsatellite.Name)
	if err != nil && err != lclient.NotFoundError {
		return err
	}

	controllerutil.RemoveFinalizer(lsatellite, vars.SatelliteFinalizer)
	err = r.Client.Update(ctx, lsatellite)
	if err != nil {
		return err
	}

	return nil
}

func (r *LinstorSatelliteReconciler) kustomLabels(instance string) []kusttypes.Label {
	return []kusttypes.Label{
		{
			Pairs: map[string]string{
				"app.kubernetes.io/name":     vars.ProjectName,
				"app.kubernetes.io/instance": instance,
			},
			IncludeSelectors: true,
			IncludeTemplates: true,
		},
		{
			Pairs: vars.ExtraLabels,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinstorSatelliteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	kustomizer, err := resources.NewKustomizer(&satellite.Resources, krusty.MakeDefaultOptions())
	if err != nil {
		return err
	}
	r.Kustomizer = kustomizer

	r.log = mgr.GetLogger().WithName("LinstorSatelliteReconciler")

	return ctrl.NewControllerManagedBy(mgr).
		For(&piraeusiov1.LinstorSatellite{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Owns(&corev1.Secret{}, builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}))).
		Watches(
			&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: object.GetName()}}}
			}),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{}, predicate.AnnotationChangedPredicate{}))).
		Complete(r)
}
