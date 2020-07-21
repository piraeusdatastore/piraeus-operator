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

// LinstorControllerSetSpec defines the desired state of LinstorControllerSet
type LinstorControllerSetSpec struct {
	LinstorControllerSpec `json:",inline"`
}

// LinstorControllerSetStatus defines the observed state of LinstorControllerSet
type LinstorControllerSetStatus struct {
	LinstorControllerStatus `json:",inline"`

	// ResourceMigrated indicates that this LinstorControllerSet was already converted into a LinstorController.
	// +optional
	ResourceMigrated bool `json:"ResourceMigrated"`

	// DependantsMigrated indicated that all resources created from this LinstorControllerSet have a new owner.
	// +optional
	DependantsMigrated bool `json:"DependantsMigrated"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorControllerSet is the Schema for the linstorcontrollersets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorcontrollersets,scope=Namespaced
// DEPRECATED: use LinstorController
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
