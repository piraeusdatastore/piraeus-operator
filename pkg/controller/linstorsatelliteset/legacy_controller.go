package linstorsatelliteset

import (
	"context"
	"time"

	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func newLegacyReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileLegacyLinstorNodeSet{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

func addLegacyReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	log.Debug("satellite legacy: Adding a PNS controller ")
	c, err := controller.New("LinstorSatelliteSet-legacy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorNodeSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.LinstorNodeSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLegacyLinstorNodeSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLegacyLinstorNodeSet{}

type ReconcileLegacyLinstorNodeSet struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *ReconcileLegacyLinstorNodeSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
	})
	log.Info("legacy reconciler: got new request")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	log.Debug("fetch resource")

	nodeSet := &piraeusv1alpha1.LinstorNodeSet{}
	err := r.Client.Get(ctx, request.NamespacedName, nodeSet)
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

	if nodeSet.Status.ResourceMigrated && nodeSet.Status.DependantsMigrated {
		log.Info("nodeSet already migrated, nothing to do")
		return reconcile.Result{}, nil
	}

	log.Debug("convert LinstorNodeSet to LinstorSatelliteSet")

	satelliteSet := &piraeusv1alpha1.LinstorSatelliteSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       nodeSet.Name,
			Namespace:  nodeSet.Namespace,
			Finalizers: nodeSet.Finalizers,
		},
		Spec:   nodeSet.Spec.LinstorSatelliteSetSpec,
		Status: nodeSet.Status.LinstorSatelliteSetStatus,
	}

	if satelliteSet.Spec.KernelModuleInjectionImage == "" {
		satelliteSet.Spec.KernelModuleInjectionImage = nodeSet.Spec.KernelModImage
	}

	if satelliteSet.Spec.KernelModuleInjectionMode == "" {
		satelliteSet.Spec.KernelModuleInjectionMode = nodeSet.Spec.DRBDKernelModuleInjectionMode
	}

	resourceErr := r.Client.Create(ctx, satelliteSet)
	if errors.IsAlreadyExists(resourceErr) {
		log.Debug("LinstorSatelliteSet already exists")
		resourceErr = nil
	}

	log.Debug("migrate old resource to new owner")

	dependantsErr := r.migrateDependants(ctx, nodeSet, satelliteSet)

	log.Debug("remove finalizer from old resource")

	var finalizerError error
	if util.HasFinalizer(nodeSet, linstorSatelliteFinalizer) {
		controllerutil.RemoveFinalizer(nodeSet, linstorSatelliteFinalizer)
		finalizerError = r.Client.Update(ctx, nodeSet)
	}

	log.Debug("update status of LinstorNodeSet")

	nodeSet.Status.ResourceMigrated = resourceErr == nil
	nodeSet.Status.DependantsMigrated = dependantsErr == nil
	nodeSet.Status.Errors = reconcileutil.ErrorStrings(resourceErr, dependantsErr, finalizerError)
	err = r.Client.Status().Update(ctx, nodeSet)
	if err != nil {
		log.WithFields(logrus.Fields{
			"resourceErr":   resourceErr,
			"dependantsErr": dependantsErr,
		}).Debug("failed to update status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileLegacyLinstorNodeSet) migrateDependants(ctx context.Context, legacy *piraeusv1alpha1.LinstorNodeSet, target *piraeusv1alpha1.LinstorSatelliteSet) error {
	transferer := reconcileutil.NewOwnershipTransferer(r.Client, r.Scheme, legacy, target)

	log.Debug("transfer ownership of daemonSets")

	err := transferer.TransferOwnershipOfAll(ctx, &appsv1.DaemonSetList{})
	if err != nil {
		return err
	}

	log.Debug("transfer ownership of configMaps")

	err = transferer.TransferOwnershipOfAll(ctx, &corev1.ConfigMapList{})
	if err != nil {
		return err
	}

	return nil
}
