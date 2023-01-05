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
	"reflect"
	"sort"
	"strings"
	"time"

	lapi "github.com/LINBIT/golinstor/client"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netwv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/merge"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/cluster"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// LinstorClusterReconciler reconciles a LinstorCluster object
type LinstorClusterReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Namespace     string
	PullSecret    string
	ImageVersions *imageversions.Config
	Kustomizer    *resources.Kustomizer
}

//+kubebuilder:rbac:groups=piraeus.io,resources=linstorclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatellites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatelliteconfigurations,verbs=get;list;watch
//+kubebuilder:rbac:groups=piraeus.io,resources=linstorsatelliteconfigurations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumes;events;configmaps;secrets;services;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=list;watch;delete
//+kubebuilder:rbac:groups=apps,resources=daemonsets;deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;clusterroles;rolebindings;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes;persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=patch
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=internal.linstor.linbit.com,resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csinodes,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments/status,verbs=patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=csistoragecapacities,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotclasses;volumesnapshots,verbs=get;list;watch
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotcontents,verbs=get;list;watch;patch;update;delete
//+kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshotcontents/status,verbs=patch;update
//+kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *LinstorClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	lcluster := &piraeusiov1.LinstorCluster{}
	err := r.Get(ctx, req.NamespacedName, lcluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	conds := conditions.New()

	applyErr := r.reconcileAppliedResource(ctx, lcluster)
	if applyErr != nil {
		conds.AddError(conditions.Applied, applyErr)
	} else {
		conds.AddSuccess(conditions.Applied, "Resources applied")
	}

	stateErr := r.reconcileClusterState(ctx, lcluster, conds)

	_, condErr := controllerutil.CreateOrPatch(ctx, r.Client, lcluster, func() error {
		for _, cond := range conds.ToConditions(lcluster.Generation) {
			meta.SetStatusCondition(&lcluster.Status.Conditions, cond)
		}

		return nil
	})

	result := ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}

	if !conds.AllHaveStatus(metav1.ConditionTrue) {
		result.RequeueAfter = 10 * time.Second
	}

	return result, utils.AnyError(applyErr, stateErr, condErr)
}

func (r *LinstorClusterReconciler) reconcileAppliedResource(ctx context.Context, lcluster *piraeusiov1.LinstorCluster) error {
	satelliteNodes := corev1.NodeList{}
	err := r.Client.List(ctx, &satelliteNodes, client.MatchingLabels(lcluster.Spec.NodeSelector))
	if err != nil {
		return err
	}

	satelliteConfigs := piraeusiov1.LinstorSatelliteConfigurationList{}
	err = r.Client.List(ctx, &satelliteConfigs)
	if err != nil {
		return err
	}

	resMap, err := r.kustomizeResources(lcluster, satelliteNodes.Items, satelliteConfigs.Items)
	if err != nil {
		return err
	}

	for _, res := range resMap.Resources() {
		raw, err := res.Map()
		if err != nil {
			return err
		}

		u := &unstructured.Unstructured{Object: raw}
		err = controllerutil.SetControllerReference(lcluster, u, r.Scheme)
		if err != nil {
			return err
		}

		// We don't need to check the delete-flag here for requeue: if a controlled item changes, we will get notified
		// and run the reconcile-loop again.
		err = r.Client.Patch(ctx, u, client.Apply, client.ForceOwnership, client.FieldOwner(vars.FieldOwner))
		if err != nil {
			return err
		}
	}

	// Update conditions on satellite configs
	for i := range satelliteConfigs.Items {
		config := &satelliteConfigs.Items[i]
		_, err = controllerutil.CreateOrPatch(ctx, r.Client, config, func() error {
			meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
				Type:               string(conditions.Applied),
				Reason:             string(conditions.ReasonAsExpected),
				Status:             metav1.ConditionTrue,
				ObservedGeneration: config.Generation,
			})

			return nil
		})
		if err != nil {
			return err
		}
	}

	err = utils.PruneResources(ctx, r.Client, lcluster, r.Namespace, resMap,
		&piraeusiov1.LinstorSatellite{},
		&corev1.Service{},
		&corev1.ServiceAccount{},
		&corev1.ConfigMap{},
		&corev1.Secret{},
		&appsv1.DaemonSet{},
		&appsv1.Deployment{},
		&rbacv1.Role{},
		&rbacv1.ClusterRole{},
		&rbacv1.RoleBinding{},
		&rbacv1.ClusterRoleBinding{},
		&netwv1.NetworkPolicy{},
		&certmanagerv1.Certificate{},
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *LinstorClusterReconciler) kustomizeResources(lcluster *piraeusiov1.LinstorCluster, satelliteNodes []corev1.Node, configs []piraeusiov1.LinstorSatelliteConfiguration) (resmap.ResMap, error) {
	ctrlRes, err := r.kustomizeControllerResources(lcluster)
	if err != nil {
		return nil, err
	}

	csiRes, err := r.kustomizeCsiResources(lcluster)
	if err != nil {
		return nil, err
	}

	commonNodeRes, err := r.kustomizeNodeCommonResources(lcluster)
	if err != nil {
		return nil, err
	}

	resMap := resmap.New()

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Name < configs[j].Name
	})

	for i := range satelliteNodes {
		satRes, err := r.kustomizeLinstorSatellite(lcluster, &satelliteNodes[i], configs)
		if err != nil {
			return nil, err
		}

		err = resMap.AppendAll(satRes)
		if err != nil {
			return nil, err
		}
	}

	err = resMap.AppendAll(ctrlRes)
	if err != nil {
		return nil, err
	}

	err = resMap.AppendAll(csiRes)
	if err != nil {
		return nil, err
	}

	err = resMap.AppendAll(commonNodeRes)
	if err != nil {
		return nil, err
	}

	return resMap, nil
}

