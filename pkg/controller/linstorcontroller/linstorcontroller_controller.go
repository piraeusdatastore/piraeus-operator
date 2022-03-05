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

package linstorcontroller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/LINBIT/golinstor/linstortoml"
	awaitelection "github.com/linbit/k8s-await-election/pkg/consts"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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

// CreateBackups controls if the operator will create a backup of the LINSTOR resources before upgrading.
var CreateBackups = true

// CreateMonitoring controls if the operator will create a monitoring resources.
var CreateMonitoring = true

// newControllerReconciler returns a new reconcile.Reconciler
func newControllerReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorController{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// addControllerReconciler adds a new Controller to mgr with r as the reconcile.Reconciler
func addControllerReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("LinstorController-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorController
	err = c.Watch(&source.Kind{Type: &piraeusv1.LinstorController{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &piraeusv1.LinstorController{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorController implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorController{}

// ReconcileLinstorController reconciles a LinstorController object
type ReconcileLinstorController struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a LinstorController object and makes changes based
// on the state read and what is in the LinstorController.Spec
func (r *ReconcileLinstorController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"resquestName":      request.Name,
		"resquestNamespace": request.Namespace,
	})

	log.Info("controller Reconcile: Entering")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	// Fetch the LinstorController instance
	controllerResource := &piraeusv1.LinstorController{}
	err := r.client.Get(ctx, request.NamespacedName, controllerResource)
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

	resErr := r.reconcileSpec(ctx, controllerResource)

	statusErr := r.reconcileStatus(ctx, controllerResource, resErr)
	if statusErr != nil {
		log.Warnf("failed to update status. original error: %v", resErr)
		return reconcile.Result{}, statusErr
	}

	result, err := reconcileutil.ToReconcileResult(resErr)

	log.WithFields(logrus.Fields{
		"result": result,
		"err":    err,
	}).Info("controller Reconcile: reconcile loop end")

	triggerStatusUpdate := reconcile.Result{RequeueAfter: 1 * time.Minute}

	return reconcileutil.CombineReconcileResults(result, triggerStatusUpdate), err
}

func (r *ReconcileLinstorController) reconcileSpec(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	log := log.WithFields(logrus.Fields{
		"Op":         "reconcileSpec",
		"Controller": "linstorcontroller",
		"Spec":       controllerResource.Spec,
	})

	log.Info("reconcile spec with env")

	specs := []reconcileutil.EnvSpec{
		{Env: kubeSpec.ImageLinstorControllerEnv, Target: &controllerResource.Spec.ControllerImage},
	}

	err := reconcileutil.UpdateFromEnv(ctx, r.client, controllerResource, specs...)
	if err != nil {
		return err
	}

	log.Debug("check for deletion flag")

	markedForDeletion := controllerResource.GetDeletionTimestamp() != nil
	if markedForDeletion {
		return r.finalizeControllerSet(ctx, controllerResource)
	}

	log.Debug("reconcile finalizer")

	err = r.addFinalizer(ctx, controllerResource)
	if err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}

	log.Debug("reconcile legacy config map name")

	err = reconcileutil.DeleteIfOwned(ctx, r.client, &corev1.ConfigMap{ObjectMeta: getObjectMeta(controllerResource, "%s-config")}, controllerResource)
	if err != nil {
		return fmt.Errorf("failed to delete legacy config map: %w", err)
	}

	log.Debug("reconcile LINSTOR Service")

	ctrlService := newServiceForResource(controllerResource)
	_, err = reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, ctrlService, controllerResource, reconcileutil.OnPatchErrorReturn)
	if err != nil {
		return fmt.Errorf("failed to reconcile LINSTOR Service: %w", err)
	}

	log.Debug("reconcile LINSTOR Controller ConfigMap")

	configMap, err := NewConfigMapForResource(controllerResource)
	if err != nil {
		return fmt.Errorf("failed to render config for LINSTOR: %w", err)
	}

	configmapChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, configMap, controllerResource, reconcileutil.OnPatchErrorReturn)
	if err != nil {
		return fmt.Errorf("failed to reconcile LINSTOR Controller ConfigMap: %w", err)
	}

	if controllerResource.Spec.DBConnectionURL == "k8s" && CreateBackups {
		err := r.reconcileLinstorControllerDatabaseBackup(ctx, controllerResource)
		if err != nil {
			return fmt.Errorf("failed to reconcile LINSTOR database backup: %w", err)
		}
	}

	log.Debug("reconcile LINSTOR Controller Deployment")

	ctrlDeployment := newDeploymentForResource(controllerResource)
	deploymentChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, ctrlDeployment, controllerResource, reconcileutil.OnPatchErrorRecreate)
	if err != nil {
		return fmt.Errorf("failed to reconcile LINSTOR Controller Deployment: %w", err)
	}

	if configmapChanged && !deploymentChanged {
		log.Debug("restart LINSTOR Controller")

		err := reconcileutil.RestartRollout(ctx, r.client, ctrlDeployment)
		if err != nil {
			return fmt.Errorf("failed to restart LINSTOR Controller after ConfigMap change: %w", err)
		}
	}

	if monitoring.Enabled(ctx, r.client, r.scheme) {
		log.Debug("monitoring is available in cluster, reconciling monitoring")

		log.Debug("reconciling ServiceMonitor definition")

		serviceMonitor := monitoring.MonitorForService(ctrlService)

		serviceMonitor.Spec.Endpoints[0].Path = "/metrics"

		if !controllerResource.Spec.SslConfig.IsPlain() {
			serviceMonitor.Spec.Endpoints[0].Scheme = "https"
			serviceMonitor.Spec.Endpoints[0].TLSConfig = &monitoringv1.TLSConfig{
				SafeTLSConfig: monitoringv1.SafeTLSConfig{
					ServerName: fmt.Sprintf("%s.%s.svc", controllerResource.Name, controllerResource.Namespace),
					KeySecret: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: controllerResource.Spec.LinstorHttpsClientSecret},
						Key:                  lc.SecretKeyName,
					},
					CA: monitoringv1.SecretOrConfigMap{
						Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: controllerResource.Spec.LinstorHttpsClientSecret},
							Key:                  lc.SecretCARootName,
						},
					},
					Cert: monitoringv1.SecretOrConfigMap{
						Secret: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: controllerResource.Spec.LinstorHttpsClientSecret},
							Key:                  lc.SecretCertName,
						},
					},
				},
			}
		}

		if CreateMonitoring {
			serviceMonitorChanged, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, serviceMonitor, controllerResource, reconcileutil.OnPatchErrorRecreate)
			if err != nil {
				return fmt.Errorf("failed to reconcile servicemonitor definition: %w", err)
			}

			log.WithField("changed", serviceMonitorChanged).Debug("reconciling monitoring service definition: done")
		} else {
			err = reconcileutil.DeleteIfOwned(ctx, r.client, &monitoringv1.ServiceMonitor{ObjectMeta: getObjectMeta(controllerResource, "%s")}, controllerResource)
			if err != nil {
				return fmt.Errorf("failed to delete monitoring servicemonitor: %w", err)
			}
		}
	}

	log.Debug("reconcile LINSTOR")

	return r.reconcileControllers(ctx, controllerResource)
}

