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

// LinstorControllerSetSpec defines the desired state of LinstorControllerSet
type LinstorControllerSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// PriorityClassName is the name of the PriorityClass for the controller set
	PriorityClassName string `json:"priorityClassName"`

	DBConnectionURL string `json:"dbConnectionURL"`
	DBCertSecret    string `json:"dbCertSecret"`
	// Name of the secret containing the master passphrase for LUKS devices as `MASTER_PASSPHRASE`
	LuksSecret		string `json:"luksSecret"`
	// Configuration options for secure communication between nodes and controllers
	SslConfig       *LinstorSSLConfig `json:"sslSecret"`
	DrbdRepoCred    string            `json:"drbdRepoCred"`
	ControllerImage string            `json:"controllerImage"`
}

// LinstorControllerSetStatus defines the observed state of LinstorControllerSet
type LinstorControllerSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Keep the first letters in the json for Errors lowercase and all other
	// statuses uppercase. This causes `kubectl get TYPE NAME -oyaml` to sort
	// errors to below the other potentially very long Statuses which is the preferred UX.

	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
	// ControllerStatus information.
	ControllerStatus *NodeStatus `json:"ControllerStatus"`
	// SatelliteStatuses by hostname.
	SatelliteStatuses []*SatelliteStatus `json:"SatelliteStatuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorControllerSet is the Schema for the linstorcontrollersets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorcontrollersets,scope=Namespaced
type LinstorControllerSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorControllerSetSpec   `json:"spec,omitempty"`
	Status LinstorControllerSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorControllerSetList contains a list of LinstorControllerSet
type LinstorControllerSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorControllerSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorControllerSet{}, &LinstorControllerSetList{})
}
