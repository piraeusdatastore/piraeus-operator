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
	"strings"
	"sync"
	"time"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
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

	//logf "sigs.k8s.io/controller-runtime/pkg/log"
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

// linstorNodeFinalizer can only be removed if the linstor node containers are
// ready to be shutdown. For now, this means that they have no resources assigned
// to them.
const linstorNodeFinalizer = "finalizer.linstor-node.linbit.com"

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

func newCompoundErrorMsg(errs []error) []string {
	var errStrs = make([]string, 0)

	for _, err := range errs {
		errStrs = append(errStrs, err.Error())
	}

	return errStrs
}

// Reconcile reads that state of the cluster for a LinstorNodeSet object and makes changes based on
// the state read and what is in the LinstorNodeSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorNodeSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the LinstorNodeSet instance
	pns := &piraeusv1alpha1.LinstorNodeSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, pns)
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

	if pns.Spec.DrbdRepoCred == "" {
		return reconcile.Result{}, fmt.Errorf("NS Reconcile: missing required parameter drbdRepoCred: outdated schema")
	}

	if pns.Spec.PriorityClassName == "" {
		return reconcile.Result{}, fmt.Errorf("NS Reconcile: missing required parameter priorityClassName: outdated schema")
	}

	if pns.Spec.SatelliteImage == "" {
		return reconcile.Result{}, fmt.Errorf("NS Reconcile: missing required parameter satelliteImage: outdated schema")
	}

	if pns.Spec.KernelModImage == "" {
		return reconcile.Result{}, fmt.Errorf("NS Reconcile: missing required parameter kernelModImage: outdated schema")
	}

	log := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})
	log.Info("NS Reconcile: reconciling LinstorNodeSet")

	logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	}).Debug("NS Reconcile: found LinstorNodeSet")

	if pns.Status.SatelliteStatuses == nil {
		pns.Status.SatelliteStatuses = make(map[string]*piraeusv1alpha1.SatelliteStatus)
	}
	if pns.Spec.StoragePools == nil {
		pns.Spec.StoragePools = &piraeusv1alpha1.StoragePools{}
	}
	if pns.Spec.StoragePools.LVMPools == nil {
		pns.Spec.StoragePools.LVMPools = make([]*piraeusv1alpha1.StoragePoolLVM, 0)
	}
	if pns.Spec.StoragePools.LVMThinPools == nil {
		pns.Spec.StoragePools.LVMThinPools = make([]*piraeusv1alpha1.StoragePoolLVMThin, 0)
	}

	r.linstorClient, err = lc.NewHighLevelLinstorClientForObject(pns)
	if err != nil {
		return reconcile.Result{}, err
	}

	markedForDeletion := pns.GetDeletionTimestamp() != nil
	if markedForDeletion {
		errs := r.finalizeSatelliteSet(pns)

		logrus.WithFields(logrus.Fields{
			"errs": newCompoundErrorMsg(errs),
		}).Debug("NS Reconcile: reconcile loop end")

		// Resources need to be removed by human intervention, so we don't want to
		// requeue the reconcile loop immediately. We can't return the error with
		// the loop or it will automatically requeue, we log it above and it also
		// appears in the pns's Status.
		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if err := r.addFinalizer(pns); err != nil {
		return reconcile.Result{}, err
	}

	// Define a service for the controller.
	ctrlService := newServiceForPNS(pns)

	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pns, ctrlService, r.scheme); err != nil {
		logrus.Debug("NS SVC Controller did not set correctly")
		return reconcile.Result{}, err
	}
	logrus.Debug("NS SVC Set up")

	foundSrv := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ctrlService.Name, Namespace: ctrlService.Namespace}, foundSrv)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"NS SVC name":      ctrlService.Name,
			"NS SVC namespace": ctrlService.Namespace,
		}).Info("NS Reconcile: creating a new Service")
		err = r.client.Create(context.TODO(), ctrlService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Define a new DaemonSet
	ds := newDaemonSetforPNS(pns)

	// Set LinstorNodeSet pns as the owner and controller for the daemon set
	if err := controllerutil.SetControllerReference(pns, ds, r.scheme); err != nil {
		logrus.Debug("NS DS Controller did not set correctly")
		return reconcile.Result{}, err
	}
	logrus.Debug("NS DS Set up")

	// Check if this Pod already exists
	found := &apps.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      ds.Name,
			"namespace": ds.Namespace,
		}).Info("NS Reconcile: creating a new DaemonSet")
		err = r.client.Create(context.TODO(), ds)
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

	errs := r.reconcileSatNodes(pns)

	compoundErrorMsg := newCompoundErrorMsg(errs)
	pns.Status.Errors = compoundErrorMsg

	if err := r.client.Status().Update(context.TODO(), pns); err != nil {
		logrus.Error(err, "NS Reconcile: Failed to update LinstorNodeSet status")
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"errs": compoundErrorMsg,
	}).Debug("NS Reconcile: reconcile loop end")

	var compoundError error
	if len(compoundErrorMsg) != 0 {
		compoundError = fmt.Errorf("NS Reconcile: requeuing NodeSet reconcile loop for the following reason(s): %s", strings.Join(compoundErrorMsg, " ;; "))
	}

	return reconcile.Result{}, compoundError
}

