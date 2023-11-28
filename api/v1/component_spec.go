package v1

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

func ValidatePodTemplate(template json.RawMessage, fieldPrefix *field.Path) field.ErrorList {
	if len(template) == 0 {
		return nil
	}

	var decoded corev1.PodTemplateSpec
	err := json.Unmarshal(template, &decoded)
	if err != nil {
		return field.ErrorList{field.Invalid(
			fieldPrefix,
			string(template),
			fmt.Sprintf("invalid pod template: %s", err),
		)}
	}

	return nil
}

func ValidateComponentSpec(curSpec *ComponentSpec, fieldPrefix *field.Path) field.ErrorList {
	if curSpec == nil {
		return nil
	}

	return ValidatePodTemplate(curSpec.PodTemplate, fieldPrefix.Child("podTemplate"))
}