// Create the LINSTOR Controller resources.
//
// Applies the following changes over the base resources:
// * Namespace
// * default labels
// * default images
// * pull secret (if any)
// * user defined patches
func (r *LinstorClusterReconciler) kustomizeControllerResources(lcluster *piraeusiov1.LinstorCluster) (resmap.ResMap, error) {
	var patches []kusttypes.Patch
	resourceDirs := []string{"controller"}

	if lcluster.Spec.LinstorPassphraseSecret != "" {
		p, err := ClusterLinstorPassphrasePatch(lcluster.Spec.LinstorPassphraseSecret)
		if err != nil {
			return nil, err
		}

		patches = append(patches, p...)
	}

	if lcluster.Spec.InternalTLS != nil {
		secretName := lcluster.Spec.InternalTLS.SecretName
		if secretName == "" {
			secretName = "linstor-controller-internal-tls"
		}

		p, err := ClusterLinstorInternalTLSPatch(secretName)
		if err != nil {
			return nil, err
		}
		patches = append(patches, p...)

		if lcluster.Spec.InternalTLS.CertManager != nil {
			resourceDirs = append(resourceDirs, "controller/cert-manager")

			p, err := ClusterLinstorInternalTLSCertManagerPatch(secretName, lcluster.Spec.InternalTLS.CertManager)
			if err != nil {
				return nil, err
			}

			patches = append(patches, p...)
		}
	}

	return r.kustomize(resourceDirs, lcluster, patches...)
}

// Create the CSI controller and node agent resources.
//
// Applies the following changes over the base resources:
// * Namespace
// * default labels
// * default images
// * pull secret (if any)
// * restrict CSI driver daemon set to cluster's node selector
// * user defined patches
func (r *LinstorClusterReconciler) kustomizeCsiResources(lcluster *piraeusiov1.LinstorCluster) (resmap.ResMap, error) {
	p, err := ClusterCSINodeSelectorPatch(lcluster.Spec.NodeSelector)
	if err != nil {
		return nil, err
	}

	return r.kustomize([]string{"csi"}, lcluster, p...)
}