func (r *ReconcileLinstorController) reconcileControllers(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	log := log.WithFields(logrus.Fields{
		"name":      controllerResource.Name,
		"namespace": controllerResource.Namespace,
		"spec":      fmt.Sprintf("%+v", controllerResource.Spec),
	})
	log.Info("controller Reconcile: reconciling controller Nodes")

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		expectedEndpoint(controllerResource),
		&controllerResource.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, controllerResource.Namespace),
	)
	if err != nil {
		return err
	}

	log.Debug("wait for controller service to come online")

	err = r.controllerReachable(ctx, linstorClient)
	if err != nil {
		return &reconcileutil.TemporaryError{
			Source:       fmt.Errorf("failed to contact controller: %w", err),
			RequeueAfter: connectionRetrySeconds * time.Second,
		}
	}

	log.Debug("ensuring additional properties are set")

	allProperties, err := linstorClient.Controller.GetProps(ctx)
	if err != nil {
		return fmt.Errorf("could not fetch existing properties: %w", err)
	}

	modify := lapi.GenericPropsModify{OverrideProps: make(lapi.OverrideProps)}

	for k, v := range controllerResource.Spec.AdditionalProperties {
		existing, ok := allProperties[k]
		if !ok || existing != v {
			modify.OverrideProps[k] = v
		}
	}

	err = linstorClient.Controller.Modify(ctx, modify)
	if err != nil {
		return fmt.Errorf("could not reconcile additional properties: %w", err)
	}

	log.Debug("find existing controller nodes")
	allNodes, err := linstorClient.Nodes.GetAll(ctx)
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

	meta := getObjectMeta(controllerResource, "%s-controller")
	ourPods := &corev1.PodList{}
	err = r.client.List(ctx, ourPods, client.InNamespace(controllerResource.Namespace), client.MatchingLabels(meta.Labels))
	if err != nil {
		return err
	}

	log.Debug("register controller pods in LINSTOR")

	for _, pod := range ourPods.Items {
		log.WithField("pod", pod.Name).Debug("register controller pod")
		_, err := linstorClient.GetNodeOrCreate(ctx, lapi.Node{
			Name: pod.Name,
			Type: lc.Controller,
			NetInterfaces: []lapi.NetInterface{
				{
					Name:                    "default",
					Address:                 pod.Status.PodIP,
					IsActive:                true,
					SatellitePort:           controllerResource.Spec.SslConfig.Port(),
					SatelliteEncryptionType: controllerResource.Spec.SslConfig.Type(),
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
			err = linstorClient.Nodes.Delete(ctx, linstorController.Name)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileLinstorController) reconcileStatus(ctx context.Context, controllerResource *piraeusv1.LinstorController, resErr error) error {
	log := log.WithFields(logrus.Fields{
		"Name":      controllerResource.Name,
		"Namespace": controllerResource.Namespace,
	})
	log.Info("reconcile status")

	linstorStatusErr := r.reconcileLinstorStatus(ctx, controllerResource)

	controllerResource.Status.Errors = reconcileutil.ErrorStrings(resErr, linstorStatusErr)

	log.Debug("update status in resource")

	// Status update should always happen, even if the actual update context is canceled
	updateCtx, updateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer updateCancel()

	return r.client.Status().Update(updateCtx, controllerResource)
}

func (r *ReconcileLinstorController) reconcileLinstorStatus(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	log := log.WithFields(logrus.Fields{
		"Name":      controllerResource.Name,
		"Namespace": controllerResource.Namespace,
		"Op":        "reconcileLinstorStatus",
	})

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		expectedEndpoint(controllerResource),
		&controllerResource.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, controllerResource.Namespace),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = r.controllerReachable(ctx, linstorClient)
	if err != nil {
		log.Debug("controller not reachable, status checks will be skipped")
		cancel()
	}

	log.Debug("find active controller pod")

	controllerName, err := r.findActiveControllerPodName(ctx, linstorClient)
	if err != nil {
		log.Warnf("failed to find active controller pod: %v", err)
	}

	controllerResource.Status.ControllerStatus = &shared.NodeStatus{
		NodeName:               controllerName,
		RegisteredOnController: false,
	}

	log.Debug("check if controller pod is registered")

	allNodes, err := linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		log.Warnf("failed to fetch list of LINSTOR nodes: %v", err)
	}

	for _, node := range allNodes {
		if node.Name == controllerName {
			controllerResource.Status.ControllerStatus.RegisteredOnController = true
		}
	}

	log.Debug("fetch all properties set on controller")

	allProps, err := linstorClient.Controller.GetProps(ctx)
	if err != nil {
		log.Warnf("failed to fetch list of properties from controller: %v", err)
	}

	if allProps == nil {
		// No nil values in the status
		allProps = make(map[string]string)
	}

	controllerResource.Status.ControllerProperties = allProps

	log.Debug("fetch information about storage nodes")

	nodes, err := linstorClient.GetAllStorageNodes(ctx)
	if err != nil {
		log.Warnf("unable to get LINSTOR storage nodes: %v, continue with empty node list", err)
		nodes = nil
	}

	controllerResource.Status.SatelliteStatuses = make([]*shared.SatelliteStatus, len(nodes))

	for i := range nodes {
		node := &nodes[i]

		controllerResource.Status.SatelliteStatuses[i] = &shared.SatelliteStatus{
			NodeStatus: shared.NodeStatus{
				NodeName:               node.Name,
				RegisteredOnController: true,
			},
			ConnectionStatus:    node.ConnectionStatus,
			StoragePoolStatuses: make([]*shared.StoragePoolStatus, len(node.StoragePools)),
		}

		for j := range node.StoragePools {
			pool := &node.StoragePools[j]

			controllerResource.Status.SatelliteStatuses[i].StoragePoolStatuses[j] = shared.NewStoragePoolStatus(pool)
		}
	}

	return nil
}

func (r *ReconcileLinstorController) findActiveControllerPodName(ctx context.Context, linstorClient *lc.HighLevelClient) (string, error) {
	allNodes, err := linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch nodes from linstor: %w", err)
	}

	var onlineControllers []*lapi.Node

	for i := range allNodes {
		node := &allNodes[i]

		registrar, ok := node.Props[kubeSpec.LinstorRegistrationProperty]
		if ok && registrar == kubeSpec.Name && node.Type == lc.Controller && node.ConnectionStatus == lc.Online {
			onlineControllers = append(onlineControllers, node)
		}
	}

	if len(onlineControllers) != 1 {
		return "", fmt.Errorf("expected one online controller, instead got: %v", onlineControllers)
	}

	return onlineControllers[0].Name, nil
}

// finalizeControllerSet returns whether it is finished as well as potentially an error
func (r *ReconcileLinstorController) finalizeControllerSet(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	log := log.WithFields(logrus.Fields{
		"name":      controllerResource.Name,
		"namespace": controllerResource.Namespace,
		"spec":      fmt.Sprintf("%+v", controllerResource.Spec),
	})
	log.Info("controller finalizeControllerSet: found LinstorController marked for deletion, finalizing...")

	if !mdutil.HasFinalizer(controllerResource, linstorControllerFinalizer) {
		return nil
	}

	linstorClient, err := lc.NewHighLevelLinstorClientFromConfig(
		expectedEndpoint(controllerResource),
		&controllerResource.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, controllerResource.Namespace),
	)
	if err != nil {
		return err
	}

	nodesOnControllerErr := r.ensureNoNodesOnController(ctx, controllerResource, linstorClient)

	statusErr := r.reconcileStatus(ctx, controllerResource, nodesOnControllerErr)
	if statusErr != nil {
		log.Warnf("failed to update status. original error: %v", nodesOnControllerErr)
		return statusErr
	}

	if nodesOnControllerErr != nil {
		return nodesOnControllerErr
	}

	log.Info("controller finalizing finished, removing finalizer")

	return r.deleteFinalizer(ctx, controllerResource)
}

