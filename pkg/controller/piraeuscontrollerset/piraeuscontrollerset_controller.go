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

package piraeuscontrollerset

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	lapi "github.com/LINBIT/golinstor/client"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
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

const linstorControllerFinalizer = "finalizer.linstor-controller.linbit.com"

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

var log = logrus.WithFields(logrus.Fields{
	"controller": "PiraeusControllerSet",
})

// Add creates a new PiraeusControllerSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePiraeusControllerSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("piraeuscontrollerset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PiraeusControllerSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.PiraeusControllerSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1beta2.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1alpha1.PiraeusControllerSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePiraeusControllerSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePiraeusControllerSet{}

// ReconcilePiraeusControllerSet reconciles a PiraeusControllerSet object
type ReconcilePiraeusControllerSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client        client.Client
	scheme        *runtime.Scheme
	linstorClient *lapi.Client
}

func newCompoundErrorMsg(errs []error) []string {
	var errStrs = make([]string, 0)

	for _, err := range errs {
		if err != nil {
			errStrs = append(errStrs, err.Error())
		}
	}

	return errStrs
}

// Reconcile reads that state of the cluster for a PiraeusControllerSet object and makes changes based on the state read
// and what is in the PiraeusControllerSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePiraeusControllerSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Debug("entering reconcile loop")

	// Fetch the PiraeusControllerSet instance
	pcs := &piraeusv1alpha1.PiraeusControllerSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, pcs)
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
		"PiraeusNodeSet":    fmt.Sprintf("%+v", pcs),
	})
	log.Info("reconciling PiraeusNodeSet")

	logrus.WithFields(logrus.Fields{
		"PiraeusNodeSet": fmt.Sprintf("%+v", pcs),
	}).Debug("found PiraeusNodeSet")

	if pcs.Status.SatelliteStatuses == nil {
		pcs.Status.SatelliteStatuses = make(map[string]*piraeusv1alpha1.CtrlSatelliteStatus)
	}

	markedForDeletion := pcs.GetDeletionTimestamp() != nil
	if markedForDeletion {
		err := r.finalizeControllerSet(pcs)

		log.Debug("reconcile loop end")
		return reconcile.Result{}, err
	}

	if err := r.addFinalizer(pcs); err != nil {
		return reconcile.Result{}, err
	}

	// Define a new Pod object
	ctrlSet := newStatefulSetforPCS(pcs)

	// Set PiraeusControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &appsv1beta2.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ctrlSet.Name, Namespace: ctrlSet.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"controllerSet": fmt.Sprintf("%+v", ctrlSet),
		}).Info("creating a new DaemonSet")
		err = r.client.Create(context.TODO(), ctrlSet)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - requeue for registration
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"controllerSet": fmt.Sprintf("%+v", ctrlSet),
	}).Debug("controllerSet already exists")

	errs := r.reconcileControllers(pcs)
	compoundErrorMsg := newCompoundErrorMsg(errs)
	pcs.Status.Errors = compoundErrorMsg

	if err := r.client.Status().Update(context.TODO(), pcs); err != nil {
		logrus.Error(err, "Failed to update PiraeusControllerSet status")
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"errs": compoundErrorMsg,
	}).Debug("reconcile loop end")

	var compoundError error
	if len(compoundErrorMsg) != 0 {
		compoundError = fmt.Errorf("requeuing reconcile loop for the following reason(s): %s", strings.Join(compoundErrorMsg, " ;; "))
	}

	return reconcile.Result{RequeueAfter: time.Minute * 1}, compoundError
}

func (r *ReconcilePiraeusControllerSet) reconcileControllers(pcs *piraeusv1alpha1.PiraeusControllerSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"PiraeusControllerSet": fmt.Sprintf("%+v", pcs),
	})
	log.Info("reconciling PiraeusControllerSet Nodes")

	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"app": pcs.Name, "role": "piraeus-controller"})
	listOps := &client.ListOptions{Namespace: pcs.Namespace, LabelSelector: labelSelector}
	err := r.client.List(context.TODO(), listOps, pods)
	if err != nil {
		return []error{err}
	}
	logrus.WithFields(logrus.Fields{
		"pods": fmt.Sprintf("%+v", pods),
	}).Debug("found pods")

	if len(pods.Items) != 1 {
		return []error{fmt.Errorf("waiting until there is exactly one controller pod")}
	}

	return []error{r.reconcileControllerNodeWithControllers(pcs, pods.Items[0])}
}

