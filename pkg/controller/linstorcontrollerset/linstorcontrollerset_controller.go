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

	lapi "github.com/LINBIT/golinstor/client"
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"

	"github.com/BurntSushi/toml"
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

const (
	linstorControllerFinalizer = "finalizer.linstor-controller.linbit.com"

	// requeue reconciliation after connectionRetrySeconds
	connectionRetrySeconds = 10
)

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

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
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
	log := logrus.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})

	log.Info("CS Reconcile: Entering")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	// Fetch the LinstorControllerSet instance
	pcs := &piraeusv1alpha1.LinstorControllerSet{}
	err := r.client.Get(ctx, request.NamespacedName, pcs)
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

	if pcs.Status.ControllerStatus == nil {
		pcs.Status.ControllerStatus = &piraeusv1alpha1.NodeStatus{}
	}

	if pcs.Status.SatelliteStatuses == nil {
		pcs.Status.SatelliteStatuses = make([]*piraeusv1alpha1.SatelliteStatus, 0)
	}

	if pcs.Spec.DrbdRepoCred == "" {
		return reconcile.Result{}, fmt.Errorf("CS Reconcile: missing required parameter drbdRepoCred: outdated schema")
	}

	if pcs.Spec.ControllerImage == "" {
		return reconcile.Result{}, fmt.Errorf("CS Reconcile: missing required parameter controllerImage: outdated schema")
	}

	if pcs.Spec.DBConnectionURL == "" {
		return reconcile.Result{}, fmt.Errorf("CS Reconcile: missing required parameter dbConnectionURL: outdated schema")
	}

	log.Info("reconciling LinstorControllerSet")

	getSecret := func(secretName string) (map[string][]byte, error) {
		secret := corev1.Secret{}
		err := r.client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: pcs.Namespace}, &secret)
		if err != nil {
			return nil, err
		}
		return secret.Data, nil
	}

	endpoint := expectedEndpoint(pcs)
	r.linstorClient, err = lc.NewHighLevelLinstorClientFromConfig(endpoint, &pcs.Spec.LinstorClientConfig, getSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	markedForDeletion := pcs.GetDeletionTimestamp() != nil
	if markedForDeletion {
		result, err := r.finalizeControllerSet(ctx, pcs)

		log.WithFields(logrus.Fields{
			"result": result,
			"err":    err,
		}).Info("CS Reconcile: reconcile loop end")

		return result, err
	}

	if err := r.addFinalizer(ctx, pcs); err != nil {
		return reconcile.Result{}, err
	}

	// Define a service for the controller.
	ctrlService := newServiceForPCS(pcs)
	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlService, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundSrv := &corev1.Service{}
	err = r.client.Get(ctx, types.NamespacedName{Name: ctrlService.Name, Namespace: ctrlService.Namespace}, foundSrv)
	if err != nil && errors.IsNotFound(err) {
		log.Info("CS Reconcile: creating a new Service")

		err = r.client.Create(ctx, ctrlService)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	log.Debug("CS Reconcile: CS already exists")

	// Define a configmap for the controller.
	configMap, err := NewConfigMapForPCS(pcs)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, configMap, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	foundConfigMap := &corev1.ConfigMap{}
	err = r.client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
	if err != nil && errors.IsNotFound(err) {
		log.Info("CS Reconcile: creating a new ConfigMap")

		err = r.client.Create(ctx, configMap)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	log.Debug("CS Reconcile: controllerConfigMap already exists")

	// Define a new Deployment object
	ctrlSet := newDeploymentForResource(pcs)

	// Set LinstorControllerSet instance as the owner and controller
	if err := controllerutil.SetControllerReference(pcs, ctrlSet, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	found := &appsv1.Deployment{}
	err = r.client.Get(ctx, types.NamespacedName{Name: ctrlSet.Name, Namespace: ctrlSet.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("CS Reconcile: creating a new Deployment")

		err = r.client.Create(ctx, ctrlSet)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - requeue for registration
		return reconcile.Result{Requeue: true}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	log.Debug("CS Reconcile: Deployment already exists")

	resErr := r.reconcileControllers(ctx, pcs)

	statusErr := r.reconcileStatus(ctx, pcs, resErr)
	if statusErr != nil {
		log.Warnf("failed to update status. original error: %v", resErr)
		return reconcile.Result{}, statusErr
	}

	result, err := reconcileutil.ToReconcileResult(resErr)

	log.WithFields(logrus.Fields{
		"result": result,
		"err":    err,
	}).Info("CS Reconcile: reconcile loop end")

	return result, err
}

func (r *ReconcileLinstorControllerSet) reconcileControllers(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	})
	log.Info("CS Reconcile: reconciling CS Nodes")

	log.Debug("wait for controller service to come online")

	_, err := r.linstorClient.Nodes.GetControllerVersion(ctx)
	if err != nil {
		return &reconcileutil.TemporaryError{
			Source:       err,
			RequeueAfter: connectionRetrySeconds * time.Second,
		}
	}

	allNodes, err := r.linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		return err
	}

	var ourControllers []lapi.Node
	for _, node := range allNodes {
		registrar, ok := node.Props[kubeSpec.LinstorRegistrationProperty]
		if ok && registrar == kubeSpec.Name && node.Type == lc.Controller {
			ourControllers = append(ourControllers, node)
		}
	}

	ourPods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(pcsLabels(pcs))
	err = r.client.List(ctx, ourPods, client.InNamespace(pcs.Namespace), client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		return err
	}

	log.Debug("register controller pods in LINSTOR")

	for _, pod := range ourPods.Items {
		log.WithField("pod", pod.Name).Debug("register controller pod")
		_, err := r.linstorClient.GetNodeOrCreate(ctx, lapi.Node{
			Name: pod.Name,
			Type: lc.Controller,
			NetInterfaces: []lapi.NetInterface{
				{
					Name:                    "default",
					Address:                 pod.Status.PodIP,
					SatellitePort:           pcs.Spec.SslConfig.Port(),
					SatelliteEncryptionType: pcs.Spec.SslConfig.Type(),
				},
			},
			Props: map[string]string{
				kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
			},
		})
		if err != nil {
			return err
		}
	}

	log.Debug("remove controllers without pods from LINSTOR")

	for _, linstorController := range ourControllers {
		found := false
		for _, pod := range ourPods.Items {
			if pod.Name == linstorController.Name {
				found = true
				break
			}
		}

		if !found {
			log.WithField("node", linstorController.Name).Debug("remove controller pod")
			err = r.linstorClient.Nodes.Delete(ctx, linstorController.Name)
			if err != nil {
				return err
			}
		}
	}

	if len(ourPods.Items) > 1 {
		log.WithField("#controllerPods", len(ourPods.Items)).Debug("requeue because multiple controller pods are present")
		return &reconcileutil.TemporaryError{
			RequeueAfter: time.Minute,
			Source: fmt.Errorf("multiple controller pods present"),
		}
	}

	return nil
}

func (r *ReconcileLinstorControllerSet) reconcileStatus(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet, resErr error) error {
	log := logrus.WithFields(logrus.Fields{
		"Name":      pcs.Name,
		"Namespace": pcs.Namespace,
	})
	log.Info("reconcile status")

	log.Debug("find active controller pod")
	pod, err := r.findActiveControllerPod(ctx, pcs)
	if err != nil {
		log.Warnf("failed to find active controller pod: %v", err)
	}

	controllerName := ""
	if pod != nil {
		controllerName = pod.Name
	}

	pcs.Status.ControllerStatus = &piraeusv1alpha1.NodeStatus{
		NodeName: controllerName,
		RegisteredOnController: false,
	}

	log.Debug("check if controller pod is registered")

	allNodes, err := r.linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		log.Warnf("failed to fetch list of LINSTOR nodes: %v", err)
	}

	for _, node := range allNodes {
		if pod != nil && node.Name == pod.Name {
			pcs.Status.ControllerStatus.RegisteredOnController = true
		}
	}

	log.Debug("fetch information about storage nodes")

	nodes, err := r.linstorClient.GetAllStorageNodes(ctx)
	if err != nil {
		log.Warnf("unable to get LINSTOR storage nodes: %v, continue with empty node list", err)
		nodes = nil
	}

	pcs.Status.SatelliteStatuses = make([]*piraeusv1alpha1.SatelliteStatus, len(nodes))

	for i := range nodes {
		node := &nodes[i]

		pcs.Status.SatelliteStatuses[i] = &piraeusv1alpha1.SatelliteStatus{
			NodeStatus: piraeusv1alpha1.NodeStatus{
				NodeName:               node.Name,
				RegisteredOnController: true,
			},
			ConnectionStatus:    node.ConnectionStatus,
			StoragePoolStatuses: make([]*piraeusv1alpha1.StoragePoolStatus, len(node.StoragePools)),
		}

		for j := range node.StoragePools {
			pool := &node.StoragePools[j]

			pcs.Status.SatelliteStatuses[i].StoragePoolStatuses[j] = piraeusv1alpha1.NewStoragePoolStatus(pool)
		}
	}

	pcs.Status.Errors = reconcileutil.ErrorStrings(resErr)

	log.Debug("update status in resource")

	return r.client.Status().Update(ctx, pcs)
}

func (r *ReconcileLinstorControllerSet) findActiveControllerPod(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) (*corev1.Pod, error) {
	ourPods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(pcsLabels(pcs))
	err := r.client.List(ctx, ourPods, client.InNamespace(pcs.Namespace), client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		return nil, err
	}

	// Find the single currently serving pod
	var candidatePods []corev1.Pod
	for _, pod := range ourPods.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				candidatePods = append(candidatePods, pod)
				break
			}
		}
	}

	switch len(candidatePods) {
	case 1:
		return &candidatePods[0], nil
	case 0:
		return nil, nil
	default:
		return nil, fmt.Errorf("expected one controller pod, got multiple: %v", candidatePods)
	}

}