// returns an error if nodes are still registered.
func (r *ReconcileLinstorController) ensureNoNodesOnController(ctx context.Context, controllerResource *piraeusv1.LinstorController, linstorClient *lc.HighLevelClient) error {
	log := log.WithFields(logrus.Fields{
		"name":      controllerResource.Name,
		"namespace": controllerResource.Namespace,
		"spec":      fmt.Sprintf("%+v", controllerResource.Spec),
	})
	if controllerResource.Status.ControllerStatus.NodeName == "" {
		log.Info("controller never deployed; finalization OK")
		return nil
	}

	nodes, err := linstorClient.Nodes.GetAll(ctx)
	if err != nil {
		if err != lapi.NotFoundError {
			return fmt.Errorf("controller unable to get cluster nodes: %v", err)
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
			Source:       fmt.Errorf("controller controller still has active satellites which must be cleared before deletion: %v", nodeNames),
			RequeueAfter: 1 * time.Minute,
		}
	}

	return nil
}

func (r *ReconcileLinstorController) addFinalizer(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	mdutil.AddFinalizer(controllerResource, linstorControllerFinalizer)

	err := r.client.Update(ctx, controllerResource)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileLinstorController) deleteFinalizer(ctx context.Context, controllerResource *piraeusv1.LinstorController) error {
	mdutil.DeleteFinalizer(controllerResource, linstorControllerFinalizer)

	err := r.client.Update(ctx, controllerResource)
	if err != nil {
		return err
	}
	return nil
}

