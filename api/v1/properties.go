package v1

import (
	"strconv"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	allNames := sets.NewString()

	for i := range props {
		p := &props[i]
		if allNames.Has(p.Name) {
			result = append(result, field.Duplicate(path.Child(strconv.Itoa(i), "name"), p.Name))
		}

		valSet := p.Value != ""
		fromSet := p.ValueFrom != nil
		if valSet == fromSet {
			result = append(result, field.Invalid(path.Child(strconv.Itoa(i)), p, "Expected exactly one of 'value' and 'valueFrom' to be set"))
		}
	}

	return result
}

func ValidateControllerProperties(props []LinstorControllerProperty, path *field.Path) field.ErrorList {
	var result field.ErrorList

	allNames := sets.NewString()

	for i := range props {
		p := &props[i]
		if allNames.Has(p.Name) {
			result = append(result, field.Duplicate(path.Child(strconv.Itoa(i), "name"), p.Name))
		}
	}

	return result
}
