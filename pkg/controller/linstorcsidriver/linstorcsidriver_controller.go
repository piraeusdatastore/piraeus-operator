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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lapiconst "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	mdutil "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	lc "github.com/piraeusdatastore/piraeus-operator/pkg/linstor/client"
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
	err = c.Watch(&source.Kind{Type: &piraeusv1.LinstorCSIDriver{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	createdResources := []client.Object{
		&appsv1.Deployment{},
		&appsv1.DaemonSet{},
		&storagev1.CSIDriver{},
	}

	for _, createdResource := range createdResources {
		err = c.Watch(&source.Kind{Type: createdResource}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &piraeusv1.LinstorCSIDriver{},
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
func (r *ReconcileLinstorCSIDriver) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := logrus.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
	})
	reqLogger.Info("Reconciling LinstorCSIDriver")

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	// Fetch the LinstorCSIDriver instance
	csiResource := &piraeusv1.LinstorCSIDriver{}
	err := r.client.Get(ctx, request.NamespacedName, csiResource)
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

	reqLogger.Debug("reconcile spec with env")

	specs := []reconcileutil.EnvSpec{
		{Env: kubeSpec.ImageCSIPluginEnv, Target: &csiResource.Spec.LinstorPluginImage},
		{Env: kubeSpec.ImageCSIAttacherEnv, Target: &csiResource.Spec.CSIAttacherImage},
		{Env: kubeSpec.ImageCSILivenessProbeEnv, Target: &csiResource.Spec.CSILivenessProbeImage},
		{Env: kubeSpec.ImageCSINodeRegistrarEnv, Target: &csiResource.Spec.CSINodeDriverRegistrarImage},
		{Env: kubeSpec.ImageCSIProvisionerEnv, Target: &csiResource.Spec.CSIProvisionerImage},
		{Env: kubeSpec.ImageCSIResizerEnv, Target: &csiResource.Spec.CSIResizerImage},
		{Env: kubeSpec.ImageCSISnapshotterEnv, Target: &csiResource.Spec.CSISnapshotterImage},
	}

	err = reconcileutil.UpdateFromEnv(ctx, r.client, csiResource, specs...)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Debug("reconcile spec with resources")

	resourceErr := r.reconcileResource(ctx, csiResource)
	if resourceErr != nil {
		return reconcile.Result{}, resourceErr
	}

	specErr := r.reconcileSpec(ctx, csiResource)

	statusErr := r.reconcileStatus(ctx, csiResource, specErr)

	if specErr != nil {
		return reconcile.Result{}, specErr
	}

	return reconcile.Result{RequeueAfter: 1 * time.Minute}, statusErr
}

func (r *ReconcileLinstorCSIDriver) reconcileResource(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
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

	if csiResource.Spec.CSILivenessProbeImage == "" {
		csiResource.Spec.CSILivenessProbeImage = DefaultLivenessProbeImage
		changed = true

		logger.Infof("set csi liveness probe image to '%s'", csiResource.Spec.CSILivenessProbeImage)
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

	if csiResource.Spec.CSIResizerImage == "" {
		csiResource.Spec.CSIResizerImage = DefaultResizerImage
		changed = true

		logger.Infof("set csi resizer image to '%s'", csiResource.Spec.CSIResizerImage)
	}

	logger.Debugf("finished upgrade/fill: #1 -> Set default image names for CSI: changed=%t", changed)

	logger.Debug("performing upgrade/fill: #2 -> Set default endpoint URL for client")

	if csiResource.Spec.ControllerEndpoint == "" {
		serviceName := types.NamespacedName{Name: csiResource.Name + "-cs", Namespace: csiResource.Namespace}
		useHTTPS := csiResource.Spec.LinstorClientConfig.LinstorHttpsClientSecret != ""
		defaultEndpoint := lc.DefaultControllerServiceEndpoint(serviceName, useHTTPS)
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

	logger.Debugf("performing upgrade/fill: #4 -> Set kubelet path to default")

	if csiResource.Spec.KubeletPath == "" {
		csiResource.Spec.KubeletPath = DefaultKubeletPath
		changed = true

		logger.Infof("set kubelet path to '%s'", csiResource.Spec.KubeletPath)
	}

	logger.Debugf("finished upgrade/fill: #4 -> Set kubelet path to: changed=%t", changed)

	logger.Debug("finished all upgrades/fills")
	if changed {
		logger.Info("save updated spec")
		return r.client.Update(ctx, csiResource)
	}
	return nil
}

func (r *ReconcileLinstorCSIDriver) reconcileSpec(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
	err := r.reconcileNodes(ctx, csiResource)
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

func (r *ReconcileLinstorCSIDriver) reconcileStatus(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver, specError error) error {
	nodeReady := false
	controllerReady := false

	dsMeta := getObjectMeta(csiResource, NodeDaemonSet, kubeSpec.CSINodeRole)
	ds := appsv1.DaemonSet{}
	err := r.client.Get(ctx, types.NamespacedName{Name: dsMeta.Name, Namespace: dsMeta.Namespace}, &ds)
	// We ignore these errors, they most likely mean the resource is not yet ready
	if err == nil {
		nodeReady = ds.Status.DesiredNumberScheduled == ds.Status.NumberReady
	}

	deployMeta := getObjectMeta(csiResource, ControllerDeployment, kubeSpec.CSIControllerRole)
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

	// Status update should always happen, even if the actual update context is canceled
	updateCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = r.client.Status().Update(updateCtx, csiResource)
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

func (r *ReconcileLinstorCSIDriver) reconcileNodes(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileNodes",
	})
	logger.Debug("creating csi node daemon set")

	nodeDaemonSet := newCSINodeDaemonSet(csiResource)

	_, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, nodeDaemonSet, csiResource, reconcileutil.OnPatchErrorRecreate)
	if err != nil {
		return fmt.Errorf("failed to reconcile daemonset: %w", err)
	}

	logger.Debug("reconciling csi node objects")

	err = r.reconcileCSINodeObjects(ctx, csiResource)
	if err != nil {
		return fmt.Errorf("failed to reconcile CSI nodes: %w", err)
	}

	return nil
}

// reconcileCSINodeObjects ensures that existing CSINode object report the right topology keys.
//
// Topology keys are only queried once at start-up. LINSTOR's keys are updated periodically by the operator, and so
// the set of supported keys can change. The only reliable way to update them is restart the whole pod.
func (r *ReconcileLinstorCSIDriver) reconcileCSINodeObjects(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileCSINodeObjects",
	})
	logger.Debug("creating linstor client")

	lclient, err := lc.NewHighLevelLinstorClientFromConfig(
		csiResource.Spec.ControllerEndpoint,
		&csiResource.Spec.LinstorClientConfig,
		lc.NamedSecret(ctx, r.client, csiResource.Namespace),
	)
	if err != nil {
		return fmt.Errorf("failed to create linstor client: %w", err)
	}

	if !lclient.ControllerReachable(ctx) {
		logger.Debug("controller not online, nothing to reconcile")

		return nil
	}

	logger.Debug("fetching linstor nodes")

	lnodes, err := lclient.Nodes.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch lisntor nodes: %w", err)
	}

	nodePods := corev1.PodList{}
	meta := getObjectMeta(csiResource, NodeDaemonSet, kubeSpec.CSINodeRole)

	err = r.client.List(ctx, &nodePods, client.MatchingLabels(meta.Labels))
	if err != nil {
		return fmt.Errorf("failed to list csi node pods: %w", err)
	}

	csiNodes := storagev1.CSINodeList{}

	err = r.client.List(ctx, &csiNodes)
	if err != nil {
		return fmt.Errorf("failed to list csi node objects: %w", err)
	}

	for i := range nodePods.Items {
		pod := &nodePods.Items[i]

		err := r.reconcileCSINodeForPod(ctx, pod, lnodes, csiNodes.Items)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileLinstorCSIDriver) reconcileCSINodeForPod(ctx context.Context, pod *corev1.Pod, lnodes []lapi.Node, csiNodes []storagev1.CSINode) error {
	logger := logrus.WithField("pod", pod.Name)

	logger.Debug("searching matching linstor node")

	lnode := nodeByName(lnodes, pod.Spec.NodeName)
	if lnode == nil {
		logger.Debug("no linstor node found, skipping")

		return nil
	}

	logger.Debug("searching matching csi driver spec")

	csiDriver := csiDriverForNode(csiNodes, pod.Spec.NodeName)
	if csiDriver == nil {
		logger.Debug("no csi driver found, skipping")

		return nil
	}

	hasAllKeys := true

	for k := range lnode.Props {
		if !strings.HasPrefix(k, lapiconst.NamespcAuxiliary+"/") {
			// Only Aux/ properties are used for scheduling
			continue
		}

		expectedKey := k[len(lapiconst.NamespcAuxiliary+"/"):]

		if !mdutil.SliceContains(csiDriver.TopologyKeys, expectedKey) {
			logger.WithField("topologyKey", expectedKey).Debug("key missing in exported topology keys")

			hasAllKeys = false

			break
		}
	}

	if hasAllKeys {
		return nil
	}

	logger.Debug("not all labels are marked as exported, removing csi node")

	err := r.client.Patch(
		ctx,
		&storagev1.CSINode{ObjectMeta: metav1.ObjectMeta{Name: pod.Spec.NodeName}},
		client.RawPatch(types.StrategicMergePatchType, []byte(`{"spec":{"drivers":[{"name": "linstor.csi.linbit.com", "$patch": "delete"}]}}`)),
	)
	if err != nil {
		return fmt.Errorf("failed to remove outdated csi node object: %w", err)
	}

	logger.Debug("not all labels are marked as exported, removing pod to trigger recreation")

	err = r.client.Delete(ctx, pod)
	if err != nil {
		return fmt.Errorf("failed to remove oudated csi node pod: %w", err)
	}

	return nil
}

func nodeByName(nodes []lapi.Node, name string) *lapi.Node {
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}

	return nil
}

