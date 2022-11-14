package utils

import (
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/fieldpath"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

func ResolveNodeProperties(node *corev1.Node, props ...v1.LinstorNodeProperty) (map[string]string, error) {
	result := make(map[string]string)
	for i := range props {
		k := props[i].Name
		switch {
		case props[i].Value != "":
			result[k] = props[i].Value
		case props[i].ValueFrom != nil && props[i].ValueFrom.NodeFieldRef != "":
			val, err := fieldpath.ExtractFieldPathAsString(node, props[i].ValueFrom.NodeFieldRef)
			if err != nil {
				return nil, err
			}

			if !props[i].Optional || val != "" {
				result[k] = val
			}
		}
	}

	return result, nil
}

func ResolveClusterProperties(props ...v1.LinstorControllerProperty) map[string]string {
	result := make(map[string]string)

	for k, v := range vars.DefaultControllerProperties {
		result[k] = v
	}

	for i := range props {
		result[props[i].Name] = props[i].Value
	}

	return result
}
