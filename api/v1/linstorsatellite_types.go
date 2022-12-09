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

// LinstorSatelliteSpec defines the desired state of LinstorSatellite
type LinstorSatelliteSpec struct {
	// ClusterRef references the LinstorCluster used to create this LinstorSatellite.
	ClusterRef ClusterReference `json:"clusterRef"`

	// Repository used to pull workload images.
	// +kubebuilder:validation:Optional
	Repository string `json:"repository,omitempty"`

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

	// InternalTLS configures secure communication for the LINSTOR Satellite.
	//
	// If set, the control traffic between LINSTOR Controller and Satellite will be encrypted using mTLS.
	// The Controller will use the client key from `LinstorCluster.spec.internalTLS` when connecting.
	// +kubebuilder:validation:Optional
	// + Without "nullable" the k8s API does not accept patches with 'internalTLS: {}', which seems to be a bug.
	// +nullable
	InternalTLS *TLSConfig `json:"internalTLS,omitempty"`
}

// LinstorSatelliteStatus defines the observed state of LinstorSatellite
type LinstorSatelliteStatus struct {
	// Current LINSTOR Satellite state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ClusterReference struct {
	// Name of the LinstorCluster resource controlling this satellite.
	Name string `json:"name,omitempty"`
}

// LinstorSatellite is the Schema for the linstorsatellites API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type LinstorSatellite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorSatelliteSpec   `json:"spec,omitempty"`
	Status LinstorSatelliteStatus `json:"status,omitempty"`
}

// LinstorSatelliteList contains a list of LinstorSatellite
// +kubebuilder:object:root=true
type LinstorSatelliteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorSatellite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorSatellite{}, &LinstorSatelliteList{})
}
