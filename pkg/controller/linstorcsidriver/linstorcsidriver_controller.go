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

package linstorcsidriver

import (
	"context"
	"os"
	"time"

	linstorClient "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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

// Add creates a new LinstorCSIDriver Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorCSIDriver{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("linstorcsidriver-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorCSIDriver
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.LinstorCSIDriver{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	createdResources := []runtime.Object{
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&storagev1beta1.CSIDriver{},
	}

	for _, createdResource := range createdResources {
		err = c.Watch(&source.Kind{Type: createdResource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &piraeusv1alpha1.LinstorCSIDriver{},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorCSIDriver implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorCSIDriver{}

// ReconcileLinstorCSIDriver reconciles a LinstorCSIDriver object
type ReconcileLinstorCSIDriver struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a LinstorCSIDriver object and makes changes based on the state read
// and what is in the LinstorCSIDriver.Spec
func (r *ReconcileLinstorCSIDriver) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logrus.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
	})
	reqLogger.Info("Reconciling LinstorCSIDriver")

	// Fetch the LinstorCSIDriver instance
	csiResource := &piraeusv1alpha1.LinstorCSIDriver{}
	err := r.client.Get(context.TODO(), request.NamespacedName, csiResource)
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

	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)

	resourceErr := r.reconcileResource(ctx, csiResource)
	if resourceErr != nil {
		return reconcile.Result{}, resourceErr
	}

	specErr := r.reconcileSpec(ctx, csiResource)

	statusErr := r.reconcileStatus(ctx, csiResource, specErr)

	if specErr != nil {
		return reconcile.Result{}, specErr
	}
	return reconcile.Result{}, statusErr
}

func (r *ReconcileLinstorCSIDriver) reconcileResource(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileResource",
	})
	logger.Debug("performing upgrades and fill defaults in resource")

	changed := false

	logger.Debug("performing upgrade/fill: #1 -> Set default image names for CSI")

	if csiResource.Spec.CSIAttacherImage == "" {
		csiResource.Spec.CSIAttacherImage = DefaultAttacherImage
		changed = true

		logger.Infof("set csi attacher image to '%s'", csiResource.Spec.CSIAttacherImage)
	}

	if csiResource.Spec.CSINodeDriverRegistrarImage == "" {
		csiResource.Spec.CSINodeDriverRegistrarImage = DefaultNodeDriverRegistrarImage
		changed = true

		logger.Infof("set csi node driver registrar image to '%s'", csiResource.Spec.CSINodeDriverRegistrarImage)
	}

	if csiResource.Spec.CSIProvisionerImage == "" {
		csiResource.Spec.CSIProvisionerImage = DefaultProvisionerImage
		changed = true

		logger.Infof("set csi provisioner image to '%s'", csiResource.Spec.CSIProvisionerImage)
	}

	if csiResource.Spec.CSISnapshotterImage == "" {
		csiResource.Spec.CSISnapshotterImage = DefaultSnapshotterImage
		changed = true

		logger.Infof("set csi snapshotter image to '%s'", csiResource.Spec.CSISnapshotterImage)
	}

	logger.Debugf("finished upgrade/fill: #1 -> Set default image names for CSI: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #2 -> Set default endpoint URL for client")

	if csiResource.Spec.ControllerEndpoint == "" {
		serviceName := types.NamespacedName{Name: csiResource.Name + "-cs", Namespace: csiResource.Namespace}
		useHTTPS := csiResource.Spec.LinstorClientConfig.LinstorHttpsClientSecret != ""
		defaultEndpoint := linstorClient.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
		csiResource.Spec.ControllerEndpoint = defaultEndpoint
		changed = true

		logger.Infof("set controller endpoint URL to '%s'", csiResource.Spec.ControllerEndpoint)
	}

	logger.Debugf("finished upgrade/fill: #2 -> Set default endpoint URL for client: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #3 -> Set service account names to previous implicit values")

	if csiResource.Spec.CSINodeServiceAccountName == "" {
		csiResource.Spec.CSINodeServiceAccountName = csiResource.Name + NodeServiceAccount
		changed = true

		logger.Infof("set csi node service account to '%s'", csiResource.Spec.CSINodeServiceAccountName)
	}

	if csiResource.Spec.CSIControllerServiceAccountName == "" {
		csiResource.Spec.CSIControllerServiceAccountName = csiResource.Name + ControllerServiceAccount
		changed = true

		logger.Infof("set csi controller service account to '%s'", csiResource.Spec.CSIControllerServiceAccountName)
	}

	logger.Debugf("finished upgrade/fill: #3 -> Set service account names to previous implicit values: changed=%t", changed)

	logger.Debug("finished all upgrades/fills")
	if changed {
		logger.Info("save updated spec")
		return r.client.Update(ctx, csiResource)
	}
	return nil
}