// This function is a mini-main function and has a lot of boilerplate code that doesn't make a lot of
// sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorNodeSet) reconcileSatNodes(pns *piraeusv1alpha1.LinstorNodeSet) []error {

	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(pnsLabels(pns))
	listOpts := []client.ListOption{
		// Namespace: pns.Namespace, LabelSelector: labelSelector}
		client.InNamespace(pns.Namespace), client.MatchingLabelsSelector{labelSelector}}
	err := r.client.List(context.TODO(), pods, listOpts...)
	if err != nil {
		return []error{err}
	}

	type satStat struct {
		sat *piraeusv1alpha1.SatelliteStatus
		err error
	}

	satelliteStatusIn := make(chan satStat)
	satelliteStatusOut := make(chan satStat)

	maxConcurrentNodes := 5
	tokens := make(chan struct{}, maxConcurrentNodes)

	var wg sync.WaitGroup

	for i := range pods.Items {
		wg.Add(1)
		pod := pods.Items[i]

		log := logrus.WithFields(logrus.Fields{
			"podName":                        pod.Name,
			"podNameSpace":                   pod.Namespace,
			"podPhase":                       pod.Status.Phase,
			"podNumber":                      i,
			"maxConcurrentNodeRegistrations": maxConcurrentNodes,
		})
		log.Debug("NS reconcileSatNodes: reconciling node")

		sat, ok := pns.Status.SatelliteStatuses[pod.Spec.NodeName]
		if !ok {
			pns.Status.SatelliteStatuses[pod.Spec.NodeName] = &piraeusv1alpha1.SatelliteStatus{
				NodeStatus: piraeusv1alpha1.NodeStatus{NodeName: pod.Spec.NodeName},
			}
			sat = pns.Status.SatelliteStatuses[pod.Spec.NodeName]
		}
		if sat.StoragePoolStatuses == nil {
			sat.StoragePoolStatuses = make(map[string]*piraeusv1alpha1.StoragePoolStatus)
		}

		go func() {
			l := log
			l.Debug("NS reconcileSatNodes: waiting to acquire token...")
			tokens <- struct{}{} // Acquire a token
			l.Debug("NS reconcileSatNodes: token acquired, registering node")

			err := r.reconcileSatNodeWithController(sat, pod)
			if err != nil {
				l.Debug("NS reconcileSatNodes: Error with reconcileSatNodeWithController")
			}
			satelliteStatusIn <- satStat{sat, err}

		}()

		pools := r.aggregateStoragePools(pns)

		go func() {
			l := log
			defer func() {
				<-tokens // Work done, release token.
				l.Debug("NS reconcileSatNodes: token released")
				wg.Done()
			}()

			in := <-satelliteStatusIn
			if in.err != nil {
				satelliteStatusOut <- satStat{in.sat, in.err}
				return
			}

			err := r.reconcileStoragePoolsOnNode(in.sat, pools, pod)
			if err != nil {
				l.Debug("NS reconcileSatNodes: Error with reconcileStoragePoolsOnNode")
			}
			satelliteStatusOut <- satStat{sat, err}
		}()
	}

	go func() {
		wg.Wait()
		close(satelliteStatusOut)
	}()

	var compoundError []error

	for out := range satelliteStatusOut {
		if out.err != nil {
			compoundError = append(compoundError, out.err)
		}

		// Guard against empty statuses.
		if out.sat == nil || out.sat.NodeName != "" {
			pns.Status.SatelliteStatuses[out.sat.NodeName] = out.sat
		}
	}

	return compoundError
}

