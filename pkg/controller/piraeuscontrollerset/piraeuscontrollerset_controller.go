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
	"os"
	"strings"
	"time"

	lapiconst "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"

	"github.com/sirupsen/logrus"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
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
	linstorClient *lc.HighLevelClient
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
// This function is a mini-main function and has a lot of boilerplate code
// that doesn't make a lot of sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
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
		pcs.Status.SatelliteStatuses = make(map[string]*piraeusv1alpha1.SatelliteStatus)
	}

	r.linstorClient, err = lc.NewHighLevelLinstorClientForObject(pcs)
	if err != nil {
		return reconcile.Result{}, err
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

	// Define a service for the controller.
	ctrlService := newServiceForPCS(pcs)
	// Set PiraeusControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlService, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundSrv := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ctrlService.Name, Namespace: ctrlService.Namespace}, foundSrv)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"controllerService": fmt.Sprintf("%+v", ctrlService),
		}).Info("creating a new Service")
		err = r.client.Create(context.TODO(), ctrlService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"controllerService": fmt.Sprintf("%+v", ctrlService),
	}).Debug("controllerSet already exists")

	// Define a new StatefulSet object
	ctrlSet := newStatefulSetForPCS(pcs)

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
	labelSelector := labels.SelectorFromSet(pcsLabels(pcs))
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
		return fmt.Errorf("pod %s not running, delaying registration on controller", pod.Name)
	}

	ctrl := pcs.Status.ControllerStatus
	if ctrl == nil {
		pcs.Status.ControllerStatus = &piraeusv1alpha1.NodeStatus{NodeName: pod.Spec.NodeName}
		ctrl = pcs.Status.ControllerStatus
	}

	// Mark this true on successful exit from this function.
	ctrl.RegisteredOnController = false

	node, err := r.linstorClient.GetNodeOrCreate(context.TODO(), lapi.Node{
		Name: pod.Name,
		Type: lc.Controller,
		NetInterfaces: []lapi.NetInterface{
			{
				Name:                    "default",
				Address:                 pod.Status.PodIP,
				SatellitePort:           lapiconst.DfltStltPortPlain,
				SatelliteEncryptionType: lapiconst.ValNetcomTypePlain,
			},
		},
	})
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{
		"linstorNode": fmt.Sprintf("%+v", node),
	}).Debug("found node")

	nodes, err := r.linstorClient.GetAllStorageNodes(context.TODO())
	if err != nil {
		return fmt.Errorf("unable to get cluster storage nodes: %v", err)
	}

	if pcs.Status.SatelliteStatuses == nil {
		pcs.Status.SatelliteStatuses = make(map[string]*piraeusv1alpha1.SatelliteStatus)
	}

	for i := range nodes {
		node := &nodes[i]

		pcs.Status.SatelliteStatuses[node.Name] = &piraeusv1alpha1.SatelliteStatus{
			NodeStatus: piraeusv1alpha1.NodeStatus{
				NodeName:               node.Name,
				RegisteredOnController: true,
			},
			ConnectionStatus:    node.ConnectionStatus,
			StoragePoolStatuses: make(map[string]*piraeusv1alpha1.StoragePoolStatus),
		}

		for i := range node.StoragePools {
			pool := node.StoragePools[i]

			pcs.Status.SatelliteStatuses[node.Name].StoragePoolStatuses[pool.StoragePoolName] = piraeusv1alpha1.NewStoragePoolStatus(pool)
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

	if mdutil.HasFinalizer(pcs, linstorControllerFinalizer) {
		// Run finalization logic for PiraeusControllerSet. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.

		nodes, err := r.linstorClient.Nodes.GetAll(context.TODO())
		if err != nil {
			if err != lapi.NotFoundError {
				return fmt.Errorf("unable to get cluster nodes: %v", err)
			}
		}

		var nodeNames = make([]string, 0)
		for _, node := range nodes {
			if node.Type == lc.Satellite {
				nodeNames = append(nodeNames, node.Name)
			}
		}

		if len(nodeNames) != 0 {
			return fmt.Errorf("controller still has active satellites which must be cleared before deletion: %v", nodeNames)
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		log.Info("finalizing finished, removing finalizer")
		if err := r.deleteFinalizer(pcs); err != nil {
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
	mdutil.AddFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(context.TODO(), pcs)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcilePiraeusControllerSet) deleteFinalizer(pcs *piraeusv1alpha1.PiraeusControllerSet) error {
	mdutil.DeleteFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(context.TODO(), pcs)
	if err != nil {
		return err
	}
	return nil
}

func newStatefulSetForPCS(pcs *piraeusv1alpha1.PiraeusControllerSet) *appsv1beta2.StatefulSet {
	var (
		replicas = int32(1)
	)

	labels := pcsLabels(pcs)

	return &appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-controller",
			Namespace: pcs.Namespace,
			Labels:    labels,
		},
		Spec: appsv1beta2.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pcs.Name + "-controller",
					Namespace: pcs.Namespace,
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
												Key:      kubeSpec.PiraeusControllerNode,
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"true"},
											},
										},
									},
								},
							},
						},
					},
					PriorityClassName: kubeSpec.PiraeusPriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            "linstor-controller",
							Image:           kubeSpec.PiraeusServerImage + ":" + kubeSpec.PiraeusVersion,
							Args:            []string{"startController"}, // Run linstor-controller.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      3376,
									ContainerPort: 3376,
								},
								{
									HostPort:      3370,
									ContainerPort: 3370,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      kubeSpec.LinstorDatabaseDirName,
									MountPath: kubeSpec.LinstorDatabaseDir,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: kubeSpec.LinstorDatabaseDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: kubeSpec.LinstorDatabaseDir,
									Type: &kubeSpec.HostPathDirectoryOrCreateType,
								}}},
					},
				},
			},
		},
	}
}

func newServiceForPCS(pcs *piraeusv1alpha1.PiraeusControllerSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name,
			Namespace: pcs.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:       "rest-http",
					Port:       3370,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(3370),
				},
			},
			Selector: pcsLabels(pcs),
			Type:     "ClusterIP",
		},
	}
}

func pcsLabels(pcs *piraeusv1alpha1.PiraeusControllerSet) map[string]string {
	return map[string]string{
		"app":  pcs.Name,
		"role": "piraeus-controller",
	}
}