func (r *ReconcilePiraeusControllerSet) reconcileControllerNodeWithControllers(pcs *piraeusv1alpha1.PiraeusControllerSet, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":              pod.Name,
		"podNameSpace":         pod.Namespace,
		"podPase":              pod.Status.Phase,
		"PiraeusControllerSet": fmt.Sprintf("%+v", pcs),
	})
	log.Debug("reconciling node")

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod %s not running, delaying registration on controller", pod.Spec.NodeName)
	}

	ctrl := pcs.Status.ControllerStatus
	if ctrl == nil {
		pcs.Status.ControllerStatus = &piraeusv1alpha1.NodeStatus{NodeName: pod.Spec.NodeName}
		ctrl = pcs.Status.ControllerStatus
	}

	// Mark this true on successful exit from this function.
	ctrl.RegisteredOnController = false

	wantedDefaultNetInterface := lapi.NetInterface{
		Name:    "default",
		Address: pod.Status.HostIP,
	}

	var err error
	r.linstorClient, err = newLinstorClientFromPod(pod)
	if err != nil {
		return err
	}

	node, err := r.linstorClient.Nodes.Get(context.TODO(), pod.Spec.NodeName)
	if err != nil {
		if err == lapi.NotFoundError {
			if err := r.linstorClient.Nodes.Create(context.TODO(), lapi.Node{
				Name:          pod.Spec.NodeName,
				Type:          "Controller",
				NetInterfaces: []lapi.NetInterface{wantedDefaultNetInterface},
			}); err != nil {
				return fmt.Errorf("unable to create node %s: %v", pod.Spec.NodeName, err)
			}
			return fmt.Errorf("node %s created, allowing controller time to configure it", pod.Spec.NodeName)
		}
		return fmt.Errorf("unable to get node %s: %v", pod.Spec.NodeName, err)
	}

	log.WithFields(logrus.Fields{
		"linstorNode": fmt.Sprintf("%+v", node),
	}).Debug("found node")

	// Make sure default network interface is to spec.
	for _, nodeIf := range node.NetInterfaces {
		ifLog := log.WithFields(logrus.Fields{
			"foundInterface":  fmt.Sprintf("%+v", nodeIf),
			"WantedInterface": fmt.Sprintf("%+v", wantedDefaultNetInterface),
		})
		if nodeIf.Name == wantedDefaultNetInterface.Name {

			// TODO: Maybe we should error out here.
			if nodeIf.Address != wantedDefaultNetInterface.Address {
				if err := r.linstorClient.Nodes.ModifyNetInterface(
					context.TODO(), pod.Spec.NodeName, wantedDefaultNetInterface.Name, wantedDefaultNetInterface); err != nil {
					return fmt.Errorf("unable to modify default network interface on %s: %v", pod.Spec.NodeName, err)
				}
			}
			break
		}

		ifLog.Info("node doesn't have a default interface, creating it")
		if err := r.linstorClient.Nodes.CreateNetInterface(
			context.TODO(), pod.Spec.NodeName, wantedDefaultNetInterface); err != nil {
			return fmt.Errorf("unable to create default network interface on %s: %v", pod.Spec.NodeName, err)
		}
	}

	// TODO: Expand LINSTOR API for an all nodes plus storage pools view?
	nodes, err := r.linstorClient.Nodes.GetAll(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to get cluster nodes: %v", err)
	}
	pools, err := r.linstorClient.Nodes.GetStoragePoolView(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to get cluster storage pools: %v", err)
	}

	if pcs.Status.SatelliteStatuses == nil {
		pcs.Status.SatelliteStatuses = make(map[string]*piraeusv1alpha1.CtrlSatelliteStatus, 0)
	}

	for _, node := range nodes {
		if node.Type == "SATELLITE" {
			pcs.Status.SatelliteStatuses[node.Name] = &piraeusv1alpha1.CtrlSatelliteStatus{
				NodeStatus: piraeusv1alpha1.NodeStatus{
					NodeName:               node.Name,
					RegisteredOnController: true,
				},
				ConnectionStatus:    node.ConnectionStatus,
				StoragePoolStatuses: make(map[string]*piraeusv1alpha1.StoragePoolStatus, 0),
			}
			for _, pool := range pools {
				if pool.NodeName == node.Name {
					pcs.Status.SatelliteStatuses[node.Name].StoragePoolStatuses[pool.StoragePoolName] = newStoragePoolStatus(pool)
				}
			}
		}
	}

	ctrl.RegisteredOnController = true
	return nil
}

