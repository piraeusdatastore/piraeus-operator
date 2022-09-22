/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

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

package linstorsatelliteset

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/LINBIT/golinstor/linstortoml"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/monitoring"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"
)

// CreateMonitoring controls if the operator will create a monitoring resources.
var CreateMonitoring = true

func newSatelliteReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorSatelliteSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func addSatelliteReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	log.Debug("satellite add: Adding a satelliteSet controller ")
	c, err := controller.New("LinstorSatelliteSet-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorSatelliteSet
	err = c.Watch(&source.Kind{Type: &piraeusv1.LinstorSatelliteSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &apps.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1.LinstorSatelliteSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorSatelliteSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorSatelliteSet{}

// ReconcileLinstorSatelliteSet reconciles a LinstorSatelliteSet object
type ReconcileLinstorSatelliteSet struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a LinstorSatelliteSet object and makes changes based on
// the state read and what is in the LinstorSatelliteSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorSatelliteSet) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
		"Controller":       "linstorsatelliteset",
	})
	log.Info("reconciling LinstorSatelliteSet")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	log.Debug("fetch resource")

	satelliteSet := &piraeusv1.LinstorSatelliteSet{}
	err := r.client.Get(ctx, request.NamespacedName, satelliteSet)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	errs := r.reconcileSpec(ctx, satelliteSet)

	statusErr := r.reconcileStatus(ctx, satelliteSet, errs)
	if statusErr != nil {
		log.Warnf("failed to update status. original errors: %v", errs)
		return reconcile.Result{}, statusErr
	}

	result, err := reconcileutil.ToReconcileResult(errs...)

	log.WithFields(logrus.Fields{
		"result": result,
		"err":    err,
	}).Info("satellite Reconcile: reconcile loop end")

	triggerStatusUpdate := reconcile.Result{RequeueAfter: 1 * time.Minute}

	return reconcileutil.CombineReconcileResults(result, triggerStatusUpdate), err
}

func (r *ReconcileLinstorSatelliteSet) reconcileSpec(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) []error {
	log := log.WithFields(logrus.Fields{
		"Op":         "reconcileSpec",
		"Controller": "linstorsatelliteset",
		"Spec":       satelliteSet.Spec,
	})
	log.Info("reconcile spec")

	log.Debug("reconcile spec with env")

	specs := []reconcileutil.EnvSpec{
		{Env: kubeSpec.ImageLinstorSatelliteEnv, Target: &satelliteSet.Spec.SatelliteImage},
		{Env: kubeSpec.ImageKernelModuleInjectionEnv, Target: &satelliteSet.Spec.KernelModuleInjectionImage},
		{Env: kubeSpec.ImageMonitoringEnv, Target: &satelliteSet.Spec.MonitoringImage},
	}

	err := reconcileutil.UpdateFromEnv(ctx, r.client, satelliteSet, specs...)
	if err != nil {
		return []error{fmt.Errorf("failed to update spec with env: %w", err)}
	}

	log.Debug("upgrade spec and set default values")

	err = r.reconcileResource(ctx, satelliteSet)
	if err != nil {
		return []error{fmt.Errorf("failed to update spec using default values: %w", err)}
	}

	log.Debug("check for deletion flag")

	markedForDeletion := satelliteSet.GetDeletionTimestamp() != nil
	if markedForDeletion {
		return r.finalizeSatelliteSet(ctx, satelliteSet)
	}

	log.Debug("add finalizer")

	if err := r.addFinalizer(ctx, satelliteSet); err != nil {
		return []error{fmt.Errorf("failed to add finalizer to resource: %w", err)}
	}

	log.Debug("reconcile legacy config map name")

	err = reconcileutil.DeleteIfOwned(ctx, r.client, &corev1.ConfigMap{ObjectMeta: getObjectMeta(satelliteSet, "%s-config")}, satelliteSet)
	if err != nil {
		return []error{fmt.Errorf("failed to delete legacy config map: %w", err)}
	}

	log.Debug("reconcile satellite configmap")

	// Create the satellite configuration
	satelliteCM, err := newSatelliteConfigMap(satelliteSet)
	if err != nil {
		return []error{fmt.Errorf("failed to reconcile satellite configmap: %w", err)}
	}

	satelliteCMChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, satelliteCM, satelliteSet, reconcileutil.OnPatchErrorReturn)
	if err != nil {
		return []error{fmt.Errorf("failed to reconcile satellite configmap: %w", err)}
	}

	log.WithField("changed", satelliteCMChanged).Debug("reconcile satellite configmap: done")

	drbdReactorCM, err := r.reconcileMonitoring(ctx, satelliteSet)
	if err != nil {
		return []error{fmt.Errorf("failed to reconcile monitoring resources: %w", err)}
	}

	log.Debug("reconcile satellite daemonset")

	ds := newSatelliteDaemonSet(satelliteSet, satelliteCM, drbdReactorCM)

	daemonsetChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, ds, satelliteSet, reconcileutil.OnPatchErrorRecreate)
	if err != nil {
		return []error{fmt.Errorf("failed to reconcile satellite daemonset: %w", err)}
	}

	log.WithField("changed", daemonsetChanged).Debug("reconcile satellite daemonset: done")

	if satelliteCMChanged && !daemonsetChanged {
		log.Debug("restart LINSTOR Satellites")

		err := reconcileutil.RestartRollout(ctx, r.client, ds)
		if err != nil {
			return []error{fmt.Errorf("failed to restart LINSTOR Controller after ConfigMap change: %w", err)}
		}
	}

	return r.reconcileAllNodesOnController(ctx, satelliteSet)
}

