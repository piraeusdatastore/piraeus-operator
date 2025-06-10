package imageversions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	kusttypes "sigs.k8s.io/kustomize/api/types"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/imageversions"
)

var (
	BaseConfig = imageversions.Config{
		Base: "repo.example.com/base",
		Components: map[string]imageversions.ComponentConfig{
			"linstor-satellite": {
				Image: "satellite",
				Tag:   "v1",
			},
			"drbd-module-loader": {
				Tag:   "v2",
				Image: "fallback",
				Match: []imageversions.OsMatch{
					{OsImage: "Ubuntu", Image: "ubuntu"},
					{OsImage: "AlmaLinux [7-8]", Image: "old-alma", Precompiled: true},
					{OsImage: "AlmaLinux 9", Image: "new-alma", Precompiled: true},
				},
			},
		},
	}
	OverrideConfig = imageversions.Config{
		Base: "example.com/override",
		Components: map[string]imageversions.ComponentConfig{
			"linstor-satellite": {
				Image: "different-satellite",
				Tag:   "v2",
			},
		},
	}
	DigestConfig = imageversions.Config{
		Base: "example.com/digest",
		Components: map[string]imageversions.ComponentConfig{
			"image-with-digest": {
				Image:  "fallback",
				Tag:    "v1",
				Digest: "sha256:abcd",
				Match: []imageversions.OsMatch{
					{OsImage: "Ubuntu", Image: "ubuntu"},
					{OsImage: "AlmaLinux 9", Image: "new-alma", Precompiled: true, Digest: "sha256:dcba"},
				},
			},
		},
	}
)

func TestConfig_GetVersions(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name              string
		base              string
		os                string
		expected          []kusttypes.Image
		expectPrecompiled bool
	}{
		{
			name: "default-ubuntu",
			os:   "Ubuntu 20.04.5 LTS",
			expected: []kusttypes.Image{
				{Name: "linstor-satellite", NewName: "repo.example.com/base/satellite", NewTag: "v1"},
				{Name: "drbd-module-loader", NewName: "repo.example.com/base/ubuntu", NewTag: "v2"},
			},
		},
		{
			name: "with-base-new-alma",
			base: "quay.io/example",
			os:   "AlmaLinux 9.0 (Emerald Puma)",
			expected: []kusttypes.Image{
				{Name: "linstor-satellite", NewName: "quay.io/example/satellite", NewTag: "v1"},
				{Name: "drbd-module-loader", NewName: "quay.io/example/new-alma", NewTag: "v2"},
			},
			expectPrecompiled: true,
		},
		{
			name: "with-base-fallback",
			base: "quay.io/example2",
			os:   "Debian GNU/Linux 11 (bullseye)",
			expected: []kusttypes.Image{
				{Name: "linstor-satellite", NewName: "quay.io/example2/satellite", NewTag: "v1"},
				{Name: "drbd-module-loader", NewName: "quay.io/example2/fallback", NewTag: "v2"},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual, precompiled := BaseConfig.GetVersions(tcase.base, tcase.os)
			assert.Equal(t, tcase.expectPrecompiled, precompiled)
			assert.ElementsMatch(t, tcase.expected, actual)
		})
	}
}

func TestConfigs_GetVersions_prefer_later_config(t *testing.T) {
	configs := imageversions.Configs{&BaseConfig, &OverrideConfig}
	actual, precompiled := configs.GetVersions("", "Ubuntu 20.04.5 LTS")
	assert.False(t, precompiled)
	assert.ElementsMatch(t, []kusttypes.Image{
		{Name: "linstor-satellite", NewName: "example.com/override/different-satellite", NewTag: "v2"},
		{Name: "drbd-module-loader", NewName: "repo.example.com/base/ubuntu", NewTag: "v2"},
	}, actual)

	reversedConfigs := imageversions.Configs{&OverrideConfig, &BaseConfig}
	actual, precompiled = reversedConfigs.GetVersions("", "Ubuntu 20.04.5 LTS")
	assert.False(t, precompiled)
	assert.ElementsMatch(t, []kusttypes.Image{
		{Name: "linstor-satellite", NewName: "repo.example.com/base/satellite", NewTag: "v1"},
		{Name: "drbd-module-loader", NewName: "repo.example.com/base/ubuntu", NewTag: "v2"},
	}, actual)
}

func TestConfigs_GetVersions_use_digests_if_set(t *testing.T) {
	t.Parallel()
	configs := imageversions.Configs{&DigestConfig}

	actual, _ := configs.GetVersions("", "SomeOs")
	assert.ElementsMatch(t, actual, []kusttypes.Image{
		{Name: "image-with-digest", NewName: "example.com/digest/fallback", NewTag: "v1", Digest: "sha256:abcd"},
	})

	actual, _ = configs.GetVersions("", "Ubuntu")
	assert.ElementsMatch(t, actual, []kusttypes.Image{
		{Name: "image-with-digest", NewName: "example.com/digest/ubuntu", NewTag: "v1"},
	})

	actual, _ = configs.GetVersions("", "AlmaLinux 9.0 (Emerald Puma)")
	assert.ElementsMatch(t, actual, []kusttypes.Image{
		{Name: "image-with-digest", NewName: "example.com/digest/new-alma", NewTag: "v1", Digest: "sha256:dcba"},
	})
}