func (r *ReconcileLinstorCSIDriver) reconcileSpec(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	err := r.reconcileNodeDaemonSet(ctx, csiResource)
	if err != nil {
		return err
	}

	err = r.reconcileControllerDeployment(ctx, csiResource)
	if err != nil {
		return err
	}

	err = r.reconcileCSIDriver(ctx, csiResource)
	if err != nil {
		return err
	}

	return nil
}

func (r *ReconcileLinstorCSIDriver) reconcileStatus(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver, specError error) error {
	nodeReady := false
	controllerReady := false

	dsMeta := makeMeta(csiResource, NodeDaemonSet)
	ds := appsv1.DaemonSet{}
	err := r.client.Get(ctx, types.NamespacedName{Name: dsMeta.Name, Namespace: dsMeta.Namespace}, &ds)
	// We ignore these errors, they most likely mean the resource is not yet ready
	if err == nil {
		nodeReady = ds.Status.DesiredNumberScheduled == ds.Status.NumberReady
	}

	deployMeta := makeMeta(csiResource, ControllerDeployment)
	deploy := appsv1.Deployment{}
	err = r.client.Get(ctx, types.NamespacedName{Name: deployMeta.Name, Namespace: deployMeta.Namespace}, &deploy)
	// We ignore these errors, they most likely mean the resource is not yet ready
	if err == nil {
		controllerReady = deploy.Status.Replicas == deploy.Status.ReadyReplicas
	}

	if specError != nil {
		csiResource.Status.Errors = []string{specError.Error()}
	} else {
		csiResource.Status.Errors = []string{}
	}

	csiResource.Status.NodeReady = nodeReady
	csiResource.Status.ControllerReady = controllerReady

	err = r.client.Status().Update(ctx, csiResource)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"requestName":      csiResource.Name,
			"requestNamespace": csiResource.Namespace,
			"Op":               "reconcileStatus",
			"originalError":    specError,
			"updateError":      err,
		}).Error("Failed to update status")
	}

	return err
}

func (r *ReconcileLinstorCSIDriver) reconcileNodeDaemonSet(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileNodeDaemonSet",
	})
	logger.Debugf("creating csi node daemon set")
	nodeDaemonSet := newCSINodeDaemonSet(csiResource)
	return r.createOrReplaceWithOwner(ctx, nodeDaemonSet, csiResource)
}

func (r *ReconcileLinstorCSIDriver) reconcileControllerDeployment(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileControllerDeployment",
	})
	logger.Debugf("creating csi controller deployment")
	controllerDeployment := newCSIControllerDeployment(csiResource)
	return r.createOrReplaceWithOwner(ctx, controllerDeployment, csiResource)
}

func (r *ReconcileLinstorCSIDriver) reconcileCSIDriver(ctx context.Context, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileCSIDriver",
	})
	logger.Debugf("creating csi driver resource")
	csiDriver := newCSIDriver(csiResource)

	return r.createOrReplace(ctx, csiDriver)
}

var (
	ControllerReplicas            = int32(1)
	IsPrivileged                  = true
	MountPropagationBidirectional = corev1.MountPropagationBidirectional
	HostPathDirectoryOrCreate     = corev1.HostPathDirectoryOrCreate
	HostPathDirectory             = corev1.HostPathDirectory
)

