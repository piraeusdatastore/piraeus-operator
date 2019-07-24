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

package v1alpha1

import (
	"reflect"
	"testing"

	lapi "github.com/LINBIT/golinstor/client"
)

func TestToLinstorStoragePool(t *testing.T) {
	var tableTest = []struct {
		from     StoragePool
		expected lapi.StoragePool
	}{
		{
			&StoragePoolLVM{
				Name:        "test0",
				VolumeGroup: "test0VolumeGroup",
			},
			lapi.StoragePool{
				StoragePoolName: "test0",
				ProviderKind:    lapi.LVM,
				Props: map[string]string{
					"StorDriver/LvmVg": "test0VolumeGroup",
				},
			},
		},
		{
			&StoragePoolLVMThin{
				StoragePoolLVM: StoragePoolLVM{
					Name:        "test0",
					VolumeGroup: "test0VolumeGroup",
				},
				ThinVolume: "test0ThinPool",
			},
			lapi.StoragePool{
				StoragePoolName: "test0",
				ProviderKind:    lapi.LVM_THIN,
				Props: map[string]string{
					"StorDriver/LvmVg":    "test0VolumeGroup",
					"StorDriver/ThinPool": "test0ThinPool",
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
