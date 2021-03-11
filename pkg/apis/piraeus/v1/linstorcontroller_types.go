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

// LinstorControllerSpec defines the desired state of LinstorController
type LinstorControllerSpec struct {
	// priorityClassName is the name of the PriorityClass for the controller pods
	PriorityClassName shared.PriorityClassName `json:"priorityClassName"`

	// DBConnectionURL is the URL of the ETCD endpoint for LINSTOR Controller
	DBConnectionURL string `json:"dbConnectionURL"`

	// DBCertSecret is the name of the kubernetes secret that holds the CA certificate used to verify
	// the datatbase connection. The secret must contain a key "ca.pem" which holds the certificate in
	// PEM format
	// +nullable
	// +optional
	DBCertSecret string `json:"dbCertSecret"`

	// Use a TLS client certificate for authentication with the database (etcd). If set to true,
	// `dbCertSecret` must be set and contain two additional entries "client.cert" (PEM encoded)
	// and "client.key" (PKCS8 encoded, without passphrase).
	// +optional
	DBUseClientCert bool `json:"dbUseClientCert"`

	// Name of the secret containing the master passphrase for LUKS devices as `MASTER_PASSPHRASE`
	// +nullable
	// +optional
	LuksSecret string `json:"luksSecret"`

	// Name of k8s secret that holds the SSL key for a node (called `keystore.jks`) and the
	// trusted certificates (called `certificates.jks`)
	// +nullable
	// +optional
	SslConfig *shared.LinstorSSLConfig `json:"sslSecret"`

	// DrbdRepoCred is the name of the kubernetes secret that holds the credential for the
	// DRBD repositories
	DrbdRepoCred string `json:"drbdRepoCred"`

	// controllerImage is the image (location + tag) for the LINSTOR controller/server container
	ControllerImage string `json:"controllerImage"`

	// Pull policy applied to all pods started from this controller
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy"`

	// Name of the secret containing the java keystore (`keystore.jks`) used to enable HTTPS on the
	// controller. The controller will create a secured https endpoint on port 3371 with the key
	// stored in `keystore.jks`. The keystore must be secured using the passphrase "linstor". Also
	// needs to contain a truststore `truststore.jks`, which will be used to authenticate clients.
	// +optional
	LinstorHttpsControllerSecret string `json:"linstorHttpsControllerSecret"`

	// Resource requirements for the LINSTOR controller pod
	// +optional
	// +nullable
	Resources corev1.ResourceRequirements `json:"resources"`

	// Affinity for scheduling the controller pod
	// +optional
	// +nullable
	Affinity *corev1.Affinity `json:"affinity"`

	// Tolerations for scheduling the controller pod
	// +optional
	// +nullable
	Tolerations []corev1.Toleration `json:"tolerations"`

	// Number of replicas in the controller deployment
	// +optional
	// +nullable
	Replicas *int32 `json:"replicas"`

	// AdditionalEnv is a list of extra environments variables to pass to the controller container
	// +optional
	// +nullable
	AdditionalEnv []corev1.EnvVar `json:"additionalEnv"`

	// AdditionalProperties is a map of additional properties to set on the Linstor controller
	// +optional
	// +nullable
	AdditionalProperties map[string]string `json:"additionalProperties"`

	// Name of the service account that runs leader elections for linstor
	// +optional
	ServiceAccountName string `json:"serviceAccountName"`

	shared.LinstorClientConfig `json:",inline"`
}

// LinstorControllerStatus defines the observed state of LinstorController
type LinstorControllerStatus struct {
	// Errors remaining that will trigger reconciliations.
	Errors []string `json:"errors"`
	// ControllerStatus information.
	ControllerStatus *shared.NodeStatus `json:"ControllerStatus"`
	// SatelliteStatuses by hostname.
	SatelliteStatuses []*shared.SatelliteStatus `json:"SatelliteStatuses"`
	// properties set on the Linstor controller
	ControllerProperties map[string]string `json:"controllerProperties"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorController is the Schema for the linstorcontrollers API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=linstorcontrollers,scope=Namespaced
// +kubebuilder:storageversion
type LinstorController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorControllerSpec   `json:"spec,omitempty"`
	Status LinstorControllerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LinstorControllerList contains a list of LinstorController
type LinstorControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorController `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorController{}, &LinstorControllerList{})
}
