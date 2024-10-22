package v1

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

func ValidatePodTemplate(template json.RawMessage, fieldPrefix *field.Path) field.ErrorList {
	if len(template) == 0 {
		return nil
	}

	var decoded corev1.PodTemplateSpec
	err := json.Unmarshal(template, &decoded)
	if err != nil {
		return field.ErrorList{field.Invalid(
			fieldPrefix,
			string(template),
			fmt.Sprintf("invalid pod template: %s", err),
		)}
	}

	return nil
}

func ValidateComponentSpec(curSpec *piraeusiov1.ComponentSpec, fieldPrefix *field.Path) field.ErrorList {
	if curSpec == nil {
		return nil
	}

	return ValidatePodTemplate(curSpec.PodTemplate, fieldPrefix.Child("podTemplate"))
}