func (r *ReconcileLinstorSatelliteSet) reconcileMonitoring(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) (*corev1.ConfigMap, error) {
	if satelliteSet.Spec.MonitoringImage == "" {
		return nil, nil
	}

	log.Debug("reconcile legacy monitoring config map name")

	err := reconcileutil.DeleteIfOwned(ctx, r.client, &corev1.ConfigMap{ObjectMeta: getObjectMeta(satelliteSet, "%s-monitoring")}, satelliteSet)
	if err != nil {
		return nil, fmt.Errorf("failed to delete legacy monitoring config map: %w", err)
	}

	log.Debug("reconcile drbd-reactor configmap")

	drbdReactorCM := newMonitoringConfigMap(satelliteSet)

	drbdReactorCMChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, drbdReactorCM, satelliteSet, reconcileutil.OnPatchErrorReturn)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile drbd-reactor configmap")
	}

	log.WithField("changed", drbdReactorCMChanged).Debug("reconcile drbd-reactor configmap: done")

	log.Debug("reconcile legacy monitoring service name")

	err = reconcileutil.DeleteIfOwned(ctx, r.client, &corev1.Service{ObjectMeta: getObjectMeta(satelliteSet, "%s-monitoring")}, satelliteSet)
	if err != nil {
		return nil, fmt.Errorf("failed to delete legacy monitoring service: %w", err)
	}

	log.Debug("reconciling monitoring service definition")

	monitoringService := newMonitoringService(satelliteSet)

	if CreateMonitoring {
		monitoringServiceChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, monitoringService, satelliteSet, reconcileutil.OnPatchErrorReturn)
		if err != nil {
			return nil, fmt.Errorf("failed to reconcile monitoring service definition")
		}

		log.WithField("changed", monitoringServiceChanged).Debug("reconciling monitoring service definition: done")
	} else {
		err = reconcileutil.DeleteIfOwned(ctx, r.client, &corev1.Service{ObjectMeta: getObjectMeta(satelliteSet, "%s-node-monitoring")}, satelliteSet)
		if err != nil {
			return nil, fmt.Errorf("failed to delete monitoring service: %w", err)
		}
	}

	if monitoring.Enabled(ctx, r.client, r.scheme) {
		if CreateMonitoring {
			log.Debug("monitoring is available in cluster, reconciling monitoring")

			log.Debug("reconciling ServiceMonitor definition")

			serviceMonitor := monitoring.MonitorForService(monitoringService)

			serviceMonitorChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, serviceMonitor, satelliteSet, reconcileutil.OnPatchErrorRecreate)
			if err != nil {
				return nil, fmt.Errorf("failed to reconcile servicemonitor definition: %w", err)
			}

			log.WithField("changed", serviceMonitorChanged).Debug("reconciling monitoring service definition: done")
		} else {
			err = reconcileutil.DeleteIfOwned(ctx, r.client, &monitoringv1.ServiceMonitor{ObjectMeta: getObjectMeta(satelliteSet, "%s-node-monitoring")}, satelliteSet)
			if err != nil {
				return nil, fmt.Errorf("failed to delete monitoring servicemonitor: %w", err)
			}
		}
	}

	return drbdReactorCM, nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileResource(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
		"Op":        "reconcileResource",
	})
	logger.Debug("performing upgrades and fill defaults in resource")

	changed := false

	logger.Debug("performing upgrade/fill: #0 -> replace nil with zero objects")

	if satelliteSet.Spec.StoragePools == nil {
		satelliteSet.Spec.StoragePools = &shared.StoragePools{}
		changed = true

		logger.Info("set storage pool to empty default object")
	}

	if satelliteSet.Spec.StoragePools.LVMPools == nil {
		satelliteSet.Spec.StoragePools.LVMPools = make([]*shared.StoragePoolLVM, 0)
		changed = true

		logger.Info("set storage pool 'LVM' to empty list")
	}

	if satelliteSet.Spec.StoragePools.LVMThinPools == nil {
		satelliteSet.Spec.StoragePools.LVMThinPools = make([]*shared.StoragePoolLVMThin, 0)
		changed = true

		logger.Info("set storage pool 'LVMThin' to empty list")
	}

	if satelliteSet.Spec.StoragePools.ZFSPools == nil {
		satelliteSet.Spec.StoragePools.ZFSPools = make([]*shared.StoragePoolZFS, 0)
		changed = true

		logger.Info("set storage pool 'ZFSPool' to empty list")
	}

	logger.Debugf("finished upgrade/fill: #0 -> replace nil with zero objects: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #1 -> Set default endpoint URL for Client")

	if satelliteSet.Spec.ControllerEndpoint == "" {
		serviceName := types.NamespacedName{Name: satelliteSet.Name[:len(satelliteSet.Name)-3] + "-cs", Namespace: satelliteSet.Namespace}
		useHTTPS := satelliteSet.Spec.LinstorHttpsClientSecret != ""
		defaultEndpoint := lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
		satelliteSet.Spec.ControllerEndpoint = defaultEndpoint
		changed = true

		logger.Infof("set controller endpoint URL to '%s'", satelliteSet.Spec.ControllerEndpoint)
	}

	logger.Debugf("finished upgrade/fill: #1 -> Set default endpoint URL for Client: changed=%t", changed)

	logger.Debugf("performing upgrade/fill: #2 -> Set default automatic storage setup type")

	if satelliteSet.Spec.AutomaticStorageType == "" {
		satelliteSet.Spec.AutomaticStorageType = automaticStorageTypeNone
		changed = true

		logger.Infof("set default automatic storage setup type to '%s'", automaticStorageTypeNone)
	}

	logger.Debugf("finished upgrade/fill: #2 -> Set default automatic storage setup type: changed=%t", changed)

	logger.Debug("performing upgrade/full: #3 -> Set default VG name for LVMTHIN pools with device spec")

	// linstor will automatically create a VG named "linstor_$THINNAME" when creating LVMTHIN pools.
	for _, pool := range satelliteSet.Spec.StoragePools.LVMThinPools {
		if len(pool.DevicePaths) == 0 {
			continue
		}

		if pool.VolumeGroup == "" || pool.VolumeGroup == pool.CreatedVolumeGroup() {
			continue
		}

		return fmt.Errorf("lvmThinPools: `devicePaths` is set, but `volumeGroup` is not empty and does not match expected `linstor_$THINVOLUME` value: '%s'", pool.VolumeGroup)
	}

	logger.Debugf("performing upgrade/full: #3 -> Set default VG name for LVMTHIN pools with device spec: changed=%t", changed)

	logger.Debug("finished all upgrades/fills")

	if changed {
		logger.Info("save updated spec")
		return r.client.Update(ctx, satelliteSet)
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileAllNodesOnController(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) []error {
	logger := log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
		"Op":        "reconcileAllNodesOnController",
	})
	logger.Debug("start per-node reconciliation")

	logger.Debug("ensure LINSTOR controller is reachable")

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		satelliteSet.Spec.ControllerEndpoint,
		&satelliteSet.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, satelliteSet.Namespace),
	)
	if err != nil {
		return []error{err}
	}

	ok := linstorClient.ControllerReachable(ctx)
	if !ok {
		return []error{&reconcileutil.TemporaryError{
			Source:       fmt.Errorf("failed to contact controller: %w", err),
			RequeueAfter: connectionRetrySeconds * time.Second,
		}}
	}

	pods, err := r.getAllNodePods(ctx, satelliteSet)
	if err != nil {
		return []error{err}
	}

	k8sNodes := &corev1.NodeList{}

	err = r.client.List(ctx, k8sNodes)
	if err != nil {
		return []error{fmt.Errorf("failed to get kubernetes nodes: %w", err)}
	}

	wg := sync.WaitGroup{}
	errs := make([]error, len(pods))

	for i := range pods {
		i := i
		pod := &pods[i]

		k8sNode := findK8sNode(k8sNodes.Items, pod.Spec.NodeName)

		if k8sNode == nil {
			logger.WithField("pod", pod.Name).Debug("Node for pod not found, assuming node deleted.")

			continue
		}

		if pod.Status.Phase != corev1.PodRunning {
			logger.WithField("pod", pod.Name).Debug("Pod not running.")

			continue
		}

		// Registration can be done in parallel, so we handle per-node work in a separate go-routine
		wg.Add(1)

		go func() {
			defer wg.Done()

			errs[i] = r.reconcilePod(ctx, linstorClient, satelliteSet, pod, k8sNode)
		}()
	}

	wg.Wait()

	nonNilErrs := make([]error, 0, len(errs))

	for i := range errs {
		if errs[i] != nil {
			nonNilErrs = append(nonNilErrs, errs[i])
		}
	}

	if len(nonNilErrs) > 0 {
		return nonNilErrs
	}

	logger.Debug("remove registered satellites without Kubernetes node")

	err = r.removeDanglingSatellites(ctx, linstorClient, k8sNodes.Items)
	if err != nil {
		return []error{err}
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcilePod(ctx context.Context, linstorClient *lc.HighLevelClient, satelliteSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod, k8sNode *corev1.Node) error {
	podLog := log.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"Op":           "reconcilePod",
	})

	podLog.Debug("reconcile node registration")

	err := r.reconcileSingleNodeRegistration(ctx, linstorClient, satelliteSet, pod, k8sNode)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile automatic device setup")

	err = r.reconcileAutomaticDeviceSetup(ctx, linstorClient, satelliteSet, pod)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile storage pool setup")

	err = r.reconcileStoragePoolsOnNode(ctx, linstorClient, satelliteSet, pod)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile node registration: success")

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileSingleNodeRegistration(ctx context.Context, linstorClient *lc.HighLevelClient, satelliteSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod, k8sNode *corev1.Node) error {
	lNode, err := linstorClient.GetNodeOrCreate(ctx, lapi.Node{
		Name:  pod.Spec.NodeName,
		Type:  lc.Satellite,
		Props: nodeLabelsToProps(k8sNode.Labels),
		NetInterfaces: []lapi.NetInterface{
			{
				Name:                    "default",
				Address:                 pod.Status.HostIP,
				IsActive:                true,
				SatellitePort:           satelliteSet.Spec.SslConfig.Port(),
				SatelliteEncryptionType: satelliteSet.Spec.SslConfig.Type(),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile satellite: %w", err)
	}

	nodeReady := false

	for _, cond := range k8sNode.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			nodeReady = cond.Status == corev1.ConditionTrue
		}
	}

	if nodeReady && mdutil.SliceContains(lNode.Flags, linstor.FlagEvicted) {
		// The pod exists, so there is no reason not to restore it.
		err := linstorClient.Nodes.Restore(ctx, lNode.Name, lapi.NodeRestore{})
		if err != nil {
			return fmt.Errorf("node '%s' failed to restore: %w", lNode.Name, err)
		}
	}

	if lNode.ConnectionStatus != lc.Online {
		return &reconcileutil.TemporaryError{
			Source:       fmt.Errorf("node '%s' registered, but not online (%s)", lNode.Name, lNode.ConnectionStatus),
			RequeueAfter: connectionRetrySeconds * time.Second,
		}
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileAutomaticDeviceSetup(ctx context.Context, linstorClient *lc.HighLevelClient, satelliteSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
		"Pod":       pod.Name,
		"Op":        "reconcileAutomaticDeviceSetup",
	})

	logger.Debug("get existing storage pools on node")

	cached := true

	existingPools, err := linstorClient.Nodes.GetStoragePools(ctx, pod.Spec.NodeName, &lapi.ListOpts{Cached: &cached})
	if err != nil {
		return fmt.Errorf("failed to list existing storage pools: %w", err)
	}

	logger.Debug("check for re-used device paths or existing storage pools")

	devsToConfigure := sets.NewString()

	for _, poolConfig := range satelliteSet.Spec.StoragePools.AllPhysicalStorageCreators() {
		if devsToConfigure.HasAny(poolConfig.GetDevicePaths()...) {
			return fmt.Errorf("a device referenced in the storage pools is referenced twice")
		}

		if findStoragePool(existingPools, poolConfig.GetName()) != nil {
			// Pool already configured
			continue
		}

		devsToConfigure.Insert(poolConfig.GetDevicePaths()...)
	}

	logger.Debug("no device re-used")

	if devsToConfigure.Len() == 0 {
		logger.Debug("no device to configure")

		return nil
	}

	logger.Debug("fetch available devices for node")

	storageList, err := linstorClient.Nodes.GetPhysicalStorage(ctx, pod.Spec.NodeName)
	if err != nil {
		return err
	}

	emptyDevices := sets.String{}

	for _, entry := range storageList {
		emptyDevices.Insert(entry.Device)
	}

	logger.WithField("emptyDevices", emptyDevices).Debug("got available devices")

	for _, pool := range satelliteSet.Spec.StoragePools.AllPhysicalStorageCreators() {
		logger := logger.WithField("pool", pool)

		logger.Debug("checking configuration for storage pool")

		if !emptyDevices.HasAny(pool.GetDevicePaths()...) {
			logger.Debug("no device to configure, skipping")
			continue
		}

		if !emptyDevices.HasAll(pool.GetDevicePaths()...) {
			return fmt.Errorf("failed to prepare storage devices for pool '%s' on node '%s': not all devices present and empty", pool.GetName(), pod.Spec.NodeName)
		}

		err := linstorClient.Nodes.CreateDevicePool(ctx, pod.Spec.NodeName, pool.ToPhysicalStorageCreate())
		if err != nil {
			return err
		}

		emptyDevices.Delete(pool.GetDevicePaths()...)
	}

	logger.Debug("finished setting up devices for storage pool")

	// Skip setting up remaining devices
	if satelliteSet.Spec.AutomaticStorageType == automaticStorageTypeNone {
		return nil
	}

	for emptyDevice := range emptyDevices {
		// Note: not found returns -1, so in this case name == path, which is exactly what we want
		idx := strings.LastIndex(emptyDevice, "/")
		name := "autopool-" + emptyDevice[idx+1:]

		err := linstorClient.Nodes.CreateDevicePool(ctx, pod.Spec.NodeName, lapi.PhysicalStorageCreate{
			DevicePaths: []string{emptyDevice},
			PoolName:    name,
			WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
				Name: name,
			},
			ProviderKind: lapi.ProviderKind(satelliteSet.Spec.AutomaticStorageType),
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileStoragePoolsOnNode(ctx context.Context, linstorClient *lc.HighLevelClient, satelliteSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	log := log.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"podPhase":     pod.Status.Phase,
	})
	log.Debug("reconcile storage pools: started")

	cached := true

	currentPools, err := linstorClient.Nodes.GetStoragePools(ctx, pod.Spec.NodeName, &lapi.ListOpts{Cached: &cached})
	if err != nil {
		return fmt.Errorf("failed to fetch storage pools: %w", err)
	}

	log.WithField("currentPools", currentPools).Debug("got current storage pools")

	poolsFromSpec := satelliteSet.Spec.StoragePools.All()

	for i := range currentPools {
		existingPool := &currentPools[i]

		log := log.WithField("existing pool", existingPool.StoragePoolName)

		registeredByOperator := false
		if val, ok := existingPool.Props[kubeSpec.LinstorRegistrationProperty]; ok {
			registeredByOperator = val == kubeSpec.Name
		}

		if !registeredByOperator {
			log.Debug("skipping pool not managed by operator")
			continue
		}

		log.Debug("searching matching spec")

		var matchingSpec shared.StoragePool

		for j, poolSpec := range poolsFromSpec {
			if existingPool.StoragePoolName == poolSpec.GetName() {
				matchingSpec = poolSpec

				poolsFromSpec = append(poolsFromSpec[:j], poolsFromSpec[j+1:]...)

				break
			}
		}

		log.WithField("spec", matchingSpec).Debug("search complete")

		if matchingSpec == nil {
			log.WithField("pool", existingPool.StoragePoolName).Debug("removing outdated storage pool")

			// LINSTOR already ensures that the storage pool does not contain any resources
			err := linstorClient.Nodes.DeleteStoragePool(ctx, pod.Spec.NodeName, existingPool.StoragePoolName)
			if err != nil {
				return err
			}

			continue
		}

		// TODO: Should we ever create a new v2 operator: Use admission controller to prevent mutating existing pools
		fromSpec := matchingSpec.ToLinstorStoragePool()
		existingMatchesSpec := fromSpec.ProviderKind != existingPool.ProviderKind

		// We check that properties that are set from the spec are present and match.
		// Any properties that are in LINSTOR but not in the spec are ignored.
		for k, v := range fromSpec.Props {
			existing, ok := existingPool.Props[k]
			if !ok || existing != v {
				existingMatchesSpec = false
				break
			}
		}

		if existingMatchesSpec {
			return fmt.Errorf("pool '%s' does not match the spec: existing: %+v, spec: %+v", existingPool.StoragePoolName, existingPool, fromSpec)
		}
	}

	for _, pool := range poolsFromSpec {
		log.WithField("spec pool", pool).Debug("creating missing storage pool")
		err := linstorClient.Nodes.CreateStoragePool(ctx, pod.Spec.NodeName, pool.ToLinstorStoragePool())
		if err != nil {
			return err
		}
	}

	log.Debug("reconcile storage pools: finished")

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileStatus(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet, errs []error) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
	})

	logger.Debug("reconcile status of all nodes")

	// This always needs to be non-nil
	satelliteSet.Status.SatelliteStatuses = []*shared.SatelliteStatus{}

	err := r.reconcileLinstorStatus(ctx, satelliteSet)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to fetch updated status for satellites: %w", err))
	}

	logger.Debug("reconcile error list")

	satelliteSet.Status.Errors = reconcileutil.ErrorStrings(errs...)

	// Status update should always happen, even if the actual update context is canceled
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer updateCancel()

	return r.client.Status().Update(updateCtx, satelliteSet)
}

