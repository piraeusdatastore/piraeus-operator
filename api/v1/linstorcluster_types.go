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
	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinstorClusterSpec defines the desired state of LinstorCluster
type LinstorClusterSpec struct {
	// Repository used to pull workload images.
	// +kubebuilder:validation:Optional
	Repository string `json:"repository,omitempty"`

	// ExternalController references an external controller.
	// When set, the Operator will skip deploying a LINSTOR Controller and instead use the external cluster
	// to register satellites.
	// +kubebuilder:validation:Optional
	ExternalController *LinstorExternalControllerRef `json:"externalController,omitempty"`

	// NodeSelector selects the nodes on which LINSTOR Satellites will be deployed.
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// NodeAffinity selects the nodes on which LINSTOR Satellite will be deployed.
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	NodeAffinity *corev1.NodeSelector `json:"nodeAffinity,omitempty"`

	// Properties to apply on the cluster level.
	//
	// Use to create default settings for DRBD that should apply to all resources or to configure some other cluster
	// wide default.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// +patchMergeKey=name
	// +patchStrategy=merge
	Properties []LinstorControllerProperty `json:"properties,omitempty"`

	// Patches is a list of kustomize patches to apply.
	//
	// See https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/ for how to create patches.
	// +kubebuilder:validation:Optional
	Patches []Patch `json:"patches,omitempty"`

	// LinstorPassphraseSecret used to configure the LINSTOR master passphrase.
	//
	// The referenced secret must contain a single key "MASTER_PASSPHRASE". The master passphrase is used to
	// * Derive encryption keys for volumes using the LUKS layer.
	// * Store credentials for accessing remotes for backups.
	// See https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-encrypt_commands for more information.
	// +kubebuilder:validation:Optional
	LinstorPassphraseSecret string `json:"linstorPassphraseSecret,omitempty"`

	// InternalTLS secures the connection between LINSTOR Controller and Satellite.
	//
	// This configures the client certificate used when the Controller connects to a Satellite. This only has an effect
	// when the Satellite is configured to for secure connections using `LinstorSatellite.spec.internalTLS`.
	// +kubebuilder:validation:Optional
	// + See LinstorSatelliteSpec.InternalTLS for why nullable is needed.
	// +nullable
	InternalTLS *TLSConfig `json:"internalTLS,omitempty"`

	// ApiTLS secures the LINSTOR API.
	//
	// This configures the TLS key and certificate used to secure the LINSTOR API.
	// +kubebuilder:validation:Optional
	// + See LinstorSatelliteSpec.InternalTLS for why nullable is needed.
	// +nullable
	ApiTLS *LinstorClusterApiTLS `json:"apiTLS,omitempty"`
}

type LinstorExternalControllerRef struct {
	// URL of the external controller.
	//+kubebuilder:validation:MinLength=3
	URL string `json:"url"`
}

type LinstorClusterApiTLS struct {
	// ApiSecretName references a secret holding the TLS key and certificate used to protect the API.
	// Defaults to "linstor-api-tls".
	//+kubebuilder:validation:Optional
	ApiSecretName string `json:"apiSecretName,omitempty"`

	// ClientSecretName references a secret holding the TLS key and certificate used by the operator to configure
	// the cluster. Defaults to "linstor-client-tls".
	//+kubebuilder:validation:Optional
	ClientSecretName string `json:"clientSecretName,omitempty"`

	// CsiControllerSecretName references a secret holding the TLS key and certificate used by the CSI Controller
	// to provision volumes. Defaults to "linstor-csi-controller-tls".
	//+kubebuilder:validation:Optional
	CsiControllerSecretName string `json:"csiControllerSecretName,omitempty"`

	// CsiNodeSecretName references a secret holding the TLS key and certificate used by the CSI Nodes to query
	// the volume state. Defaults to "linstor-csi-node-tls".
	//+kubebuilder:validation:Optional
	CsiNodeSecretName string `json:"csiNodeSecretName,omitempty"`

	// CertManager references a cert-manager Issuer or ClusterIssuer.
	// If set, cert-manager.io/Certificate resources will be created, provisioning the secrets referenced in
	// *SecretName using the issuer configured here.
	//+kubebuilder:validation:Optional
	CertManager *cmmetav1.ObjectReference `json:"certManager,omitempty"`
}

func (l *LinstorClusterApiTLS) GetApiSecretName() string {
	if l.ApiSecretName == "" {
		return "linstor-api-tls"
	}

	return l.ApiSecretName
}

func (l *LinstorClusterApiTLS) GetClientSecretName() string {
	if l.ClientSecretName == "" {
		return "linstor-client-tls"
	}

	return l.ClientSecretName
}

func (l *LinstorClusterApiTLS) GetCsiControllerSecretName() string {
	if l.CsiControllerSecretName == "" {
		return "linstor-csi-controller-tls"
	}

	return l.CsiControllerSecretName
}

func (l *LinstorClusterApiTLS) GetCsiNodeSecretName() string {
	if l.CsiNodeSecretName == "" {
		return "linstor-csi-node-tls"
	}

	return l.CsiNodeSecretName
}

// LinstorClusterStatus defines the observed state of LinstorCluster
type LinstorClusterStatus struct {
	// Current LINSTOR Cluster state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// LinstorCluster is the Schema for the linstorclusters API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
type LinstorCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorClusterSpec   `json:"spec,omitempty"`
	Status LinstorClusterStatus `json:"status,omitempty"`
}

// LinstorClusterList contains a list of LinstorCluster
// +kubebuilder:object:root=true
type LinstorClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorCluster{}, &LinstorClusterList{})
}
