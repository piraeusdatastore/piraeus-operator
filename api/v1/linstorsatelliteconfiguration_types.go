/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinstorSatelliteConfigurationSpec defines a partial, desired state of a LinstorSatelliteSpec.
//
// All the LinstorSatelliteConfiguration resources with matching NodeSelector will
// be merged into a single LinstorSatelliteSpec.
type LinstorSatelliteConfigurationSpec struct {
	// NodeSelector selects which LinstorSatellite resources this spec should be applied to.
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Patches is a list of kustomize patches to apply.
	//
	// See https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/ for how to create patches.
	// +kubebuilder:validation:Optional
	Patches []Patch `json:"patches,omitempty"`

	// StoragePools is a list of storage pools to configure on the node.
	// +kubebuilder:validation:Optional
	StoragePools []LinstorStoragePool `json:"storagePools,omitempty"`

	// Properties is a list of properties to set on the node.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// +patchMergeKey=name
	// +patchStrategy=merge
	Properties []LinstorNodeProperty `json:"properties,omitempty"`
}

// LinstorSatelliteConfigurationStatus defines the observed state of LinstorSatelliteConfiguration
type LinstorSatelliteConfigurationStatus struct {
	// Current LINSTOR Satellite Config state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// LinstorSatelliteConfiguration is the Schema for the linstorsatelliteconfigurations API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type LinstorSatelliteConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorSatelliteConfigurationSpec   `json:"spec,omitempty"`
	Status LinstorSatelliteConfigurationStatus `json:"status,omitempty"`
}

// LinstorSatelliteConfigurationList contains a list of LinstorSatelliteConfiguration
// +kubebuilder:object:root=true
type LinstorSatelliteConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorSatelliteConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorSatelliteConfiguration{}, &LinstorSatelliteConfigurationList{})
}
