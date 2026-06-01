package vars

import (
	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck

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
	FieldOwner                     = Domain + "/operator"
	ApplyAnnotation                = Domain + "/last-applied"
	NodeInterfaceAnnotation        = Domain + "/configured-interfaces"
	ManagedByLabel                 = Domain + "/managed-by"
	AppliedConfigurationAnnotation = Domain + "/applied-configurations"
	SatelliteNodeLabel             = Domain + "/linstor-satellite"
	SatelliteFinalizer             = Domain + "/satellite-protection"
	// SatelliteContainerName is the name of the container running the LINSTOR satellite in the satellite Pod. It is
	// used to exec backend storage probes (vgs/lvs/zfs) against the same host view the satellite itself uses.
	SatelliteContainerName            = "linstor-satellite"
	EvacuationActionAnnotation        = linstorcsi.DriverName + "/evacuation-action"
	MachinePreDrainHookAnnotation     = clusterapiv1beta1.PreDrainDeleteHookAnnotationPrefix + "/linstor-prepare-for-drain"
	MachinePreTerminateHookAnnotation = clusterapiv1beta1.PreTerminateDeleteHookAnnotationPrefix + "/linstor-wait-for-complete-evacuation"
	GenCertLeaderElectionID           = OperatorName + "-gencert"
	// PersistentVolumeWaitForReattachAnnotationPrefix is the annotation on the PV used during node evacuation to
	// indicate that the PV needs to attached on another node before executing the actual LINSTOR Node evacuation.
	// The value is "true" if the volume was actually attached at the start of the evacuation, "false" if not.
	PersistentVolumeWaitForReattachAnnotationPrefix = "wait-for-reattach.evacuation." + Domain
	// PersistentVolumeWaitForReattachSinceAnnotationPrefix is the annotation on the PV used during node evacuation to
	// store the timestamp of the start of the "wait-for-reattach" phase, used to calculate the timeout.
	PersistentVolumeWaitForReattachSinceAnnotationPrefix = "wait-for-reattach-since.evacuation." + Domain
)