func (r *ReconcileLinstorSatelliteSet) reconcileLinstorStatus(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) error {
	log := log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
		"Op":        "reconcileLinstorStatus",
	})

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		satelliteSet.Spec.ControllerEndpoint,
		&satelliteSet.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, satelliteSet.Namespace),
	)
	if err != nil {
		return err
	}

	log.Debug("get all node pods")

	pods, err := r.getAllNodePods(ctx, satelliteSet)
	if err != nil {
		log.Warnf("could not find pods: %v, continue with empty pod list...", err)
	}

	nodeNames := make([]string, 0)
	for i := range pods {
		nodeNames = append(nodeNames, pods[i].Spec.NodeName)
	}

	log.Debug("find all satellite nodes from linstor")

	timeoutCtx, cancel := context.WithTimeout(ctx, reachableTimeout)
	defer cancel()

	linstorNodes, err := linstorClient.Nodes.GetAll(timeoutCtx, &lapi.ListOpts{Node: nodeNames})
	if err != nil {
		log.Warnf("could not fetch nodes from LINSTOR: %v, continue with empty node list", err)
	}

	satelliteSet.Status.SatelliteStatuses = make([]*shared.SatelliteStatus, len(pods))

	for i := range pods {
		pod := &pods[i]

		var matchingNode *lapi.Node

		for i := range linstorNodes {
			node := &linstorNodes[i]
			if node.Name == pod.Spec.NodeName {
				matchingNode = node
				break
			}
		}

		var pools []lapi.StoragePool

		if matchingNode != nil {
			timeoutCtx, cancel := context.WithTimeout(ctx, reachableTimeout)

			cached := true

			pools, err = linstorClient.Nodes.GetStoragePools(timeoutCtx, pod.Spec.NodeName, &lapi.ListOpts{Cached: &cached})
			if err != nil {
				log.Warnf("failed to get storage pools for node %s: %v", pod.Spec.NodeName, err)
			}

			cancel()
		}

		satelliteSet.Status.SatelliteStatuses[i] = satelliteStatusFromLinstor(pod, matchingNode, pools)
	}

	// Sort for stable status reporting
	sort.Slice(satelliteSet.Status.SatelliteStatuses, func(i, j int) bool {
		return satelliteSet.Status.SatelliteStatuses[i].NodeName < satelliteSet.Status.SatelliteStatuses[j].NodeName
	})

	return nil
}

