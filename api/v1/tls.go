package v1

// TLSConfig configures TLS for a component.
type TLSConfig struct {
	// SecretName references a secret holding the TLS key and certificates.
	//+kubebuilder:validation:Optional
	SecretName string `json:"secretName,omitempty"`
}
