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
	"sort"
	"strings"
	"time"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"

	"github.com/BurntSushi/toml"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"

	lapi "github.com/LINBIT/golinstor/client"

	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func newSatelliteReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorSatelliteSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

func addSatelliteReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	log.Debug("satellite add: Adding a PNS controller ")
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
	client        client.Client
	scheme        *runtime.Scheme
	linstorClient *lc.HighLevelClient
}

// Reconcile reads that state of the cluster for a LinstorSatelliteSet object and makes changes based on
// the state read and what is in the LinstorSatelliteSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorSatelliteSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
	})
	log.Info("satellite Reconcile: reconciling LinstorSatelliteSet")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	log.Debug("fetch resource")

	pns := &piraeusv1.LinstorSatelliteSet{}
	err := r.client.Get(ctx, request.NamespacedName, pns)
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

	log.Debug("reconcile spec with env")

	specs := []reconcileutil.EnvSpec{
		{Env: kubeSpec.ImageLinstorSatelliteEnv, Target: &pns.Spec.SatelliteImage},
		{Env: kubeSpec.ImageKernelModuleInjectionEnv, Target: &pns.Spec.KernelModuleInjectionImage},
	}

	err = reconcileutil.UpdateFromEnv(ctx, r.client, pns, specs...)
	if err != nil {
		return reconcile.Result{}, err
	}

	log.Debug("check if all required fields are filled")

	err = r.reconcileResource(ctx, pns)
	if err != nil {
		return reconcile.Result{}, err
	}

	getSecret := func(secretName string) (map[string][]byte, error) {
		secret := corev1.Secret{}
		err := r.client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: pns.Namespace}, &secret)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}

	r.linstorClient, err = lc.NewHighLevelLinstorClientFromConfig(pns.Spec.ControllerEndpoint, &pns.Spec.LinstorClientConfig, getSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	markedForDeletion := pns.GetDeletionTimestamp() != nil
	if markedForDeletion {
		result, err := r.finalizeSatelliteSet(ctx, pns)

		log.WithFields(logrus.Fields{
			"result": result,
			"err":    err,
		}).Info("satellite Reconcile: reconcile loop end")

		// Resources need to be removed by human intervention, so we don't want to
		// requeue the reconcile loop immediately. We can't return the error with
		// the loop or it will automatically requeue, we log it above and it also
		// appears in the pns's Status.
		return result, err
	}

	if err := r.addFinalizer(ctx, pns); err != nil {
		return reconcile.Result{}, err
	}

	// Create the satellite configuration
	configMap, err := reconcileSatelliteConfiguration(ctx, pns, r)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Define a new DaemonSet
	ds := newDaemonSetforPNS(pns, configMap)

	// Set LinstorSatelliteSet pns as the owner and controller for the daemon set
	if err := controllerutil.SetControllerReference(pns, ds, r.scheme); err != nil {
		logrus.Debug("satellite DS Controller did not set correctly")
		return reconcile.Result{}, err
	}
	logrus.Debug("satellite DS Set up")

	// Check if this Pod already exists
	found := &apps.DaemonSet{}
	err = r.client.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.WithFields(logrus.Fields{
			"name":      ds.Name,
			"namespace": ds.Namespace,
		}).Info("satellite Reconcile: creating a new DaemonSet")

		err = r.client.Create(ctx, ds)
		if err != nil {
			log.WithFields(logrus.Fields{
				"name":      ds.Name,
				"namespace": ds.Namespace,
			}).Debug("satellite Reconcile: Error w/ Daemonset")
			return reconcile.Result{}, err
		}

		// Pod created successfully - requeue for registration
		logrus.Debug("satellite Reconcile: Daemonset created successfully")
		return reconcile.Result{Requeue: true}, nil

	} else if err != nil {
		logrus.Debug("satellite Reconcile: Error on Client.Get")
		return reconcile.Result{}, err
	}

	log.WithFields(logrus.Fields{
		"name":      ds.Name,
		"namespace": ds.Namespace,
	}).Debug("satellite Reconcile: DaemonSet already exists")

	errs := r.reconcileAllNodesOnController(ctx, pns)

	statusErr := r.reconcileStatus(ctx, pns, errs)
	if statusErr != nil {
		log.Warnf("failed to update status. original errors: %v", errs)
		return reconcile.Result{}, statusErr
	}

	result, err := reconcileutil.ToReconcileResult(errs...)

	log.WithFields(logrus.Fields{
		"result": result,
		"err":    err,
	}).Info("satellite Reconcile: reconcile loop end")

	return result, err
}