func satelliteStatusFromLinstor(pod *corev1.Pod, node *lapi.Node, pools []lapi.StoragePool) *shared.SatelliteStatus {
	status := &shared.SatelliteStatus{
		NodeStatus: shared.NodeStatus{
			NodeName: pod.Spec.NodeName,
		},
		StoragePoolStatuses: []*shared.StoragePoolStatus{},
	}

	if node == nil {
		return status
	}

	status.ConnectionStatus = node.ConnectionStatus
	status.RegisteredOnController = node.ConnectionStatus == lc.Online

	poolsStatus := make([]*shared.StoragePoolStatus, 0)
	for i := range pools {
		poolsStatus = append(poolsStatus, shared.NewStoragePoolStatus(&pools[i]))
	}

	status.StoragePoolStatuses = poolsStatus

	// Sort for stable status reporting
	sort.Slice(status.StoragePoolStatuses, func(i, j int) bool {
		return status.StoragePoolStatuses[i].Name < status.StoragePoolStatuses[j].Name
	})

	return status
}

func (r *ReconcileLinstorSatelliteSet) getAllNodePods(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) ([]corev1.Pod, error) {
	log.WithFields(logrus.Fields{
		"Name":      satelliteSet.Name,
		"Namespace": satelliteSet.Namespace,
	}).Debug("list all node pods to register on controller")

	// Filters
	namespaceSelector := client.InNamespace(satelliteSet.Namespace)
	defaultLabels := getDefaultLabels(satelliteSet)
	labelSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(defaultLabels)}
	pods := &corev1.PodList{}

	err := r.client.List(ctx, pods, namespaceSelector, labelSelector)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func newSatelliteDaemonSet(satelliteSet *piraeusv1.LinstorSatelliteSet, satelliteCM, drbdReactorConfig *corev1.ConfigMap) *apps.DaemonSet {
	var pullSecrets []corev1.LocalObjectReference
	if satelliteSet.Spec.DrbdRepoCred != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: satelliteSet.Spec.DrbdRepoCred})
	}

	meta := getObjectMeta(satelliteSet, "%s-node")
	ds := &apps.DaemonSet{
		ObjectMeta: meta,
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: getDefaultLabels(satelliteSet)},
			UpdateStrategy: apps.DaemonSetUpdateStrategy{
				Type: apps.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &apps.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "100%"},
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec: corev1.PodSpec{
					Affinity:           satelliteSet.Spec.Affinity,
					Tolerations:        satelliteSet.Spec.Tolerations,
					HostNetwork:        true, // INFO: Per Roland, set to true
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					PriorityClassName:  satelliteSet.Spec.PriorityClassName.GetName(satelliteSet.Namespace),
					ServiceAccountName: getServiceAccountName(satelliteSet),
					Containers: append([]corev1.Container{
						{
							Name:  "linstor-satellite",
							Image: satelliteSet.Spec.SatelliteImage,
							Args: []string{
								"startSatellite",
							}, // Run linstor-satellite.
							Env:             satelliteSet.Spec.AdditionalEnv,
							ImagePullPolicy: satelliteSet.Spec.ImagePullPolicy,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      satelliteSet.Spec.SslConfig.Port(),
									ContainerPort: satelliteSet.Spec.SslConfig.Port(),
									Protocol:      "TCP",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      kubeSpec.LinstorConfDirName,
									MountPath: kubeSpec.LinstorConfDir,
								},
								{
									Name:      kubeSpec.DevDirName,
									MountPath: kubeSpec.DevDir,
								},
								{
									Name:      kubeSpec.SysDirName,
									MountPath: kubeSpec.SysDir,
								},
								{
									Name:             kubeSpec.ModulesDirName,
									MountPath:        kubeSpec.ModulesDir,
									MountPropagation: &kubeSpec.MountPropagationBidirectional,
								},
							},
							Resources: satelliteSet.Spec.Resources,
						},
					}, satelliteSet.Spec.Sidecars...),
					Volumes: append([]corev1.Volume{
						{
							Name: kubeSpec.LinstorConfDirName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: satelliteCM.Name,
									},
								},
							},
						},
						{
							Name: kubeSpec.DevDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.DevDir,
								},
							},
						},
						{
							Name: kubeSpec.SysDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.SysDir,
									Type: &kubeSpec.HostPathDirectoryType,
								},
							},
						},
						{
							Name: kubeSpec.ModulesDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.ModulesDir,
									Type: &kubeSpec.HostPathDirectoryOrCreateType,
								},
							},
						},
					}, satelliteSet.Spec.ExtraVolumes...),
					ImagePullSecrets: pullSecrets,
				},
			},
		},
	}

	ds = daemonSetWithDRBDKernelModuleInjection(ds, satelliteSet)
	ds = daemonsetWithMonitoringContainer(ds, satelliteSet, drbdReactorConfig)
	ds = daemonSetWithSslConfiguration(ds, satelliteSet)
	ds = daemonSetWithHttpsConfiguration(ds, satelliteSet)
	ds = daemonSetWithDrbdHostPaths(ds, satelliteSet)
	return ds
}

