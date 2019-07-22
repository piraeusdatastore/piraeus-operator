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

// PiraeusControllerSetSpec defines the desired state of PiraeusControllerSet
// +k8s:openapi-gen=true
type PiraeusControllerSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// PiraeusControllerSetStatus defines the observed state of PiraeusControllerSet
// +k8s:openapi-gen=true
type PiraeusControllerSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Errors remaining that will trigger reconciliations.
	Errors []string
	// ControllerStatus information.
	ControllerStatus *NodeStatus
	// SatelliteStatuses by hostname.
	SatelliteStatuses map[string]*CtrlSatelliteStatus `json:"satelliteStatuses"`
}

// NodeStatus simple status of the node in the linstor cluster.
type NodeStatus struct {
	// Indicates if the node has been created on the controller.
	RegisteredOnController bool `json:"registeredOnController"`
	// The hostname of the kubelet running the node
	NodeName string `json:"nodeName"`
}

// SatelliteStatus should provide all the information that the reconsile loop
// needs to manage the operation of the LINSTOR Satellite.
type CtrlSatelliteStatus struct {
	NodeStatus
	// As indicated by Linstor
	ConnectionStatus string `json:"connectionStatus"`
	// StoragePoolStatuses by storage pool name.
	StoragePoolStatuses map[string]*StoragePoolStatus `json:"storagePoolStatus"`
}

// StoragePoolStatus reports basic information about storage pool state.
type CtrlStoragePoolStatus struct {
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

// PiraeusControllerSet is the Schema for the piraeuscontrollersets API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type PiraeusControllerSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PiraeusControllerSetSpec   `json:"spec,omitempty"`
	Status PiraeusControllerSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PiraeusControllerSetList contains a list of PiraeusControllerSet
type PiraeusControllerSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PiraeusControllerSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PiraeusControllerSet{}, &PiraeusControllerSetList{})
}
