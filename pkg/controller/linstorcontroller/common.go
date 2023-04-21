package linstorcontroller

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	linstorControllerFinalizer = "finalizer.linstor-controller.linbit.com"

	// requeue reconciliation after connectionRetrySeconds
	connectionRetrySeconds = 10
)

// Add creates a new LinstorController Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	err := addControllerReconciler(mgr, newControllerReconciler(mgr))
	if err != nil {
		return err
	}

	return nil
}
