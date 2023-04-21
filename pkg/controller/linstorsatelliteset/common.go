package linstorsatelliteset

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// linstorSatelliteFinalizer can only be removed if the linstor node containers are
	// ready to be shutdown. For now, this means that they have no resources assigned
	// to them.
	linstorSatelliteFinalizer = "finalizer.linstor-node.linbit.com"

	// Default value for automaticStorageType. If set, no automatic setup of storage devices happens.
	automaticStorageTypeNone = "None"

	// requeue reconciliation after connectionRetrySeconds
	connectionRetrySeconds = 10
)

// Add creates a new LinstorSatelliteSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	err := addSatelliteReconciler(mgr, newSatelliteReconciler(mgr))
	if err != nil {
		return err
	}

	return err
}