func (r *ReconcileLinstorSatelliteSet) reconcileResource(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      pns.Name,
		"Namespace": pns.Namespace,
		"Op":        "reconcileResource",
	})
	logger.Debug("performing upgrades and fill defaults in resource")

	changed := false

	logger.Debug("performing upgrade/fill: #0 -> replace nil with zero objects")

	if pns.Spec.StoragePools == nil {
		pns.Spec.StoragePools = &shared.StoragePools{}
		changed = true

		logger.Info("set storage pool to empty default object")
	}

	if pns.Spec.StoragePools.LVMPools == nil {
		pns.Spec.StoragePools.LVMPools = make([]*shared.StoragePoolLVM, 0)
		changed = true

		logger.Info("set storage pool 'LVM' to empty list")
	}

	if pns.Spec.StoragePools.LVMThinPools == nil {
		pns.Spec.StoragePools.LVMThinPools = make([]*shared.StoragePoolLVMThin, 0)
		changed = true

		logger.Info("set storage pool 'LVMThin to empty list")
	}

	logger.Debugf("finished upgrade/fill: #0 -> replace nil with zero objects: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #1 -> Set default endpoint URL for Client")

	if pns.Spec.ControllerEndpoint == "" {
		serviceName := types.NamespacedName{Name: pns.Name[:len(pns.Name)-3] + "-cs", Namespace: pns.Namespace}
		useHTTPS := pns.Spec.LinstorHttpsClientSecret != ""
		defaultEndpoint := lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
		pns.Spec.ControllerEndpoint = defaultEndpoint
		changed = true

		logger.Infof("set controller endpoint URL to '%s'", pns.Spec.ControllerEndpoint)
	}

	logger.Debugf("finished upgrade/fill: #1 -> Set default endpoint URL for Client: changed=%t", changed)

	logger.Debugf("performing upgrade/fill: #2 -> Set default automatic storage setup type")

	if pns.Spec.AutomaticStorageType == "" {
		pns.Spec.AutomaticStorageType = automaticStorageTypeNone
		changed = true

		logger.Infof("set default automatic storage setup type to '%s'", automaticStorageTypeNone)
	}

	logger.Debugf("finished upgrade/fill: #2 -> Set default automatic storage setup type: changed=%t", changed)

	logger.Debug("finished all upgrades/fills")

	if changed {
		logger.Info("save updated spec")
		return r.client.Update(ctx, pns)
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileAllNodesOnController(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet) []error {
	logger := log.WithFields(logrus.Fields{
		"Name":      nodeSet.Name,
		"Namespace": nodeSet.Namespace,
		"Op":        "reconcileAllNodesOnController",
	})
	logger.Debug("start per-node reconciliation")

	pods, err := r.getAllNodePods(ctx, nodeSet)
	if err != nil {
		return []error{err}
	}

	// Every registration routine gets its own channel to send a list of errors
	outputs := make([]chan error, len(pods))

	for i := range pods {
		pod := &pods[i]
		output := make(chan error)
		outputs[i] = output

		// Registration can be done in parallel, so we handle per-node work in a separate go-routine
		go func() {
			defer close(output)
			output <- r.reconcilePod(ctx, nodeSet, pod)
		}()
	}

	logger.Debug("start collecting per-node reconciliation results")

	registerErrors := make([]error, 0)

	// Every pod has its own error channel. This preserves order of errors on multiple runs
	for i := range pods {
		err := <-outputs[i]
		if err != nil {
			registerErrors = append(registerErrors, err)
		}
	}

	return registerErrors
}

func (r *ReconcileLinstorSatelliteSet) reconcilePod(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	podLog := log.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"Op":           "reconcilePod",
	})

	podLog.Debug("ensure LINSTOR controller is reachable")

	_, err := r.linstorClient.Nodes.GetControllerVersion(ctx)
	if err != nil {
		return &reconcileutil.TemporaryError{
			Source:       err,
			RequeueAfter: connectionRetrySeconds * time.Second,
		}
	}

	podLog.Debug("reconcile node registration")

	err = r.reconcileSingleNodeRegistration(ctx, nodeSet, pod)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile automatic device setup")

	err = r.reconcileAutomaticDeviceSetup(ctx, nodeSet, pod)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile storage pool setup")

	err = r.reconcileStoragePoolsOnNode(ctx, nodeSet, pod)
	if err != nil {
		return err
	}

	podLog.Debug("reconcile node registration: success")

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileSingleNodeRegistration(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	lNode, err := r.linstorClient.GetNodeOrCreate(ctx, lapi.Node{
		Name: pod.Spec.NodeName,
		Type: lc.Satellite,
		NetInterfaces: []lapi.NetInterface{
			{
				Name:                    "default",
				Address:                 pod.Status.HostIP,
				SatellitePort:           nodeSet.Spec.SslConfig.Port(),
				SatelliteEncryptionType: nodeSet.Spec.SslConfig.Type(),
			},
		},
	})
	if err != nil {
		return err
	}

	if lNode.ConnectionStatus != lc.Online {
		return &reconcileutil.TemporaryError{
			Source:       fmt.Errorf("node '%s' registered, but not online (%s)", lNode.Name, lNode.ConnectionStatus),
			RequeueAfter: connectionRetrySeconds * time.Second,
		}
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileAutomaticDeviceSetup(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      nodeSet.Name,
		"Namespace": nodeSet.Namespace,
		"Pod":       pod.Name,
		"Op":        "reconcileAutomaticDeviceSetup",
	})
	// No setup required
	if nodeSet.Spec.AutomaticStorageType == automaticStorageTypeNone {
		return nil
	}

	logger.Debug("fetch available devices for node")

	storageList, err := r.linstorClient.Nodes.GetPhysicalStorage(ctx, &lapi.ListOpts{Node: []string{pod.Spec.NodeName}})
	if err != nil {
		return err
	}

	paths := make([]string, 0)

	for _, entry := range storageList {
		ourNode, ok := entry.Nodes[pod.Spec.NodeName]
		if !ok {
			continue
		}

		for _, dev := range ourNode {
			paths = append(paths, dev.Device)
		}
	}

	for _, path := range paths {
		logger.WithField("path", path).Debug("prepare device")

		// Note: not found returns -1, so in this case name == path, which is exactly what we want
		idx := strings.LastIndex(path, "/")
		name := path[idx+1:]

		err := r.linstorClient.Nodes.CreateDevicePool(ctx, pod.Spec.NodeName, lapi.PhysicalStorageCreate{
			DevicePaths: []string{path},
			PoolName:    name,
			WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
				Name: name,
			},
			ProviderKind: lapi.ProviderKind(nodeSet.Spec.AutomaticStorageType),
		})
		if err != nil {
			return err
		}
	}

	logger.Debug("devices set up")

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileStoragePoolsOnNode(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet, pod *corev1.Pod) error {
	log := log.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"podPhase":     pod.Status.Phase,
	})
	log.Debug("reconcile storage pools: started")

	log.Debug("reconcile LVM storage pools")

	for _, pool := range nodeSet.Spec.StoragePools.LVMPools {
		_, err := r.linstorClient.GetStoragePoolOrCreateOnNode(ctx, pool.ToLinstorStoragePool(), pod.Spec.NodeName)
		if err != nil {
			return err
		}
	}

	log.Debug("reconcile LVM thin storage pools")

	for _, pool := range nodeSet.Spec.StoragePools.LVMThinPools {
		_, err := r.linstorClient.GetStoragePoolOrCreateOnNode(ctx, pool.ToLinstorStoragePool(), pod.Spec.NodeName)
		if err != nil {
			return err
		}
	}

	log.Debug("reconcile storage pools: finished")

	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileStatus(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet, errs []error) error {
	logger := log.WithFields(logrus.Fields{
		"Name":      nodeSet.Name,
		"Namespace": nodeSet.Namespace,
	})
	logger.Debug("reconcile status of all nodes")

	logger.Debug("reconcile error list")

	nodeSet.Status.Errors = reconcileutil.ErrorStrings(errs...)

	logger.Debug("get all node pods")

	pods, err := r.getAllNodePods(ctx, nodeSet)
	if err != nil {
		logger.Warnf("could not find pods: %v, continue with empty pod list...", err)
	}

	nodeNames := make([]string, 0)
	for i := range pods {
		nodeNames = append(nodeNames, pods[i].Spec.NodeName)
	}

	logger.Debug("find all satellite nodes from linstor")

	linstorNodes, err := r.linstorClient.Nodes.GetAll(ctx, &lapi.ListOpts{Node: nodeNames})
	if err != nil {
		logger.Warnf("could not fetch nodes from LINSTOR: %v, continue with empty node list", err)
	}

	nodeSet.Status.SatelliteStatuses = make([]*shared.SatelliteStatus, len(pods))

	for i := range pods {
		pod := &pods[i]

		var matchingNode *lapi.Node = nil

		for i := range linstorNodes {
			node := &linstorNodes[i]
			if node.Name == pod.Spec.NodeName {
				matchingNode = node
				break
			}
		}

		pools, err := r.linstorClient.Nodes.GetStoragePools(ctx, pod.Spec.NodeName)
		if err != nil {
			logger.Warnf("failed to get storage pools for node %s: %v", pod.Spec.NodeName, err)
		}

		nodeSet.Status.SatelliteStatuses[i] = satelliteStatusFromLinstor(pod, matchingNode, pools)
	}

	// Sort for stable status reporting
	sort.Slice(nodeSet.Status.SatelliteStatuses, func(i, j int) bool {
		return nodeSet.Status.SatelliteStatuses[i].NodeName < nodeSet.Status.SatelliteStatuses[j].NodeName
	})

	return r.client.Status().Update(ctx, nodeSet)
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

func (r *ReconcileLinstorSatelliteSet) getAllNodePods(ctx context.Context, nodeSet *piraeusv1.LinstorSatelliteSet) ([]corev1.Pod, error) {
	log.WithFields(logrus.Fields{
		"Name":      nodeSet.Name,
		"Namespace": nodeSet.Namespace,
	}).Debug("list all node pods to register on controller")

	// Filters
	namespaceSelector := client.InNamespace(nodeSet.Namespace)
	labelSelector := client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(pnsLabels(nodeSet))}
	pods := &corev1.PodList{}

	err := r.client.List(ctx, pods, namespaceSelector, labelSelector)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func newDaemonSetforPNS(pns *piraeusv1.LinstorSatelliteSet, config *corev1.ConfigMap) *apps.DaemonSet {
	labels := pnsLabels(pns)

	var pullSecrets []corev1.LocalObjectReference
	if pns.Spec.DrbdRepoCred != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: pns.Spec.DrbdRepoCred})
	}

	ds := &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pns.Name + "-node",
			Namespace: pns.Namespace,
			Labels:    labels,
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pns.Name + "-node",
					Namespace: pns.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Affinity:           pns.Spec.Affinity,
					Tolerations:        pns.Spec.Tolerations,
					HostNetwork:        true, // INFO: Per Roland, set to true
					DNSPolicy:          corev1.DNSClusterFirstWithHostNet,
					PriorityClassName:  pns.Spec.PriorityClassName.GetName(pns.Namespace),
					ServiceAccountName: kubeSpec.LinstorSatelliteServiceAccount,
					Containers: []corev1.Container{
						{
							Name:  "linstor-satellite",
							Image: pns.Spec.SatelliteImage,
							Args: []string{
								"startSatellite",
							}, // Run linstor-satellite.
							ImagePullPolicy: pns.Spec.ImagePullPolicy,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      pns.Spec.SslConfig.Port(),
									ContainerPort: pns.Spec.SslConfig.Port(),
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
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(int(pns.Spec.SslConfig.Port())),
									},
								},
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								FailureThreshold:    10,
								InitialDelaySeconds: 10,
							},
							Resources: pns.Spec.Resources,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: kubeSpec.LinstorConfDirName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: config.Name,
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
					},
					ImagePullSecrets: pullSecrets,
				},
			},
		},
	}

	ds = daemonSetWithDRBDKernelModuleInjection(ds, pns)
	ds = daemonSetWithSslConfiguration(ds, pns)
	ds = daemonSetWithHttpsConfiguration(ds, pns)
	return ds
}

