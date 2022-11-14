package utils

import (
	"encoding/json"

	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

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