func daemonSetWithDrbdHostPaths(ds *apps.DaemonSet, set *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	if set.Spec.MountDrbdResourceDirectoriesFromHost {
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: "etc-drbd-conf",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/drbd.conf",
						Type: &kubeSpec.FileType,
					},
				},
			},
			{
				Name: "etc-drbd-d",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/drbd.d",
						Type: &kubeSpec.HostPathDirectoryType,
					},
				},
			},
			{
				Name: "var-lib-drbd",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/lib/drbd",
						Type: &kubeSpec.HostPathDirectoryType,
					},
				},
			},
			{
				Name: "var-lib-linstor-d",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/lib/linstor.d",
						Type: &kubeSpec.HostPathDirectoryType,
					},
				},
			},
		}...)

		ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(ds.Spec.Template.Spec.Containers[0].VolumeMounts, []corev1.VolumeMount{
			{
				Name:      "etc-drbd-conf",
				MountPath: "/etc/drbd.conf",
			},
			{
				Name:      "etc-drbd-d",
				MountPath: "/etc/drbd.d",
			},
			{
				Name:      "var-lib-drbd",
				MountPath: "/var/lib/drbd",
			},
			{
				Name:      "var-lib-linstor",
				MountPath: "/var/lib/linstor",
			},
			{
				Name:      "var-lib-linstor-d",
				MountPath: "/var/lib/linstor.d",
			},
		}...)
	}
	return ds
}

