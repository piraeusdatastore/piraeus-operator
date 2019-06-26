package v1alpha1

import (
	lapi "github.com/LINBIT/golinstor/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LinstorSatelliteSetSpec defines the desired state of LinstorSatelliteSet
// +k8s:openapi-gen=true
type LinstorSatelliteSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// ControllerEndpoint is the API endpoint for the LINSTOR controller associated
	// With this LinstorSatelliteSet.
	// TODO: USE a route with a containerized Controller.
	ControllerEndpoint string `json:"controllerEndpoint"`
	// StoragePools is a list of StoragePools for LinstorSatelliteSet to manage.
	StoragePools *StoragePools `json:"storagePools"`
}

type StoragePools struct {
	// LVMPools for LinstorSatelliteSet to manage.
	LVMPools []*StoragePoolLVM `json:"lvmPools"`
	// LVMThinPools for LinstorSatelliteSet to manage.
	LVMThinPools []*StoragePoolLVMThin `json:"lvmThinPools"`
}

type StoragePool interface {
	ToLinstorStoragePool() lapi.StoragePool
}

// StoragePoolLVM represents LVM storage pool to be managed by a
// LinstorSatelliteSet
type StoragePoolLVM struct {
	// Name of the storage pool.
	Name string `json:"name"`
	// Name of underlying lvm group
	VolumeGroup string `json:"volumeGroup"`
}

func (s *StoragePoolLVM) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: s.Name,
		ProviderKind:    lapi.LVM,
		Props: map[string]string{
			"StorDriver/LvmVg": s.VolumeGroup,
		},
	}
}

// StoragePoolLVMThin represents LVM Thin storage pool to be
// managed by a LinstorSatelliteSet
type StoragePoolLVMThin struct {
	StoragePoolLVM
	// Name of underlying lvm thin volume
	ThinVolume string `json:"thinVolume"`
}

func (s *StoragePoolLVMThin) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: s.Name,
		ProviderKind:    lapi.LVM_THIN,
		Props: map[string]string{
			"StorDriver/LvmVg":    s.VolumeGroup,
			"StorDriver/ThinPool": s.ThinVolume,
		},
	}
}

// LinstorSatelliteSetStatus defines the observed state of LinstorSatelliteSet
// +k8s:openapi-gen=true
type LinstorSatelliteSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Errors remaining that will trigger reconciliations.
	Errors []string
	// SatelliteStatuses by hostname.
	SatelliteStatuses map[string]*SatelliteStatus `json:"satelliteStatuses"`
}

// SatelliteStatus should provide all the information that the reconsile loop
// needs to manage the operation of the LINSTOR Satellite.
type SatelliteStatus struct {
	// Indicates if the satellite node has been created on the controller.
	RegisteredOnController bool `json:"registeredOnController"`
	// As indicated by Linstor
	ConnectionStatus string `json:"connectionStatus"`
	// The hostname of the kubelet running the satellite
	NodeName string `json:"nodeName"`

	// StoragePoolStatuses by storage pool name.
	StoragePoolStatuses map[string]*StoragePoolStatus `json:"storagePoolStatus"`
}

// StoragePoolStatus reports basic information about storage pool state.
type StoragePoolStatus struct {
	// The name of the storage pool.
	Name string `json:"name"`
	// The hostname of the kubelet hosting the storage pool.
	NodeName string `json:"nodeName"`
	// Provider is the underlying storage, lvm, zfs, etc.
	Provider string `json:"provider"`
	// Usage reporting
	FreeCapacity  int64 `json:"freeCapacity"`
	TotalCapacity int64 `json:"totalCapacity"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorSatelliteSet is the Schema for the linstorsatellitesets API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type LinstorSatelliteSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorSatelliteSetSpec   `json:"spec,omitempty"`
	Status LinstorSatelliteSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorSatelliteSetList contains a list of LinstorSatelliteSet
type LinstorSatelliteSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorSatelliteSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorSatelliteSet{}, &LinstorSatelliteSetList{})
}
