package vars

import (
	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

var (
	Version     = "2.0.0"
	ExtraLabels = map[string]string{
		"app.kubernetes.io/version":    Version,
		"app.kubernetes.io/managed-by": OperatorName,
	}
	FallbackAPIVersion = utils.APIVersion{
		Major: 1,
		Minor: 20,
	}
)

const (
	FieldOwner                                      = Domain + "/operator"
	ApplyAnnotation                                 = Domain + "/last-applied"
	NodeInterfaceAnnotation                         = Domain + "/configured-interfaces"
	ManagedByLabel                                  = Domain + "/managed-by"
	SatelliteNodeLabel                              = Domain + "/linstor-satellite"
	SatelliteFinalizer                              = Domain + "/satellite-protection"
	PersistentVolumeWaitForReattachAnnotationPrefix = "wait-for-reattach.evacuation." + Domain
	EvacuationActionAnnotation                      = linstorcsi.DriverName + "/evacuation-action"
	MachinePreDrainHookAnnotation                   = clusterapiv1beta1.PreDrainDeleteHookAnnotationPrefix + "/linstor-prepare-for-drain"
	MachinePreTerminateHookAnnotation               = clusterapiv1beta1.PreTerminateDeleteHookAnnotationPrefix + "/linstor-wait-for-complete-evacuation"
	GenCertLeaderElectionID                         = OperatorName + "-gencert"
)
