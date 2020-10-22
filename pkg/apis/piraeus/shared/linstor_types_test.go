/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package shared_test

import (
	"reflect"
	"testing"

	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"

	lapi "github.com/LINBIT/golinstor/client"
)

func TestToLinstorStoragePool(t *testing.T) {
	tableTest := []struct {
		from     shared.StoragePool
		expected lapi.StoragePool
	}{
		{
			&shared.StoragePoolLVM{
				CommonStoragePoolOptions: shared.CommonStoragePoolOptions{
					Name: "test0",
				},
				VolumeGroup: "test0VolumeGroup",
			},
			lapi.StoragePool{
				StoragePoolName: "test0",
				ProviderKind:    lapi.LVM,
				Props: map[string]string{
					"StorDriver/LvmVg":                   "test0VolumeGroup",
					kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
				},
			},
		},
		{
			&shared.StoragePoolLVMThin{
				CommonStoragePoolOptions: shared.CommonStoragePoolOptions{
					Name: "test0",
				},
				VolumeGroup: "test0VolumeGroup",
				ThinVolume:  "test0ThinPool",
			},
			lapi.StoragePool{
				StoragePoolName: "test0",
				ProviderKind:    lapi.LVM_THIN,
				Props: map[string]string{
					"StorDriver/LvmVg":                   "test0VolumeGroup",
					"StorDriver/ThinPool":                "test0ThinPool",
					"StorDriver/StorPoolName":            "test0VolumeGroup/test0ThinPool",
					kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
				},
			},
		},
	}

	for _, tt := range tableTest {
		actual := tt.from.ToLinstorStoragePool()

		if !reflect.DeepEqual(tt.expected, actual) {
			t.Errorf("expected\n\t%+v\nto convert into\n\t%+v\ngot\n\t%+v",
				tt.from, tt.expected, actual)
		}
	}
}

func TestToPhysicalStorageCreate(t *testing.T) {
	tableTest := []struct {
		from     shared.PhysicalStorageCreator
		expected lapi.PhysicalStorageCreate
	}{
		{
			&shared.StoragePoolLVM{
				CommonStoragePoolOptions: shared.CommonStoragePoolOptions{
					Name: "test0",
				},
				RaidLevel:         "raid(-1)",
				VolumeGroup:       "test0VolumeGroup",
				VDO:               true,
				VdoSlabSizeKib:    23,
				VdoLogicalSizeKib: 127,
			},
			lapi.PhysicalStorageCreate{
				PoolName:          "test0VolumeGroup",
				ProviderKind:      lapi.LVM,
				RaidLevel:         "raid(-1)",
				VdoEnable:         true,
				VdoSlabSizeKib:    23,
				VdoLogicalSizeKib: 127,
				WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
					Name: "test0",
					Props: map[string]string{
						"StorDriver/LvmVg":                   "test0VolumeGroup",
						kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
					},
				},
			},
		},
		{
			&shared.StoragePoolLVMThin{
				CommonStoragePoolOptions: shared.CommonStoragePoolOptions{
					Name: "test0",
				},
				RaidLevel:   "raid(-1)",
				VolumeGroup: "test0VolumeGroup",
				ThinVolume:  "test0ThinPool",
			},
			lapi.PhysicalStorageCreate{
				PoolName:     "test0ThinPool",
				ProviderKind: lapi.LVM_THIN,
				RaidLevel:    "raid(-1)",
				WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
					Name: "test0",
					Props: map[string]string{
						"StorDriver/LvmVg":                   "test0VolumeGroup",
						"StorDriver/ThinPool":                "test0ThinPool",
						"StorDriver/StorPoolName":            "test0VolumeGroup/test0ThinPool",
						kubeSpec.LinstorRegistrationProperty: kubeSpec.Name,
					},
				},
			},
		},
	}

	for _, tt := range tableTest {
		actual := tt.from.ToPhysicalStorageCreate()

		if !reflect.DeepEqual(tt.expected, actual) {
			t.Errorf("expected\n\t%+v\nto convert into\n\t%+v\ngot\n\t%+v",
				tt.from, tt.expected, actual)
		}
	}
}
