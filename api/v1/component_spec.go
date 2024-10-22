package v1

import (
	"encoding/json"
)

type ComponentSpec struct {
	// Enable the component.
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	Enabled bool `json:"enabled,omitempty"`

	// Template to apply to Pods of the component.
	//
	// The template is applied as a patch to the default deployment, so it can be "sparse", not listing any
	// containers or volumes that should remain unchanged.
	// See https://kubernetes.io/docs/concepts/workloads/pods/#pod-templates
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Schemaless
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	// +structType=atomic
	PodTemplate json.RawMessage `json:"podTemplate,omitempty"`
}

func (c *ComponentSpec) IsEnabled() bool {
	return c == nil || c.Enabled
}

func (c *ComponentSpec) GetTemplate() json.RawMessage {
	if c == nil {
		return nil
	}

	return c.PodTemplate
}
