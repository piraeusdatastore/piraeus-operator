package imageversions

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"

	imageutil "sigs.k8s.io/kustomize/api/pkg/util"
	kusttypes "sigs.k8s.io/kustomize/api/types"
)

const ImageOverrideEnvPrefix = "RELATED_IMAGE_"

// Configs is a list of Config, where later
type Configs []*Config

// Config represents a default image mapping used by the operator.
type Config struct {
	Base       string                     `yaml:"base"`
	Components map[string]ComponentConfig `yaml:"components"`
}

type ComponentConfig struct {
	Tag    string    `yaml:"tag"`
	Match  []OsMatch `yaml:"match"`
	Image  string    `yaml:"image"`
	Digest string    `yaml:"digest,omitempty"`
}

type OsMatch struct {
	OsImage     string `yaml:"osImage"`
	Image       string `yaml:"image"`
	Precompiled bool   `yaml:"precompiled"`
	Digest      string `yaml:"digest,omitempty"`
}

func (c Configs) GetVersions(base string, osImage string) ([]kusttypes.Image, bool) {
	uniqImages := make(map[string]*kusttypes.Image)
	precompiled := false

	for _, cfg := range c {
		imgs, compiled := cfg.GetVersions(base, osImage)
		precompiled = precompiled || compiled
		for i := range imgs {
			uniqImages[imgs[i].Name] = &imgs[i]
		}
	}

	result := make([]kusttypes.Image, 0, len(uniqImages))
	for _, img := range uniqImages {
		result = append(result, *img)
	}

	return result, precompiled
}

func (f *Config) GetVersions(base string, osImage string) ([]kusttypes.Image, bool) {
	result := make([]kusttypes.Image, 0, len(f.Components))

	precompiled := false

	for c := range f.Components {
		img, compiled := f.get(f.Components[c], base, c, osImage)

		precompiled = precompiled || compiled

		if img != nil {
			img.Name = c
			result = append(result, *img)
		}
	}

	return result, precompiled
}

func (f *Config) get(img ComponentConfig, base, component, osImage string) (*kusttypes.Image, bool) {
	if base == "" {
		base = f.Base
	}

	for _, matchRule := range img.Match {
		if ok, _ := regexp.MatchString(matchRule.OsImage, osImage); ok {
			if img := getOverrideForImageFromEnv(component, matchRule.Image); img != nil {
				return img, matchRule.Precompiled
			}

			return &kusttypes.Image{
				NewName: fmt.Sprintf("%s/%s", base, matchRule.Image),
				NewTag:  img.Tag,
				Digest:  matchRule.Digest,
			}, matchRule.Precompiled
		}
	}

	if img.Image == "" {
		return nil, false
	}

	if img := getOverrideForImageFromEnv(component, img.Image); img != nil {
		return img, false
	}

	return &kusttypes.Image{
		NewName: fmt.Sprintf("%s/%s", base, img.Image),
		NewTag:  img.Tag,
		Digest:  img.Digest,
	}, false
}

func getOverrideForImageFromEnv(component, image string) *kusttypes.Image {
	img := os.Getenv(ImageOverrideEnvPrefix + component + "_" + image)
	if img == "" {
		img = os.Getenv(ImageOverrideEnvPrefix + component)
	}

	if img == "" {
		return nil
	}

	name, tag, digest := imageutil.SplitImageName(img)
	return &kusttypes.Image{
		NewName: name,
		NewTag:  tag,
		Digest:  digest,
	}
}