func (r *ReconcilePiraeusControllerSet) finalizeControllerSet(pcs *piraeusv1alpha1.PiraeusControllerSet) error {
	log := logrus.WithFields(logrus.Fields{
		"PiraeusControllerSet": fmt.Sprintf("%+v", pcs),
	})
	log.Info("found PiraeusControllerSet marked for deletion, finalizing...")

	if contains(pcs.GetFinalizers(), linstorControllerFinalizer) {
		// Run finalization logic for PiraeusControllerSet. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.

		// Right now we can't set the client until we've reconcilded at least
		// once. This is a hack until we have proper networking.
		if r.linstorClient == nil {
			errs := r.reconcileControllers(pcs)
			compoundErrorMsg := newCompoundErrorMsg(errs)
			pcs.Status.Errors = compoundErrorMsg

			if err := r.client.Status().Update(context.TODO(), pcs); err != nil {
				logrus.Error(err, "Failed to update PiraeusControllerSet status")
				return err
			}

			logrus.WithFields(logrus.Fields{
				"errs": compoundErrorMsg,
			}).Debug("reconcile loop end")

			var compoundError error
			if len(compoundErrorMsg) != 0 {
				compoundError = fmt.Errorf("requeuing reconcile loop for the following reason(s): %s", strings.Join(compoundErrorMsg, " ;; "))
			}
			return compoundError
		}

		nodes, err := r.linstorClient.Nodes.GetAll(context.TODO())
		if err != nil {
			if err == lapi.NotFoundError {
			}
			return fmt.Errorf("unable to get cluster nodes: %v", err)
		}

		var nodeNames = make([]string, 0)
		for _, node := range nodes {
			if node.Type == "SATELLITE" {
				nodeNames = append(nodeNames, node.Name)
			}
		}

		if len(nodeNames) != 0 {
			return fmt.Errorf("controller still has active satellites which must be cleared before deletion: %v", nodeNames)
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		log.Info("finalizing finished, removing finalizer")
		pcs.SetFinalizers(remove(pcs.GetFinalizers(), linstorControllerFinalizer))
		if err := r.client.Update(context.TODO(), pcs); err != nil {
			return err
		}

		if err := r.client.Status().Update(context.TODO(), pcs); err != nil {
			return err
		}

		return nil
	}
	return nil
}

func (r *ReconcilePiraeusControllerSet) addFinalizer(pcs *piraeusv1alpha1.PiraeusControllerSet) error {
	if !contains(pcs.GetFinalizers(), linstorControllerFinalizer) {
		pcs.SetFinalizers(append(pcs.GetFinalizers(), linstorControllerFinalizer))

		err := r.client.Update(context.TODO(), pcs)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func newStatefulSetforPCS(pcs *piraeusv1alpha1.PiraeusControllerSet) *appsv1beta2.StatefulSet {
	var (
		isPrivileged           = true
		directoryType          = corev1.HostPathDirectory
		linstorDatabaseDirName = "linstor-db"
		linstorDatabaseDir     = "/var/lib/linstor"
		replicas               = int32(1)
	)

	labels := map[string]string{
		"app":  pcs.Name,
		"role": "piraeus-controller",
	}
	return &appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-controller",
			Namespace: "kube-system",
			Labels:    labels,
		},
		Spec: appsv1beta2.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pcs.Name + "-controller",
					Namespace: "kube-system",
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									corev1.NodeSelectorTerm{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											corev1.NodeSelectorRequirement{
												Key:      "linstor.linbit.com/linstor-node-type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"controller"},
											},
										},
									},
								},
							},
						},
					},
					PriorityClassName: "system-node-critical",
					HostNetwork:       true,
					Containers: []corev1.Container{
						{
							Name:            "linstor-controller",
							Image:           "quay.io/piraeusdatastore/piraeus-server:latest",
							Args:            []string{"startController"}, // Run linstor-controller.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &isPrivileged},
							Ports: []corev1.ContainerPort{
								corev1.ContainerPort{
									HostPort:      3376,
									ContainerPort: 3376,
								},
								corev1.ContainerPort{
									HostPort:      3370,
									ContainerPort: 3370,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      linstorDatabaseDirName,
									MountPath: linstorDatabaseDir,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: linstorDatabaseDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: linstorDatabaseDir,
									Type: &directoryType,
								}}},
					},
				},
			},
		},
	}
}

func newLinstorClientFromPod(pod corev1.Pod) (*lapi.Client, error) {
	if pod.Spec.NodeName == "" {
		return nil, fmt.Errorf("unable to create LINSTOR API client: ControllerIP cannot be empty")
	}
	u, err := url.Parse("http://" + pod.Spec.NodeName + ":3370")
	//u, err := url.Parse("http://" + "localhost" + ":3370")
	if err != nil {
		return nil, fmt.Errorf("unable to create LINSTOR API client: %v", err)
	}
	c, err := lapi.NewClient(
		lapi.BaseURL(u),
		lapi.Log(&lapi.LogCfg{Level: "debug", Out: os.Stdout, Formatter: &logrus.TextFormatter{}}),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create LINSTOR API client: %v", err)
	}

	return c, nil
}

func newStoragePoolStatus(pool lapi.StoragePool) *piraeusv1alpha1.StoragePoolStatus {
	return &piraeusv1alpha1.StoragePoolStatus{
		Name:          pool.StoragePoolName,
		NodeName:      pool.NodeName,
		Provider:      string(pool.ProviderKind),
		FreeCapacity:  pool.FreeCapacity,
		TotalCapacity: pool.TotalCapacity,
	}
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