func reconcileSatelliteConfiguration(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet, r *ReconcileLinstorSatelliteSet) (*corev1.ConfigMap, error) {
	meta := metav1.ObjectMeta{
		Name:      pns.Name + "-config",
		Namespace: pns.Namespace,
	}

	// Check to see if map already exists
	foundConfigMap := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, foundConfigMap)
	if err == nil {
		log.WithFields(logrus.Fields{
			"Name":      meta.Name,
			"Namespace": meta.Namespace,
		}).Debugf("reconcileSatelliteConfiguration: ConfigMap already exists")
		return foundConfigMap, nil
	} else if !errors.IsNotFound(err) {
		return nil, err
	}
	// ConfigMap does not exist, create it next

	// Create linstor satellite configuration
	type SatelliteNetcomConfig struct {
		Type                string `toml:"type,omitempty,omitzero"`
		Port                int32  `toml:"port,omitempty,omitzero"`
		ServerCertificate   string `toml:"server_certificate,omitempty,omitzero"`
		TrustedCertificates string `toml:"trusted_certificates,omitempty,omitzero"`
		KeyPassword         string `toml:"key_password,omitempty,omitzero"`
		KeystorePassword    string `toml:"keystore_password,omitempty,omitzero"`
		TruststorePassword  string `toml:"truststore_password,omitempty,omitzero"`
		SslProtocol         string `toml:"ssl_protocol,omitempty,omitzero"`
	}

	type SatelliteConfig struct {
		Netcom SatelliteNetcomConfig `toml:"netcom,omitempty,omitzero"`
	}

	config := SatelliteConfig{}

	if !pns.Spec.SslConfig.IsPlain() {
		config.Netcom = SatelliteNetcomConfig{
			Type:                pns.Spec.SslConfig.Type(),
			Port:                pns.Spec.SslConfig.Port(),
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

	clientConfig := lc.NewClientConfigForAPIResource(pns.Spec.ControllerEndpoint, &pns.Spec.LinstorClientConfig)
	clientConfigFile, err := clientConfig.ToConfigFile()
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: meta,
		Data: map[string]string{
			kubeSpec.LinstorSatelliteConfigFile: tomlConfigBuilder.String(),
			kubeSpec.LinstorClientConfigFile:    clientConfigFile,
		},
	}

	// Set LinstorSatelliteSet pns as the owner and controller for the config map
	if err := controllerutil.SetControllerReference(pns, cm, r.scheme); err != nil {
		logrus.Debugf("satellite CM did not set correctly")
		return nil, err
	}
	logrus.Debugf("satellite CM Set up")

	err = r.client.Create(ctx, cm)
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func daemonSetWithDRBDKernelModuleInjection(ds *apps.DaemonSet, pns *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	var kernelModHow string

	mode := pns.Spec.KernelModuleInjectionMode
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

	ds.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            "kernel-module-injector",
			Image:           pns.Spec.KernelModuleInjectionImage,
			ImagePullPolicy: pns.Spec.ImagePullPolicy,
			SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
			Env: []corev1.EnvVar{
				{
					Name:  kubeSpec.LinstorKernelModHow,
					Value: kernelModHow,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      kubeSpec.SrcDirName,
					MountPath: kubeSpec.SrcDir,
					ReadOnly:  true,
				},
				// VolumumeSource for this directory is already present on the base
				// daemonset.
				{
					Name:      kubeSpec.ModulesDirName,
					MountPath: kubeSpec.ModulesDir,
				},
			},
			Resources: pns.Spec.KernelModuleInjectionResources,
		},
	}

	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, []corev1.Volume{
		{
			Name: kubeSpec.SrcDirName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: kubeSpec.SrcDir,
					Type: &kubeSpec.HostPathDirectoryType,
				},
			},
		},
	}...)

	return ds
}