// finalizeControllerSet returns whether it is finished as well as potentially an error
func (r *ReconcileLinstorControllerSet) finalizeControllerSet(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) (reconcile.Result, error) {
	log := logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	})
	log.Info("CS finalizeControllerSet: found LinstorControllerSet marked for deletion, finalizing...")

	if !mdutil.HasFinalizer(pcs, linstorControllerFinalizer) {
		return reconcile.Result{}, nil
	}

	nodesOnControllerErr := r.ensureNoNodesOnController(ctx, pcs)

	statusErr := r.reconcileStatus(ctx, pcs, nodesOnControllerErr)
	if statusErr != nil {
		log.Warnf("failed to update status. original error: %v", nodesOnControllerErr)
		return reconcile.Result{}, statusErr
	}

	if nodesOnControllerErr != nil {
		return reconcileutil.ToReconcileResult(nodesOnControllerErr)
	}

	log.Info("CS finalizing finished, removing finalizer")

	err := r.deleteFinalizer(ctx, pcs)

	return reconcile.Result{}, err
}

// returns an error if nodes are still registered.
func (r *ReconcileLinstorControllerSet) ensureNoNodesOnController(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) error {
	log := logrus.WithFields(logrus.Fields{
		"name":      pcs.Name,
		"namespace": pcs.Namespace,
		"spec":      fmt.Sprintf("%+v", pcs.Spec),
	})
	if pcs.Status.ControllerStatus.NodeName == "" {
		log.Info("CS never deployed; finalization OK")
		return nil
	}

	nodes, err := r.linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		if err != lapi.NotFoundError {
			return fmt.Errorf("CS unable to get cluster nodes: %v", err)
		}
	}

	nodeNames := make([]string, 0)
	for _, node := range nodes {
		if node.Type == lc.Satellite {
			nodeNames = append(nodeNames, node.Name)
		}
	}

	if len(nodeNames) != 0 {
		return &reconcileutil.TemporaryError{
			Source:       fmt.Errorf("CS controller still has active satellites which must be cleared before deletion: %v", nodeNames),
			RequeueAfter: 1 * time.Minute,
		}
	}

	return nil
}