func (r *ReconcileLinstorNodeSet) reconcileSatNodeWithController(sat *piraeusv1alpha1.SatelliteStatus, pod corev1.Pod) error {

	// Mark this true on successful exit from this function.
	sat.RegisteredOnController = false

	// TODO: Add Context w/ an infinite loop
	if len(pod.Status.ContainerStatuses) != 0 && !pod.Status.ContainerStatuses[0].Ready {
		return fmt.Errorf("NS reconcileSatNodeWithController: Nodeset pod %s is not ready, delaying registration on controller", pod.Spec.NodeName)
	}

	node, err := r.linstorClient.GetNodeOrCreate(context.TODO(), lapi.Node{
		Name: pod.Spec.NodeName,
		Type: lc.Satellite,
		NetInterfaces: []lapi.NetInterface{
			{
				Name:                    "default",
				Address:                 pod.Status.HostIP,
				SatellitePort:           3366,
				SatelliteEncryptionType: "plain",
			},
		},
	})
	if err != nil {
		log.Debug("NS reconcileSatNodeWithController error")
		return err
	}

	log.WithFields(logrus.Fields{
		"nodeName":         node.Name,
		"nodeType":         node.Type,
		"connectionStatus": node.ConnectionStatus,
	}).Debug("NS reconcileSatNodeWithController: Found / Added a Satellite Node")

	sat.ConnectionStatus = node.ConnectionStatus
	if sat.ConnectionStatus != lc.Online {
		return fmt.Errorf("NS reconcileSatNodeWithController: waiting for node %s ConnectionStatus to be %s, current ConnectionStatus: %s",
			pod.Spec.NodeName, lc.Online, sat.ConnectionStatus)
	}

	sat.RegisteredOnController = true
	return nil
}

func (r *ReconcileLinstorNodeSet) reconcileStoragePoolsOnNode(sat *piraeusv1alpha1.SatelliteStatus, pools []piraeusv1alpha1.StoragePool, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"podPhase":     pod.Status.Phase,
	})
	log.Info("NS reconcileStoragePoolsOnNode: reconciling storagePools")

	if !sat.RegisteredOnController {
		return fmt.Errorf("NS reconcileStoragePoolsOnNode: waiting for %s to be registered on controller, not able to reconcile storage pools", pod.Spec.NodeName)
	}

	// Get status for all pools.
	for _, pool := range pools {
		p, err := r.linstorClient.GetStoragePoolOrCreateOnNode(context.TODO(), pool.ToLinstorStoragePool(), pod.Spec.NodeName)
		if err != nil {
			return err
		}

		status := piraeusv1alpha1.NewStoragePoolStatus(p)

		log.WithFields(logrus.Fields{
			"storagePool": fmt.Sprintf("%+v", status),
		}).Debug("NS reconcileStoragePoolsOnNode: found storage pool")

		// Guard against empty statuses.
		if status == nil || status.Name != "" {
			sat.StoragePoolStatuses[status.Name] = status
		}
	}

	return nil
}

