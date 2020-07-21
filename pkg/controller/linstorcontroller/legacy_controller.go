package linstorcontroller

import (
	"context"
	"time"

	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/metadata/util"
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
	return &ReconcileLegacyController{Client: mgr.GetClient(), Scheme: mgr.GetScheme()}
}

func addLegacyReconciler(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("LinstorControllerSet-legacy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource LinstorControllerSet
	err = c.Watch(&source.Kind{Type: &piraeusv1alpha1.LinstorControllerSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileLegacyController implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileLegacyController{}

type ReconcileLegacyController struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func (r *ReconcileLegacyController) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := log.WithFields(logrus.Fields{
		"requestName":      request.Name,
		"requestNamespace": request.Namespace,
	})
	log.Info("legacy reconciler: got new request")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	log.Debug("fetch resource")

	controllerSet := &piraeusv1alpha1.LinstorControllerSet{}
	err := r.Client.Get(ctx, request.NamespacedName, controllerSet)
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

	if controllerSet.Status.ResourceMigrated && controllerSet.Status.DependantsMigrated {
		log.Info("controllerSet already migrated, nothing to do")
		return reconcile.Result{}, nil
	}

	log.Debug("convert LinstorControllerSet to LinstorController")

	linstorController := &piraeusv1alpha1.LinstorController{
		ObjectMeta: metav1.ObjectMeta{
			Name:       controllerSet.Name,
			Namespace:  controllerSet.Namespace,
			Finalizers: controllerSet.Finalizers,
		},
		Spec:   controllerSet.Spec.LinstorControllerSpec,
		Status: controllerSet.Status.LinstorControllerStatus,
	}

	resourceErr := r.Client.Create(ctx, linstorController)
	if errors.IsAlreadyExists(resourceErr) {
		log.Debug("LinstorController already exists")
		resourceErr = nil
	}

	log.Debug("migrate old resource to new owner")

	dependantsErr := r.migrateDependants(ctx, controllerSet, linstorController)

	log.Debug("remove finalizer from old resource")

	var finalizerError error
	if util.HasFinalizer(controllerSet, linstorControllerFinalizer) {
		controllerutil.RemoveFinalizer(controllerSet, linstorControllerFinalizer)
		finalizerError = r.Client.Update(ctx, controllerSet)
	}

	log.Debug("update status of LinstorControllerSet")

	controllerSet.Status.ResourceMigrated = resourceErr == nil
	controllerSet.Status.DependantsMigrated = dependantsErr == nil
	controllerSet.Status.Errors = reconcileutil.ErrorStrings(resourceErr, dependantsErr, finalizerError)
	err = r.Client.Status().Update(ctx, controllerSet)
	if err != nil {
		log.WithFields(logrus.Fields{
			"resourceErr":   resourceErr,
			"dependantsErr": dependantsErr,
		}).Debug("failed to update status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileLegacyController) migrateDependants(ctx context.Context, legacy *piraeusv1alpha1.LinstorControllerSet, target *piraeusv1alpha1.LinstorController) error {
	transferer := reconcileutil.NewOwnershipTransferer(r.Client, r.Scheme, legacy, target)

	log.Debug("transfer ownership of deployments")

	err := transferer.TransferOwnershipOfAll(ctx, &appsv1.DeploymentList{})
	if err != nil {
		return err
	}

	log.Debug("transfer ownership of configMaps")

	err = transferer.TransferOwnershipOfAll(ctx, &corev1.ConfigMapList{})
	if err != nil {
		return err
	}

	log.Debug("transfer ownership of services")

	err = transferer.TransferOwnershipOfAll(ctx, &corev1.ServiceList{})
	if err != nil {
		return err
	}

	return nil
}
