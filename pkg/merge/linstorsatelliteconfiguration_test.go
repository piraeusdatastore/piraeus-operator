package merge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/merge"
)

var (
	Config1 = piraeusv1.LinstorSatelliteConfiguration{
		Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
			NodeSelector: map[string]string{
				"config1": "true",
			},
			Patches: []piraeusv1.Patch{
				{Patch: "patch1"},
				{Patch: "patch2"},
			},
			StoragePools: []piraeusv1.LinstorStoragePool{
				{Name: "sp1", Lvm: &piraeusv1.LinstorStoragePoolLvm{VolumeGroup: "vg1"}, Source: &piraeusv1.LinstorStoragePoolSource{HostDevices: []string{"/dev/foobar"}}},
				{Name: "sp2", Lvm: &piraeusv1.LinstorStoragePoolLvm{VolumeGroup: "vg2"}},
			},
			Properties: []piraeusv1.LinstorNodeProperty{
				{Name: "prop1", Value: "config1"},
				{Name: "prop2", Value: "config1"},
			},
			InternalTLS: &piraeusv1.TLSConfig{
				SecretName: "config1",
			},
		},
	}
	Config2 = piraeusv1.LinstorSatelliteConfiguration{
		Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
			NodeSelector: map[string]string{
				"config2": "true",
			},
			Patches: []piraeusv1.Patch{
				{Patch: "patch3"},
			},
			StoragePools: []piraeusv1.LinstorStoragePool{
				{Name: "sp1", LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}},
			},
			Properties: []piraeusv1.LinstorNodeProperty{
				{Name: "prop1", Value: "config2"},
				{Name: "prop3", Value: "config2"},
			},
		},
	}
	Config3 = piraeusv1.LinstorSatelliteConfiguration{
		Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
			NodeSelector: map[string]string{
				"config3": "true",
			},
			Patches: []piraeusv1.Patch{
				{Patch: "patch4"},
				{Patch: "patch5"},
			},
			StoragePools: []piraeusv1.LinstorStoragePool{
				{Name: "sp2", LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{VolumeGroup: "vg2", ThinPool: "thin2"}},
				{Name: "sp3", Source: &piraeusv1.LinstorStoragePoolSource{HostDevices: []string{"/dev/bla"}}},
			},
			Properties: []piraeusv1.LinstorNodeProperty{
				{Name: "prop2", Value: "config3"},
				{Name: "prop3", Value: "config3"},
			},
			InternalTLS: &piraeusv1.TLSConfig{
				SecretName: "config3",
			},
		},
	}
)

func TestMergeSatelliteConfigurations(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name    string
		labels  map[string]string
		configs []piraeusv1.LinstorSatelliteConfiguration
		result  *piraeusv1.LinstorSatelliteConfiguration
	}{
		{
			name:   "empty",
			result: &piraeusv1.LinstorSatelliteConfiguration{},
		},
		{
			name:    "config-no-match",
			configs: []piraeusv1.LinstorSatelliteConfiguration{Config1, Config2, Config3},
			result:  &piraeusv1.LinstorSatelliteConfiguration{},
		},
		{
			name: "merge-all",
			labels: map[string]string{
				"config1": "true",
				"config2": "true",
				"config3": "true",
			},
			configs: []piraeusv1.LinstorSatelliteConfiguration{Config1, Config2, Config3},
			result: &piraeusv1.LinstorSatelliteConfiguration{
				Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
					Patches: []piraeusv1.Patch{
						{Patch: "patch1"},
						{Patch: "patch2"},
						{Patch: "patch3"},
						{Patch: "patch4"},
						{Patch: "patch5"},
					},
					StoragePools: []piraeusv1.LinstorStoragePool{
						{Name: "sp1", LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}},
						{Name: "sp2", LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{VolumeGroup: "vg2", ThinPool: "thin2"}},
						{Name: "sp3", Source: &piraeusv1.LinstorStoragePoolSource{HostDevices: []string{"/dev/bla"}}},
					},
					Properties: []piraeusv1.LinstorNodeProperty{
						{Name: "prop1", Value: "config2"},
						{Name: "prop2", Value: "config3"},
						{Name: "prop3", Value: "config3"},
					},
					InternalTLS: &piraeusv1.TLSConfig{
						SecretName: "config3",
					},
				},
			},
		},
		{
			name:    "filter",
			labels:  map[string]string{"config2": "true"},
			configs: []piraeusv1.LinstorSatelliteConfiguration{Config1, Config2, Config3},
			result: &piraeusv1.LinstorSatelliteConfiguration{
				Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
					Patches:      Config2.Spec.Patches,
					StoragePools: Config2.Spec.StoragePools,
					Properties:   Config2.Spec.Properties,
				},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := merge.SatelliteConfigurations(tcase.labels, tcase.configs...)
			assert.Equal(t, tcase.result, actual)
		})
	}
}
