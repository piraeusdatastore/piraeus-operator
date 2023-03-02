package imageversions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	kusttypes "sigs.k8s.io/kustomize/api/types"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/imageversions"
)

func TestConfigs_GetVersions(t *testing.T) {
	t.Parallel()

	base := imageversions.Config{
		Base: "repo.example.com/base",
		Components: map[imageversions.Component]imageversions.ComponentConfig{
			imageversions.LinstorSatellite: {
				Image: "satellite",
				Tag:   "v1",
			},
			imageversions.DrbdModuleLoader: {
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
				{Name: string(imageversions.LinstorSatellite), NewName: "repo.example.com/base/satellite", NewTag: "v1"},
				{Name: string(imageversions.DrbdModuleLoader), NewName: "repo.example.com/base/ubuntu", NewTag: "v2"},
			},
		},
		{
			name: "with-base-new-alma",
			base: "quay.io/example",
			os:   "AlmaLinux 9.0 (Emerald Puma)",
			expected: []kusttypes.Image{
				{Name: string(imageversions.LinstorSatellite), NewName: "quay.io/example/satellite", NewTag: "v1"},
				{Name: string(imageversions.DrbdModuleLoader), NewName: "quay.io/example/new-alma", NewTag: "v2"},
			},
			expectPrecompiled: true,
		},
		{
			name: "with-base-fallback",
			base: "quay.io/example2",
			os:   "Debian GNU/Linux 11 (bullseye)",
			expected: []kusttypes.Image{
				{Name: string(imageversions.LinstorSatellite), NewName: "quay.io/example2/satellite", NewTag: "v1"},
				{Name: string(imageversions.DrbdModuleLoader), NewName: "quay.io/example2/fallback", NewTag: "v2"},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual, precompiled, err := base.GetVersions(tcase.base, tcase.os)
			assert.NoError(t, err)
			assert.Equal(t, tcase.expectPrecompiled, precompiled)
			assert.ElementsMatch(t, tcase.expected, actual)
		})
	}
}