func newCSINodeDaemonSet(csiResource *piraeusv1alpha1.LinstorCSIDriver) *appsv1.DaemonSet {
	registrationDir := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/kubelet/plugins_registry/",
				Type: &HostPathDirectoryOrCreate,
			},
		},
	}
	pluginDir := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/kubelet/plugins/linstor.csi.linbit.com/",
				Type: &HostPathDirectoryOrCreate,
			},
		},
	}
	podsMountDir := corev1.Volume{
		Name: "pods-mount-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/lib/kubelet",
				Type: &HostPathDirectory,
			},
		},
	}
	deviceDir := corev1.Volume{
		Name: "device-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/dev",
			},
		},
	}

	csiEndpoint := corev1.EnvVar{
		Name:  "CSI_ENDPOINT",
		Value: "/csi/csi.sock",
	}
	driverSocket := corev1.EnvVar{
		Name:  "DRIVER_REG_SOCK_PATH",
		Value: "/var/lib/kubelet/plugins/linstor.csi.linbit.com/csi.sock",
	}
	kubeNodeName := corev1.EnvVar{
		Name: "KUBE_NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
		},
	}

	env := []corev1.EnvVar{
		csiEndpoint,
		driverSocket,
		kubeNodeName,
	}

	env = append(env, linstorClient.APIResourceAsEnvVars(csiResource.Spec.ControllerEndpoint, &csiResource.Spec.LinstorClientConfig)...)

	driverRegistrar := corev1.Container{
		Name:  "csi-node-driver-registrar",
		Image: csiResource.Spec.CSINodeDriverRegistrarImage,
		Args:  []string{"--v=5", "--csi-address=$(CSI_ENDPOINT)", "--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)"},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.Handler{
				Exec: &corev1.ExecAction{Command: []string{"/bin/sh", "-c", "rm -rf /registration/linstor.csi.linbit.com /registration/linstor.csi.linbit.com-reg.sock"}},
			},
		},
		Env: env,
		SecurityContext: &corev1.SecurityContext{
			Privileged:               &IsPrivileged,
			Capabilities:             &corev1.Capabilities{Add: []corev1.Capability{"SYS_ADMIN"}},
			AllowPrivilegeEscalation: &IsPrivileged,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pluginDir.Name,
				MountPath: "/csi/",
			},
			{
				Name:      registrationDir.Name,
				MountPath: "/registration/",
			},
		},
	}

	linstorPluginContainer := corev1.Container{
		Name:            "csi-node-driver-linstor-plugin",
		Image:           csiResource.Spec.LinstorPluginImage,
		ImagePullPolicy: "Always",
		Args:            []string{"--csi-endpoint=unix://$(CSI_ENDPOINT)", "--node=$(KUBE_NODE_NAME)", "--linstor-endpoint=$(LS_CONTROLLERS)", "--log-level=debug"},
		Env:             env,
		SecurityContext: &corev1.SecurityContext{
			Privileged:               &IsPrivileged,
			Capabilities:             &corev1.Capabilities{Add: []corev1.Capability{"SYS_ADMIN"}},
			AllowPrivilegeEscalation: &IsPrivileged,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pluginDir.Name,
				MountPath: "/csi/",
			},
			{
				Name:             podsMountDir.Name,
				MountPath:        "/var/lib/kubelet/",
				MountPropagation: &MountPropagationBidirectional,
			},
			{
				Name:      deviceDir.Name,
				MountPath: "/dev",
			},
		},
	}

	return &appsv1.DaemonSet{
		ObjectMeta: makeMeta(csiResource, NodeDaemonSet),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(csiResource),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: makeMeta(csiResource, NodeDaemonSet),
				Spec: corev1.PodSpec{
					PriorityClassName:  csiResource.Spec.PriorityClassName.GetName(csiResource.Namespace),
					ServiceAccountName: csiResource.Spec.CSINodeServiceAccountName,
					Containers: []corev1.Container{
						driverRegistrar,
						linstorPluginContainer,
					},
					Volumes: []corev1.Volume{
						deviceDir,
						pluginDir,
						podsMountDir,
						registrationDir,
					},
					HostNetwork: true,
					DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: csiResource.Spec.ImagePullSecret,
					}},
				},
			},
		},
	}
}

func newCSIControllerDeployment(csiResource *piraeusv1alpha1.LinstorCSIDriver) *appsv1.Deployment {
	socketAddress := corev1.EnvVar{
		Name:  "ADDRESS",
		Value: "/var/lib/csi/sockets/pluginproxy/csi.sock",
	}

	kubeNodeName := corev1.EnvVar{
		Name:      "KUBE_NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}},
	}

	socketVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	linstorEnvVars := linstorClient.APIResourceAsEnvVars(csiResource.Spec.ControllerEndpoint, &csiResource.Spec.LinstorClientConfig)

	csiProvisioner := corev1.Container{
		Name:  "csi-provisioner",
		Image: csiResource.Spec.CSIProvisionerImage,
		Args: []string{
			"--provisioner=linstor.csi.linbit.com",
			"--csi-address=$(ADDRESS)",
			"--v=5",
			"--feature-gates=Topology=false",
			"--connection-timeout=4m",
		},
		Env: []corev1.EnvVar{socketAddress},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: "/var/lib/csi/sockets/pluginproxy/",
		}},
	}
	csiAttacher := corev1.Container{
		Name:  "csi-attacher",
		Image: csiResource.Spec.CSIAttacherImage,
		Args: []string{
			"--v=5",
			"--csi-address=$(ADDRESS)",
			"--timeout=4m",
		},
		Env: []corev1.EnvVar{socketAddress},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: "/var/lib/csi/sockets/pluginproxy/",
		}},
	}
	csiSnapshotter := corev1.Container{
		Name:  "csi-snapshotter",
		Image: csiResource.Spec.CSISnapshotterImage,
		Args: []string{
			"-timeout=4m",
			"-csi-address=$(ADDRESS)",
		},
		Env: []corev1.EnvVar{socketAddress},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: "/var/lib/csi/sockets/pluginproxy/",
		}},
	}
	linstorPlugin := corev1.Container{
		Name:  "linstor-csi-plugin",
		Image: csiResource.Spec.LinstorPluginImage,
		Args: []string{
			"--csi-endpoint=unix://$(ADDRESS)",
			"--node=$(KUBE_NODE_NAME)",
			"--linstor-endpoint=$(LS_CONTROLLERS)",
			"--log-level=debug",
		},
		Env: append(
			[]corev1.EnvVar{
				socketAddress,
				kubeNodeName,
			},
			linstorEnvVars...,
		),
		ImagePullPolicy: "Always",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: "/var/lib/csi/sockets/pluginproxy/",
		}},
	}

	return &appsv1.Deployment{
		ObjectMeta: makeMeta(csiResource, ControllerDeployment),
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(csiResource),
			},
			Replicas: &ControllerReplicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: makeMeta(csiResource, ControllerDeployment),
				Spec: corev1.PodSpec{
					PriorityClassName:  csiResource.Spec.PriorityClassName.GetName(csiResource.Namespace),
					ServiceAccountName: csiResource.Spec.CSIControllerServiceAccountName,
					Containers: []corev1.Container{
						csiAttacher,
						csiProvisioner,
						csiSnapshotter,
						linstorPlugin,
					},
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: csiResource.Spec.ImagePullSecret,
					}},
					Volumes: []corev1.Volume{socketVolume},
				},
			},
		},
	}
}

