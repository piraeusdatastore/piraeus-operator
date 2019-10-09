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

package piraeusnodeset

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	lapi "github.com/LINBIT/golinstor/client"
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"

	"github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	"controller": "PiraeusNodeSet",
})

// linstorNodeFinalizer can only be removed if the linstor node containers are
// ready to be shutdown. For now, this means that they have no resources assigned
// to them.
const linstorNodeFinalizer = "finalizer.linstor-node.linbit.com"

// Add creates a new PiraeusNodeSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePiraeusNodeSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("piraeusnodeset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PiraeusNodeSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.PiraeusNodeSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &apps.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1alpha1.PiraeusNodeSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePiraeusNodeSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePiraeusNodeSet{}

// ReconcilePiraeusNodeSet reconciles a PiraeusNodeSet object
type ReconcilePiraeusNodeSet struct {
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

// Reconcile reads that state of the cluster for a PiraeusNodeSet object and makes changes based on the state read
// and what is in the PiraeusNodeSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcilePiraeusNodeSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Debug("entering reconcile loop")

	// Fetch the PiraeusNodeSet instance
	pns := &piraeusv1alpha1.PiraeusNodeSet{}
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

	log := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})
	log.Info("reconciling PiraeusNodeSet")

	logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	}).Debug("found PiraeusNodeSet")

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
		}).Debug("reconcile loop end")

		// Resouces need to be removed by human intervention, so we don't want to
		// requeue the reconcile loop immediately. We can't return the error with
		// the loop or it will automatically requeue, we log it above and it also
		// appears in the pns's Status.
		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if err := r.addFinalizer(pns); err != nil {
		return reconcile.Result{}, err
	}

	// Define a new DaemonSet
	ds := newDaemonSetforPNS(pns)

	// Set PiraeusNodeSet pns as the owner and controller
	if err := controllerutil.SetControllerReference(pns, ds, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &apps.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      ds.Name,
			"namespace": ds.Namespace,
		}).Info("creating a new DaemonSet")
		err = r.client.Create(context.TODO(), ds)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - requeue for registration
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      ds.Name,
		"namespace": ds.Namespace,
	}).Debug("DaemonSet already exists")

	errs := r.reconcileSatNodes(pns)

	compoundErrorMsg := newCompoundErrorMsg(errs)
	pns.Status.Errors = compoundErrorMsg

	if err := r.client.Status().Update(context.TODO(), pns); err != nil {
		logrus.Error(err, "Failed to update PiraeusNodeSet status")
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"errs": compoundErrorMsg,
	}).Debug("reconcile loop end")

	var compoundError error
	if len(compoundErrorMsg) != 0 {
		compoundError = fmt.Errorf("requeuing reconcile loop for the following reason(s): %s", strings.Join(compoundErrorMsg, " ;; "))
	}

	return reconcile.Result{}, compoundError
}

func (r *ReconcilePiraeusNodeSet) reconcileSatNodes(pns *piraeusv1alpha1.PiraeusNodeSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	})
	log.Info("reconciling PiraeusNodeSet Nodes")

	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(pnsLabels(pns))
	listOps := &client.ListOptions{Namespace: pns.Namespace, LabelSelector: labelSelector}
	err := r.client.List(context.TODO(), listOps, pods)
	if err != nil {
		return []error{err}
	}

	// Append all disperate StoragePool types together, so they can be processed together.
	var pools = make([]piraeusv1alpha1.StoragePool, 0)
	for _, thickPool := range pns.Spec.StoragePools.LVMPools {
		pools = append(pools, thickPool)
	}

	for _, thinPool := range pns.Spec.StoragePools.LVMThinPools {
		pools = append(pools, thinPool)
	}

	type satStat struct {
		sat *piraeusv1alpha1.SatelliteStatus
		err error
	}

	satelliteStatusIn := make(chan satStat)
	satelliteStatusOut := make(chan satStat)

	var wg sync.WaitGroup

	for i := range pods.Items {
		wg.Add(1)
		pod := pods.Items[i]

		log := logrus.WithFields(logrus.Fields{
			"podName":      pod.Name,
			"podNameSpace": pod.Namespace,
			"podPase":      pod.Status.Phase,
			"podNumber":    i,
		})
		log.Debug("reconciling node")

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
			err := r.reconcileSatNodeWithController(sat, pod)
			satelliteStatusIn <- satStat{sat, err}

		}()

		go func() {
			defer wg.Done()

			aaaaah := <-satelliteStatusIn
			if aaaaah.err != nil {
				satelliteStatusOut <- satStat{aaaaah.sat, aaaaah.err}
				return
			}

			err := r.reconcileStoragePoolsOnNode(in.sat, pools, pod)
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

func (r *ReconcilePiraeusNodeSet) reconcileSatNodeWithController(sat *piraeusv1alpha1.SatelliteStatus, pod corev1.Pod) error {
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod %s not running, delaying registration on controller", pod.Spec.NodeName)
	}

	// Mark this true on successful exit from this function.
	sat.RegisteredOnController = false

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
		return err
	}

	log.WithFields(logrus.Fields{
		"nodeName":         node.Name,
		"nodeType":         node.Type,
		"connectionStatus": node.ConnectionStatus,
	}).Debug("found node")

	sat.ConnectionStatus = node.ConnectionStatus
	if sat.ConnectionStatus != lc.Online {
		return fmt.Errorf("waiting for node %s ConnectionStatus to be %s, current ConnectionStatus: %s",
			pod.Spec.NodeName, lc.Online, sat.ConnectionStatus)
	}

	sat.RegisteredOnController = true
	return nil
}

