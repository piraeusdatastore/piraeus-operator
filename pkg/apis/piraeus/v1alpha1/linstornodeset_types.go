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
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LinstorNodeSetSpec defines the desired state of LinstorNodeSet
type LinstorNodeSetSpec struct {
	LinstorSatelliteSetSpec `json:",inline"`

	// kernelModImage is the image (location + tag) for the LINSTOR/DRBD kernel module injector container
	// DEPRECATED: use kernelModuleInjectionImage
	// +optional
	KernelModImage string `json:"kernelModImage"`

	// drbdKernelModuleInjectionMode selects the source for the DRBD kernel module
	// +kubebuilder:validation:Enum=None;Compile;ShippedModules;DepsOnly
	// DEPRECATED: use kernelModuleInjectionMode
	// +optional
	DRBDKernelModuleInjectionMode shared.KernelModuleInjectionMode `json:"drbdKernelModuleInjectionMode"`
}

// LinstorNodeSetStatus defines the observed state of LinstorNodeSet
type LinstorNodeSetStatus struct {
	LinstorSatelliteSetStatus `json:",inline"`

	// ResourceMigrated indicates that this LinstorNodeSet was already converted into a LinstorSatelliteSet.
	// +optional
	ResourceMigrated bool `json:"ResourceMigrated"`

	// DependantsMigrated indicated that all resources created from this LinstorNodeSet have a new owner.
	// +optional
	DependantsMigrated bool `json:"DependantsMigrated"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorNodeSet is the Schema for the linstornodesets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstornodesets,scope=Namespaced
// DEPRECATED: use LinstorSatelliteSet
type LinstorNodeSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorNodeSetSpec   `json:"spec,omitempty"`
	Status LinstorNodeSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorNodeSetList contains a list of LinstorNodeSet
// DEPRECATED: use LinstorSatelliteSetList.
type LinstorNodeSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorNodeSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorNodeSet{}, &LinstorNodeSetList{})
}
