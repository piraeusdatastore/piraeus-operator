package linstorsatelliteset

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	lapi "github.com/LINBIT/golinstor/client"
	linstorv1alpha1 "github.com/LINBIT/linstor-operator/pkg/apis/linstor/v1alpha1"
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
	"controller": "LinstorSatelliteSet",
})

const linstorNodeFinalizer = "finalizer.linstor.linbit.com"

// Add creates a new LinstorSatelliteSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLinstorSatelliteSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("linstorsatelliteset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorSatelliteSet
	err = c.Watch(&source.Kind{Type: &linstorv1alpha1.LinstorSatelliteSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &apps.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &linstorv1alpha1.LinstorSatelliteSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLinstorSatelliteSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLinstorSatelliteSet{}

// ReconcileLinstorSatelliteSet reconciles a LinstorSatelliteSet object
type ReconcileLinstorSatelliteSet struct {
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

// Reconcile reads that state of the cluster for a LinstorSatelliteSet object and makes changes based on the state read
// and what is in the LinstorSatelliteSet.Spec
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileLinstorSatelliteSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Debug("entering reconcile loop")

	// Fetch the LinstorSatelliteSet instance
	satSet := &linstorv1alpha1.LinstorSatelliteSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, satSet)
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
		"resquestName":        request.Name,
		"resquestNamespace":   request.Namespace,
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	})
	log.Info("reconciling LinstorSatelliteSet")

	logrus.WithFields(logrus.Fields{
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	}).Debug("found LinstorSatelliteSet")

	if satSet.Status.SatelliteStatuses == nil {
		satSet.Status.SatelliteStatuses = make(map[string]*linstorv1alpha1.SatelliteStatus)
	}
	if satSet.Spec.StoragePools == nil {
		satSet.Spec.StoragePools = &linstorv1alpha1.StoragePools{}
	}
	if satSet.Spec.StoragePools.LVMPools == nil {
		satSet.Spec.StoragePools.LVMPools = make([]*linstorv1alpha1.StoragePoolLVM, 0)
	}
	if satSet.Spec.StoragePools.LVMThinPools == nil {
		satSet.Spec.StoragePools.LVMThinPools = make([]*linstorv1alpha1.StoragePoolLVMThin, 0)
	}

	r.linstorClient, err = newLinstorClientFromSatSet(satSet)
	if err != nil {
		return reconcile.Result{}, err
	}

	markedForDeletion := satSet.GetDeletionTimestamp() != nil
	if markedForDeletion {
		errs := r.finalizeSatelliteSet(satSet)

		logrus.WithFields(logrus.Fields{
			"errs": newCompoundErrorMsg(errs),
		}).Debug("reconcile loop end")

		// Resouces need to be removed by human intervention, so we don't want to
		// requeue the reconcile loop immediately. We can't return the error with
		// the loop or it will automatically requeue, we log it above and it also
		// appears in the satSet's Status.
		return reconcile.Result{RequeueAfter: time.Minute * 1}, nil
	}

	if err := r.addFinalizer(satSet); err != nil {
		return reconcile.Result{}, err
	}

	// Define a new DaemonSet
	ds := newDaemonSetforSatSet(satSet)

	// Set LinstorSatelliteSet satSet as the owner and controller
	if err := controllerutil.SetControllerReference(satSet, ds, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &apps.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logrus.WithFields(logrus.Fields{
			"daemonSet": fmt.Sprintf("%+v", ds),
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
		"daemonSet": fmt.Sprintf("%+v", ds),
	}).Debug("DaemonSet already exists")

	errs := r.reconcileSatNodes(satSet)
	compoundErrorMsg := newCompoundErrorMsg(errs)
	satSet.Status.Errors = compoundErrorMsg

	if err := r.client.Status().Update(context.TODO(), satSet); err != nil {
		logrus.Error(err, "Failed to update LinstorSatelliteSet status")
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

func (r *ReconcileLinstorSatelliteSet) reconcileSatNodes(satSet *linstorv1alpha1.LinstorSatelliteSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	})
	log.Info("reconciling LinstorSatelliteSet Nodes")

	pods := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(map[string]string{"app": satSet.Name})
	listOps := &client.ListOptions{Namespace: satSet.Namespace, LabelSelector: labelSelector}
	err := r.client.List(context.TODO(), listOps, pods)
	if err != nil {
		return []error{err}
	}
	logrus.WithFields(logrus.Fields{
		"pods": fmt.Sprintf("%+v", pods),
	}).Debug("found pods")

	var errs []error
	for _, pod := range pods.Items {
		errs = append(errs, r.reconcileSatNodeWithController(satSet, pod))
		errs = append(errs, r.reconcileStoragePoolsOnNode(satSet, pod))
	}

	return errs
}