// Create the common resources for LINSTOR satellites, but not the actual LinstorSatellite resources.
//
// The resources here are shared by all LinstorSatellite instances. This is used for:
// * A common ServiceAccount, with optional pull secret configured
// * A NetworkPolicy to protect DRBD ports from unauthorized access.
//
// Applies the following changes over the base resources:
// * Namespace
// * default labels
// * default images
// * pull secret (if any)
// * user defined patches
func (r *LinstorClusterReconciler) kustomizeNodeCommonResources(lcluster *piraeusiov1.LinstorCluster) (resmap.ResMap, error) {
	return r.kustomize([]string{"satellite-common"}, lcluster)
}

// Create the LINSTOR Satellite resources for a specific node.
//
// Applies the following changes over the base resources:
// * Use exact names for LinstorSatellite resources (== node name)
// * default labels
// * Set the cluster reference to the owning LinstorCluster
// * Apply the result of merging all LinstorSatelliteConfigurations to the LinstorSatellite
// * user defined patches
func (r *LinstorClusterReconciler) kustomizeLinstorSatellite(lcluster *piraeusiov1.LinstorCluster, node *corev1.Node, configs []piraeusiov1.LinstorSatelliteConfiguration) (resmap.ResMap, error) {
	renamePatch := utils.JsonPatch{
		Op:    utils.Replace,
		Path:  "/metadata/name",
		Value: node.Name,
	}

	repositoryPatch := utils.JsonPatch{
		Op:    utils.Replace,
		Path:  "/spec/repository",
		Value: lcluster.Spec.Repository,
	}

	clusterRefPatch := utils.JsonPatch{
		Op:   utils.Replace,
		Path: "/spec/clusterRef",
		Value: &piraeusiov1.ClusterReference{
			Name: lcluster.Name,
		},
	}

	patches := []utils.JsonPatch{renamePatch, repositoryPatch, clusterRefPatch}

	cfg := merge.SatelliteConfigurations(node.ObjectMeta.Labels, configs...)

	if cfg.Spec.InternalTLS != nil {
		patches = append(patches, utils.JsonPatch{
			Op:    utils.Add,
			Path:  "/spec/internalTLS",
			Value: cfg.Spec.InternalTLS,
		})
	}

	for j := range cfg.Spec.Properties {
		patches = append(patches, utils.JsonPatch{
			Op:    utils.Add,
			Path:  "/spec/properties/-",
			Value: &cfg.Spec.Properties[j],
		})
	}

	for j := range cfg.Spec.StoragePools {
		patches = append(patches, utils.JsonPatch{
			Op:    utils.Add,
			Path:  "/spec/storagePools/-",
			Value: &cfg.Spec.StoragePools[j],
		})
	}

	for j := range cfg.Spec.Patches {
		patches = append(patches, utils.JsonPatch{
			Op:    utils.Add,
			Path:  "/spec/patches/-",
			Value: &cfg.Spec.Patches[j],
		})
	}

	patch, err := utils.ToEncodedPatch(
		&kusttypes.Selector{ResId: resid.ResId{Gvk: resid.NewGvk(piraeusiov1.GroupVersion.Group, piraeusiov1.GroupVersion.Version, "LinstorSatellite"), Name: "satellite"}},
		patches,
	)
	if err != nil {
		return nil, err
	}

	return r.kustomize([]string{"satellite"}, lcluster, *patch)
}

// kustomize applies the common Kustomizations along with the given patches.
func (r *LinstorClusterReconciler) kustomize(resources []string, lcluster *piraeusiov1.LinstorCluster, patches ...kusttypes.Patch) (resmap.ResMap, error) {
	imgs, err := r.ImageVersions.GetVersions(lcluster.Spec.Repository, "")
	if err != nil {
		return nil, err
	}

	saPatch, err := r.pullSecretPatch()
	if err != nil {
		return nil, err
	}

	k := &kusttypes.Kustomization{
		Namespace: r.Namespace,
		Labels:    r.kustomLabels(lcluster),
		Resources: resources,
		Images:    imgs,
		Patches:   append(append(utils.MakeKustPatches(lcluster.Spec.Patches...), saPatch...), patches...),
	}

	return r.Kustomizer.Kustomize(k)
}

func (r *LinstorClusterReconciler) pullSecretPatch() ([]kusttypes.Patch, error) {
	if r.PullSecret == "" {
		return nil, nil
	}

	return PullSecretPatch(r.PullSecret)
}

