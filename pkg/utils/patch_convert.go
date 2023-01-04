package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/yaml"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

func ToEncodedPatch(target *kusttypes.Selector, patch any) (*kusttypes.Patch, error) {
	switch p := patch.(type) {
	case string:
		return &kusttypes.Patch{
			Target: target,
			Patch:  p,
		}, nil
	default:
		encoded, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}

		return &kusttypes.Patch{
			Target: target,
			Patch:  string(encoded),
		}, nil
	}
}

func MakeKustPatches(patches ...piraeusiov1.Patch) []kusttypes.Patch {
	var result []kusttypes.Patch
	for i := range patches {
		result = append(result, kusttypes.Patch{
			Target:  makeKustSelector(patches[i].Target),
			Patch:   patches[i].Patch,
			Options: patches[i].Options,
		})
	}

	return result
}

func makeKustSelector(selector *piraeusiov1.Selector) *kusttypes.Selector {
	if selector == nil {
		return nil
	}

	return &kusttypes.Selector{
		ResId: resid.ResId{
			Gvk:       resid.NewGvk(selector.Group, selector.Version, selector.Kind),
			Name:      selector.Name,
			Namespace: selector.Namespace,
		},
		AnnotationSelector: selector.AnnotationSelector,
		LabelSelector:      selector.LabelSelector,
	}
}

func RenderPatches(params map[string]any, patches ...kusttypes.Patch) ([]kusttypes.Patch, error) {
	result := make([]kusttypes.Patch, len(patches))

	for i := range patches {
		var decoded any
		err := yaml.Unmarshal([]byte(patches[i].Patch), &decoded)
		if err != nil {
			return nil, err
		}

		replaced, err := replaceVal(params, decoded)
		if err != nil {
			return nil, err
		}

		p, err := yaml.Marshal(&replaced)
		if err != nil {
			return nil, err
		}

		result[i].Target = patches[i].Target
		result[i].Options = patches[i].Options
		result[i].Patch = string(p)
	}

	return result, nil
}

func replaceVal(params map[string]any, v any) (any, error) {
	switch vv := v.(type) {
	case string:
		if strings.HasPrefix(vv, "$") {
			replacement, ok := params[vv[1:]]
			if !ok {
				return nil, fmt.Errorf("parameter '%s' has no value", vv)
			}

			return replacement, nil
		}

		return vv, nil
	case []any:
		for i := range vv {
			r, err := replaceVal(params, vv[i])
			if err != nil {
				return nil, err
			}
			vv[i] = r
		}

		return vv, nil
	case map[string]any:
		for k, v := range vv {
			r, err := replaceVal(params, v)
			if err != nil {
				return nil, err
			}
			vv[k] = r
		}

		return vv, nil
	}

	return v, nil
}