func (r *ReconcileLinstorSatelliteSet) reconcileSatNodeWithController(satSet *linstorv1alpha1.LinstorSatelliteSet, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":             pod.Name,
		"podNameSpace":        pod.Namespace,
		"podPase":             pod.Status.Phase,
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	})
	log.Debug("reconciling node")

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod %s not running, delaying registration on controller", pod.Spec.NodeName)
	}

	sat, ok := satSet.Status.SatelliteStatuses[pod.Spec.NodeName]
	if !ok {
		satSet.Status.SatelliteStatuses[pod.Spec.NodeName] = &linstorv1alpha1.SatelliteStatus{NodeName: pod.Spec.NodeName}
		sat = satSet.Status.SatelliteStatuses[pod.Spec.NodeName]
	}

	// Mark this true on successful exit from this function.
	sat.RegisteredOnController = false

	wantedDefaultNetInterface := lapi.NetInterface{
		Name:                    "default",
		Address:                 pod.Status.HostIP,
		SatellitePort:           3366,
		SatelliteEncryptionType: "plain",
	}

	node, err := r.linstorClient.Nodes.Get(context.TODO(), pod.Spec.NodeName)
	if err != nil {
		if err == lapi.NotFoundError {
			if err := r.linstorClient.Nodes.Create(context.TODO(), lapi.Node{
				Name:          pod.Spec.NodeName,
				Type:          "Satellite",
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
			if nodeIf.Address != wantedDefaultNetInterface.Address ||
				nodeIf.SatellitePort != wantedDefaultNetInterface.SatellitePort {
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

	wantedConnStatus := "ONLINE"
	sat.ConnectionStatus = node.ConnectionStatus
	if sat.ConnectionStatus != wantedConnStatus {
		return fmt.Errorf("waiting for node %s ConnectionStatus to be %s, current ConnectionStatus: %s",
			pod.Spec.NodeName, wantedConnStatus, sat.ConnectionStatus)
	}

	sat.RegisteredOnController = true
	return nil
}

func (r *ReconcileLinstorSatelliteSet) reconcileStoragePoolsOnNode(satSet *linstorv1alpha1.LinstorSatelliteSet, pod corev1.Pod) error {
	log := logrus.WithFields(logrus.Fields{
		"podName":             pod.Name,
		"podNameSpace":        pod.Namespace,
		"podPase":             pod.Status.Phase,
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	})
	log.Info("reconciling storagePools")

	sat, ok := satSet.Status.SatelliteStatuses[pod.Spec.NodeName]
	if !ok {
		return fmt.Errorf("expected %s to be present in Status, not able to reconcile storage pools", pod.Spec.NodeName)
	}
	if !sat.RegisteredOnController {
		return fmt.Errorf("waiting for %s to be registered on controller, not able to reconcile storage pools", pod.Spec.NodeName)
	}
	if sat.StoragePoolStatuses == nil {
		sat.StoragePoolStatuses = make(map[string]*linstorv1alpha1.StoragePoolStatus)
	}

	// Append all disperate StoragePool types together, so they can be processed together.
	var pools = make([]linstorv1alpha1.StoragePool, 0)
	for _, thickPool := range satSet.Spec.StoragePools.LVMPools {
		pools = append(pools, thickPool)
	}

	for _, thinPool := range satSet.Spec.StoragePools.LVMThinPools {
		pools = append(pools, thinPool)
	}

	// Get status for all pools.
	for _, pool := range pools {
		status, err := r.getStatusOrCreateOnNode(context.TODO(), pool.ToLinstorStoragePool(), pod.Spec.NodeName)
		if err != nil {
			return err
		}

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

func (r *ReconcileLinstorSatelliteSet) getStatusOrCreateOnNode(ctx context.Context, pool lapi.StoragePool, nodeName string) (*linstorv1alpha1.StoragePoolStatus, error) {
	foundPool, err := r.linstorClient.Nodes.GetStoragePool(ctx, nodeName, pool.StoragePoolName)
	// StoragePool doesn't exists, create it.
	if err != nil && err == lapi.NotFoundError {
		if err := r.linstorClient.Nodes.CreateStoragePool(ctx, nodeName, pool); err != nil {
			return newStoragePoolStatus(pool), fmt.Errorf("unable to create storage pool %s on node %s: %v", pool.StoragePoolName, nodeName, err)
		}
		return newStoragePoolStatus(foundPool), nil
	}
	// Other error.
	if err != nil {
		return newStoragePoolStatus(pool), fmt.Errorf("unable to get storage pool %s on node %s: %v", pool.StoragePoolName, nodeName, err)
	}

	return newStoragePoolStatus(foundPool), nil
}

func newDaemonSetforSatSet(satSet *linstorv1alpha1.LinstorSatelliteSet) *apps.DaemonSet {
	var (
		isPrivileged                  = true
		directoryType                 = corev1.HostPathDirectory
		devDirName                    = "device-dir"
		devDir                        = "/dev/"
		modulesDirName                = "modules-dir"
		modulesDir                    = "/usr/lib/modules/"
		udevDirName                   = "udev"
		udevDir                       = "/run/udev"
		mountPropagationBidirectional = corev1.MountPropagationBidirectional
	)

	labels := map[string]string{
		"app": satSet.Name,
	}
	return &apps.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      satSet.Name + "-node",
			Namespace: "kube-system",
			Labels:    labels,
		},
		Spec: apps.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      satSet.Name + "-node",
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
												Values:   []string{"storage"},
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
							Name:            "linstor-satellite",
							Image:           "quay.io/piraeusdatastore/piraeus-server:latest",
							Args:            []string{"startSatellite"}, // Run linstor-satellite.
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{Privileged: &isPrivileged},
							Ports: []corev1.ContainerPort{
								corev1.ContainerPort{
									HostPort:      3366,
									ContainerPort: 3366,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      devDirName,
									MountPath: devDir,
								},
								corev1.VolumeMount{
									Name:      udevDirName,
									MountPath: udevDir,
									ReadOnly:  true,
								},
								corev1.VolumeMount{
									Name:             modulesDirName,
									MountPath:        modulesDir,
									MountPropagation: &mountPropagationBidirectional,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: devDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: devDir,
								}}},
						corev1.Volume{
							Name: modulesDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: modulesDir,
									Type: &directoryType,
								}}},
						corev1.Volume{
							Name: udevDirName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: udevDir,
									Type: &directoryType,
								}}},
					},
				},
			},
		},
	}
}

