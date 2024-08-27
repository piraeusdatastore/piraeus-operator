package v1

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/fieldpath"
)

type LinstorControllerProperty struct {
	// Name of the property to set.
	//+kubebuilder:validation:MinLength=1
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	// Value to set the property to.
	Value string `json:"value,omitempty"`
}

type LinstorNodeProperty struct {
	// Name of the property to set.
	//+kubebuilder:validation:MinLength=1
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	// Value to set the property to.
	//+kubebuilder:validation:Optional
	Value string `json:"value,omitempty"`

	// ValueFrom sets the value from an existing resource.
	//+kubebuilder:validation:Optional
	ValueFrom *LinstorNodePropertyValueFrom `json:"valueFrom,omitempty"`

	// ExpandFrom can reference multiple resource fields at once.
	// It either sets the property to an aggregate value based on matched resource fields, or expands to multiple
	// properties.
	//+kubebuilder:validation:Optional
	ExpandFrom *LinstorNodePropertyExpandFrom `json:"expandFrom,omitempty"`

	// Optional values are only set if they have a non-empty value
	//+kubebuilder:validation:Optional
	Optional bool `json:"optional,omitempty"`
}

type LinstorNodePropertyValueFrom struct {
	// Select a field of the node. Supports `metadata.name`, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`.
	//+kubebuilder:validation:MinLength=1
	//+kubebuilder:validation:Required
	NodeFieldRef string `json:"nodeFieldRef,omitempty"`
}

type LinstorNodePropertyExpandFrom struct {
	LinstorNodePropertyValueFrom `json:",inline"`

	// NameTemplate defines how the property key is expanded.
	// If set, the template is appended to the defined property name, creating multiple properties instead of one
	// aggregate.
	// * $1 is replaced with the matched key.
	// * $2 is replaced with the matched value.
	//+kubebuilder:validation:Optional
	NameTemplate string `json:"nameTemplate,omitempty"`

	// ValueTemplate defines how the property value is expanded.
	// * $1 is replaced with the matched key.
	// * $2 is replaced with the matched value.
	//+kubebuilder:validation:Optional
	ValueTemplate string `json:"valueTemplate,omitempty"`

	// Delimiter used to join multiple key and value pairs together.
	//+kubebuilder:validation:Optional
	Delimiter string `json:"delimiter,omitempty"`
}

func ValidateNodeProperties(props []LinstorNodeProperty, path *field.Path) field.ErrorList {
	var result field.ErrorList

	for i := range props {
		p := &props[i]

		sourcesSet := 0

		if p.Value != "" {
			sourcesSet++
		}

		if p.ValueFrom != nil {
			sourcesSet++
		}

		if p.ExpandFrom != nil {
			sourcesSet++
		}

		if sourcesSet != 1 {
			result = append(result, field.Invalid(path.Child(strconv.Itoa(i)), p, "Expected exactly one of 'value', 'valueFrom' or 'joinValuesFrom' to be set"))
		}

		if p.ValueFrom != nil {
			_, keys, err := fieldpath.ExtractFieldPath(&corev1.Node{}, p.ValueFrom.NodeFieldRef)
			if err != nil {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "valueFrom", "nodeFieldRef"), p.ValueFrom.NodeFieldRef, fmt.Sprintf("Invalid reference format: %s", err)))
			}

			if keys != nil {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "valueFrom", "nodeFieldRef"), p.ValueFrom.NodeFieldRef, "Wildcard property not allowed, use expandFrom instead"))
			}
		}

		if p.ExpandFrom != nil {
			_, keys, err := fieldpath.ExtractFieldPath(&corev1.Node{}, p.ExpandFrom.NodeFieldRef)
			if err != nil {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "expandFrom", "nodeFieldRef"), p.ExpandFrom.NodeFieldRef, fmt.Sprintf("Invalid reference format: %s", err)))
			}

			if keys == nil {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "expandFrom", "nodeFieldRef"), p.ExpandFrom.NodeFieldRef, "Wildcard property required"))
			}

			if p.ExpandFrom.NameTemplate != "" && p.ExpandFrom.Delimiter != "" {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "expandFrom"), p.ExpandFrom, "Expected only one of 'nameTemplate' and 'delimiter' to be set"))
			}
		}
	}

	return result
}