func newCSIDriver(csiResource *piraeusv1alpha1.LinstorCSIDriver) *storagev1beta1.CSIDriver {
	// should be const, but required to be var so that we can take the address to get a *bool
	attachRequired := true
	podInfoOnMount := true

	return &storagev1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			// Name must match exactly the one reported by the CSI plugin
			Name:      "linstor.csi.linbit.com",
			Namespace: csiResource.Namespace,
		},
		Spec: storagev1beta1.CSIDriverSpec{
			AttachRequired: &attachRequired,
			PodInfoOnMount: &podInfoOnMount,
		},
	}
}

func makeMeta(csiResource *piraeusv1alpha1.LinstorCSIDriver, namePostfix string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      csiResource.Name + namePostfix,
		Namespace: csiResource.Namespace,
		Labels:    defaultLabels(csiResource),
	}
}

func defaultLabels(csiResource *piraeusv1alpha1.LinstorCSIDriver) map[string]string {
	return map[string]string{
		"app": csiResource.Name,
	}
}

// Create a resource at the cluster scope.
//
// cluster scoped resource are not allowed to have owner references, so these objects will not be cleaned up
// automatically.
func (r *ReconcileLinstorCSIDriver) createOrReplace(ctx context.Context, obj runtime.Object) error {
	err := r.client.Create(ctx, obj)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return err
	}

	// TODO: support update operation.
	// Updates automatically trigger reconciliation, which means we get an endless loop of .Reconcile() calls. To
	// support this properly we would need to check for spec equality in some way.
	return nil
}

// Create a resource at current owning resource scope.
//
// Once the owning resource is cleaned up, the created items will be removed as well.
func (r *ReconcileLinstorCSIDriver) createOrReplaceWithOwner(ctx context.Context, obj GCRuntimeObject, csiResource *piraeusv1alpha1.LinstorCSIDriver) error {
	err := controllerutil.SetControllerReference(csiResource, obj, r.scheme)
	// If it is already owned, we don't treat the SetControllerReference() call as a failure condition
	if err != nil {
		_, isAlreadyOwned := err.(*controllerutil.AlreadyOwnedError)
		if !isAlreadyOwned {
			return err
		}
	}

	return r.createOrReplace(ctx, obj)
}

func (r *ReconcileLinstorCSIDriver) deleteIfExists(ctx context.Context, obj GCRuntimeObject) error {
	err := r.client.Delete(ctx, obj)
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

type GCRuntimeObject interface {
	metav1.Object
	runtime.Object
}

const (
	NodeServiceAccount       = "-csi-node"
	ControllerServiceAccount = "-csi-controller"
	NodeDaemonSet            = "-csi-node"
	SnapshotterRole          = "-csi-snapshotter"
	ProvisionerRole          = "-csi-provisioner"
	DriverRegistrarRole      = "-csi-driver-registrar"
	AttacherRole             = "-csi-attacher"
	AttacherBinding          = "-csi-attacher"
	DriverRegistrarBinding   = "-csi-driver-registrar"
	ProvisionerBinding       = "-csi-provisioner"
	SnapshotterBinding       = "-csi-snapshotter"
	ControllerDeployment     = "-csi-controller"
)
