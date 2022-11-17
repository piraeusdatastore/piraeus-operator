package imageversions

import (
	_ "embed"
	"fmt"
	"regexp"

	kusttypes "sigs.k8s.io/kustomize/api/types"
)

// Config represents a default image mapping used by the operator.
type Config struct {
	Base       string                        `yaml:"base"`
	Components map[Component]ComponentConfig `yaml:"components"`
}

type ComponentConfig struct {
	Tag   string    `yaml:"tag"`
	Match []OsMatch `yaml:"match"`
	Image string    `yaml:"image"`
}

type OsMatch struct {
	OsImage string `yaml:"osImage"`
	Image   string `yaml:"image"`
}

type Component string

const (
	LinstorController Component = "linstor-controller"
	LinstorSatellite  Component = "linstor-satellite"
	LinstorCSI        Component = "linstor-csi"
	DrbdReactor       Component = "drbd-reactor"
	DrbdModuleLoader  Component = "drbd-module-loader"
)

type notConfigured struct {
	c Component
}

func (n *notConfigured) Error() string {
	return fmt.Sprintf("missing configuration for component '%s'", n.c)
}

var _ error = &notConfigured{}

func (f *Config) GetVersions(base string, osImage string) ([]kusttypes.Image, error) {
	result := make([]kusttypes.Image, 0, len(f.Components))
	for c := range f.Components {
		name, tag, err := f.get(c, base, osImage)
		if err != nil {
			return nil, err
		}

		if name != "" {
			result = append(result, kusttypes.Image{
				Name:    string(c),
				NewName: name,
				NewTag:  tag,
			})
		}
	}

	return result, nil
}

func (f *Config) get(c Component, base string, osImage string) (string, string, error) {
	if base == "" {
		base = f.Base
	}

	img, ok := f.Components[c]
	if !ok {
		return "", "", &notConfigured{c: c}
	}

	for _, matchRule := range img.Match {
		if ok, _ := regexp.MatchString(matchRule.OsImage, osImage); ok {
			return fmt.Sprintf("%s/%s", base, matchRule.Image), img.Tag, nil
		}
	}

	if img.Image == "" {
		return "", "", nil
	}

	return fmt.Sprintf("%s/%s", base, img.Image), img.Tag, nil
}
