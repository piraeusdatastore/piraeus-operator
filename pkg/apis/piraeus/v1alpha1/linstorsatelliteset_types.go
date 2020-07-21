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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinstorSatelliteSetSpec defines the desired state of a LinstorSatelliteSet.
type LinstorSatelliteSetSpec struct {
	// priorityClassName is the name of the PriorityClass for the node pods
	PriorityClassName PriorityClassName `json:"priorityClassName"`

	// StoragePools is a list of StoragePools for LinstorNodeSet to manage.
	// +optional
	// +nullable
	StoragePools *StoragePools `json:"storagePools"`

	// If set, the operator will automatically create storage pools of the specified type for all devices that can
	// be found. The name of the storage pools matches the device name. For example, all devices `/dev/sdc` will be
	// part of the `sdc` storage pool.
	// +optional
	// +kubebuilder:validation:Enum=None;LVM;LVMTHIN;ZFS
	AutomaticStorageType string `json:"automaticStorageType"`

	// drbdKernelModuleInjectionMode selects the source for the DRBD kernel module
	// +kubebuilder:validation:Enum=None;Compile;ShippedModules;DepsOnly
	DRBDKernelModuleInjectionMode KernelModuleInjectionMode `json:"drbdKernelModuleInjectionMode"`

	// Name of k8s secret that holds the SSL key for a node (called `keystore.jks`) and
	// the trusted certificates (called `certificates.jks`)
	// +optional
	// +nullable
	SslConfig *LinstorSSLConfig `json:"sslSecret"`

	// drbdRepoCred is the name of the kubernetes secret that holds the credential for the DRBD repositories
	DrbdRepoCred string `json:"drbdRepoCred"`

	// Pull policy applied to all pods started from this controller
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// satelliteImage is the image (location + tag) for the LINSTOR satellite container
	SatelliteImage string `json:"satelliteImage"`

	// kernelModImage is the image (location + tag) for the LINSTOR/DRBD kernel module injector container
	KernelModImage string `json:"kernelModImage"`

	// Cluster URL of the linstor controller.
	// If not set, will be determined from the current resource name.
	// +optional
	ControllerEndpoint string `json:"controllerEndpoint"`

	// Resource requirements for the LINSTOR satellite pod
	// +optional
	// +nullable
	Resources corev1.ResourceRequirements `json:"resources"`

	// Affinity for scheduling the satellite pods
	// +optional
	// +nullable
	Affinity *corev1.Affinity `json:"affinity"`

	// Tolerations for scheduling the satellite pods
	// +optional
	// +nullable
	Tolerations []corev1.Toleration `json:"tolerations"`

	LinstorClientConfig `json:",inline"`
}

// LinstorSatelliteSetStatus defines the observed state of LinstorSatelliteSet
type LinstorSatelliteSetStatus struct {
	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
	// SatelliteStatuses by hostname.
	SatelliteStatuses []*SatelliteStatus `json:"SatelliteStatuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorSatelliteSet is the Schema for the linstorsatellitesets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorsatellitesets,scope=Namespaced
type LinstorSatelliteSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorSatelliteSetSpec   `json:"spec,omitempty"`
	Status LinstorSatelliteSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorSatelliteSetList contains a list of LinstorSatelliteSet.
type LinstorSatelliteSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorSatelliteSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorSatelliteSet{}, &LinstorSatelliteSetList{})
}