func daemonSetWithSslConfiguration(ds *apps.DaemonSet, pns *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	if pns.Spec.SslConfig.IsPlain() {
		// TODO: Implement automatic SSL cert provisioning. For now we just disable SSL
		return ds
	}

	ds.Spec.Template.Spec.Containers[0].VolumeMounts = append(ds.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      kubeSpec.LinstorSslDirName,
		MountPath: kubeSpec.LinstorSslDir,
		ReadOnly:  true,
	})

	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: kubeSpec.LinstorSslDirName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: string(*pns.Spec.SslConfig),
			},
		},
	})

	return ds
}

func daemonSetWithHttpsConfiguration(ds *apps.DaemonSet, pns *piraeusv1.LinstorSatelliteSet) *apps.DaemonSet {
	if pns.Spec.LinstorHttpsClientSecret == "" {
		return ds
	}

	if pns.Spec.LinstorHttpsClientSecret != "" {
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: kubeSpec.LinstorClientDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pns.Spec.LinstorHttpsClientSecret,
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

func pnsLabels(pns *piraeusv1.LinstorSatelliteSet) map[string]string {
	return map[string]string{
		"app":  pns.Name,
		"role": kubeSpec.NodeRole,
	}
}

// aggregateStoragePools appends all disparate StoragePool types together, so they can be processed together.
func (r *ReconcileLinstorSatelliteSet) aggregateStoragePools(pns *piraeusv1.LinstorSatelliteSet) []shared.StoragePool {
	pools := make([]shared.StoragePool, 0)

	for _, thickPool := range pns.Spec.StoragePools.LVMPools {
		pools = append(pools, thickPool)
	}

	for _, thinPool := range pns.Spec.StoragePools.LVMThinPools {
		pools = append(pools, thinPool)
	}

	log := log.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"SPs":       fmt.Sprintf("%+v", pns.Spec.StoragePools),
	})
	log.Debug("satellite Aggregate storage pools")

	return pools
}

