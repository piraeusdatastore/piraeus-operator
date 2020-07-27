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
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinstorCSIDriverSpec defines the desired state of LinstorCSIDriver
type LinstorCSIDriverSpec struct {
	// Name of the CSI external attacher image.
	// See https://kubernetes-csi.github.io/docs/external-attacher.html
	// +optional
	CSIAttacherImage string `json:"csiAttacherImage"`
	// Name of the CSI node driver registrar image.
	// See https://kubernetes-csi.github.io/docs/node-driver-registrar.html
	// +optional
	CSINodeDriverRegistrarImage string `json:"csiNodeDriverRegistrarImage"`
	// Name of the CSI external provisioner image.
	// See https://kubernetes-csi.github.io/docs/external-provisioner.html
	// +optional
	CSIProvisionerImage string `json:"csiProvisionerImage"`
	// Name of the CSI external snapshotter image.
	// See https://kubernetes-csi.github.io/docs/external-snapshotter.html
	// +optional
	CSISnapshotterImage string `json:"csiSnapshotterImage"`

	// Name of the CSI external resizer image.
	// See https://kubernetes-csi.github.io/docs/external-resizer.html
	// +optional
	CSIResizerImage string `json:"csiResizerImage"`

	// Name of a secret with authentication details for the `LinstorPluginImage` registry
	ImagePullSecret string `json:"imagePullSecret"`

	// Pull policy applied to all pods started from this controller
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// Image that contains the linstor-csi driver plugin
	LinstorPluginImage string `json:"linstorPluginImage"`

	// Name of the service account used by the CSI node pods
	// +optional
	CSINodeServiceAccountName string `json:"csiNodeServiceAccountName"`

	// Name of the service account used by the CSI controller pods
	// +optional
	CSIControllerServiceAccountName string `json:"csiControllerServiceAccountName"`

	// priorityClassName is the name of the PriorityClass for the csi driver pods
	// +optional
	PriorityClassName shared.PriorityClassName `json:"priorityClassName"`

	// controllerReplicas is the number of replicas created for the CSI controller
	// deployment.
	// +optional
	ControllerReplicas *int32 `json:"controllerReplicas"`

	// Cluster URL of the linstor controller.
	// If not set, will be determined from the current resource name.
	// +optional
	ControllerEndpoint string `json:"controllerEndpoint"`

	// Resource requirements for the csi pods.
	// The requirements are re-used for all pods (node/controller).
	// +optional
	// +nullable
	Resources corev1.ResourceRequirements `json:"resources"`

	// Affinity for scheduling the CSI node pods
	// +optional
	// +nullable
	NodeAffinity *corev1.Affinity `json:"nodeAffinity"`

	// Tolerations for scheduling CSI node pods
	// +optional
	// +nullable
	NodeTolerations []corev1.Toleration `json:"nodeTolerations"`

	// Affinity for scheduling the CSI controller pod
	// +optional
	// +nullable
	ControllerAffinity *corev1.Affinity `json:"controllerAffinity"`

	// Tolerations for schedluing CSI controller pods
	// +optional
	// +nullable
	ControllerTolerations []corev1.Toleration `json:"controllerTolerations"`

	shared.LinstorClientConfig `json:",inline"`
}

// LinstorCSIDriverStatus defines the observed state of LinstorCSIDriver
type LinstorCSIDriverStatus struct {
	// CSI node components ready status
	NodeReady bool `json:"NodeReady"`

	// CSI controller ready status
	ControllerReady bool `json:"ControllerReady"`

	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorCSIDriver is the Schema for the linstorcsidrivers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorcsidrivers,scope=Namespaced
// +kubebuilder:printcolumn:name="NodeReady",type="boolean",JSONPath=".status.NodeReady"
// +kubebuilder:printcolumn:name="ControllerReady",type="boolean",JSONPath=".status.ControllerReady"
// +kubebuilder:storageversion
type LinstorCSIDriver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorCSIDriverSpec   `json:"spec,omitempty"`
	Status LinstorCSIDriverStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorCSIDriverList contains a list of LinstorCSIDriver
type LinstorCSIDriverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorCSIDriver `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorCSIDriver{}, &LinstorCSIDriverList{})
}