func (r *ReconcileLinstorControllerSet) addFinalizer(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) error {
	mdutil.AddFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(ctx, pcs)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorControllerSet) deleteFinalizer(ctx context.Context, pcs *piraeusv1alpha1.LinstorControllerSet) error {
	mdutil.DeleteFinalizer(pcs, linstorControllerFinalizer)

	err := r.client.Update(ctx, pcs)
	if err != nil {
		return err
	}
	return nil
}

func newDeploymentForResource(pcs *piraeusv1alpha1.LinstorControllerSet) *appsv1.Deployment {
	labels := pcsLabels(pcs)

	env := []corev1.EnvVar{
		{
			Name: kubeSpec.JavaOptsName,
			// Workaround for https://github.com/LINBIT/linstor-server/issues/123
			Value: "-Djdk.tls.acknowledgeCloseNotify=true",
		},
	}

	volumes := []corev1.Volume{
		{
			Name: kubeSpec.LinstorConfDirName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: pcs.Name + "-config",
					},
				},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      kubeSpec.LinstorConfDirName,
			MountPath: kubeSpec.LinstorConfDir,
		},
	}

	if pcs.Spec.LuksSecret != "" {
		env = append(env, corev1.EnvVar{
			Name: kubeSpec.LinstorLUKSPassphraseEnvName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: pcs.Spec.LuksSecret,
					},
					Key: kubeSpec.LinstorLUKSPassphraseEnvName,
				},
			},
		})
	}

	if pcs.Spec.DBCertSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorCertDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pcs.Spec.DBCertSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorCertDirName,
			MountPath: kubeSpec.LinstorCertDir,
			ReadOnly:  true,
		})
	}

	if pcs.Spec.LinstorHttpsControllerSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorHttpsCertDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pcs.Spec.LinstorHttpsControllerSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorHttpsCertDirName,
			MountPath: kubeSpec.LinstorHttpsCertDir,
			ReadOnly:  true,
		})
	}

	if pcs.Spec.LinstorHttpsClientSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorClientDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: pcs.Spec.LinstorHttpsClientSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorClientDirName,
			MountPath: kubeSpec.LinstorClientDir,
		})
	}

	if !pcs.Spec.SslConfig.IsPlain() {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorSslDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: string(*pcs.Spec.SslConfig),
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorSslDirName,
			MountPath: kubeSpec.LinstorSslDir,
			ReadOnly:  true,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-controller",
			Namespace: pcs.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pcs.Name + "-controller",
					Namespace: pcs.Namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					PriorityClassName: pcs.Spec.PriorityClassName.GetName(pcs.Namespace),
					Containers: []corev1.Container{
						{
							Name:            "linstor-controller",
							Image:           pcs.Spec.ControllerImage,
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
									HostPort:      lc.DefaultHttpPort,
									ContainerPort: lc.DefaultHttpPort,
								},
								{
									HostPort:      lc.DefaultHttpsPort,
									ContainerPort: lc.DefaultHttpsPort,
								},
							},
							VolumeMounts: volumeMounts,
							Env:          env,
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
										// Http is always enabled (it will redirect to https if configured)
										Scheme: corev1.URISchemeHTTP,
										Port:   intstr.FromInt(lc.DefaultHttpPort),
									},
								},
								TimeoutSeconds:      10,
								PeriodSeconds:       20,
								FailureThreshold:    10,
								InitialDelaySeconds: 5,
							},
						},
					},
					Volumes: volumes,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: pcs.Spec.DrbdRepoCred,
						},
					},
					Affinity:    pcs.Spec.Affinity,
					Tolerations: pcs.Spec.Tolerations,
				},
			},
		},
	}
}

