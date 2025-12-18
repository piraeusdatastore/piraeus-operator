package v1

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var (
	SPRegexp = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_-]{1,46}[A-Za-z0-9]$")
	VGRegexp = regexp.MustCompile("^[A-Za-z0-9.+_-]+$")
)

func ValidateStoragePools(curSPs, oldSPs []piraeusv1.LinstorStoragePool, fieldPrefix *field.Path) (admission.Warnings, field.ErrorList) {
	var result field.ErrorList
	var warnings admission.Warnings

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

		var oldSP *piraeusv1.LinstorStoragePool
		for j := range oldSPs {
			if oldSPs[j].Name == curSP.Name {
				oldSP = &oldSPs[j]
				break
			}
		}

		numPoolTypes := 0
		if curSP.LvmThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvmThinPool"))...)
			warns, errs := validateLinstorStoragePoolLvmThin(curSP.LvmThinPool, oldSP, curSP.Source, fieldPrefix.Child(strconv.Itoa(i), "lvmThinPool"))
			result = append(result, errs...)
			warnings = append(warnings, warns...)
		}

		if curSP.LvmPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "lvmPool"))...)
			warns, errs := validateLinstorStoragePoolLvm(curSP.LvmPool, oldSP, curSP.Source, fieldPrefix.Child(strconv.Itoa(i), "lvmPool"))
			result = append(result, errs...)
			warnings = append(warnings, warns...)
		}

		if curSP.FilePool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "filePool"))...)
			result = append(result, ValidateLinstorStoragePoolFile(curSP.FilePool, oldSP, fieldPrefix.Child(strconv.Itoa(i), "filePool"), curSP.Name, false)...)
			result = append(result, validateNoSource(curSP.Source, fieldPrefix.Child(strconv.Itoa(i)), "filePool")...)
		}

		if curSP.FileThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "fileThinPool"))...)
			result = append(result, ValidateLinstorStoragePoolFile(curSP.FileThinPool, oldSP, fieldPrefix.Child(strconv.Itoa(i), "fileThinPool"), curSP.Name, true)...)
			result = append(result, validateNoSource(curSP.Source, fieldPrefix.Child(strconv.Itoa(i)), "fileThinPool")...)
		}

		if curSP.ZfsPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "zfsPool"))...)
			warns, errs := curSP.ZfsPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i)), "zfsPool", false)
			result = append(result, errs...)
			warnings = append(warnings, warns...)
			warnings = append(warnings, validateCreateArgsRequireSource(curSP.Source, curSP.ZfsPool.ZpoolCreateArguments, fieldPrefix.Child(strconv.Itoa(i), "zfsPool", "zpoolCreateArguments"))...)
		}

		if curSP.ZfsThinPool != nil {
			result = append(result, validateStoragePoolType(&numPoolTypes, fieldPrefix.Child(strconv.Itoa(i), "zfsThinPool"))...)
			warns, errs := curSP.ZfsThinPool.Validate(oldSP, fieldPrefix.Child(strconv.Itoa(i)), "zfsThinPool", true)
			result = append(result, errs...)
			warnings = append(warnings, warns...)
			warnings = append(warnings, validateCreateArgsRequireSource(curSP.Source, curSP.ZfsThinPool.ZpoolCreateArguments, fieldPrefix.Child(strconv.Itoa(i), "zfsThinPool", "zpoolCreateArguments"))...)
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

	return warnings, result
}

func validateNoSource(src *piraeusv1.LinstorStoragePoolSource, p *field.Path, name string) field.ErrorList {
	if src != nil {
		return field.ErrorList{
			field.Invalid(p, src, fmt.Sprintf("Storage Pool Type '%s' does not support setting a source", name)),
		}
	}

	return nil
}

