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

package linstornodeset

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
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

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

var log = logrus.WithFields(logrus.Fields{
	"controller": "LinstorNodeSet",
})

const (
	// linstorNodeFinalizer can only be removed if the linstor node containers are
	// ready to be shutdown. For now, this means that they have no resources assigned
	// to them.
	linstorNodeFinalizer = "finalizer.linstor-node.linbit.com"

	// Default value for automaticStorageType. If set, no automatic setup of storage devices happens.
	automaticStorageTypeNone = "None"

	// requeue reconciliation after connectionRetrySeconds
	connectionRetrySeconds = 10
)

// Add creates a new LinstorNodeSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorNodeSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	log.Debug("NS add: Adding a PNS controller ")
	c, err := controller.New("linstornodeset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorNodeSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.LinstorNodeSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &apps.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1alpha1.LinstorNodeSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorNodeSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorNodeSet{}

// ReconcileLinstorNodeSet reconciles a LinstorNodeSet object
type ReconcileLinstorNodeSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        client.Client
	scheme        *runtime.Scheme
	linstorClient *lc.HighLevelClient
}

// Reconcile reads that state of the cluster for a LinstorNodeSet object and makes changes based on
// the state read and what is in the LinstorNodeSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorNodeSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})
	log.Info("NS Reconcile: reconciling LinstorNodeSet")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	log.Debug("fetch resource")

	pns := &piraeusv1alpha1.LinstorNodeSet{}
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

	log.Debug("check if all required fields are filled")

	err = checkRequiredSpec(pns)
	if err != nil {
		return reconcile.Result{}, err
	}

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
		}).Info("NS Reconcile: reconcile loop end")

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

	// Set LinstorNodeSet pns as the owner and controller for the daemon set
	if err := controllerutil.SetControllerReference(pns, ds, r.scheme); err != nil {
		logrus.Debug("NS DS Controller did not set correctly")
		return reconcile.Result{}, err
	}
	logrus.Debug("NS DS Set up")

	// Check if this Pod already exists
	found := &apps.DaemonSet{}
	err = r.client.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      ds.Name,
			"namespace": ds.Namespace,
		}).Info("NS Reconcile: creating a new DaemonSet")

		err = r.client.Create(ctx, ds)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"name":      ds.Name,
				"namespace": ds.Namespace,
			}).Debug("NS Reconcile: Error w/ Daemonset")
			return reconcile.Result{}, err
		}

		// Pod created successfully - requeue for registration
		logrus.Debug("NS Reconcile: Daemonset created successfully")
		return reconcile.Result{Requeue: true}, nil

	} else if err != nil {
		logrus.Debug("NS Reconcile: Error on client.Get")
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      ds.Name,
		"namespace": ds.Namespace,
	}).Debug("NS Reconcile: DaemonSet already exists")

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
	}).Info("NS Reconcile: reconcile loop end")

	return result, err
}

func checkRequiredSpec(pns *piraeusv1alpha1.LinstorNodeSet) error {
	if pns.Spec.DrbdRepoCred == "" {
		return fmt.Errorf("NS Reconcile: missing required parameter drbdRepoCred: outdated schema")
	}

	if pns.Spec.SatelliteImage == "" {
		return fmt.Errorf("NS Reconcile: missing required parameter satelliteImage: outdated schema")
	}

	if pns.Spec.KernelModImage == "" {
		return fmt.Errorf("NS Reconcile: missing required parameter kernelModImage: outdated schema")
	}

	return nil
}

func (r *ReconcileLinstorNodeSet) reconcileResource(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      pns.Name,
		"Namespace": pns.Namespace,
		"Op":        "reconcileResource",
	})
	logger.Debug("performing upgrades and fill defaults in resource")

	changed := false

	logger.Debug("performing upgrade/fill: #0 -> replace nil with zero objects")

	if pns.Spec.StoragePools == nil {
		pns.Spec.StoragePools = &piraeusv1alpha1.StoragePools{}
		changed = true

		logger.Info("set storage pool to empty default object")
	}

	if pns.Spec.StoragePools.LVMPools == nil {
		pns.Spec.StoragePools.LVMPools = make([]*piraeusv1alpha1.StoragePoolLVM, 0)
		changed = true

		logger.Info("set storage pool 'LVM' to empty list")
	}

	if pns.Spec.StoragePools.LVMThinPools == nil {
		pns.Spec.StoragePools.LVMThinPools = make([]*piraeusv1alpha1.StoragePoolLVMThin, 0)
		changed = true

		logger.Info("set storage pool 'LVMThin to empty list")
	}

	logger.Debugf("finished upgrade/fill: #0 -> replace nil with zero objects: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #1 -> Set default endpoint URL for client")

	if pns.Spec.ControllerEndpoint == "" {
		serviceName := types.NamespacedName{Name: pns.Name[:len(pns.Name)-3] + "-cs", Namespace: pns.Namespace}
		useHTTPS := pns.Spec.LinstorHttpsClientSecret != ""
		defaultEndpoint := lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
		pns.Spec.ControllerEndpoint = defaultEndpoint
		changed = true

		logger.Infof("set controller endpoint URL to '%s'", pns.Spec.ControllerEndpoint)
	}

	logger.Debugf("finished upgrade/fill: #1 -> Set default endpoint URL for client: changed=%t", changed)

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

