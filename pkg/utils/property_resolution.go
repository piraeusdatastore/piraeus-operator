package utils

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/fieldpath"
)

func ResolveNodeProperties(node *corev1.Node, props ...v1.LinstorNodeProperty) (map[string]string, error) {
	result := make(map[string]string)
	for i := range props {
		k := props[i].Name
		switch {
		case props[i].Value != "":
			result[k] = props[i].Value
		case props[i].ValueFrom != nil && props[i].ValueFrom.NodeFieldRef != "":
			vals, keys, err := fieldpath.ExtractFieldPath(node, props[i].ValueFrom.NodeFieldRef)
			if err != nil {
				return nil, err
			}

			if keys != nil && !strings.Contains(k, "$1") {
				return nil, fmt.Errorf("property name '%s' does not contain placeholder '$1'", k)
			}

			if keys != nil {
				for i, key := range keys {
					newK := strings.ReplaceAll(k, "$1", key)
					result[newK] = vals[i]
				}
			} else if len(vals) > 0 {
				result[k] = vals[0]
			} else if !props[i].Optional {
				result[k] = ""
			}
		}
	}

	return result, nil
}

func ResolveClusterProperties(defaults map[string]string, props ...v1.LinstorControllerProperty) map[string]string {
	result := make(map[string]string)

	for k, v := range defaults {
		result[k] = v
	}

	for i := range props {
		result[props[i].Name] = props[i].Value
	}

	return result
}
