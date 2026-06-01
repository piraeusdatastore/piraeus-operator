package v1_test

import (
	"slices"
	"testing"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

func TestLinstorStoragePool_BackendProbeCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pool piraeusiov1.LinstorStoragePool
		want []string
	}{
		{
			name: "lvm",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1"}},
			want: []string{"vgs", "--noheadings", "--options", "vg_name"},
		},
		{
			name: "lvm-thin",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}},
			want: []string{"lvs", "--noheadings", "--separator", "/", "--options", "vg_name,lv_name"},
		},
		{
			name: "zfs",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", ZfsPool: &piraeusiov1.LinstorStoragePoolZfs{ZPool: "tank"}},
			want: []string{"zfs", "list", "-H", "-o", "name"},
		},
		{
			name: "zfs-thin",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", ZfsThinPool: &piraeusiov1.LinstorStoragePoolZfs{ZPool: "tank"}},
			want: []string{"zfs", "list", "-H", "-o", "name"},
		},
		{
			name: "file",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", FilePool: &piraeusiov1.LinstorStoragePoolFile{}},
			want: nil,
		},
		{
			name: "file-thin",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1", FileThinPool: &piraeusiov1.LinstorStoragePoolFile{}},
			want: nil,
		},
		{
			name: "empty",
			pool: piraeusiov1.LinstorStoragePool{Name: "pool1"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.pool.BackendProbeCommand()
			if !slices.Equal(got, tt.want) {
				t.Errorf("BackendProbeCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
