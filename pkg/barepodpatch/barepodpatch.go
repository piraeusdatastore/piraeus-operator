package barepodpatch

import (
	"fmt"

	"sigs.k8s.io/yaml"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

// ConvertBarePodPatch converts a patch for a bare "satellite" pod into a patch for a "linstor-satellite" daemonset
//
// Bare pod patches are recognized by the detected target using "kind: Pod" and "name: satellite". It converts
// both strategic merge patches and JSON Patches to an equivalent patch targeting a DaemonSet "linstor-satellite".
func ConvertBarePodPatch(patches ...piraeusiov1.Patch) ([]piraeusiov1.Patch, error) {
	for i := range patches {
		target := patches[i].GetTarget()
		if target == nil {
			continue
		}

		if target.Kind != "Pod" || target.Name != "satellite" {
			continue
		}

		if convertedPatch := convertStrategicMergePatch([]byte(patches[i].Patch)); len(convertedPatch) > 0 {
			patches[i].Patch = string(convertedPatch)
		} else if convertedPatch := convertJsonPatch([]byte(patches[i].Patch)); len(convertedPatch) > 0 {
			patches[i].Patch = string(convertedPatch)
		} else {
			return nil, fmt.Errorf("failed to convert bare pod patch: %v", patches[i])
		}

		patches[i].Target = &piraeusiov1.Selector{
			Version: "v1",
			Group:   "apps",
			Kind:    "DaemonSet",
			Name:    "linstor-satellite",
		}
	}

	return patches, nil
}

type strategicDaemonSetPatch struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata map[string]any `json:"metadata,omitempty"`
			Spec     map[string]any `json:"spec,omitempty"`
		} `json:"template"`
	} `json:"spec"`
}

func newStrategicDaemonsetPatch(metadata, spec map[string]any) *strategicDaemonSetPatch {
	var result strategicDaemonSetPatch
	result.ApiVersion = "apps/v1"
	result.Kind = "DaemonSet"
	result.Metadata.Name = "linstor-satellite"
	result.Spec.Template.Metadata = metadata
	result.Spec.Template.Spec = spec
	return &result
}

func convertStrategicMergePatch(patch []byte) []byte {
	var decoded map[string]any
	err := yaml.Unmarshal(patch, &decoded)
	if err != nil {
		return nil
	}

	spec := decoded["spec"].(map[string]any)
	metadata := decoded["metadata"].(map[string]any)
	if spec == nil && metadata == nil {
		return nil
	}

	// Delete the name, as that needs to be set on bare pod patches, but we don't want them in the pod template
	delete(metadata, "name")

	encoded, err := yaml.Marshal(newStrategicDaemonsetPatch(metadata, spec))
	if err != nil {
		return nil
	}

	return encoded
}

func convertJsonPatch(patch []byte) []byte {
	var patches []utils.JsonPatch
	err := yaml.Unmarshal(patch, &patches)
	if err != nil {
		return nil
	}

	for i := range patches {
		if patches[i].Path != "" {
			patches[i].Path = "/spec/template" + patches[i].Path
		}

		if patches[i].From != "" {
			patches[i].From = "/spec/template" + patches[i].From
		}
	}

	encoded, err := yaml.Marshal(patches)
	if err != nil {
		return nil
	}

	return encoded
}
