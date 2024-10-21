package vars

import "github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"

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
	FieldOwner              = Domain + "/operator"
	ApplyAnnotation         = Domain + "/last-applied"
	NodeInterfaceAnnotation = Domain + "/configured-interfaces"
	ManagedByLabel          = Domain + "/managed-by"
	SatelliteNodeLabel      = Domain + "/linstor-satellite"
	SatelliteFinalizer      = Domain + "/satellite-protection"
	GenCertLeaderElectionID = OperatorName + "-gencert"
)