func (r *ReconcileLinstorSatelliteSet) finalizeNode(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet, nodeName string) error {
	log := log.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
		"node":      nodeName,
	})
	log.Debug("satellite finalizing node")
	// Determine if any resources still remain on the node.
	resList, err := r.linstorClient.GetAllResourcesOnNode(ctx, nodeName)
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
	if err := r.linstorClient.Nodes.Delete(ctx, nodeName); err != nil && err != lapi.NotFoundError {
		return fmt.Errorf("unable to delete node %s: %v", nodeName, err)
	}

	return nil
}

func (r *ReconcileLinstorSatelliteSet) addFinalizer(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet) error {
	mdutil.AddFinalizer(pns, linstorSatelliteFinalizer)

	err := r.client.Update(ctx, pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorSatelliteSet) deleteFinalizer(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet) error {
	mdutil.DeleteFinalizer(pns, linstorSatelliteFinalizer)

	err := r.client.Update(ctx, pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorSatelliteSet) finalizeSatelliteSet(ctx context.Context, pns *piraeusv1.LinstorSatelliteSet) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	})
	log.Info("found LinstorSatelliteSet marked for deletion, finalizing...")

	if !mdutil.HasFinalizer(pns, linstorSatelliteFinalizer) {
		return reconcile.Result{}, nil
	}

	// Run finalization logic for LinstorSatelliteSet. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	errs := make([]error, 0)
	keepNodes := make([]*shared.SatelliteStatus, 0)

	for _, node := range pns.Status.SatelliteStatuses {
		if err := r.finalizeNode(ctx, pns, node.NodeName); err != nil {
			errs = append(errs, err)
			keepNodes = append(keepNodes, node)
		}
	}

	pns.Status.SatelliteStatuses = keepNodes

	statusErr := r.reconcileStatus(ctx, pns, errs)

	if statusErr != nil {
		log.Warnf("failed to update status. original errors: %v", errs)
		return reconcile.Result{}, statusErr
	}

	// Remove finalizer. Once all finalizers have been
	// removed, the object will be deleted.
	if len(errs) == 0 {
		log.Info("finalizing finished, removing finalizer")

		err := r.deleteFinalizer(ctx, pns)

		return reconcile.Result{}, err
	}

	return reconcileutil.ToReconcileResult(errs...)
}
