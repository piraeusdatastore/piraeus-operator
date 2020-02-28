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

package linstorcontrollerset

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
	appsv1 "k8s.io/api/apps/v1"
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

// var log = logf.Log.WithName("controller_linstorcontrollerset")

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)
}

// var log = logrus.WithFields(logrus.Fields{
// 	"controller": "LinstorControllerSet",
// })

// Add creates a new LinstorControllerSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorControllerSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("linstorcontrollerset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorControllerSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.LinstorControllerSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1alpha1.LinstorControllerSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorControllerSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorControllerSet{}

// ReconcileLinstorControllerSet reconciles a LinstorControllerSet object
type ReconcileLinstorControllerSet struct {
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

// Reconcile reads that state of the cluster for a LinstorControllerSet object and makes changes based
// on the state read and what is in the LinstorControllerSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
// This function is a mini-main function and has a lot of boilerplate code that doesn't make a lot of
// sense to put elsewhere, so don't lint it for cyclomatic complexity.
// nolint:gocyclo
func (r *ReconcileLinstorControllerSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	reqLogger := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})

	reqLogger.Info("CS Reconcile: Entering")

	// Fetch the LinstorControllerSet instance
	pcs := &piraeusv1alpha1.LinstorControllerSet{}
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

	if pcs.Spec.DrbdRepoCred == "" {
		pcs.Spec.DrbdRepoCred = kubeSpec.DrbdRepoCred
	}

	log := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
		"name":              pcs.Name,
		"namespace":         pcs.Namespace,
	})
	log.Info("reconciling LinstorControllerSet")

	logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	}).Debug("found LinstorControllerSet")

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

		log.Debug("CS Reconcile: reconcile loop end")
		return reconcile.Result{}, err
	}

	if err := r.addFinalizer(pcs); err != nil {
		return reconcile.Result{}, err
	}

	// Define a service for the controller.
	ctrlService := newServiceForPCS(pcs)
	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlService, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundSrv := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ctrlService.Name, Namespace: ctrlService.Namespace}, foundSrv)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      ctrlService.Name,
			"namespace": ctrlService.Namespace,
		}).Info("CS Reconcile: creating a new Service")
		err = r.client.Create(context.TODO(), ctrlService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      ctrlService.Name,
		"namespace": ctrlService.Namespace,
	}).Debug("CS Reconcile: CS already exists")

	// Define a configmap for the controller.
	configMap := newConfigMapForPCS(pcs)
	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, configMap, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundConfigMap := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      configMap.Name,
			"namespace": configMap.Namespace,
		}).Info("CS Reconcile: creating a new ConfigMap")
		err = r.client.Create(context.TODO(), configMap)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"name":      configMap.Name,
		"namespace": configMap.Namespace,
	}).Debug("CS Reconcile: controllerConfigMap already exists")

	// Define a new StatefulSet object
	ctrlSet := newStatefulSetForPCS(pcs)

	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &appsv1.StatefulSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ctrlSet.Name, Namespace: ctrlSet.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"name":      ctrlSet.Name,
			"namespace": ctrlSet.Namespace,
		}).Info("CS Reconcile: creating a new StatefulSet")
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
		"name":      ctrlSet.Name,
		"namespace": ctrlSet.Namespace,
	}).Debug("CS Reconcile: StatefulSet already exists")

	errs := r.reconcileControllers(pcs)
	compoundErrorMsg := newCompoundErrorMsg(errs)
	pcs.Status.Errors = compoundErrorMsg

	if err := r.client.Status().Update(context.TODO(), pcs); err != nil {
		logrus.Error(err, "CS Reconcile: Failed to update LinstorControllerSet status")
		return reconcile.Result{}, err
	}

	logrus.WithFields(logrus.Fields{
		"errs": compoundErrorMsg,
	}).Debug("reconcile loop end")

	var compoundError error
	if len(compoundErrorMsg) != 0 {
		compoundError = fmt.Errorf("CS Reconcile: requeuing reconcile loop for the following reason(s): %s", strings.Join(compoundErrorMsg, " ;; "))
	}

	return reconcile.Result{RequeueAfter: time.Minute * 1}, compoundError
}

func (r *ReconcileLinstorControllerSet) reconcileControllers(pcs *piraeusv1alpha1.LinstorControllerSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	})
	log.Info("CS Reconcile: reconciling CS Nodes")

	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(pcsLabels(pcs))
	listOpts := []client.ListOption{
		// Namespace: pns.Namespace, LabelSelector: labelSelector}
		client.InNamespace(pcs.Namespace), client.MatchingLabelsSelector{labelSelector}}
	err := r.client.List(context.TODO(), pods, listOpts...)
	if err != nil {
		return []error{err}
	}

	if len(pods.Items) != 1 {
		return []error{fmt.Errorf("CS Reconcile: waiting until there is exactly one controller pod, found %d pods instead", len(pods.Items))}
	}

	return []error{r.reconcileControllerNodeWithControllers(pcs, pods.Items[0])}
}

