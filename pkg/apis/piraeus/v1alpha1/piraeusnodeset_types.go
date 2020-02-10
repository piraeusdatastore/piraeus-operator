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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PiraeusNodeSetSpec defines the desired state of PiraeusNodeSet
type PiraeusNodeSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// StoragePools is a list of StoragePools for PiraeusNodeSet to manage.
	StoragePools *StoragePools `json:"storagePools"`

	//DisableDRBDKernelModuleInjection turns off automatic injection of the DRBD
	// kernel module on the host system when set to true.
	DisableDRBDKernelModuleInjection bool `json:"disableDRBDKernelModuleInjection"`

	//DrbdRepoCred is the name of the k8s secret with the repo credential
	DrbdRepoCred string `json:"drbdRepoCred"`

	//SatelliteImage is the LINSTOR Satellite image location
	SatelliteImage string `json:"satelliteImage"`

	//SatelliteVersion is the LINSTOR Satellite image location
	SatelliteVersion string `json:"satelliteVersion"`
}

// PiraeusNodeSetStatus defines the observed state of PiraeusNodeSet
type PiraeusNodeSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Keep the first letters in the json for Errors lowercase and all other
	// statuses uppercase. This causes `kubectl get TYPE NAME -oyaml` to sort
	// errors to below the other potentially very long Statuses which is the preferred UX.

	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
	// SatelliteStatuses by hostname.
	SatelliteStatuses map[string]*SatelliteStatus `json:"SatelliteStatuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PiraeusNodeSet is the Schema for the piraeusnodesets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=piraeusnodesets,scope=Namespaced
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