func newServiceForPCS(pcs *piraeusv1alpha1.LinstorControllerSet) *corev1.Service {
	port := lc.DefaultHttpPort
	if pcs.Spec.LinstorHttpsControllerSecret != "" {
		port = lc.DefaultHttpsPort
	}

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
					Port:       int32(port),
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(port),
				},
			},
			Selector: pcsLabels(pcs),
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func NewConfigMapForPCS(pcs *piraeusv1alpha1.LinstorControllerSet) (*corev1.ConfigMap, error) {
	dbCertificatePath := ""
	dbClientCertPath := ""
	dbClientKeyPath := ""
	if pcs.Spec.DBCertSecret != "" {
		dbCertificatePath = kubeSpec.LinstorCertDir + "/ca.pem"
		if pcs.Spec.DBUseClientCert {
			dbClientCertPath = kubeSpec.LinstorCertDir + "/client.cert"
			dbClientKeyPath = kubeSpec.LinstorCertDir + "/client.key"
		}
	}

	https := lapi.ControllerConfigHttps{}
	if pcs.Spec.LinstorHttpsControllerSecret != "" {
		https.Enabled = true
		https.Keystore = kubeSpec.LinstorHttpsCertDir + "/keystore.jks"
		https.KeystorePassword = kubeSpec.LinstorHttpsCertPassword
		https.Truststore = kubeSpec.LinstorHttpsCertDir + "/truststore.jks"
		https.TruststorePassword = kubeSpec.LinstorHttpsCertPassword
	}

	linstorControllerConfig := lapi.ControllerConfig{
		Db: lapi.ControllerConfigDb{
			ConnectionUrl:     pcs.Spec.DBConnectionURL,
			CaCertificate:     dbCertificatePath,
			ClientCertificate: dbClientCertPath,
			ClientKeyPkcs8Pem: dbClientKeyPath,
		},
		Https: https,
	}

	controllerConfigBuilder := strings.Builder{}
	if err := toml.NewEncoder(&controllerConfigBuilder).Encode(linstorControllerConfig); err != nil {
		return nil, err
	}

	endpoint := expectedEndpoint(pcs)
	clientConfig := lc.NewClientConfigForAPIResource(endpoint, &pcs.Spec.LinstorClientConfig)
	clientConfigFile, err := clientConfig.ToConfigFile()
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcs.Name + "-config",
			Namespace: pcs.Namespace,
		},
		Data: map[string]string{
			kubeSpec.LinstorControllerConfigFile: controllerConfigBuilder.String(),
			kubeSpec.LinstorClientConfigFile:     clientConfigFile,
		},
	}

	return cm, nil
}

func expectedEndpoint(pcs *piraeusv1alpha1.LinstorControllerSet) string {
	serviceName := types.NamespacedName{Name: pcs.Name, Namespace: pcs.Namespace}
	useHTTPS := pcs.Spec.LinstorHttpsClientSecret != ""

	return lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
}

func pcsLabels(pcs *piraeusv1alpha1.LinstorControllerSet) map[string]string {
	return map[string]string{
		"app":  pcs.Name,
		"role": kubeSpec.ControllerRole,
	}
}
