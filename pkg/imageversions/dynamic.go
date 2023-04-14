package imageversions

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

func FromConfigMap(ctx context.Context, client client.Client, name types.NamespacedName) (*Config, error) {
	var cfg corev1.ConfigMap
	err := client.Get(ctx, name, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image config map: %w", err)
	}

	content, ok := cfg.Data["images.yaml"]
	if !ok {
		return nil, fmt.Errorf("failed to find images.yaml in image configuration config map")
	}

	var config Config
	err = yaml.Unmarshal([]byte(content), &config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image configuration: %w", err)
	}

	return &config, nil
}
