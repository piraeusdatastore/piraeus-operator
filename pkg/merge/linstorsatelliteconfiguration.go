package merge

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	schedulingcorev1 "k8s.io/component-helpers/scheduling/corev1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

// SatelliteConfigurations merges all configurations that apply based on the given node labels
//
// Merging happens by:
// * Concatenating all patches in the matching configs
// * Merging all properties by name. A property defined in a "later" config overrides previous property definitions.
// * Merging all storage pools by name. A storage pool defined in a "later" config overrides previous property definitions.
func SatelliteConfigurations(ctx context.Context, node *corev1.Node, configs ...piraeusv1.LinstorSatelliteConfiguration) *piraeusv1.LinstorSatelliteConfiguration {
	result := &piraeusv1.LinstorSatelliteConfiguration{}

	propsMap := make(map[string]*piraeusv1.LinstorNodeProperty)
	storPoolMap := make(map[string]*piraeusv1.LinstorStoragePool)

	for i := range configs {
		cfg := &configs[i]

		if !SubsetOf(cfg.Spec.NodeSelector, node.ObjectMeta.Labels) {
			continue
		}

		if cfg.Spec.NodeAffinity != nil {
			if matches, _ := schedulingcorev1.MatchNodeSelectorTerms(node, cfg.Spec.NodeAffinity); !matches {
				continue
			}
		}

		for j := range cfg.Spec.Properties {
			propsMap[cfg.Spec.Properties[j].Name] = &cfg.Spec.Properties[j]
		}

		for j := range cfg.Spec.StoragePools {
			storPoolMap[cfg.Spec.StoragePools[j].Name] = &cfg.Spec.StoragePools[j]
		}

		patch, err := ConvertTemplateToPatch(cfg.Spec.PodTemplate)
		if err != nil {
			log.FromContext(ctx, "config", cfg.Name).Error(err, "Failed to convert podTemplate to patch")
		} else if patch != nil {
			result.Spec.Patches = append(result.Spec.Patches, *patch)
		}

		result.Spec.Patches = append(result.Spec.Patches, cfg.Spec.Patches...)

		if cfg.Spec.InternalTLS != nil {
			result.Spec.InternalTLS = cfg.Spec.InternalTLS
		}
	}

	for _, v := range propsMap {
		result.Spec.Properties = append(result.Spec.Properties, *v)
	}

	sort.Slice(result.Spec.Properties, func(i, j int) bool {
		return result.Spec.Properties[i].Name < result.Spec.Properties[j].Name
	})

	for _, v := range storPoolMap {
		result.Spec.StoragePools = append(result.Spec.StoragePools, *v)
	}

	sort.Slice(result.Spec.StoragePools, func(i, j int) bool {
		return result.Spec.StoragePools[i].Name < result.Spec.StoragePools[j].Name
	})

	return result
}

func ConvertTemplateToPatch(podTemplate json.RawMessage) (*piraeusv1.Patch, error) {
	if len(podTemplate) == 0 {
		return nil, nil
	}

	var u unstructured.Unstructured
	err := json.Unmarshal(podTemplate, &u.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to convert podTemplate to unstructured: %w", err)
	}

	u.SetAPIVersion("v1")
	u.SetKind("Pod")
	u.SetName("satellite")

	encoded, err := u.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to encode podTemplate: %w", err)
	}

	return &piraeusv1.Patch{
		Target: &piraeusv1.Selector{
			Kind: "Pod",
			Name: "satellite",
		},
		Patch: string(encoded),
	}, nil
}

// SubsetOf returns true if all key and values in sub also appear in super.
func SubsetOf(sub, super map[string]string) bool {
	for k, subv := range sub {
		superv, ok := super[k]
		if !ok || superv != subv {
			return false
		}
	}

	return true
}
