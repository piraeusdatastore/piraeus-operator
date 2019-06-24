package v1alpha1

import (
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
}

// LinstorSatelliteSetStatus defines the observed state of LinstorSatelliteSet
// +k8s:openapi-gen=true
type LinstorSatelliteSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

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
	// The hostname of the kubelet  running the satellite
	NodeName string `json:"nodeName"`
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
