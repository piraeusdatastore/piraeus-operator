package v1

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	lclient "github.com/LINBIT/golinstor/client"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type LinstorStoragePool struct {
	// Name of the storage pool in linstor.
	//+kubebuilder:validation:MinLength=3
	Name string `json:"name"`

	// Properties to set on the storage pool.
	// +listType=map
	// +listMapKey=name
	// +patchMergeKey=name
	// +patchStrategy=merge
	Properties []LinstorNodeProperty `json:"properties,omitempty"`

	// Configures a LVM Volume Group as storage pool.
	// +kubebuilder:validation:Optional
	LvmPool *LinstorStoragePoolLvm `json:"lvmPool,omitempty"`
	// Configures a LVM Thin Pool as storage pool.
	// +kubebuilder:validation:Optional
	LvmThinPool *LinstorStoragePoolLvmThin `json:"lvmThinPool,omitempty"`
	// Configures a file system based storage pool, allocating a regular file per volume.
	// +kubebuilder:validation:Optional
	FilePool *LinstorStoragePoolFile `json:"filePool,omitempty"`
	// Configures a file system based storage pool, allocating a sparse file per volume.
	// +kubebuilder:validation:Optional
	FileThinPool *LinstorStoragePoolFile `json:"fileThinPool,omitempty"`
	// Configures a ZFS system based storage pool, allocating zvols from the given zpool.
	// +kubebuilder:validation:Optional
	ZfsPool *LinstorStoragePoolZfs `json:"zfsPool,omitempty"`
	// Configures a ZFS system based storage pool, allocating sparse zvols from the given zpool.
	// +kubebuilder:validation:Optional
	ZfsThinPool *LinstorStoragePoolZfs `json:"zfsThinPool,omitempty"`

	Source *LinstorStoragePoolSource `json:"source,omitempty"`
}

func (p *LinstorStoragePool) ProviderKind() lclient.ProviderKind {
	switch {
	case p.LvmPool != nil:
		return lclient.LVM
	case p.LvmThinPool != nil:
		return lclient.LVM_THIN
	case p.FilePool != nil:
		return lclient.FILE
	case p.FileThinPool != nil:
		return lclient.FILE_THIN
	case p.ZfsPool != nil:
		return lclient.ZFS
	case p.ZfsThinPool != nil:
		return lclient.ZFS_THIN
	}

	return ""
}

func (p *LinstorStoragePool) PoolName() string {
	switch {
	case p.LvmPool != nil:
		if p.LvmPool.VolumeGroup != "" {
			return p.LvmPool.VolumeGroup
		}

		return p.Name
	case p.LvmThinPool != nil:
		lvName := p.LvmThinPool.ThinPool
		if lvName == "" {
			lvName = p.Name
		}

		vgName := p.LvmThinPool.VolumeGroup
		if vgName == "" {
			vgName = fmt.Sprintf("linstor_%s", lvName)
		}

		return fmt.Sprintf("%s/%s", vgName, lvName)
	case p.FilePool != nil:
		return p.FilePool.DirectoryOrDefault(p.Name)
	case p.FileThinPool != nil:
		return p.FileThinPool.DirectoryOrDefault(p.Name)
	case p.ZfsPool != nil:
		if p.ZfsPool.ZPool == "" {
			return p.Name
		}
		return p.ZfsPool.ZPool
	case p.ZfsThinPool != nil:
		if p.ZfsThinPool.ZPool == "" {
			return p.Name
		}
		return p.ZfsThinPool.ZPool
	}
	return ""
}

type LinstorStoragePoolLvm struct {
	VolumeGroup string `json:"volumeGroup,omitempty"`
}

type LinstorStoragePoolLvmThin struct {
	VolumeGroup string `json:"volumeGroup,omitempty"`
	// ThinPool is the name of the thinpool LV (without VG prefix).
	ThinPool string `json:"thinPool,omitempty"`
}

type LinstorStoragePoolFile struct {
	// Directory is the path to the host directory used to store volume data.
	Directory string `json:"directory,omitempty"`
}

type LinstorStoragePoolZfs struct {
	// ZPool is the name of the ZFS zpool.
	ZPool string `json:"zPool,omitempty"`
}

type LinstorStoragePoolSource struct {
	// HostDevices is a list of device paths used to configure the given pool.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems:=1
	HostDevices []string `json:"hostDevices,omitempty"`
}

func (l *LinstorStoragePoolFile) DirectoryOrDefault(name string) string {
	if l.Directory == "" {
		return filepath.Join("/var/lib/linstor-pools", name)
	}

	return l.Directory
}

func (l *LinstorStoragePoolZfs) Validate(oldSP *LinstorStoragePool, fieldPrefix *field.Path, name string, thin bool) field.ErrorList {
	var result field.ErrorList

	if oldSP != nil {
		if thin && oldSP.ZfsThinPool == nil {
			result = append(result, field.Forbidden(fieldPrefix, "Cannot change storage pool type"))
		} else if !thin && oldSP.ZfsPool == nil {
			result = append(result, field.Forbidden(fieldPrefix, "Cannot change storage pool type"))
		}
	}

	return result
}

func (s *LinstorStoragePoolSource) Validate(oldSP *LinstorStoragePool, knownDevices sets.Set[string], fieldPrefix *field.Path) field.ErrorList {
	if s == nil {
		return nil
	}

	if oldSP != nil {
		if !reflect.DeepEqual(s, oldSP.Source) {
			return field.ErrorList{
				field.Forbidden(fieldPrefix, "Cannot change source"),
			}
		}
	}

	var result field.ErrorList

	if s.HostDevices != nil {
		for j, src := range s.HostDevices {
			if !strings.HasPrefix(src, "/dev/") {
				result = append(result, field.Invalid(
					fieldPrefix.Child("hostDevices", strconv.Itoa(j)),
					src,
					"Path not rooted in /dev",
				))
			}

			if path.Clean(src) != src {
				result = append(result, field.Invalid(
					fieldPrefix.Child("hostDevices", strconv.Itoa(j)),
					src,
					"Not an absolute device path",
				))
			}

			if knownDevices.Has(src) {
				result = append(result, field.Duplicate(
					fieldPrefix.Child("hostDevices", strconv.Itoa(j)),
					src,
				))
			}

			knownDevices.Insert(src)
		}
	} else {
		result = append(result, field.Required(
			fieldPrefix,
			"Must specify exactly 1 type of storage pool source",
		))
	}

	return result
}