func (r *ReconcileLinstorNodeSet) reconcileAllNodesOnController(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet) []error {
	logger := logrus.WithFields(logrus.Fields{
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

func (r *ReconcileLinstorNodeSet) reconcilePod(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet, pod *corev1.Pod) error {
	podLog := logrus.WithFields(logrus.Fields{
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

func (r *ReconcileLinstorNodeSet) reconcileSingleNodeRegistration(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet, pod *corev1.Pod) error {
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

func (r *ReconcileLinstorNodeSet) reconcileAutomaticDeviceSetup(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet, pod *corev1.Pod) error {
	logger := logrus.WithFields(logrus.Fields{
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

func (r *ReconcileLinstorNodeSet) reconcileStoragePoolsOnNode(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet, pod *corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
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

func (r *ReconcileLinstorNodeSet) reconcileStatus(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet, errs []error) error {
	logger := logrus.WithFields(logrus.Fields{
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

	nodeSet.Status.SatelliteStatuses = make([]*piraeusv1alpha1.SatelliteStatus, len(pods))

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

func satelliteStatusFromLinstor(pod *corev1.Pod, node *lapi.Node, pools []lapi.StoragePool) *piraeusv1alpha1.SatelliteStatus {
	status := &piraeusv1alpha1.SatelliteStatus{
		NodeStatus: piraeusv1alpha1.NodeStatus{
			NodeName: pod.Spec.NodeName,
		},
		StoragePoolStatuses: []*piraeusv1alpha1.StoragePoolStatus{},
	}

	if node == nil {
		return status
	}

	status.ConnectionStatus = node.ConnectionStatus
	status.RegisteredOnController = node.ConnectionStatus == lc.Online

	poolsStatus := make([]*piraeusv1alpha1.StoragePoolStatus, 0)
	for i := range pools {
		poolsStatus = append(poolsStatus, piraeusv1alpha1.NewStoragePoolStatus(&pools[i]))
	}

	status.StoragePoolStatuses = poolsStatus

	// Sort for stable status reporting
	sort.Slice(status.StoragePoolStatuses, func(i, j int) bool {
		return status.StoragePoolStatuses[i].Name < status.StoragePoolStatuses[j].Name
	})

	return status
}

func (r *ReconcileLinstorNodeSet) getAllNodePods(ctx context.Context, nodeSet *piraeusv1alpha1.LinstorNodeSet) ([]corev1.Pod, error) {
	logrus.WithFields(logrus.Fields{
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

func newDaemonSetforPNS(pns *piraeusv1alpha1.LinstorNodeSet, config *corev1.ConfigMap) *apps.DaemonSet {
	labels := pnsLabels(pns)

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
					Affinity:          pns.Spec.Affinity,
					Tolerations:       pns.Spec.Tolerations,
					HostNetwork:       true, // INFO: Per Roland, set to true
					DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
					PriorityClassName: pns.Spec.PriorityClassName.GetName(pns.Namespace),
					Containers: []corev1.Container{
						{
							Name:  "linstor-satellite",
							Image: pns.Spec.SatelliteImage,
							Args: []string{
								"startSatellite",
							}, // Run linstor-satellite.
							ImagePullPolicy: corev1.PullIfNotPresent,
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
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: pns.Spec.DrbdRepoCred,
						},
					},
				},
			},
		},
	}

	ds = daemonSetWithDRBDKernelModuleInjection(ds, pns)
	ds = daemonSetWithSslConfiguration(ds, pns)
	ds = daemonSetWithHttpsConfiguration(ds, pns)
	return ds
}

func reconcileSatelliteConfiguration(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet, r *ReconcileLinstorNodeSet) (*corev1.ConfigMap, error) {
	meta := metav1.ObjectMeta{
		Name:      pns.Name + "-config",
		Namespace: pns.Namespace,
	}

	// Check to see if map already exists
	foundConfigMap := &corev1.ConfigMap{}
	err := r.client.Get(ctx, types.NamespacedName{Name: meta.Name, Namespace: meta.Namespace}, foundConfigMap)
	if err == nil {
		logrus.WithFields(logrus.Fields{
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

	// Set LinstorNodeSet pns as the owner and controller for the config map
	if err := controllerutil.SetControllerReference(pns, cm, r.scheme); err != nil {
		logrus.Debugf("NS CM did not set correctly")
		return nil, err
	}
	logrus.Debugf("NS CM Set up")

	err = r.client.Create(ctx, cm)
	if err != nil {
		return nil, err
	}

	return cm, nil
}

func daemonSetWithDRBDKernelModuleInjection(ds *apps.DaemonSet, pns *piraeusv1alpha1.LinstorNodeSet) *apps.DaemonSet {
	var kernelModHow string

	mode := pns.Spec.DRBDKernelModuleInjectionMode
	switch mode {
	case piraeusv1alpha1.ModuleInjectionNone:
		logrus.WithField("drbdKernelModuleInjectionMode", mode).Warnf("using deprecated injection mode: beginning with injector image version 9.0.23, it is recommended to use '%s' instead", piraeusv1alpha1.ModuleInjectionDepsOnly)
		return ds
	case piraeusv1alpha1.ModuleInjectionCompile:
		kernelModHow = kubeSpec.LinstorKernelModCompile
	case piraeusv1alpha1.ModuleInjectionShippedModules:
		kernelModHow = kubeSpec.LinstorKernelModShippedModules
	case piraeusv1alpha1.ModuleInjectionDepsOnly:
		kernelModHow = kubeSpec.LinstorKernelModDepsOnly
	default:
		logrus.WithFields(logrus.Fields{
			"mode": mode,
		}).Warn("Unknown kernel module injection mode; skipping")
		return ds
	}

	ds.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            "drbd-kernel-module-injector",
			Image:           pns.Spec.KernelModImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
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

func daemonSetWithSslConfiguration(ds *apps.DaemonSet, pns *piraeusv1alpha1.LinstorNodeSet) *apps.DaemonSet {
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

func daemonSetWithHttpsConfiguration(ds *apps.DaemonSet, pns *piraeusv1alpha1.LinstorNodeSet) *apps.DaemonSet {
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

func pnsLabels(pns *piraeusv1alpha1.LinstorNodeSet) map[string]string {
	return map[string]string{
		"app":  pns.Name,
		"role": kubeSpec.NodeRole,
	}
}

// aggregateStoragePools appends all disparate StoragePool types together, so they can be processed together.
func (r *ReconcileLinstorNodeSet) aggregateStoragePools(pns *piraeusv1alpha1.LinstorNodeSet) []piraeusv1alpha1.StoragePool {
	pools := make([]piraeusv1alpha1.StoragePool, 0)

	for _, thickPool := range pns.Spec.StoragePools.LVMPools {
		pools = append(pools, thickPool)
	}

	for _, thinPool := range pns.Spec.StoragePools.LVMThinPools {
		pools = append(pools, thinPool)
	}

	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"SPs":       fmt.Sprintf("%+v", pns.Spec.StoragePools),
	})
	log.Debug("NS Aggregate storage pools")

	return pools
}

func (r *ReconcileLinstorNodeSet) finalizeNode(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet, nodeName string) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
		"node":      nodeName,
	})
	log.Debug("NS finalizing node")
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

func (r *ReconcileLinstorNodeSet) addFinalizer(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet) error {
	mdutil.AddFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(ctx, pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorNodeSet) deleteFinalizer(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet) error {
	mdutil.DeleteFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(ctx, pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorNodeSet) finalizeSatelliteSet(ctx context.Context, pns *piraeusv1alpha1.LinstorNodeSet) (reconcile.Result, error) {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	})
	log.Info("found LinstorNodeSet marked for deletion, finalizing...")

	if !mdutil.HasFinalizer(pns, linstorNodeFinalizer) {
		return reconcile.Result{}, nil
	}

	// Run finalization logic for LinstorNodeSet. If the
	// finalization logic fails, don't remove the finalizer so
	// that we can retry during the next reconciliation.
	errs := make([]error, 0)
	keepNodes := make([]*piraeusv1alpha1.SatelliteStatus, 0)

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
