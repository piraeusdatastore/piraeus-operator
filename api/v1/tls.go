package v1

import cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

// TLSConfig configures TLS for a component.
type TLSConfig struct {
	// SecretName references a secret holding the TLS key and certificates.
	//+kubebuilder:validation:Optional
	SecretName string `json:"secretName,omitempty"`

	// CertManager references a cert-manager Issuer or ClusterIssuer.
	// If set, a Certificate resource will be created, provisioning the secret references in SecretName using the
	// issuer configured here.
	//+kubebuilder:validation:Optional
	CertManager *cmmetav1.ObjectReference `json:"certManager,omitempty"`
}