func csiDriverForNode(csiNodes []storagev1.CSINode, name string) *storagev1.CSINodeDriver {
	for i := range csiNodes {
		if csiNodes[i].Name != name {
			continue
		}

		for j := range csiNodes[i].Spec.Drivers {
			if csiNodes[i].Spec.Drivers[j].Name == "linstor.csi.linbit.com" {
				return &csiNodes[i].Spec.Drivers[j]
			}
		}
	}

	return nil
}

func (r *ReconcileLinstorCSIDriver) reconcileControllerDeployment(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileControllerDeployment",
	})
	logger.Debugf("creating csi controller deployment")
	controllerDeployment := newCSIControllerDeployment(csiResource)

	_, err := reconcileutil.CreateOrUpdateWithOwner(ctx, r.client, r.scheme, controllerDeployment, csiResource, reconcileutil.OnPatchErrorRecreate)

	return err
}

func (r *ReconcileLinstorCSIDriver) reconcileCSIDriver(ctx context.Context, csiResource *piraeusv1.LinstorCSIDriver) error {
	logger := logrus.WithFields(logrus.Fields{
		"Name":      csiResource.Name,
		"Namespace": csiResource.Namespace,
		"Op":        "reconcileCSIDriver",
	})
	logger.Debugf("creating csi driver resource")
	csiDriver := newCSIDriver(csiResource)

	_, err := reconcileutil.CreateOrUpdate(ctx, r.client, r.scheme, csiDriver, reconcileutil.OnPatchErrorRecreate)

	return err
}