func (r *ReconcileLinstorControllerSet) reconcileControllerNodeWithControllers(pcs *piraeusv1alpha1.LinstorControllerSet, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":      pod.Name,
		"podNameSpace": pod.Namespace,
		"podPhase":     pod.Status.Phase,
	})
	log.Debug("CS reconcileControllerNodeWithControllers: reconciling node")

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("CS pod %s not running, delaying registration on controller", pod.Name)
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
		"nodeName": node.Name,
		"nodeType": node.Type,
	}).Debug("CS reconcileControllerNodeWithControllers: found node")

	nodes, err := r.linstorClient.GetAllStorageNodes(context.TODO())
	if err != nil {
		return fmt.Errorf("CS unable to get cluster storage nodes: %v", err)
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

func (r *ReconcileLinstorControllerSet) finalizeControllerSet(pcs *piraeusv1alpha1.LinstorControllerSet) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	})
	log.Info("CS finalizeControllerSet: found LinstorControllerSet marked for deletion, finalizing...")

	if mdutil.HasFinalizer(pcs, linstorControllerFinalizer) {
		// Run finalization logic for LinstorControllerSet. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.

		nodes, err := r.linstorClient.Nodes.GetAll(context.TODO())
		if err != nil {
			if err != lapi.NotFoundError {
				return fmt.Errorf("CS unable to get cluster nodes: %v", err)
			}
		}

		var nodeNames = make([]string, 0)
		for _, node := range nodes {
			if node.Type == lc.Satellite {
				nodeNames = append(nodeNames, node.Name)
			}
		}

		if len(nodeNames) != 0 {
			return fmt.Errorf("CS controller still has active satellites which must be cleared before deletion: %v", nodeNames)
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		log.Info("CS finalizing finished, removing finalizer")
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

func (r *ReconcileLinstorControllerSet) addFinalizer(pcs *piraeusv1alpha1.LinstorControllerSet) error {
	mdutil.AddFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(context.TODO(), pcs)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorControllerSet) deleteFinalizer(pcs *piraeusv1alpha1.LinstorControllerSet) error {
	mdutil.DeleteFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(context.TODO(), pcs)
	if err != nil {
		return err
	}
	return nil
}

func newStatefulSetForPCS(pcs *piraeusv1alpha1.LinstorControllerSet) *appsv1.StatefulSet {
	var (
		replicas = int32(1)
	)

	labels := pcsLabels(pcs)

	if pcs.Spec.PriorityClassName == "" {
		pcs.Spec.PriorityClassName = kubeSpec.PiraeusCSPriorityClassName
	}

	if pcs.Spec.ControllerImage == "" {
		pcs.Spec.ControllerImage = kubeSpec.PiraeusControllerImage
	}

	if pcs.Spec.ControllerVersion == "" {
		pcs.Spec.ControllerVersion = kubeSpec.PiraeusControllerVersion
	}

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-controller",
			Namespace: pcs.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pcs.Name + "-controller",
					Namespace: pcs.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					PriorityClassName: pcs.Spec.PriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            "linstor-controller",
							Image:           pcs.Spec.ControllerImage + ":" + pcs.Spec.ControllerVersion,
							Args:            []string{"startController"}, // Run linstor-controller.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &kubeSpec.Privileged},
							Ports: []corev1.ContainerPort{
								{
									HostPort:      3376,
									ContainerPort: 3376,
								},
								{
									HostPort:      3377,
									ContainerPort: 3377,
								},
								{
									HostPort:      3370,
									ContainerPort: 3370,
								},
								{
									HostPort:      3371,
									ContainerPort: 3371,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      kubeSpec.LinstorConfDirName,
									MountPath: kubeSpec.LinstorConfDir,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "LS_CONTROLLERS",
									Value: "http://" + pcs.Name + ":" + "3370",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"linstor", "node", "list"},
									},
								},
								TimeoutSeconds:      10,
								PeriodSeconds:       20,
								FailureThreshold:    10,
								InitialDelaySeconds: 5,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: kubeSpec.LinstorConfDirName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: pcs.Name + "-config",
									},
								}}},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: pcs.Spec.DrbdRepoCred,
						},
					},
				},
			},
		},
	}
}

func newServiceForPCS(pcs *piraeusv1alpha1.LinstorControllerSet) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name,
			Namespace: pcs.Namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:       pcs.Name,
					Port:       3370,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(3370),
				},
			},
			Selector: pcsLabels(pcs),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func newConfigMapForPCS(pcs *piraeusv1alpha1.LinstorControllerSet) *corev1.ConfigMap {

	if pcs.Spec.EtcdURL == "" {
		if pcs.Name[len(pcs.Name)-3:len(pcs.Name)] == "-cs" {
			pcs.Spec.EtcdURL = "etcd://" + pcs.Name[0:len(pcs.Name)-3] + "-etcd:2379"
		} else {
			pcs.Spec.EtcdURL = "etcd://" + pcs.Name + "-etcd:2379"
		}
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-config",
			Namespace: pcs.Namespace,
		},
		Data: map[string]string{
			"linstor.toml": fmt.Sprintf(
				`
				[db]
					connection_url = "%s"
				`, pcs.Spec.EtcdURL)},
	}

	return cm
}

func pcsLabels(pcs *piraeusv1alpha1.LinstorControllerSet) map[string]string {
	return map[string]string{
		"app":  pcs.Name,
		"role": "piraeus-controller",
	}
}
