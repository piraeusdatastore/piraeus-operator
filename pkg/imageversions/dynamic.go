package imageversions

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func FromConfigMap(ctx context.Context, client client.Client, name types.NamespacedName) (Configs, error) {
	var cfg corev1.ConfigMap
	err := client.Get(ctx, name, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image config map: %w", err)
	}

	var cfgs []*Config
	for name, content := range cfg.Data {
		var config Config
		err = yaml.Unmarshal([]byte(content), &config)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image configuration: %w", err)
		}
		config.source = name
		cfgs = append(cfgs, &config)
	}

	sort.Slice(cfgs, func(i, j int) bool {
		return cfgs[i].source < cfgs[j].source
	})

	return cfgs, nil
}
