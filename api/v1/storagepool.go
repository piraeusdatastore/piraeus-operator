package v1

import (
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
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

var (
	SPRegexp = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_-]{1,46}[A-Za-z0-9]$")
	VGRegexp = regexp.MustCompile("^[A-Za-z0-9.+_-]+$")
)

func ValidateStoragePools(curSPs, oldSPs []LinstorStoragePool, fieldPrefix *field.Path) field.ErrorList {
	var result field.ErrorList

	devNames := sets.New[string]()

	for i := range curSPs {
		curSP := &curSPs[i]
		if !SPRegexp.MatchString(curSP.Name) {
			result = append(result, field.Invalid(
				fieldPrefix.Child(strconv.Itoa(i), "name"),
				curSP.Name,
				"Not a valid LINSTOR Storage Pool name",
			))
		}

		var oldSP *LinstorStoragePool
		for j := range oldSPs {
			if oldSPs[j].Name == curSP.Name {
				oldSP = &oldSPs[j]
				break
			}
		}

		numPoolTypes := 0
		if curSP.LvmThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvmThinPool"))...)
			result = append(result, curSP.LvmThinPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "lvmThinPool"))...)
		}

		if curSP.LvmPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvmPool"))...)
			result = append(result, curSP.LvmPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "lvmPool"))...)
		}

		if curSP.FilePool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "filePool"))...)
			result = append(result, curSP.FilePool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "filePool"), curSP.Name, false)...)
			result = append(result, validateNoSource(curSP.Source, fieldPrefix.Child(strconv.Itoa(i)), "filePool")...)
		}

		if curSP.FileThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "fileThinPool"))...)
			result = append(result, curSP.FileThinPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "fileThinPool"), curSP.Name, true)...)
			result = append(result, validateNoSource(curSP.Source, fieldPrefix.Child(strconv.Itoa(i)), "fileThinPool")...)
		}

		if curSP.ZfsPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "zfsPool"))...)
			result = append(result, curSP.ZfsPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i)), "zfsPool", false)...)
		}

		if curSP.ZfsThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "zfsThinPool"))...)
			result = append(result, curSP.ZfsThinPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i)), "zfsThinPool", true)...)
		}

		if numPoolTypes == 0 {
			result = append(result, field.Required(
				fieldPrefix.Child(strconv.Itoa(i)),
				"Must specify exactly 1 type of storage pool",
			))
		}

		result = append(result,
			curSP.Source.Validate(oldSP, devNames, fieldPrefix.Child(strconv.Itoa(i), "source"))...,
		)
	}

	return result
}

func validateNoSource(src *LinstorStoragePoolSource, p *field.Path, name string) field.ErrorList {
	if src != nil {
		return field.ErrorList{
			field.Invalid(p, src, fmt.Sprintf("Storage Pool Type '%s' does not support setting a source", name)),
		}
	}

	return nil
}

func validateStoragePoolType(numPools *int, p *field.Path) field.ErrorList {
	*numPools++
	if *numPools > 1 {
		return field.ErrorList{
			field.Forbidden(p, "Must specify exactly 1 type of storage pool"),
		}
	}

	return nil
}

func (l *LinstorStoragePoolLvmThin) Validate(oldSP *LinstorStoragePool, fieldPrefix *field.Path) field.ErrorList {
	var result field.ErrorList

	if oldSP != nil && oldSP.LvmThinPool == nil {
		result = append(result, field.Forbidden(
			fieldPrefix,
			"Cannot change storage pool type",
		))
	}

	if l.VolumeGroup != "" && !VGRegexp.MatchString(l.VolumeGroup) {
		result = append(result, field.Invalid(
			fieldPrefix.Child("volumeGroup"),
			l.VolumeGroup,
			"Not a valid VG name",
		))
	}

	if oldSP != nil && l.VolumeGroup != oldSP.LvmThinPool.VolumeGroup {
		result = append(result, field.Forbidden(
			fieldPrefix.Child("volumeGroup"),
			"Cannot change VG name",
		))
	}

	if l.ThinPool != "" && !VGRegexp.MatchString(l.ThinPool) {
		result = append(result, field.Invalid(
			fieldPrefix.Child("thinPool"),
			l.ThinPool,
			"Not a valid thinpool LV name",
		))
	}

	if oldSP != nil && l.ThinPool != oldSP.LvmThinPool.ThinPool {
		result = append(result, field.Forbidden(
			fieldPrefix.Child("thinPool"),
			"Cannot change thinpool LV name",
		))
	}

	return result
}

func (l *LinstorStoragePoolLvm) Validate(oldSP *LinstorStoragePool, fieldPrefix *field.Path) field.ErrorList {
	var result field.ErrorList

	if oldSP != nil && oldSP.LvmPool == nil {
		result = append(result, field.Forbidden(
			fieldPrefix,
			"Cannot change storage pool type",
		))
	}

	if l.VolumeGroup != "" && !VGRegexp.MatchString(l.VolumeGroup) {
		result = append(result, field.Invalid(
			fieldPrefix.Child("volumeGroup"),
			l.VolumeGroup,
			"Not a valid VG name",
		))
	}

	if oldSP != nil && l.VolumeGroup != oldSP.LvmPool.VolumeGroup {
		result = append(result, field.Forbidden(
			fieldPrefix.Child("volumeGroup"),
			"Cannot change VG name",
		))
	}

	return result
}

func (l *LinstorStoragePoolFile) Validate(oldSP *LinstorStoragePool, fieldPrefix *field.Path, name string, thin bool) field.ErrorList {
	var result field.ErrorList

	if oldSP != nil {
		if thin && oldSP.FileThinPool == nil {
			result = append(result, field.Forbidden(
				fieldPrefix,
				"Cannot change storage pool type",
			))
		} else if !thin && oldSP.FilePool == nil {
			result = append(result, field.Forbidden(
				fieldPrefix,
				"Cannot change storage pool type",
			))
		}
	}

	if !filepath.IsAbs(l.DirectoryOrDefault(name)) || filepath.Clean(l.DirectoryOrDefault(name)) != l.DirectoryOrDefault(name) {
		result = append(result, field.Invalid(
			fieldPrefix.Child("directory"),
			l.DirectoryOrDefault(name),
			"Not an absolute path",
		))
	}

	return result
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
