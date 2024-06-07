package v1

import (
	"fmt"
	"strconv"
	"strings"

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

func ValidateNodeProperties(props []LinstorNodeProperty, path *field.Path) field.ErrorList {
	var result field.ErrorList

	for i := range props {
		p := &props[i]

		valSet := p.Value != ""
		fromSet := p.ValueFrom != nil
		if valSet == fromSet {
			result = append(result, field.Invalid(path.Child(strconv.Itoa(i)), p, "Expected exactly one of 'value' and 'valueFrom' to be set"))
		}

		if fromSet {
			_, keys, err := fieldpath.ExtractFieldPath(&corev1.Node{}, p.ValueFrom.NodeFieldRef)
			if err != nil {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "valueFrom", "nodeFieldRef"), p.ValueFrom.NodeFieldRef, fmt.Sprintf("Invalid reference format: %s", err)))
			}

			if keys != nil && !strings.Contains(p.Name, "$1") {
				result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "name"), p.Name, "Wildcard property requires replacement target `$1` in name"))
			}
		}
	}

	return result
}
