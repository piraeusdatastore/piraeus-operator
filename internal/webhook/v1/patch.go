package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

func ValidatePatch(patch *piraeusv1.Patch, path *field.Path) field.ErrorList {
	var result field.ErrorList

	_, smErr := patch.GetStrategicMergePatch()
	_, jsErr := patch.GetJsonPatch()
	if smErr != nil && jsErr != nil {
		result = append(result, field.Invalid(path.Child("patch"), patch.Patch, fmt.Sprintf("Failed to parse patch as either Strategic Merge Patch (%s) or JSON Patch (%s)", smErr, jsErr)))
	}

	if patch.GetTarget() == nil {
		result = append(result, field.Required(path.Child("target"), "Patch does not have a target and is not a valid Strategic Merge Patch"))
	}

	return result
}