func (r *ReconcileLinstorSatelliteSet) finalizeNode(satSet *linstorv1alpha1.LinstorSatelliteSet, nodeName string) error {
	log := logrus.WithFields(logrus.Fields{
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
		"node":                nodeName,
	})
	log.Debug("finalizing node")
	// Determine if any resources still remain on the node.
	resList, err := r.linstorClient.Resources.GetResourceView(context.TODO()) //, &lapi.ListOpts{Node: []string{nodeName}}) : not working
	if err != nil && err != lapi.NotFoundError {
		return fmt.Errorf("unable to check for resources on node %s: %v", nodeName, err)
	}

	resList = filterNodes(resList, nodeName)

	if len(resList) != 0 {
		return fmt.Errorf("unable to remove node %s: all resources must be removed before deletion", nodeName)
	}

	// No resources, safe to delete the node.
	if err := r.linstorClient.Nodes.Delete(context.TODO(), nodeName); err != nil && err != lapi.NotFoundError {
		return fmt.Errorf("unable to delete node %s: %v", nodeName, err)
	}

	delete(satSet.Status.SatelliteStatuses, nodeName)
	return nil
}

func (r *ReconcileLinstorSatelliteSet) addFinalizer(satSet *linstorv1alpha1.LinstorSatelliteSet) error {
	if !contains(satSet.GetFinalizers(), linstorNodeFinalizer) {
		satSet.SetFinalizers(append(satSet.GetFinalizers(), linstorNodeFinalizer))

		err := r.client.Update(context.TODO(), satSet)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func (r *ReconcileLinstorSatelliteSet) finalizeSatelliteSet(satSet *linstorv1alpha1.LinstorSatelliteSet) []error {
	log := logrus.WithFields(logrus.Fields{
		"LinstorSatelliteSet": fmt.Sprintf("%+v", satSet),
	})
	log.Info("found LinstorSatelliteSet marked for deletion, finalizing...")

	if contains(satSet.GetFinalizers(), linstorNodeFinalizer) {
		// Run finalization logic for LinstorSatelliteSet. If the
		// finalization logic fails, don't remove the finalizer so
		// that we can retry during the next reconciliation.
		var errs = make([]error, 0)
		for nodeName := range satSet.Status.SatelliteStatuses {
			if err := r.finalizeNode(satSet, nodeName); err != nil {
				errs = append(errs, err)
			}
		}

		// Remove finalizer. Once all finalizers have been
		// removed, the object will be deleted.
		if len(errs) == 0 {
			log.Info("finalizing finished, removing finalizer")
			satSet.SetFinalizers(remove(satSet.GetFinalizers(), linstorNodeFinalizer))
			err := r.client.Update(context.TODO(), satSet)
			if err != nil {
				return []error{err}
			}
			return nil
		}

		err := r.client.Status().Update(context.TODO(), satSet)
		if err != nil {
			return []error{err}
		}

		return errs
	}
	return nil
}

func newLinstorClientFromSatSet(satSet *linstorv1alpha1.LinstorSatelliteSet) (*lapi.Client, error) {
	if satSet.Spec.ControllerEndpoint == "" {
		return nil, fmt.Errorf("unable to create LINSTOR API client: ControllerIP cannot be empty")
	}
	u, err := url.Parse(satSet.Spec.ControllerEndpoint)
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

func newStoragePoolStatus(pool lapi.StoragePool) *linstorv1alpha1.StoragePoolStatus {
	return &linstorv1alpha1.StoragePoolStatus{
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

func filterNodes(resources []lapi.Resource, nodeName string) []lapi.Resource {
	var nodeRes = make([]lapi.Resource, 0)
	for _, r := range resources {
		if r.NodeName == nodeName {
			nodeRes = append(nodeRes, r)
		}
	}
	return nodeRes
}
