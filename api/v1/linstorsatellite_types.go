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
	InternalTLS *TLSConfigWithHandshakeDaemon `json:"internalTLS,omitempty"`

	// IPFamilies configures the IP Family (IPv4 or IPv6) to use to connect to the LINSTOR Satellite.
	//
	// If set, the control traffic between LINSTOR Controller and Satellite will use only the given IP Family.
	// If not set, the Operator will configure all families found in the Satellites Pods' Status.
	// +kubebuilder:validation:Optional
	IPFamilies []IPFamily `json:"ipFamilies,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=Retain
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`

	// +kubebuilder:validation:Optional
	EvacuationStrategy EvacuationStrategy `json:"evacuationStrategy,omitempty"`
}

// LinstorSatelliteStatus defines the observed state of LinstorSatellite
type LinstorSatelliteStatus struct {
	// Current LINSTOR Satellite state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// The number of volumes on this Satellite.
	//+kubebuilder:validation:Optional
	NumberOfVolumes *int32 `json:"numberOfVolumes"`

	// The number of snapshots on this Satellite.
	//+kubebuilder:validation:Optional
	NumberOfSnapshots *int32 `json:"numberOfSnapshots"`

	// The number of bytes in total in all storage pools on this Satellite.
	//+kubebuilder:validation:Optional
	TotalCapacityBytes *int64 `json:"availableCapacityBytes"`

	// The number of bytes free in all storage pools on this Satellite.
	//+kubebuilder:validation:Optional
	FreeCapacityBytes *int64 `json:"freeCapacityBytes"`

	// Capacity mirrors the information from TotalCapacityBytes and FreeCapacityBytes in a human-readable string.
	//+kubebuilder:validation:Optional
	Capacity string `json:"capacity"`

	// StorageProviders lists the storage providers (LVM, ZFS, etc...) this Satellite supports.
	//+kubebuilder:validation:Optional
	StorageProviders []string `json:"storageProviders,omitempty"`

	// DeviceLayers lists the device layers (LUKS, CACHE, etc...) this Satellite supports.
	//+kubebuilder:validation:Optional
	DeviceLayers []string `json:"deviceLayers,omitempty"`
}

type ClusterReference struct {
	// Name of the LinstorCluster resource controlling this satellite.
	Name string `json:"name,omitempty"`

	// ClientSecretName references the secret used by the operator to validate the https endpoint.
	ClientSecretName string `json:"clientSecretName,omitempty"`

	// CAReference configures the CA certificate to use when validating TLS certificates.
	// If not set, the TLS secret is expected to contain a "ca.crt" containing the CA certificate.
	//+kubebuilder:validation:Optional
	CAReference *CAReference `json:"caReference,omitempty"`

	// ExternalController references an external controller.
	// When set, the Operator uses the external cluster to register satellites.
	// +kubebuilder:validation:Optional
	ExternalController *LinstorExternalControllerRef `json:"externalController,omitempty"`
}

// LinstorSatellite is the Schema for the linstorsatellites API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Connected",type=string,JSONPath=`.status.conditions[?(@.type=='Available')].status`,description="If the LINSTOR Satellite is connected"
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=='Configured')].status`,description="If the LINSTOR Satellite is fully configured"
// +kubebuilder:printcolumn:name="Applied Configurations",type=string,JSONPath=`.metadata.annotations.piraeus\.io/applied-configurations`,description="The Satellite Configurations applied to this Satellite",priority=10
// +kubebuilder:printcolumn:name="Deletion Policy",type=string,JSONPath=`.spec.deletionPolicy`,description="The deletion policy of the Satellite"
// +kubebuilder:printcolumn:name="Used Capacity",type=string,JSONPath=`.status.capacity`,description="The used capacity on the node"
// +kubebuilder:printcolumn:name="Volumes",type=integer,JSONPath=`.status.numberOfVolumes`,description="The number of volumes on the node"
// +kubebuilder:printcolumn:name="Snapshots",type=integer,JSONPath=`.status.numberOfSnapshots`,description="The number of snapshots on the node",priority=10
// +kubebuilder:printcolumn:name="Storage Providers",type=string,JSONPath=`.status.storageProviders`,description="The storage providers supported by the node",priority=10
// +kubebuilder:printcolumn:name="Device Layers",type=string,JSONPath=`.status.deviceLayers`,description="The device layers supported by the node",priority=10
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