func newDaemonSetforPNS(pns *piraeusv1alpha1.LinstorNodeSet) *apps.DaemonSet {
	labels := pnsLabels(pns)
	controllerName := pns.Name[0:len(pns.Name)-3] + "-cs"

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
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      kubeSpec.PiraeusNode,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"true"},
											},
										},
									},
								},
							},
						},
					},
					HostNetwork:       true, // INFO: Per Roland, set to true
					DNSPolicy:         corev1.DNSClusterFirstWithHostNet,
					PriorityClassName: pns.Spec.PriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            "linstor-satellite",
							Image:           pns.Spec.SatelliteImage,
							Args:            []string{"startSatellite"}, // Run linstor-satellite.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      3366,
									ContainerPort: 3366,
								},
								{
									HostPort:      3367,
									ContainerPort: 3367,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      kubeSpec.DevDirName,
									MountPath: kubeSpec.DevDir,
								},
								{
									Name:             kubeSpec.ModulesDirName,
									MountPath:        kubeSpec.ModulesDir,
									MountPropagation: &kubeSpec.MountPropagationBidirectional,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LS_CONTROLLERS",
									Value: "http://" + controllerName + ":" + "3370",
									// Value: "http://" + controllerName + ":" + "3370",
								},
							},

							// TODO: Move to kubeSpec later
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"curl", "http://" + controllerName + ":" + "3370"},
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
							Name: kubeSpec.DevDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.DevDir,
								}}},
						{
							Name: kubeSpec.ModulesDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.ModulesDir,
									Type: &kubeSpec.HostPathDirectoryOrCreateType,
								}}},
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

	return daemonSetWithDRBDKernelModuleInjection(ds, pns)
}

func newServiceForPNS(pns *piraeusv1alpha1.LinstorNodeSet) *corev1.Service {
	controllerName := pns.Name[0:len(pns.Name)-3] + "-cs"

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pns.Name,
			Namespace: pns.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:       controllerName,
					Port:       3370,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(3370),
				},
			},
			Selector: map[string]string{
				"app": controllerName,
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func daemonSetWithDRBDKernelModuleInjection(ds *apps.DaemonSet, pns *piraeusv1alpha1.LinstorNodeSet) *apps.DaemonSet {
	var kernelModHow string

	mode := pns.Spec.DRBDKernelModuleInjectionMode
	switch mode {
	case piraeusv1alpha1.ModuleInjectionNone:
		return ds
	case piraeusv1alpha1.ModuleInjectionCompile:
		kernelModHow = kubeSpec.LinstorKernelModCompile
	case piraeusv1alpha1.ModuleInjectionShippedModules:
		kernelModHow = kubeSpec.LinstorKernelModShippedModules
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
				}}},
	}...)

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
	var pools = make([]piraeusv1alpha1.StoragePool, 0)

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

func (r *ReconcileLinstorNodeSet) finalizeNode(pns *piraeusv1alpha1.LinstorNodeSet, nodeName string) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
		"node":      nodeName,
	})
	log.Debug("NS finalizing node")
	// Determine if any resources still remain on the node.
	resList, err := r.linstorClient.GetAllResourcesOnNode(context.TODO(), nodeName)
	if err != nil {
		return err
	}

	if len(resList) != 0 {
		return fmt.Errorf("unable to remove node %s: all resources must be removed before deletion", nodeName)
	}

	// No resources, safe to delete the node.
	if err := r.linstorClient.Nodes.Delete(context.TODO(), nodeName); err != nil && err != lapi.NotFoundError {
		return fmt.Errorf("unable to delete node %s: %v", nodeName, err)
	}

	delete(pns.Status.SatelliteStatuses, nodeName)
	return nil
}

func (r *ReconcileLinstorNodeSet) addFinalizer(pns *piraeusv1alpha1.LinstorNodeSet) error {
	mdutil.AddFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(context.TODO(), pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorNodeSet) deleteFinalizer(pns *piraeusv1alpha1.LinstorNodeSet) error {
	mdutil.DeleteFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(context.TODO(), pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorNodeSet) finalizeSatelliteSet(pns *piraeusv1alpha1.LinstorNodeSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	})
	log.Info("found LinstorNodeSet marked for deletion, finalizing...")

	if mdutil.HasFinalizer(pns, linstorNodeFinalizer) {
		// Run finalization logic for LinstorNodeSet. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.
		var errs = make([]error, 0)
		for nodeName := range pns.Status.SatelliteStatuses {
			if err := r.finalizeNode(pns, nodeName); err != nil {
				errs = append(errs, err)
			}
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		if len(errs) == 0 {
			log.Info("finalizing finished, removing finalizer")
			if err := r.deleteFinalizer(pns); err != nil {
				return []error{err}
			}
			return nil
		}

		err := r.client.Status().Update(context.TODO(), pns)
		if err != nil {
			return []error{err}
		}

		return errs
	}
	return nil
}
