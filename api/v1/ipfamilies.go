package v1

import corev1 "k8s.io/api/core/v1"

// IPFamily represents the IP Family (IPv4 or IPv6).
// +kubebuilder:validation:Enum:=IPv4;IPv6
type IPFamily corev1.IPFamily