func daemonsetWithMonitoringContainer(ds *apps.DaemonSet, set *piraeusv1.LinstorSatelliteSet, drbdReactorConfig *corev1.ConfigMap) *apps.DaemonSet {
	if drbdReactorConfig == nil {
		return ds
	}

	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, corev1.Container{
		Name:            "drbd-prometheus-exporter",
		Image:           set.Spec.MonitoringImage,
		ImagePullPolicy: set.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "prometheus",
				ContainerPort: monitoringPort,
				HostPort:      monitoringPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Host:   set.Spec.MonitoringBindAddress,
					Scheme: corev1.URISchemeHTTP,
					Port:   intstr.FromInt(monitoringPort),
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      kubeSpec.DrbdPrometheuscConfName,
				MountPath: "/etc/drbd-reactor.d/",
			},
		},
	})

	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: kubeSpec.DrbdPrometheuscConfName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: drbdReactorConfig.Name,
				},
			},
		},
	})

	return ds
}

func newSatelliteConfigMap(satelliteSet *piraeusv1.LinstorSatelliteSet) (*corev1.ConfigMap, error) {
	config := linstortoml.Satellite{
		Logging: &linstortoml.SatelliteLogging{
			LinstorLevel: satelliteSet.Spec.LogLevel.ToLinstor(),
		},
	}

	if !satelliteSet.Spec.SslConfig.IsPlain() {
		config.NetCom = &linstortoml.SatelliteNetCom{
			Type:                satelliteSet.Spec.SslConfig.Type(),
			Port:                int(satelliteSet.Spec.SslConfig.Port()),
			ServerCertificate:   kubeSpec.LinstorSslDir + "/keystore.jks",
			TrustedCertificates: kubeSpec.LinstorSslDir + "/certificates.jks",
			// LINSTOR is currently limited on the controller side to these passwords. Because there is not much value
			// in supporting different passwords just on one side of the connection, and because these passwords are in
			// the "less secure" configmap anyways, we just use this password everywhere.
			KeyPassword:        "linstor",
			KeystorePassword:   "linstor",
			TruststorePassword: "linstor",
			SslProtocol:        "TLSv1.2",
		}
	}

	// Create a config map from it
	tomlConfigBuilder := strings.Builder{}
	tomlEncoder := toml.NewEncoder(&tomlConfigBuilder)
	if err := tomlEncoder.Encode(config); err != nil {
		return nil, err
	}

	clientConfig := lc.NewClientConfigForAPIResource(satelliteSet.Spec.ControllerEndpoint, &satelliteSet.Spec.LinstorClientConfig)
	clientConfigFile, err := clientConfig.ToConfigFile()
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: getObjectMeta(satelliteSet, "%s-node-config"),
		Data: map[string]string{
			kubeSpec.LinstorSatelliteConfigFile: tomlConfigBuilder.String(),
			kubeSpec.LinstorClientConfigFile:    clientConfigFile,
		},
	}

	return cm, nil
}

