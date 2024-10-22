/*
Copyright 2022.

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

package v1

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var linstorsatelliteconfigurationlog = logf.Log.WithName("linstorsatelliteconfiguration-resource")

func SetupLinstorSatelliteConfigurationWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&piraeusv1.LinstorSatelliteConfiguration{}).
		WithValidator(&LinstorSatelliteConfigurationCustomValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstorsatelliteconfiguration,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstorsatelliteconfigurations,verbs=create;update,versions=v1,name=vlinstorsatelliteconfiguration.kb.io,admissionReviewVersions=v1

type LinstorSatelliteConfigurationCustomValidator struct{}

var _ webhook.CustomValidator = &LinstorSatelliteConfigurationCustomValidator{}

func (r *LinstorSatelliteConfigurationCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	satelliteConfiguration, ok := obj.(*piraeusv1.LinstorSatelliteConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatelliteConfiguration but got %T", obj)
	}

	linstorsatelliteconfigurationlog.Info("validate create", "name", satelliteConfiguration.GetName())

	warnings, errs := r.validate(satelliteConfiguration, nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(satelliteConfiguration.GroupVersionKind().GroupKind(), satelliteConfiguration.GetName(), errs)
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatelliteConfigurationCustomValidator) ValidateUpdate(ctx context.Context, obj, old runtime.Object) (admission.Warnings, error) {
	satelliteConfiguration, ok := obj.(*piraeusv1.LinstorSatelliteConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatelliteConfiguration but got %T", obj)
	}

	linstorsatelliteconfigurationlog.Info("validate update", "name", satelliteConfiguration.GetName())

	warnings, errs := r.validate(satelliteConfiguration, old.(*piraeusv1.LinstorSatelliteConfiguration))
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(satelliteConfiguration.GroupVersionKind().GroupKind(), satelliteConfiguration.GetName(), errs)
	}

	return warnings, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatelliteConfigurationCustomValidator) ValidateDelete(ctx context.Context, old runtime.Object) (admission.Warnings, error) {
	satelliteConfiguration, ok := old.(*piraeusv1.LinstorSatelliteConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatelliteConfiguration but got %T", old)
	}

	linstorsatelliteconfigurationlog.Info("validate delete", "name", satelliteConfiguration.GetName())

	return nil, nil
}

func (r *LinstorSatelliteConfigurationCustomValidator) validate(obj, old *piraeusv1.LinstorSatelliteConfiguration) (admission.Warnings, field.ErrorList) {
	var oldSPs []piraeusv1.LinstorStoragePool
	if old != nil {
		oldSPs = old.Spec.StoragePools
	}

	var warnings admission.Warnings

	errs := ValidateStoragePools(obj.Spec.StoragePools, oldSPs, field.NewPath("spec", "storagePools"))
	errs = append(errs, ValidateNodeSelector(obj.Spec.NodeSelector, field.NewPath("spec", "nodeSelector"))...)
	errs = append(errs, ValidateNodeProperties(obj.Spec.Properties, field.NewPath("spec", "properties"))...)
	errs = append(errs, ValidatePodTemplate(obj.Spec.PodTemplate, field.NewPath("spec", "podTemplate"))...)

	for i := range obj.Spec.Patches {
		path := field.NewPath("spec", "patches", strconv.Itoa(i))
		errs = append(errs, ValidatePatch(&obj.Spec.Patches[i], path)...)
		warnings = append(warnings, WarnOnBareSatellitePodPatch(&obj.Spec.Patches[i], path)...)
	}

	return warnings, errs
}

func ValidateNodeSelector(selector map[string]string, path *field.Path) field.ErrorList {
	var result field.ErrorList

	for k, v := range selector {
		errs := validation.IsQualifiedName(k)
		for _, e := range errs {
			result = append(result, field.Invalid(path, k, e))
		}

		errs = validation.IsValidLabelValue(v)
		for _, e := range errs {
			result = append(result, field.Invalid(path.Child(k), v, e))
		}
	}

	return result
}