func (r *ReconcilePiraeusNodeSet) reconcileStoragePoolsOnNode(sat *piraeusv1alpha1.SatelliteStatus, pools []piraeusv1alpha1.StoragePool, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"podPase":      pod.Status.Phase,
	})
	log.Info("reconciling storagePools")

	if !sat.RegisteredOnController {
		return fmt.Errorf("waiting for %s to be registered on controller, not able to reconcile storage pools", pod.Spec.NodeName)
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
		}).Debug("found storage pool")

		// Guard against empty statuses.
		if status == nil || status.Name != "" {
			sat.StoragePoolStatuses[status.Name] = status
		}
	}

	return nil
}

func newDaemonSetforPNS(pns *piraeusv1alpha1.PiraeusNodeSet) *apps.DaemonSet {
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
					HostNetwork:       true,
					PriorityClassName: kubeSpec.PiraeusPriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            "linstor-satellite",
							Image:           kubeSpec.PiraeusServerImage + ":" + kubeSpec.PiraeusVersion,
							Args:            []string{"startSatellite"}, // Run linstor-satellite.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      3366,
									ContainerPort: 3366,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      kubeSpec.DevDirName,
									MountPath: kubeSpec.DevDir,
								},
								{
									Name:      kubeSpec.UdevDirName,
									MountPath: kubeSpec.UdevDir,
									ReadOnly:  true,
								},
								{
									Name:             kubeSpec.ModulesDirName,
									MountPath:        kubeSpec.ModulesDir,
									MountPropagation: &kubeSpec.MountPropagationBidirectional,
								},
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
						{
							Name: kubeSpec.UdevDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.UdevDir,
									Type: &kubeSpec.HostPathDirectoryOrCreateType,
								}}},
					},
				},
			},
		},
	}

	if pns.Spec.DisableDRBDKernelModuleInjection {
		return ds
	}

	return daemonSetWithDRBDKernelModuleInjection(ds)
}

func daemonSetWithDRBDKernelModuleInjection(ds *apps.DaemonSet) *apps.DaemonSet {
	ds.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Name:            "drbd-kernel-module-injector",
			Image:           "quay.io/piraeusdatastore/drbd9-centos7:v9.0.19",
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
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
					ReadOnly:  true,
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

func pnsLabels(pns *piraeusv1alpha1.PiraeusNodeSet) map[string]string {
	return map[string]string{
		"app":  pns.Name,
		"role": "piraeus-node",
	}
}

func (r *ReconcilePiraeusNodeSet) finalizeNode(pns *piraeusv1alpha1.PiraeusNodeSet, nodeName string) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
		"node":      nodeName,
	})
	log.Debug("finalizing node")
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

func (r *ReconcilePiraeusNodeSet) addFinalizer(pns *piraeusv1alpha1.PiraeusNodeSet) error {
	mdutil.AddFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(context.TODO(), pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcilePiraeusNodeSet) deleteFinalizer(pns *piraeusv1alpha1.PiraeusNodeSet) error {
	mdutil.DeleteFinalizer(pns, linstorNodeFinalizer)

	err := r.client.Update(context.TODO(), pns)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcilePiraeusNodeSet) finalizeSatelliteSet(pns *piraeusv1alpha1.PiraeusNodeSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pns.Name,
		"namespace": pns.Namespace,
		"spec":      fmt.Sprintf("%+v", pns.Spec),
	})
	log.Info("found PiraeusNodeSet marked for deletion, finalizing...")

	if mdutil.HasFinalizer(pns, linstorNodeFinalizer) {
		// Run finalization logic for PiraeusNodeSet. If the
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