func newMonitoringConfigMap(set *piraeusv1.LinstorSatelliteSet) *corev1.ConfigMap {
	bindAddress := set.Spec.MonitoringBindAddress
	if bindAddress == "" {
		bindAddress = "0.0.0.0"
	}
	return &corev1.ConfigMap{
		ObjectMeta: getObjectMeta(set, "%s-node-monitoring"),
		Data: map[string]string{
			"prometheus.toml": fmt.Sprintf(`
[[prometheus]]
address = "%s:%d"
enums = true
`, bindAddress, monitoringPort),
		},
	}
}

func daemonSetWithDRBDKernelModuleInjection(ds *apps.DaemonSet, satelliteSet *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	var kernelModHow string

	mode := satelliteSet.Spec.KernelModuleInjectionMode
	switch mode {
	case shared.ModuleInjectionNone:
		log.WithField("drbdKernelModuleInjectionMode", mode).Warnf("using deprecated injection mode: beginning with injector image version 9.0.23, it is recommended to use '%s' instead", shared.ModuleInjectionDepsOnly)
		return ds
	case shared.ModuleInjectionCompile:
		kernelModHow = kubeSpec.LinstorKernelModCompile
	case shared.ModuleInjectionShippedModules:
		kernelModHow = kubeSpec.LinstorKernelModShippedModules
	case shared.ModuleInjectionDepsOnly:
		kernelModHow = kubeSpec.LinstorKernelModDepsOnly
	default:
		log.WithFields(logrus.Fields{
			"mode": mode,
		}).Warn("Unknown kernel module injection mode; skipping")
		return ds
	}

	env := satelliteSet.Spec.AdditionalEnv
	env = append(env,
		corev1.EnvVar{
			Name:  kubeSpec.LinstorKernelModHow,
			Value: kernelModHow,
		},
		corev1.EnvVar{
			Name:  kubeSpec.LinstorKernelModHelperCheck,
			Value: kubeSpec.LinstorKernelModHelperCheckEnabled,
		},
	)

	volumeMounts := satelliteSet.Spec.KernelModuleInjectionExtraVolumeMounts
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		// VolumumeSource for this directory is already present on the base
		// daemonset.
		Name:      kubeSpec.ModulesDirName,
		MountPath: kubeSpec.ModulesDir,
	},
	)

	ds.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            "kernel-module-injector",
			Image:           satelliteSet.Spec.KernelModuleInjectionImage,
			ImagePullPolicy: satelliteSet.Spec.ImagePullPolicy,
			SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
			Env:             env,
			VolumeMounts:    volumeMounts,
			Resources:       satelliteSet.Spec.KernelModuleInjectionResources,
		},
	}

	hostSrcDir := satelliteSet.Spec.KernelModuleInjectionAdditionalSourceDirectory
	if hostSrcDir == "" {
		hostSrcDir = kubeSpec.SrcDir
	}

	if kernelModHow == kubeSpec.LinstorKernelModCompile && path.IsAbs(hostSrcDir) {
		ds.Spec.Template.Spec.InitContainers[0].VolumeMounts = append(ds.Spec.Template.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.SrcDirName,
			MountPath: kubeSpec.SrcDir,
			ReadOnly:  true,
		})

		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: kubeSpec.SrcDirName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: hostSrcDir,
					Type: &kubeSpec.HostPathDirectoryType,
				},
			},
		})
	}

	return ds
}

func daemonSetWithSslConfiguration(ds *apps.DaemonSet, satelliteSet *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	if satelliteSet.Spec.SslConfig.IsPlain() {
		// TODO: Implement automatic SSL cert provisioning. For now we just disable SSL
		return ds
	}

	ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(ds.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      kubeSpec.LinstorSslDirName,
		MountPath: kubeSpec.LinstorSslDir,
	})

	ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(ds.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      kubeSpec.LinstorSslPemDirName,
		MountPath: kubeSpec.LinstorSslPemDir,
		ReadOnly:  true,
	})

	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: kubeSpec.LinstorSslDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: kubeSpec.LinstorSslPemDirName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: string(*satelliteSet.Spec.SslConfig),
			},
		},
	})

	return ds
}

func daemonSetWithHttpsConfiguration(ds *apps.DaemonSet, satelliteSet *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	if satelliteSet.Spec.LinstorHttpsClientSecret == "" {
		return ds
	}

	if satelliteSet.Spec.LinstorHttpsClientSecret != "" {
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: kubeSpec.LinstorClientDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: satelliteSet.Spec.LinstorHttpsClientSecret,
				},
			},
		})

		ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(ds.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorClientDirName,
			MountPath: kubeSpec.LinstorClientDir,
		})
	}

	return ds
}

