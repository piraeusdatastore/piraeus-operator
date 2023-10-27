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

type TLSConfigWithHandshakeDaemon struct {
	TLSConfig `json:",inline"`

	// TLSHandshakeDaemon enables tlshd for establishing TLS sessions for use by DRBD.
	//
	// If enabled, adds a new sidecar to the LINSTOR Satellite that runs the tlshd handshake daemon.
	// The daemon uses the TLS certificate and key to establish secure connections on behalf of DRBD.
	//+kubebuilder:validation:Optional
	TLSHandshakeDaemon bool `json:"tlsHandshakeDaemon,omitempty"`
}
