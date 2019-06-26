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
