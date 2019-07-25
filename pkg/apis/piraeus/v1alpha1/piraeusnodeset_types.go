/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PiraeusNodeSetSpec defines the desired state of PiraeusNodeSet
// +k8s:openapi-gen=true
type PiraeusNodeSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// ControllerEndpoint is the API endpoint for the LINSTOR controller associated
	// With this PiraeusNodeSet.
	ControllerEndpoint string `json:"controllerEndpoint"`
	// StoragePools is a list of StoragePools for PiraeusNodeSet to manage.
	StoragePools *StoragePools `json:"storagePools"`
}

// PiraeusNodeSetStatus defines the observed state of PiraeusNodeSet
// +k8s:openapi-gen=true
type PiraeusNodeSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Errors remaining that will trigger reconciliations.
	Errors []string
	// SatelliteStatuses by hostname.
	SatelliteStatuses map[string]*SatelliteStatus `json:"satelliteStatuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PiraeusNodeSet is the Schema for the piraeusnodesets API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type PiraeusNodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PiraeusNodeSetSpec   `json:"spec,omitempty"`
	Status PiraeusNodeSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PiraeusNodeSetList contains a list of PiraeusNodeSet
type PiraeusNodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PiraeusNodeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PiraeusNodeSet{}, &PiraeusNodeSetList{})
}