var (
	IsPrivileged                  = true
	MountPropagationBidirectional = corev1.MountPropagationBidirectional
	HostPathDirectoryOrCreate     = corev1.HostPathDirectoryOrCreate
	HostPathDirectory             = corev1.HostPathDirectory
	DefaultHealthPort             = 9808
)

func newCSINodeDaemonSet(csiResource *piraeusv1.LinstorCSIDriver) *appsv1.DaemonSet {
	registrationDir := corev1.Volume{
		Name: "registration-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: kubeletPath(csiResource, "plugins_registry"),
				Type: &HostPathDirectoryOrCreate,
			},
		},
	}
	pluginDir := corev1.Volume{
		Name: "plugin-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: kubeletPath(csiResource, "plugins", "linstor.csi.linbit.com"),
				Type: &HostPathDirectoryOrCreate,
			},
		},
	}
	// Kubelet has different paths for the mount target, depending on the volume mode
	// FileSystem volumes have the target set to something like:
	//   /var/lib/kubelet/pods/<pod-uuid>/volumes/kubernetes.io~csi/<pv-name>/mount
	// Block volumes have the target set to something like:
	//   /var/lib/kubelet/plugins/kubernetes.io/csi/volumeDevices/publish/<pv-name>/<pod-uuid>
	// So we end up bind-mounting /var/lib/kubelet (or k8s distributions equivalent)
	publishDir := corev1.Volume{
		Name: "publish-dir",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: kubeletPath(csiResource),
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
		Value: kubeletPath(csiResource, "plugins", "linstor.csi.linbit.com", "csi.sock"),
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

	var pullSecrets []corev1.LocalObjectReference
	if csiResource.Spec.ImagePullSecret != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: csiResource.Spec.ImagePullSecret})
	}

	env = append(env, lc.APIResourceAsEnvVars(csiResource.Spec.ControllerEndpoint, &csiResource.Spec.LinstorClientConfig)...)

	csiLivenessProbe := corev1.Container{
		Name:            "csi-livenessprobe",
		Image:           csiResource.Spec.CSILivenessProbeImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args:            []string{"--csi-address=$(CSI_ENDPOINT)"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      pluginDir.Name,
			MountPath: "/csi/",
		}},
		Env: []corev1.EnvVar{csiEndpoint},
	}

	driverRegistrar := corev1.Container{
		Name:            "csi-node-driver-registrar",
		Image:           csiResource.Spec.CSINodeDriverRegistrarImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args: []string{
			"--v=5",
			// No --timeout here, it's a very recent addition and not very useful for a single call that should return
			// static information
			"--csi-address=$(CSI_ENDPOINT)",
			"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)",
		},
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
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
		Resources: csiResource.Spec.Resources,
	}

	linstorWaitNodeInitContainer := corev1.Container{
		Name:            "linstor-wait-node-online",
		Image:           csiResource.Spec.LinstorPluginImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Command: []string{
			"/linstor-wait-until",
			"satellite-online",
			"$(KUBE_NODE_NAME)",
		},
		Env:       env,
		Resources: csiResource.Spec.Resources,
	}

	linstorPluginContainer := corev1.Container{
		Name:            "linstor-csi-plugin",
		Image:           csiResource.Spec.LinstorPluginImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: int32(DefaultHealthPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Args: []string{
			"--csi-endpoint=unix://$(CSI_ENDPOINT)",
			"--node=$(KUBE_NODE_NAME)",
			"--linstor-endpoint=$(LS_CONTROLLERS)",
		},
		Env: env,
		SecurityContext: &corev1.SecurityContext{
			Privileged:   &IsPrivileged,
			Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"SYS_ADMIN"}},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pluginDir.Name,
				MountPath: "/csi/",
			},
			{
				Name:             publishDir.Name,
				MountPath:        publishDir.HostPath.Path,
				MountPropagation: &MountPropagationBidirectional,
			},
			{
				Name:      deviceDir.Name,
				MountPath: "/dev",
			},
		},
		Resources: csiResource.Spec.Resources,
		// Set the liveness probe on the plugin container, it's the component that probably needs the restart
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(DefaultHealthPort),
				},
			},
		},
	}

	if csiResource.Spec.LogLevel != "" {
		linstorPluginContainer.Args = append(linstorPluginContainer.Args, fmt.Sprintf("--log-level=%s", csiResource.Spec.LogLevel))
	}

	meta := getObjectMeta(csiResource, NodeDaemonSet, kubeSpec.CSINodeRole)
	template := corev1.PodTemplateSpec{
		ObjectMeta: meta,
		Spec: corev1.PodSpec{
			PriorityClassName:  csiResource.Spec.PriorityClassName.GetName(csiResource.Namespace),
			ServiceAccountName: csiResource.Spec.CSINodeServiceAccountName,
			InitContainers:     []corev1.Container{linstorWaitNodeInitContainer},
			Containers: append([]corev1.Container{
				driverRegistrar,
				csiLivenessProbe,
				linstorPluginContainer,
			}, csiResource.Spec.NodeSidecars...),
			Volumes: append([]corev1.Volume{
				deviceDir,
				pluginDir,
				publishDir,
				registrationDir,
			}, csiResource.Spec.NodeExtraVolumes...),
			DNSPolicy:        corev1.DNSClusterFirstWithHostNet,
			ImagePullSecrets: pullSecrets,
			Affinity:         csiResource.Spec.NodeAffinity,
			Tolerations:      csiResource.Spec.NodeTolerations,
		},
	}

	return &appsv1.DaemonSet{
		ObjectMeta: meta,
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getDefaultLabels(csiResource, kubeSpec.CSINodeRole),
			},
			Template: template,
		},
	}
}

