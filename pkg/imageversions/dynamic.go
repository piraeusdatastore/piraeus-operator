package imageversions

import (
	"context"
	"fmt"
	"maps"
	"slices"

	lclient "github.com/LINBIT/golinstor/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

func FromConfigMap(ctx context.Context, client client.Client, name types.NamespacedName) (Configs, error) {
	var cfg corev1.ConfigMap
	err := client.Get(ctx, name, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image config map: %w", err)
	}

	var cfgs []*Config
	for _, name := range slices.Sorted(maps.Keys(cfg.Data)) {
		var config Config
		err = yaml.Unmarshal([]byte(cfg.Data[name]), &config)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image configuration: %w", err)
		}
		cfgs = append(cfgs, &config)
	}

	return cfgs, nil
}

// SetFromExternalCluster sets the LINSTOR Satellite image tag to the version of the LINSTOR Controller.
//
// The Configs will be updated so that the first entry containing a LINSTOR Satellite image is updated.
// Returns an error is the LINSTOR Controller could not be reached, or no LINSTOR Satellite image was found.
func SetFromExternalCluster(ctx context.Context, client *lclient.Client, configs Configs) error {
	version, err := client.Controller.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch controller version: %w", err)
	}

	newTag := "v" + version.Version
	logger := log.FromContext(ctx)

	for name, config := range configs {
		if v, ok := config.Components["linstor-satellite"]; ok {
			logger.WithValues("configName", name, "oldTag", v.Tag, "newTag", newTag).Info("updating image version based on external cluster")
			v.Tag = newTag
			config.Components["linstor-satellite"] = v
			return nil
		}
	}

	return fmt.Errorf("no linstor-satellite component found in configs")
}