func newMonitoringService(set *piraeusv1.LinstorSatelliteSet) *corev1.Service {
	meta := getObjectMeta(set, "%s-node-monitoring")

	return &corev1.Service{
		ObjectMeta: meta,
		Spec: corev1.ServiceSpec{
			Selector:  getDefaultLabels(set),
			ClusterIP: corev1.ClusterIPNone,
			Type:      corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:     kubeSpec.MonitoringPortName,
					Port:     kubeSpec.MonitorungPortNumber,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

func getServiceAccountName(satelliteSet *piraeusv1.LinstorSatelliteSet) string {
	if satelliteSet.Spec.ServiceAccountName == "" {
		return kubeSpec.LinstorSatelliteServiceAccount
	}

	return satelliteSet.Spec.ServiceAccountName
}

func (r *ReconcileLinstorSatelliteSet) finalizeNode(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet, linstorClient *lc.HighLevelClient, nodeName string) error {
	log := log.WithFields(logrus.Fields{
		"name":      satelliteSet.Name,
		"namespace": satelliteSet.Namespace,
		"spec":      fmt.Sprintf("%+v", satelliteSet.Spec),
		"node":      nodeName,
	})
	log.Debug("satellite finalizing node")
	// Determine if any resources still remain on the node.
	resList, err := linstorClient.GetAllResourcesOnNode(ctx, nodeName)
	if err != nil {
		return err
	}

	if len(resList) != 0 {
		return &reconcileutil.TemporaryError{
			Source:       fmt.Errorf("unable to remove node %s: all resources must be removed before deletion", nodeName),
			RequeueAfter: 1 * time.Minute,
		}
	}

	// No resources, safe to delete the node.
	if err := linstorClient.Nodes.Delete(ctx, nodeName); err != nil && err != lapi.NotFoundError {
		return fmt.Errorf("unable to delete node %s: %v", nodeName, err)
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) addFinalizer(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) error {
	mdutil.AddFinalizer(satelliteSet, linstorSatelliteFinalizer)

	err := r.client.Update(ctx, satelliteSet)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorSatelliteSet) deleteFinalizer(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) error {
	mdutil.DeleteFinalizer(satelliteSet, linstorSatelliteFinalizer)

	err := r.client.Update(ctx, satelliteSet)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorSatelliteSet) finalizeSatelliteSet(ctx context.Context, satelliteSet *piraeusv1.LinstorSatelliteSet) []error {
	log := log.WithFields(logrus.Fields{
		"name":      satelliteSet.Name,
		"namespace": satelliteSet.Namespace,
		"spec":      fmt.Sprintf("%+v", satelliteSet.Spec),
	})
	log.Info("found LinstorSatelliteSet marked for deletion, finalizing...")

	if !mdutil.HasFinalizer(satelliteSet, linstorSatelliteFinalizer) {
		return nil
	}

	// Run finalization logic for LinstorSatelliteSet. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	errs := make([]error, 0)
	keepNodes := make([]*shared.SatelliteStatus, 0)

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		satelliteSet.Spec.ControllerEndpoint,
		&satelliteSet.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, satelliteSet.Namespace),
	)
	if err != nil {
		return []error{err}
	}

	for _, node := range satelliteSet.Status.SatelliteStatuses {
		if err := r.finalizeNode(ctx, satelliteSet, linstorClient, node.NodeName); err != nil {
			errs = append(errs, err)
			keepNodes = append(keepNodes, node)
		}
	}

	if len(errs) == 0 {
		log.Info("finalizing finished, removing finalizer")

		err := r.deleteFinalizer(ctx, satelliteSet)

		return []error{err}
	}

	return errs
}

// Check if the controller is currently reachable.
func (r *ReconcileLinstorSatelliteSet) controllerReachable(ctx context.Context, linstorClient *lc.HighLevelClient) error {
	ctx, cancel := context.WithTimeout(ctx, reachableTimeout)
	defer cancel()

	_, err := linstorClient.Controller.GetVersion(ctx)

	return err
}

// removeDanglingSatellites removes satellites that were registered by the operator and are no longer present.
func (r *ReconcileLinstorSatelliteSet) removeDanglingSatellites(ctx context.Context, linstorClient *lc.HighLevelClient, k8sNodes []corev1.Node) error {
	lnodes, err := linstorClient.Nodes.GetAll(ctx, &lapi.ListOpts{
		Prop: []string{fmt.Sprintf("%s=%s", kubeSpec.LinstorRegistrationProperty, kubeSpec.Name)},
	})
	if err != nil {
		return fmt.Errorf("failed to list nodes")
	}

	for i := range lnodes {
		node := &lnodes[i]

		log := log.WithField("node", node.Name)

		if node.Type != lc.Satellite {
			continue
		}

		k8sNode := findK8sNode(k8sNodes, node.Name)

		if k8sNode != nil {
			log.Debug("node exists in kubernetes, no eviction")

			continue
		}

		log.Debug("node does not exist in kubernetes, evicting")

		if node.ConnectionStatus != lc.Offline {
			return fmt.Errorf("online satellite registered by operator without associated k8s node")
		}

		err := linstorClient.Nodes.Evict(ctx, node.Name)
		if err != nil {
			return fmt.Errorf("failed to evict node '%s': %w", node.Name, err)
		}

		if mdutil.SliceContains(node.Flags, linstor.FlagEvicted) {
			log.Debug("node evicted, deleting")

			err := linstorClient.Nodes.Lost(ctx, node.Name)
			if err != nil {
				return fmt.Errorf("failed to delete node '%s': %w", node.Name, err)
			}
		}
	}

	return nil
}

func findStoragePool(pools []lapi.StoragePool, name string) *lapi.StoragePool {
	for i := range pools {
		if pools[i].StoragePoolName == name {
			return &pools[i]
		}
	}

	return nil
}

func findK8sNode(nodes []corev1.Node, name string) *corev1.Node {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}

	return nil
}

func getObjectMeta(satelliteSet *piraeusv1.LinstorSatelliteSet, nameFmt string) metav1.ObjectMeta {
	defaultLabels := getDefaultLabels(satelliteSet)
	return metav1.ObjectMeta{
		Name:        fmt.Sprintf(nameFmt, satelliteSet.Name),
		Namespace:   satelliteSet.Namespace,
		Labels:      mdutil.MergeStringMap(satelliteSet.ObjectMeta.Labels, defaultLabels),
		Annotations: satelliteSet.ObjectMeta.Annotations,
	}
}

func getDefaultLabels(satelliteSet *piraeusv1.LinstorSatelliteSet) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       kubeSpec.NodeRole,
		"app.kubernetes.io/instance":   satelliteSet.Name,
		"app.kubernetes.io/managed-by": kubeSpec.Name,
	}
}

func nodeLabelsToProps(labels map[string]string) map[string]string {
	result := map[string]string{
		kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
	}

	for k, v := range labels {
		result[fmt.Sprintf("%s/%s", linstor.NamespcAuxiliary, k)] = v
	}

	return result
}

const (
	monitoringPort   = 9942
	reachableTimeout = 10 * time.Second
)
