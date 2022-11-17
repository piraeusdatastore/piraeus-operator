package v1

import (
	"fmt"
	"path"
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

	Lvm     *LinstorStoragePoolLvm     `json:"lvm,omitempty"`
	LvmThin *LinstorStoragePoolLvmThin `json:"lvmThin,omitempty"`

	Source *LinstorStoragePoolSource `json:"source,omitempty"`
}

func (p *LinstorStoragePool) ProviderKind() lclient.ProviderKind {
	switch {
	case p.Lvm != nil:
		return lclient.LVM
	case p.LvmThin != nil:
		return lclient.LVM_THIN
	}

	return ""
}

func (p *LinstorStoragePool) PoolName() string {
	switch {
	case p.Lvm != nil:
		if p.Lvm.VolumeGroup != "" {
			return p.Lvm.VolumeGroup
		}

		return p.Name
	case p.LvmThin != nil:
		lvName := p.LvmThin.ThinPool
		if lvName == "" {
			lvName = p.Name
		}

		vgName := p.LvmThin.VolumeGroup
		if vgName == "" {
			vgName = fmt.Sprintf("linstor_%s", lvName)
		}

		return fmt.Sprintf("%s/%s", vgName, lvName)
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

	spNames := sets.NewString()
	devNames := sets.NewString()

	for i := range curSPs {
		curSP := &curSPs[i]
		if !SPRegexp.MatchString(curSP.Name) {
			result = append(result, field.Invalid(
				fieldPrefix.Child(strconv.Itoa(i), "name"),
				curSP.Name,
				"Not a valid LINSTOR Storage Pool name",
			))
		}

		if spNames.Has(curSP.Name) {
			result = append(result, field.Duplicate(
				fieldPrefix.Child(strconv.Itoa(i), "name"),
				curSP.Name,
			))
		}

		spNames.Insert(curSP.Name)
		var oldSP *LinstorStoragePool
		for j := range oldSPs {
			if oldSPs[j].Name == curSP.Name {
				oldSP = &oldSPs[j]
				break
			}
		}

		numPoolTypes := 0
		if curSP.LvmThin != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvmThin"))...)
			result = append(result, curSP.LvmThin.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "lvmThin"))...)
		}

		if curSP.Lvm != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvm"))...)
			result = append(result, curSP.Lvm.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i), "lvm"))...)
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

	if oldSP != nil && oldSP.LvmThin == nil {
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

	if oldSP != nil && l.VolumeGroup != oldSP.LvmThin.VolumeGroup {
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

	if oldSP != nil && l.ThinPool != oldSP.LvmThin.ThinPool {
		result = append(result, field.Forbidden(
			fieldPrefix.Child("thinPool"),
			"Cannot change thinpool LV name",
		))
	}

	return result
}

func (l *LinstorStoragePoolLvm) Validate(oldSP *LinstorStoragePool, fieldPrefix *field.Path) field.ErrorList {
	var result field.ErrorList

	if oldSP != nil && oldSP.Lvm == nil {
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

	if oldSP != nil && l.VolumeGroup != oldSP.Lvm.VolumeGroup {
		result = append(result, field.Forbidden(
			fieldPrefix.Child("volumeGroup"),
			"Cannot change VG name",
		))
	}

	return result
}

func (s *LinstorStoragePoolSource) Validate(oldSP *LinstorStoragePool, knownDevices sets.String, fieldPrefix *field.Path) field.ErrorList {
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