func newCSIControllerDeployment(csiResource *piraeusv1.LinstorCSIDriver) *appsv1.Deployment {
	const socketDirPath = "/var/lib/csi/sockets/pluginproxy/"

	socketAddress := corev1.EnvVar{
		Name:  "ADDRESS",
		Value: socketDirPath + "./csi.sock",
	}

	kubeNodeName := corev1.EnvVar{
		Name:      "KUBE_NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"}},
	}

	podNamespace := corev1.EnvVar{
		Name:      "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
	}

	podName := corev1.EnvVar{
		Name:      "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
	}

	socketVolume := corev1.Volume{
		Name: "socket-dir",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	var pullSecrets []corev1.LocalObjectReference
	if csiResource.Spec.ImagePullSecret != "" {
		pullSecrets = append(pullSecrets, corev1.LocalObjectReference{Name: csiResource.Spec.ImagePullSecret})
	}

	linstorEnvVars := lc.APIResourceAsEnvVars(csiResource.Spec.ControllerEndpoint, &csiResource.Spec.LinstorClientConfig)

	csiLivenessProbe := corev1.Container{
		Name:            "csi-livenessprobe",
		Image:           csiResource.Spec.CSILivenessProbeImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args:            []string{"--csi-address=$(ADDRESS)"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Env: []corev1.EnvVar{socketAddress},
	}
	csiProvisioner := corev1.Container{
		Name:            "csi-provisioner",
		Image:           csiResource.Spec.CSIProvisionerImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args: []string{
			"--csi-address=$(ADDRESS)",
			"--timeout=1m",
			// restore old default fstype
			"--default-fstype=ext4",
			fmt.Sprintf("--feature-gates=Topology=%t", csiResource.Spec.EnableTopology),
			"--leader-election=true",
			"--leader-election-namespace=$(NAMESPACE)",
			"--enable-capacity",
			"--extra-create-metadata",
			"--capacity-ownerref-level=2",
			fmt.Sprintf("--worker-threads=%d", defaultIfUnset(csiResource.Spec.CSIProvisionerWorkerThreads, 10)),
		},
		Env: []corev1.EnvVar{socketAddress, podNamespace, podName},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Resources: csiResource.Spec.Resources,
	}
	csiAttacher := corev1.Container{
		Name:            "csi-attacher",
		Image:           csiResource.Spec.CSIAttacherImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args: []string{
			"--v=5",
			"--csi-address=$(ADDRESS)",
			"--timeout=1m",
			"--leader-election=true",
			"--leader-election-namespace=$(NAMESPACE)",
			fmt.Sprintf("--worker-threads=%d", defaultIfUnset(csiResource.Spec.CSIAttacherWorkerThreads, 10)),
		},
		Env: []corev1.EnvVar{socketAddress, podNamespace},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Resources: csiResource.Spec.Resources,
	}
	csiSnapshotter := corev1.Container{
		Name:            "csi-snapshotter",
		Image:           csiResource.Spec.CSISnapshotterImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args: []string{
			"--timeout=1m",
			"--csi-address=$(ADDRESS)",
			"--leader-election=true",
			"--leader-election-namespace=$(NAMESPACE)",
			fmt.Sprintf("--worker-threads=%d", defaultIfUnset(csiResource.Spec.CSISnapshotterWorkerThreads, 10)),
		},
		Env: []corev1.EnvVar{socketAddress, podNamespace},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Resources: csiResource.Spec.Resources,
	}
	csiResizer := corev1.Container{
		Name:            "csi-resizer",
		Image:           csiResource.Spec.CSIResizerImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Args: []string{
			"--v=5",
			"--csi-address=$(ADDRESS)",
			"--timeout=1m",
			// LINSTOR can resize while in use, no need to check if volume is in use
			"--handle-volume-inuse-error=false",
			"--leader-election=true",
			"--leader-election-namespace=$(NAMESPACE)",
			// For some reason this one is named differently...
			fmt.Sprintf("--workers=%d", defaultIfUnset(csiResource.Spec.CSIResizerWorkerThreads, 10)),
		},
		Env: []corev1.EnvVar{socketAddress, podNamespace},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Resources: csiResource.Spec.Resources,
	}

	linstorWaitAPIInitContainer := corev1.Container{
		Name:            "linstor-wait-api-online",
		Image:           csiResource.Spec.LinstorPluginImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Command: []string{
			"/linstor-wait-until",
			"api-online",
		},
		Env:       linstorEnvVars,
		Resources: csiResource.Spec.Resources,
	}

	linstorPlugin := corev1.Container{
		Name:            "linstor-csi-plugin",
		Image:           csiResource.Spec.LinstorPluginImage,
		ImagePullPolicy: csiResource.Spec.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          "healthz",
				ContainerPort: int32(DefaultHealthPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Args: []string{
			"--csi-endpoint=unix://$(ADDRESS)",
			"--node=$(KUBE_NODE_NAME)",
			"--linstor-endpoint=$(LS_CONTROLLERS)",
		},
		Env: append(
			[]corev1.EnvVar{
				socketAddress,
				kubeNodeName,
			},
			linstorEnvVars...,
		),
		VolumeMounts: []corev1.VolumeMount{{
			Name:      socketVolume.Name,
			MountPath: socketDirPath,
		}},
		Resources: csiResource.Spec.Resources,
		// Set the liveness probe on the plugin container, it's the component that probably needs the restart
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(DefaultHealthPort),
				},
			},
		},
	}

	if csiResource.Spec.LogLevel != "" {
		linstorPlugin.Args = append(linstorPlugin.Args, fmt.Sprintf("--log-level=%s", csiResource.Spec.LogLevel))
	}

	meta := getObjectMeta(csiResource, ControllerDeployment, kubeSpec.CSIControllerRole)
	template := corev1.PodTemplateSpec{
		ObjectMeta: meta,
		Spec: corev1.PodSpec{
			PriorityClassName:  csiResource.Spec.PriorityClassName.GetName(csiResource.Namespace),
			ServiceAccountName: csiResource.Spec.CSIControllerServiceAccountName,
			InitContainers:     []corev1.Container{linstorWaitAPIInitContainer},
			Containers: append([]corev1.Container{
				csiAttacher,
				csiLivenessProbe,
				csiProvisioner,
				csiSnapshotter,
				csiResizer,
				linstorPlugin,
			}, csiResource.Spec.ControllerSidecars...),
			ImagePullSecrets: pullSecrets,
			Volumes:          append([]corev1.Volume{socketVolume}, csiResource.Spec.ControllerExtraVolumes...),
			Affinity:         getControllerAffinity(csiResource),
			Tolerations:      csiResource.Spec.ControllerTolerations,
		},
	}

	return &appsv1.Deployment{
		ObjectMeta: meta,
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: meta.Labels,
			},
			Replicas: csiResource.Spec.ControllerReplicas,
			Strategy: csiResource.Spec.ControllerStrategy,
			Template: template,
		},
	}
}

