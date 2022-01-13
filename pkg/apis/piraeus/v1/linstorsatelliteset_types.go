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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
)

// LinstorSatelliteSetSpec defines the desired state of a LinstorSatelliteSet.
type LinstorSatelliteSetSpec struct {
	// priorityClassName is the name of the PriorityClass for the node pods
	PriorityClassName shared.PriorityClassName `json:"priorityClassName"`

	// StoragePools is a list of StoragePools for LinstorSatelliteSet to manage.
	// +optional
	// +nullable
	StoragePools *shared.StoragePools `json:"storagePools"`

	// If set, the operator will automatically create storage pools of the specified type for all devices that can
	// be found. The name of the storage pools matches the device name. For example, all devices `/dev/sdc` will be
	// part of the `autopool-sdc` storage pool.
	// Note: Using this attribute is discouraged. Using the "storagePools" to set up devices allows for more control on
	// device creation.
	// +optional
	// +kubebuilder:validation:Enum=None;LVM;LVMTHIN;ZFS
	AutomaticStorageType string `json:"automaticStorageType"`

	// Name of k8s secret that holds the SSL key for a node (called `keystore.jks`) and
	// the trusted certificates (called `certificates.jks`)
	// +optional
	// +nullable
	SslConfig *shared.LinstorSSLConfig `json:"sslSecret"`

	// drbdRepoCred is the name of the kubernetes secret that holds the credential for the DRBD repositories
	DrbdRepoCred string `json:"drbdRepoCred"`

	// Pull policy applied to all pods started from this controller
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// satelliteImage is the image (location + tag) for the LINSTOR satellite container
	SatelliteImage string `json:"satelliteImage"`

	// Cluster URL of the linstor controller.
	// If not set, will be determined from the current resource name.
	// +optional
	ControllerEndpoint string `json:"controllerEndpoint"`

	// Resource requirements for the LINSTOR satellite container
	// +optional
	// +nullable
	Resources corev1.ResourceRequirements `json:"resources"`

	// kernelModuleInjectionImage is the image (location + tag) for the LINSTOR/DRBD kernel module injector
	// +optional
	KernelModuleInjectionImage string `json:"kernelModuleInjectionImage"`

	// kernelModuleInjectionMode selects the source for the DRBD kernel module
	// +kubebuilder:validation:Enum=None;Compile;ShippedModules;DepsOnly
	// +optional
	KernelModuleInjectionMode shared.KernelModuleInjectionMode `json:"kernelModuleInjectionMode"`

	// kernelModuleInjectionAdditionalSourceDirectory is the directory containing the kernel sources and config on the
	// host. It will be mounted read-only when the injection mode is Compile. If unset, defaults to /usr/src. To
	// disable the mount, specify "none".
	// +optional
	// +nullable
	KernelModuleInjectionAdditionalSourceDirectory string `json:"kernelModuleInjectionAdditionalSourceDirectory,omitempty"`

	// Resource requirements for the kernel module builder/injector container
	// +optional
	// +nullable
	KernelModuleInjectionResources corev1.ResourceRequirements `json:"kernelModuleInjectionResources"`

	// Affinity for scheduling the satellite pods
	// +optional
	// +nullable
	Affinity *corev1.Affinity `json:"affinity"`

	// Tolerations for scheduling the satellite pods
	// +optional
	// +nullable
	Tolerations []corev1.Toleration `json:"tolerations"`

	// Name of the service account to be used for the created pods
	// +optional
	ServiceAccountName string `json:"serviceAccountName"`

	// AdditionalEnv is a list of extra environments variables to pass to the satellite container
	// +optional
	// +nullable
	AdditionalEnv []corev1.EnvVar `json:"additionalEnv"`

	// MonitoringImage is the image used to export monitoring information from DRBD and Linstor.
	// +optional
	// +nullable
	MonitoringImage string `json:"monitoringImage"`

	// LogLevel sets the log level for deployed components.
	// +nullable
	// +optional
	// +kubebuilder:validation:Enum=error;warn;info;debug;trace
	LogLevel shared.LogLevel `json:"logLevel,omitempty"`

	shared.LinstorClientConfig `json:",inline"`
}

// LinstorSatelliteSetStatus defines the observed state of LinstorSatelliteSet
type LinstorSatelliteSetStatus struct {
	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
	// SatelliteStatuses by hostname.
	SatelliteStatuses []*shared.SatelliteStatus `json:"SatelliteStatuses"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorSatelliteSet is the Schema for the linstorsatellitesets API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorsatellitesets,scope=Namespaced
// +kubebuilder:storageversion
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
