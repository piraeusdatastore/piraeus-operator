package v1

// DeletionPolicy configures the way LinstorSatellite resources are deleted.
//
// A LinstorSatellite may be deleted because:
// * It no longer matches the affinity and node selector of the LinstorCluster resource.
// * The node it references has been removed from Kubernetes.
// * It was manually deleted outside the Operator.
//
// A LinstorSatellite may store the last copy of a volume, in which case it is not desirable to unconditionally remove
// the satellite from the cluster. For this reason, the following deletion policies exist:
//
// * DeletionPolicyEvacuate will start evacuation of the LINSTOR Satellite and wait until it completes before removing the LinstorSatellite object, comparable to the "linstor node evacuate" command.
// * DeletionPolicyRetain will retain the LINSTOR Satellite, keeping it registered in LINSTOR, but removing associated Kubernetes resources.
// * DeletionPolicyDelete will remove the LINSTOR Satellite from the LINSTOR Cluster without prior eviction, comparable to the "linstor node lost" command.
// +kubebuilder:validation:Enum:=Evacuate;Retain;Delete
type DeletionPolicy string

const (
	DeletionPolicyEvacuate DeletionPolicy = "Evacuate"
	DeletionPolicyRetain   DeletionPolicy = "Retain"
	DeletionPolicyDelete   DeletionPolicy = "Delete"
)
