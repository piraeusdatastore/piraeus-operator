package v1

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/fieldpath"
)

func ValidateNodeProperties(props []piraeusv1.LinstorNodeProperty, path *field.Path) field.ErrorList {
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
