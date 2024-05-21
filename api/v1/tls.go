package v1

import (
	"fmt"

	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

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

	// CAReference configures the CA certificate to use when validating TLS certificates.
	// If not set, the TLS secret is expected to contain a "ca.crt" containing the CA certificate.
	//+kubebuilder:validation:Optional
	CAReference *CAReference `json:"caReference,omitempty"`
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

type CAReference struct {
	// Kind of the resource containing the CA Certificate, either a ConfigMap or Secret.
	// +kubebuilder:default:=Secret
	// +kubebuilder:validation:Enum:=ConfigMap;Secret
	// +kubebuilder:validation:Optional
	Kind string `json:"kind,omitempty"`
	// Name of the resource containing the CA Certificate.
	// +kubebuilder:validation:Required
	Name string `json:"name"`
	// Key to select in the resource.
	// Defaults to ca.crt if not specified.
	// +kubebuilder:default:=ca.crt
	// +kubebuilder:validation:Optional
	Key string `json:"key,omitempty"`
	// Optional specifies whether the resource and its key must exist.
	// +kubebuilder:validation:Optional
	Optional *bool `json:"optional,omitempty"`
}

func (c *CAReference) ToVolumeProjection(fallbackSecretName string) corev1.VolumeProjection {
	if c == nil {
		return corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: fallbackSecretName},
			},
		}
	}

	switch c.Kind {
	case "Secret":
		return corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: c.Name},
				Items: []corev1.KeyToPath{{
					Key:  c.Key,
					Path: cmmetav1.TLSCAKey,
				}},
				Optional: c.Optional,
			},
		}
	case "ConfigMap":
		return corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: c.Name},
				Items: []corev1.KeyToPath{{
					Key:  c.Key,
					Path: cmmetav1.TLSCAKey,
				}},
				Optional: c.Optional,
			},
		}
	default:
		panic(fmt.Sprintf("unknown CA kind %s", c.Kind))
	}
}

func (c *CAReference) ToEnvVarSource(fallbackSecretName string) *corev1.EnvVarSource {
	if c == nil {
		return &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: fallbackSecretName},
				Key:                  cmmetav1.TLSCAKey,
			},
		}
	}

	switch c.Kind {
	case "Secret":
		return &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: c.Name},
				Key:                  c.Key,
				Optional:             c.Optional,
			},
		}
	case "ConfigMap":
		return &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: c.Name},
				Key:                  c.Key,
				Optional:             c.Optional,
			},
		}
	default:
		panic(fmt.Sprintf("unknown CA kind %s", c.Kind))
	}
}