// Check if the controller is currently reachable.
func (r *ReconcileLinstorController) controllerReachable(ctx context.Context, linstorClient *lc.HighLevelClient) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := linstorClient.Controller.GetVersion(ctx)

	return err
}

func newDeploymentForResource(controllerResource *piraeusv1.LinstorController) *appsv1.Deployment {
	var pullSecrets []corev1.LocalObjectReference
	if controllerResource.Spec.DrbdRepoCred != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: controllerResource.Spec.DrbdRepoCred})
	}

	healthzPort := 9999
	port := lc.DefaultHTTPPort

	if controllerResource.Spec.LinstorHttpsControllerSecret != "" {
		port = lc.DefaultHTTPSPort
	}

	servicePorts := []corev1.EndpointPort{
		{Name: controllerResource.Name, Port: int32(port)},
	}

	servicePortsJSON, err := json.Marshal(servicePorts)
	if err != nil {
		panic(err)
	}

	env := []corev1.EnvVar{
		{
			Name: kubeSpec.JavaOptsName,
			// Workaround for https://github.com/LINBIT/linstor-server/issues/123
			Value: "-Djdk.tls.acknowledgeCloseNotify=true",
		},
		{
			Name:  awaitelection.AwaitElectionEnabledKey,
			Value: "1",
		},
		{
			Name:  awaitelection.AwaitElectionNameKey,
			Value: "linstor-controller",
		},
		{
			Name:  awaitelection.AwaitElectionLockNameKey,
			Value: controllerResource.Name,
		},
		{
			Name:  awaitelection.AwaitElectionLockNamespaceKey,
			Value: controllerResource.Namespace,
		},
		{
			Name: awaitelection.AwaitElectionIdentityKey,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: awaitelection.AwaitElectionPodIP,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name: awaitelection.AwaitElectionNodeName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name:  awaitelection.AwaitElectionServiceName,
			Value: controllerResource.Name,
		},
		{
			Name:  awaitelection.AwaitElectionServiceNamespace,
			Value: controllerResource.Namespace,
		},
		{
			Name:  awaitelection.AwaitElectionServicePortsJson,
			Value: string(servicePortsJSON),
		},
		{
			Name:  awaitelection.AwaitElectionStatusEndpointKey,
			Value: fmt.Sprintf(":%d", healthzPort),
		},
	}
	env = append(env, controllerResource.Spec.AdditionalEnv...)

	volumes := []corev1.Volume{
		{
			Name: kubeSpec.LinstorConfDirName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: controllerResource.Name + "-controller-config",
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

	if controllerResource.Spec.LuksSecret != "" {
		env = append(env, corev1.EnvVar{
			Name: kubeSpec.LinstorLUKSPassphraseEnvName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: controllerResource.Spec.LuksSecret,
					},
					Key: kubeSpec.LinstorLUKSPassphraseEnvName,
				},
			},
		})
	}

	if controllerResource.Spec.DBCertSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorCertDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: controllerResource.Spec.DBCertSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorCertDirName,
			MountPath: kubeSpec.LinstorCertDir,
			ReadOnly:  true,
		})
	}

	if controllerResource.Spec.LinstorHttpsControllerSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorHttpsCertDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorHttpsCertPemDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: controllerResource.Spec.LinstorHttpsControllerSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorHttpsCertDirName,
			MountPath: kubeSpec.LinstorHttpsCertDir,
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorHttpsCertPemDirName,
			MountPath: kubeSpec.LinstorHttpsCertPemDir,
			ReadOnly:  true,
		})
	}

	if controllerResource.Spec.LinstorHttpsClientSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorClientDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: controllerResource.Spec.LinstorHttpsClientSecret,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorClientDirName,
			MountPath: kubeSpec.LinstorClientDir,
		})
	}

	if !controllerResource.Spec.SslConfig.IsPlain() {
		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorSslDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})

		volumes = append(volumes, corev1.Volume{
			Name: kubeSpec.LinstorSslPemDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: string(*controllerResource.Spec.SslConfig),
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorSslDirName,
			MountPath: kubeSpec.LinstorSslDir,
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      kubeSpec.LinstorSslPemDirName,
			MountPath: kubeSpec.LinstorSslPemDir,
			ReadOnly:  true,
		})
	}

	// This probe should be able to deal with "new" images which start a leader election process,
	// as well as images without leader election helper
	livenessProbe := corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/",
				Port: intstr.FromInt(healthzPort),
			},
		},
	}

	meta := getObjectMeta(controllerResource, "%s-controller")

	return &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: meta.Labels},
			Replicas: controllerResource.Spec.Replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: meta,
				Spec: corev1.PodSpec{
					ServiceAccountName: getServiceAccountName(controllerResource),
					PriorityClassName:  controllerResource.Spec.PriorityClassName.GetName(controllerResource.Namespace),
					Containers: []corev1.Container{
						{
							Name:            "linstor-controller",
							Image:           controllerResource.Spec.ControllerImage,
							Args:            []string{"startController"}, // Run linstor-controller.
							ImagePullPolicy: controllerResource.Spec.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 3376,
									Protocol:      "TCP",
								},
								{
									ContainerPort: 3377,
									Protocol:      "TCP",
								},
								{
									ContainerPort: lc.DefaultHTTPPort,
									Protocol:      "TCP",
								},
								{
									ContainerPort: lc.DefaultHTTPSPort,
									Protocol:      "TCP",
								},
							},
							VolumeMounts:  volumeMounts,
							Env:           env,
							LivenessProbe: &livenessProbe,
							Resources:     controllerResource.Spec.Resources,
						},
					},
					Volumes:          volumes,
					ImagePullSecrets: pullSecrets,
					Affinity:         getDeploymentAffinity(controllerResource),
					Tolerations:      controllerResource.Spec.Tolerations,
					SecurityContext:  getSecurityContext(),
				},
			},
		},
	}
}

func getSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{SupplementalGroups: []int64{kubeSpec.LinstorControllerGID}}
}

func getDeploymentAffinity(controllerResource *piraeusv1.LinstorController) *corev1.Affinity {
	meta := getObjectMeta(controllerResource, "%s-controller")
	if controllerResource.Spec.Affinity == nil {
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{MatchLabels: meta.Labels},
						TopologyKey:   kubeSpec.DefaultTopologyKey,
					},
				},
			},
		}
	}

	return controllerResource.Spec.Affinity
}

func newServiceForResource(controllerResource *piraeusv1.LinstorController) *corev1.Service {
	port := lc.DefaultHTTPPort
	if controllerResource.Spec.LinstorHttpsControllerSecret != "" {
		port = lc.DefaultHTTPSPort
	}

	return &corev1.Service{
		ObjectMeta: getObjectMeta(controllerResource, "%s"),
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
			Ports: []corev1.ServicePort{
				{
					Name:       controllerResource.Name,
					Port:       int32(port),
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(port),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func NewConfigMapForResource(controllerResource *piraeusv1.LinstorController) (*corev1.ConfigMap, error) {
	dbCertificatePath := ""
	dbClientCertPath := ""
	dbClientKeyPath := ""
	if controllerResource.Spec.DBCertSecret != "" {
		dbCertificatePath = kubeSpec.LinstorCertDir + "/ca.crt"
		if controllerResource.Spec.DBUseClientCert {
			dbClientCertPath = kubeSpec.LinstorCertDir + "/tls.crt"
			dbClientKeyPath = kubeSpec.LinstorCertDir + "/tls.key"
		}
	}

	var http *linstortoml.ControllerHttp
	var https *linstortoml.ControllerHttps
	if controllerResource.Spec.HttpBindAddress != "" {
		http = &linstortoml.ControllerHttp{
			ListenAddr: controllerResource.Spec.HttpBindAddress,
		}
	}
	if controllerResource.Spec.LinstorHttpsControllerSecret != "" {
		yes := true

		https = &linstortoml.ControllerHttps{
			Enabled:            &yes,
			Keystore:           kubeSpec.LinstorHttpsCertDir + "/keystore.jks",
			KeystorePassword:   kubeSpec.LinstorHttpsCertPassword,
			Truststore:         kubeSpec.LinstorHttpsCertDir + "/truststore.jks",
			TruststorePassword: kubeSpec.LinstorHttpsCertPassword,
		}
		if controllerResource.Spec.HttpsBindAddress != "" {
			https.ListenAddr = controllerResource.Spec.HttpsBindAddress
		}
	}

	linstorControllerConfig := linstortoml.Controller{
		Db: &linstortoml.ControllerDb{
			ConnectionUrl:     controllerResource.Spec.DBConnectionURL,
			CaCertificate:     dbCertificatePath,
			ClientCertificate: dbClientCertPath,
			ClientKeyPkcs8Pem: dbClientKeyPath,
		},
		Http:  http,
		Https: https,
		Logging: &linstortoml.ControllerLogging{
			LinstorLevel: controllerResource.Spec.LogLevel.ToLinstor(),
		},
	}

	controllerConfigBuilder := strings.Builder{}
	if err := toml.NewEncoder(&controllerConfigBuilder).Encode(linstorControllerConfig); err != nil {
		return nil, err
	}

	endpoint := expectedEndpoint(controllerResource)
	clientConfig := lc.NewClientConfigForAPIResource(endpoint, &controllerResource.Spec.LinstorClientConfig)
	clientConfigFile, err := clientConfig.ToConfigFile()
	if err != nil {
		return nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: getObjectMeta(controllerResource, "%s-controller-config"),
		Data: map[string]string{
			kubeSpec.LinstorControllerConfigFile: controllerConfigBuilder.String(),
			kubeSpec.LinstorClientConfigFile:     clientConfigFile,
		},
	}

	return cm, nil
}

func getServiceAccountName(lc *piraeusv1.LinstorController) string {
	if lc.Spec.ServiceAccountName == "" {
		return kubeSpec.LinstorControllerServiceAccount
	}

	return lc.Spec.ServiceAccountName
}

func expectedEndpoint(controllerResource *piraeusv1.LinstorController) string {
	serviceName := types.NamespacedName{Name: controllerResource.Name, Namespace: controllerResource.Namespace}
	useHTTPS := controllerResource.Spec.LinstorHttpsClientSecret != ""

	return lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
}

func getObjectMeta(controllerResource *piraeusv1.LinstorController, nameFmt string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      fmt.Sprintf(nameFmt, controllerResource.Name),
		Namespace: controllerResource.Namespace,
		Labels: map[string]string{
			"app.kubernetes.io/name":       kubeSpec.ControllerRole,
			"app.kubernetes.io/instance":   controllerResource.Name,
			"app.kubernetes.io/managed-by": kubeSpec.Name,
		},
	}
}
