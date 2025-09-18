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

	// NodeAffinity selects the nodes on which LINSTOR Satellites will be deployed.
	// See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +kubebuilder:validation:Optional
	NodeAffinity *corev1.NodeSelector `json:"nodeAffinity,omitempty"`

	// Tolerations selects the nodes on which LINSTOR Satellites will be deployed.
	//
	// The default tolerations for DaemonSets are automatically added.
	// +kubebuilder:validation:Optional
	// +listType=atomic
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

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

	// Controller controls the deployment of the LINSTOR Controller Deployment.
	// +kubebuilder:validation:Optional
	Controller *ComponentSpec `json:"controller,omitempty"`

	// CSIController controls the deployment of the CSI Controller Deployment.
	// +kubebuilder:validation:Optional
	CSIController *DeploymentComponentSpec `json:"csiController,omitempty"`

	// CSINode controls the deployment of the CSI Node DaemonSet.
	// +kubebuilder:validation:Optional
	CSINode *ComponentSpec `json:"csiNode,omitempty"`

	// HighAvailabilityController controls the deployment of the High Availability Controller DaemonSet.
	// +kubebuilder:validation:Optional
	HighAvailabilityController *ComponentSpec `json:"highAvailabilityController,omitempty"`

	// AffinityController controls the deployment of the Affinity Controller Deployment.
	// +kubebuilder:validation:Optional
	AffinityController *DeploymentComponentSpec `json:"affinityController,omitempty"`

	// NFSServer controls the deployment of the LINSTOR CSI NFS Server DaemonSet.
	// +kubebuilder:validation:Optional
	NFSServer *ComponentSpec `json:"nfsServer,omitempty"`
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

	// AffinityControllerSecretName references a secret holding the TLS key and certificate used by the Affinity
	// Controller to monitor volume state. Defaults to "linstor-affinity-controller-tls".
	//+kubebuilder:validation:Optional
	AffinityControllerSecretName string `json:"affinityControllerSecretName,omitempty"`

	// NFSServerSecretName references a secret holding the TLS key and certificate used by the NFS Server to query
	// the cluster state. Defaults to "linstor-csi-nfs-server-tls".
	//+kubebuilder:validation:Optional
	NFSServerSecretName string `json:"nfsServerSecretName,omitempty"`

	// CertManager references a cert-manager Issuer or ClusterIssuer.
	// If set, cert-manager.io/Certificate resources will be created, provisioning the secrets referenced in
	// *SecretName using the issuer configured here.
	//+kubebuilder:validation:Optional
	CertManager *cmmetav1.ObjectReference `json:"certManager,omitempty"`

	// CAReference configures the CA certificate to use when validating TLS certificates.
	// If not set, the TLS secret is expected to contain a "ca.crt" containing the CA certificate.
	//+kubebuilder:validation:Optional
	CAReference *CAReference `json:"caReference,omitempty"`
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

func (l *LinstorClusterApiTLS) GetAffinityControllerSecretName() string {
	if l.AffinityControllerSecretName == "" {
		return "linstor-affinity-controller-tls"
	}

	return l.AffinityControllerSecretName
}

func (l *LinstorClusterApiTLS) GetNFSServerSecretName() string {
	if l.NFSServerSecretName == "" {
		return "linstor-csi-nfs-server-tls"
	}

	return l.NFSServerSecretName
}

// LinstorClusterStatus defines the observed state of LinstorCluster
type LinstorClusterStatus struct {
	// Current LINSTOR Cluster state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// The Version of the LINSTOR Cluster.
	//+kubebuilder:validation:Optional
	Version string `json:"version,omitempty"`

	// The number of LINSTOR Satellites that are expected to run.
	//+kubebuilder:validation:Optional
	ScheduledSatellites *int32 `json:"scheduledSatellites"`

	// The number of LINSTOR Satellites currently running.
	//+kubebuilder:validation:Optional
	RunningSatellites *int32 `json:"runningSatellites"`

	// The number of volumes in the LINSTOR Cluster.
	//+kubebuilder:validation:Optional
	NumberOfVolumes *int32 `json:"numberOfVolumes"`

	// The number of snapshots in the LINSTOR Cluster.
	//+kubebuilder:validation:Optional
	NumberOfSnapshots *int32 `json:"numberOfSnapshots"`

	// The number of bytes in total in all storage pools in the LINSTOR Cluster.
	//+kubebuilder:validation:Optional
	TotalCapacityBytes *int64 `json:"availableCapacityBytes"`

	// The number of bytes free in all storage pools in the LINSTOR Cluster.
	//+kubebuilder:validation:Optional
	FreeCapacityBytes *int64 `json:"freeCapacityBytes"`

	// Satellites mirrors the information from ScheduledSatellites and RunningSatellites in a human-readable string
	//+kubebuilder:validation:Optional
	Satellites string `json:"satellites"`

	// Capacity mirrors the information from TotalCapacityBytes and FreeCapacityBytes in a human-readable string
	//+kubebuilder:validation:Optional
	Capacity string `json:"capacity"`
}

// LinstorCluster is the Schema for the linstorclusters API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Available",type=string,JSONPath=`.status.conditions[?(@.type=='Available')].status`,description="If the LINSTOR Cluster is available"
// +kubebuilder:printcolumn:name="Configured",type=string,JSONPath=`.status.conditions[?(@.type=='Configured')].status`,description="If the LINSTOR Cluster is fully configured"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.version`,description="The version of the LINSTOR Cluster",priority=10
// +kubebuilder:printcolumn:name="Satellites",type=string,JSONPath=`.status.satellites`,description="The number of running/expected Satellites"
// +kubebuilder:printcolumn:name="Used Capacity",type=string,JSONPath=`.status.capacity`,description="The used capacity in all storage pools"
// +kubebuilder:printcolumn:name="Volumes",type=integer,JSONPath=`.status.numberOfVolumes`,description="The number of volumes in the cluster"
// +kubebuilder:printcolumn:name="Snapshots",type=integer,JSONPath=`.status.numberOfSnapshots`,description="The number of snapshots in the cluster",priority=10
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
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