func (r *LinstorClusterReconciler) kustomLabels(lcluster *piraeusiov1.LinstorCluster) []kusttypes.Label {
	return []kusttypes.Label{
		{
			Pairs: map[string]string{
				"app.kubernetes.io/name":     vars.ProjectName,
				"app.kubernetes.io/instance": lcluster.Name,
			},
			IncludeSelectors: true,
			IncludeTemplates: true,
		},
		{
			Pairs: vars.ExtraLabels,
		},
	}
}

func (r *LinstorClusterReconciler) reconcileClusterState(ctx context.Context, lcluster *piraeusiov1.LinstorCluster, conds conditions.Conditions) error {
	lc, err := linstorhelper.NewClientForCluster(
		ctx,
		r.Client,
		r.Namespace,
		lcluster.Name,
		linstorhelper.Logr(log.FromContext(ctx)),
	)
	if err != nil || lc == nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Controller unreachable")
		return err
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	version, err := lc.Controller.GetVersion(connectCtx)
	if err != nil {
		conds.AddError(conditions.Available, err)
		conds.AddUnknown(conditions.Configured, "Controller unreachable")
		return err
	}

	conds.AddSuccess(conditions.Available, fmt.Sprintf("Deployed Controller %s (API: %s, Git: %s)", version.Version, version.RestApiVersion, version.GitHash))

	current, err := lc.Controller.GetProps(ctx)
	if err != nil {
		conds.AddError(conditions.Configured, err)
		return err
	}

	expectedProperties := utils.ResolveClusterProperties(lcluster.Spec.Properties...)
	expectedProperties[linstorhelper.ManagedByProperty] = vars.OperatorName

	modification := linstorhelper.MakePropertiesModification(current, expectedProperties)
	if modification != nil {
		err = lc.Controller.Modify(ctx, *modification)
		if err != nil {
			conds.AddError(conditions.Configured, err)
			return err
		}
	}

	conds.AddSuccess(conditions.Configured, "Properties applied")

	return r.reconcileCSINodes(ctx, lcluster, lc, conds)
}

// reconcileCSINodes ensures that the CSINode resources are up-to-date.
//
// CSINode is a resource created by each Kubelet on registration of a CSI plugin. Among other things, it contains
// the list of Node labels the plugin uses for CSI Topology. Since we allow our users to customize the set of labels
// used, we need to ensure the CSINode resource is in-sync with those labels.
//
// Since labels are only synced once at plugin start-up, the only way we can sync the labels is by deleting the
// CSI Driver Pod. To minimize the Pod disruptions, we do some conservative checks, and only delete a Pod if:
// * the CSI Node pod is running and ready
// * the CSINode object for LINSTOR exists on that node
// * the Satellite is registered in LINSTOR
// * the set of expected labels does not match the reported labels
func (r *LinstorClusterReconciler) reconcileCSINodes(ctx context.Context, lcluster *piraeusiov1.LinstorCluster, lc *linstorhelper.Client, conds conditions.Conditions) error {
	var csiPods corev1.PodList
	err := r.Client.List(ctx, &csiPods, client.InNamespace(r.Namespace), client.MatchingLabels{
		"app.kubernetes.io/instance":  lcluster.Name,
		"app.kubernetes.io/component": "linstor-csi-node",
	})
	if err != nil {
		err := fmt.Errorf("failed to list linstor-csi-node pods: %w", err)
		conds.AddUnknown(conditions.Configured, err.Error())
		return err
	}

	for i := range csiPods.Items {
		pod := &csiPods.Items[i]

		if !PodReady(pod) {
			conds.AddUnknown(conditions.Configured, fmt.Sprintf("CSI node pod '%s' not ready", pod.Name))
			continue
		}

		var csiNode storagev1.CSINode
		err = r.Client.Get(ctx, types.NamespacedName{Namespace: r.Namespace, Name: pod.Spec.NodeName}, &csiNode)
		if err != nil {
			conds.AddError(conditions.Configured, fmt.Errorf("failed to get CSI Node: %w", err))
			continue
		}

		driver := GetCSINodeDriverFromNode(&csiNode)
		if driver == nil {
			conds.AddUnknown(conditions.Configured, fmt.Sprintf("CSI Node Driver not registered on node '%s'", pod.Spec.NodeName))
			continue
		}

		node, err := lc.Nodes.Get(ctx, pod.Spec.NodeName)
		if err != nil {
			conds.AddError(conditions.Configured, fmt.Errorf("failed to get LINSTOR Node: %w", err))
			continue
		}

		if !CSINodeMatchesLINSTOR(driver, &node) {
			err := r.Client.Patch(
				ctx,
				&storagev1.CSINode{ObjectMeta: metav1.ObjectMeta{Name: pod.Spec.NodeName}},
				client.RawPatch(types.StrategicMergePatchType, []byte(`{"spec":{"drivers":[{"name": "linstor.csi.linbit.com", "$patch": "delete"}]}}`)),
			)
			if err != nil {
				err := fmt.Errorf("failed to remove outdated csi node '%s': %w", pod.Spec.NodeName, err)
				conds.AddError(conditions.Configured, err)
				continue
			}

			err = r.Client.Delete(ctx, pod)
			if err != nil {
				err := fmt.Errorf("failed to restart outdated csi node pod '%s': %w", pod.Name, err)
				conds.AddError(conditions.Configured, err)
			}
		}
	}

	return nil
}

