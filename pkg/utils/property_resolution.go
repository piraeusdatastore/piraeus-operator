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
		case props[i].ValueFrom != nil:
			vals, keys, err := fieldpath.ExtractFieldPath(node, props[i].ValueFrom.NodeFieldRef)
			if err != nil {
				return nil, err
			}

			if keys != nil {
				return nil, fmt.Errorf("wildcards not allowed in 'valueFrom': '%s'", props[i].ValueFrom.NodeFieldRef)
			}

			if len(vals) > 0 {
				result[k] = vals[0]
			} else if !props[i].Optional {
				result[k] = ""
			}
		case props[i].ExpandFrom != nil:
			vals, keys, err := fieldpath.ExtractFieldPath(node, props[i].ExpandFrom.NodeFieldRef)
			if err != nil {
				return nil, err
			}

			if len(keys) != len(vals) {
				return nil, fmt.Errorf("wildcards are required in 'expandFrom': '%s'", props[i].ExpandFrom.NodeFieldRef)
			}

			if props[i].ExpandFrom.NameTemplate != "" {
				for j := range vals {
					key := props[i].ExpandFrom.NameTemplate
					key = strings.ReplaceAll(key, "$1", keys[j])
					key = strings.ReplaceAll(key, "$2", vals[j])
					val := props[i].ExpandFrom.ValueTemplate
					val = strings.ReplaceAll(val, "$1", keys[j])
					val = strings.ReplaceAll(val, "$2", vals[j])
					result[props[i].Name+key] = val
				}
			} else {
				toJoin := make([]string, 0, len(vals))
				for j := range vals {
					val := props[i].ExpandFrom.ValueTemplate
					val = strings.ReplaceAll(val, "$1", keys[j])
					val = strings.ReplaceAll(val, "$2", vals[j])
					toJoin = append(toJoin, val)
				}

				// No need to sort, keys (and their vals) are guaranteed in sorted order.
				result[k] = strings.Join(toJoin, props[i].ExpandFrom.Delimiter)
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