func validateCreateArgsRequireSource(src *piraeusv1.LinstorStoragePoolSource, args []string, p *field.Path) admission.Warnings {
	if len(args) != 0 && src == nil {
		return admission.Warnings{fmt.Sprintf("%s: Arguments without a pool source have no effect", p)}
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

func validateLinstorStoragePoolLvmThin(newSP *piraeusv1.LinstorStoragePoolLvmThin, oldSP *piraeusv1.LinstorStoragePool, src *piraeusv1.LinstorStoragePoolSource, fieldPrefix *field.Path) (admission.Warnings, field.ErrorList) {
	var errs field.ErrorList
	var warnings admission.Warnings

	if oldSP != nil && oldSP.LvmThinPool == nil {
		errs = append(errs, field.Forbidden(
			fieldPrefix,
			"Cannot change storage pool type",
		))
	}

	if newSP.VolumeGroup != "" && !VGRegexp.MatchString(newSP.VolumeGroup) {
		errs = append(errs, field.Invalid(
			fieldPrefix.Child("volumeGroup"),
			newSP.VolumeGroup,
			"Not a valid VG name",
		))
	}

	if oldSP != nil && oldSP.LvmThinPool != nil && newSP.VolumeGroup != oldSP.LvmThinPool.VolumeGroup {
		errs = append(errs, field.Forbidden(
			fieldPrefix.Child("volumeGroup"),
			"Cannot change VG name",
		))
	}

	if newSP.ThinPool != "" && !VGRegexp.MatchString(newSP.ThinPool) {
		errs = append(errs, field.Invalid(
			fieldPrefix.Child("thinPool"),
			newSP.ThinPool,
			"Not a valid thinpool LV name",
		))
	}

	if oldSP != nil && oldSP.LvmThinPool != nil && newSP.ThinPool != oldSP.LvmThinPool.ThinPool {
		errs = append(errs, field.Forbidden(
			fieldPrefix.Child("thinPool"),
			"Cannot change thinpool LV name",
		))
	}

	warnings = append(warnings, validateCreateArgsRequireSource(src, newSP.PhysicalVolumeCreateArguments, fieldPrefix.Child("physicalVolumeCreateArguments"))...)
	warnings = append(warnings, validateCreateArgsRequireSource(src, newSP.VolumeGroupCreateArguments, fieldPrefix.Child("volumeGroupCreateArguments"))...)
	warnings = append(warnings, validateCreateArgsRequireSource(src, newSP.LogicalVolumeCreateArguments, fieldPrefix.Child("logicalVolumeCreateArguments"))...)

	//nolint:govet // cannot deal with type inference on slices.Equal
	if oldSP != nil && oldSP.LvmThinPool != nil && !slices.Equal(newSP.PhysicalVolumeCreateArguments, oldSP.LvmThinPool.PhysicalVolumeCreateArguments) {
		warnings = append(warnings, fmt.Sprintf("%s: Update will only apply to new nodes", fieldPrefix.Child("physicalVolumeCreateArguments")))
	}

	//nolint:govet // cannot deal with type inference on slices.Equal
	if oldSP != nil && oldSP.LvmThinPool != nil && !slices.Equal(newSP.VolumeGroupCreateArguments, oldSP.LvmThinPool.VolumeGroupCreateArguments) {
		warnings = append(warnings, fmt.Sprintf("%s: Update will only apply to new nodes", fieldPrefix.Child("volumeGroupCreateArguments")))
	}

	//nolint:govet // cannot deal with type inference on slices.Equal
	if oldSP != nil && oldSP.LvmThinPool != nil && !slices.Equal(newSP.LogicalVolumeCreateArguments, oldSP.LvmThinPool.LogicalVolumeCreateArguments) {
		warnings = append(warnings, fmt.Sprintf("%s: Update will only apply to new nodes", fieldPrefix.Child("logicalVolumeCreateArguments")))
	}

	return warnings, errs
}

func validateLinstorStoragePoolLvm(newSP *piraeusv1.LinstorStoragePoolLvm, oldSP *piraeusv1.LinstorStoragePool, src *piraeusv1.LinstorStoragePoolSource, fieldPrefix *field.Path) (admission.Warnings, field.ErrorList) {
	var errs field.ErrorList
	var warnings admission.Warnings

	if oldSP != nil && oldSP.LvmPool == nil {
		errs = append(errs, field.Forbidden(
			fieldPrefix,
			"Cannot change storage pool type",
		))
	}

	if newSP.VolumeGroup != "" && !VGRegexp.MatchString(newSP.VolumeGroup) {
		errs = append(errs, field.Invalid(
			fieldPrefix.Child("volumeGroup"),
			newSP.VolumeGroup,
			"Not a valid VG name",
		))
	}

	if oldSP != nil && oldSP.LvmPool != nil && newSP.VolumeGroup != oldSP.LvmPool.VolumeGroup {
		errs = append(errs, field.Forbidden(
			fieldPrefix.Child("volumeGroup"),
			"Cannot change VG name",
		))
	}

	warnings = append(warnings, validateCreateArgsRequireSource(src, newSP.PhysicalVolumeCreateArguments, fieldPrefix.Child("physicalVolumeCreateArguments"))...)
	warnings = append(warnings, validateCreateArgsRequireSource(src, newSP.VolumeGroupCreateArguments, fieldPrefix.Child("volumeGroupCreateArguments"))...)

	//nolint:govet // cannot deal with type inference on slices.Equal
	if oldSP != nil && oldSP.LvmPool != nil && !slices.Equal(newSP.PhysicalVolumeCreateArguments, oldSP.LvmPool.PhysicalVolumeCreateArguments) {
		warnings = append(warnings, fmt.Sprintf("%s: Update will only apply to new nodes", fieldPrefix.Child("physicalVolumeCreateArguments")))
	}

	//nolint:govet // cannot deal with type inference on slices.Equal
	if oldSP != nil && oldSP.LvmPool != nil && !slices.Equal(newSP.VolumeGroupCreateArguments, oldSP.LvmPool.VolumeGroupCreateArguments) {
		warnings = append(warnings, fmt.Sprintf("%s: Update will only apply to new nodes", fieldPrefix.Child("volumeGroupCreateArguments")))
	}

	return warnings, errs
}

func ValidateLinstorStoragePoolFile(newSP *piraeusv1.LinstorStoragePoolFile, oldSP *piraeusv1.LinstorStoragePool, fieldPrefix *field.Path, name string, thin bool) field.ErrorList {
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

	if !filepath.IsAbs(newSP.DirectoryOrDefault(name)) || filepath.Clean(newSP.DirectoryOrDefault(name)) != newSP.DirectoryOrDefault(name) {
		result = append(result, field.Invalid(
			fieldPrefix.Child("directory"),
			newSP.DirectoryOrDefault(name),
			"Not an absolute path",
		))
	}

	return result
}