func GetCSINodeDriverFromNode(csiNode *storagev1.CSINode) *storagev1.CSINodeDriver {
	for i := range csiNode.Spec.Drivers {
		if csiNode.Spec.Drivers[i].Name == "linstor.csi.linbit.com" {
			return &csiNode.Spec.Drivers[i]
		}
	}

	return nil
}

func CSINodeMatchesLINSTOR(csiNodeDriver *storagev1.CSINodeDriver, linstorNode *lapi.Node) bool {
	var expectedKeys []string
	for k := range linstorNode.Props {
		if strings.HasPrefix(k, "Aux/topology/") {
			expectedKeys = append(expectedKeys, k[len("Aux/topology/"):])
		}
	}

	return sets.NewString(csiNodeDriver.TopologyKeys...).Equal(sets.NewString(expectedKeys...))
}

func PodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}

	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *LinstorClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	kustomizer, err := resources.NewKustomizer(&cluster.Resources, krusty.MakeDefaultOptions())
	if err != nil {
		return err
	}
	r.Kustomizer = kustomizer

	return ctrl.NewControllerManagedBy(mgr).
		For(&piraeusiov1.LinstorCluster{}).
		Owns(&piraeusiov1.LinstorSatellite{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&netwv1.NetworkPolicy{}, builder.WithPredicates(predicate.Or(predicate.LabelChangedPredicate{}, predicate.Funcs{
			// NetworkPolicy registers a change event when we just apply the same resource, so we have to filter out
			// events that do not touch the spec.
			UpdateFunc: func(updateEvent event.UpdateEvent) bool {
				oldO := updateEvent.ObjectOld.(*netwv1.NetworkPolicy)
				newO := updateEvent.ObjectNew.(*netwv1.NetworkPolicy)

				return !reflect.DeepEqual(oldO.Spec, newO.Spec)
			},
		}))).
		Watches(
			&source.Kind{Type: &corev1.Node{}}, handler.EnqueueRequestsFromMapFunc(r.allClustersRequests),
			builder.WithPredicates(predicate.LabelChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &piraeusiov1.LinstorSatelliteConfiguration{}}, handler.EnqueueRequestsFromMapFunc(r.allClustersRequests),
			builder.WithPredicates(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.LabelChangedPredicate{})),
		).
		Complete(r)
}

func (r *LinstorClusterReconciler) allClustersRequests(_ client.Object) []reconcile.Request {
	clusters := piraeusiov1.LinstorClusterList{}
	_ = r.Client.List(context.Background(), &clusters)
	requests := make([]reconcile.Request, 0, len(clusters.Items))

	for i := range clusters.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: clusters.Items[i].Name},
		})
	}

	return requests
}