func getControllerAffinity(resource *piraeusv1.LinstorCSIDriver) *corev1.Affinity {
	meta := getObjectMeta(resource, ControllerDeployment, kubeSpec.CSIControllerRole)
	if resource.Spec.ControllerAffinity == nil {
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

	return resource.Spec.ControllerAffinity
}

func newCSIDriver(csiResource *piraeusv1.LinstorCSIDriver) *storagev1.CSIDriver {
	// should be const, but required to be var so that we can take the address to get a *bool
	yes := true

	meta := getObjectMeta(csiResource, "%s", "cluster-config")

	return &storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			// Name must match exactly the one reported by the CSI plugin
			Name:   "linstor.csi.linbit.com",
			Labels: meta.Labels,
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired:  &yes,
			PodInfoOnMount:  &yes,
			StorageCapacity: &yes,
		},
	}
}

const (
	NodeServiceAccount       = "-csi-node"
	ControllerServiceAccount = "-csi-controller"
	NodeDaemonSet            = "%s-csi-node"
	ControllerDeployment     = "%s-csi-controller"
	DefaultKubeletPath       = "/var/lib/kubelet"
)

func defaultIfUnset(val, def int32) int32 {
	if val == 0 {
		return def
	}

	return val
}

func getObjectMeta(controllerResource *piraeusv1.LinstorCSIDriver, nameFmt string, component string) metav1.ObjectMeta {
	defaultLabels := getDefaultLabels(controllerResource, component)
	return metav1.ObjectMeta{
		Name:        fmt.Sprintf(nameFmt, controllerResource.Name),
		Namespace:   controllerResource.Namespace,
		Labels:      mdutil.MergeStringMap(controllerResource.ObjectMeta.Labels, defaultLabels),
		Annotations: controllerResource.ObjectMeta.Annotations,
	}
}

func kubeletPath(csiResource *piraeusv1.LinstorCSIDriver, subdirs ...string) string {
	return filepath.Join(append([]string{csiResource.Spec.KubeletPath}, subdirs...)...)
}

func getDefaultLabels(controllerResource *piraeusv1.LinstorCSIDriver, component string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       kubeSpec.CSIDriverRole,
		"app.kubernetes.io/instance":   controllerResource.Name,
		"app.kubernetes.io/managed-by": kubeSpec.Name,
		"app.kubernetes.io/component":  component,
	}
}
