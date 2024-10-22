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
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var linstorsatellitelog = logf.Log.WithName("linstorsatellite-resource")

func SetupLinstorSatelliteWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&piraeusiov1.LinstorSatellite{}).
		WithValidator(&LinstorSatelliteCustomValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstorsatellite,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstorsatellites,verbs=create;update,versions=v1,name=vlinstorsatellite.kb.io,admissionReviewVersions=v1

type LinstorSatelliteCustomValidator struct{}

var _ webhook.CustomValidator = &LinstorSatelliteCustomValidator{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatelliteCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	satellite, ok := obj.(*piraeusiov1.LinstorSatellite)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatellite but got %T", obj)
	}

	linstorsatellitelog.Info("validate create", "name", satellite.GetName())

	warnings, errs := r.validate(satellite, nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(satellite.GroupVersionKind().GroupKind(), satellite.GetName(), errs)
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatelliteCustomValidator) ValidateUpdate(ctx context.Context, obj, old runtime.Object) (admission.Warnings, error) {
	satellite, ok := obj.(*piraeusiov1.LinstorSatellite)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatellite but got %T", obj)
	}

	linstorsatellitelog.Info("validate update", "name", satellite.GetName())

	warnings, errs := r.validate(satellite, old.(*piraeusiov1.LinstorSatellite))
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(satellite.GroupVersionKind().GroupKind(), satellite.GetName(), errs)
	}

	return warnings, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatelliteCustomValidator) ValidateDelete(ctx context.Context, old runtime.Object) (admission.Warnings, error) {
	satellite, ok := old.(*piraeusiov1.LinstorSatellite)
	if !ok {
		return nil, fmt.Errorf("expected LinstorSatellite but got %T", old)
	}

	linstorsatellitelog.Info("validate delete", "name", satellite.GetName())

	return nil, nil
}

func (r *LinstorSatelliteCustomValidator) validate(new, old *piraeusiov1.LinstorSatellite) (admission.Warnings, field.ErrorList) {
	var oldSPs []piraeusiov1.LinstorStoragePool
	if old != nil {
		oldSPs = old.Spec.StoragePools
	}

	var warnings admission.Warnings

	errs := ValidateExternalController(new.Spec.ClusterRef.ExternalController, field.NewPath("spec", "clusterRef", "externalController"))
	errs = append(errs, ValidateStoragePools(new.Spec.StoragePools, oldSPs, field.NewPath("spec", "storagePools"))...)
	errs = append(errs, ValidateNodeProperties(new.Spec.Properties, field.NewPath("spec", "properties"))...)
	for i := range new.Spec.Patches {
		path := field.NewPath("spec", "patches", strconv.Itoa(i))
		errs = append(errs, ValidatePatch(&new.Spec.Patches[i], path)...)
		warnings = append(warnings, WarnOnBareSatellitePodPatch(&new.Spec.Patches[i], path)...)
	}

	return warnings, errs
}

func WarnOnBareSatellitePodPatch(patch *piraeusiov1.Patch, path *field.Path) admission.Warnings {
	target := patch.GetTarget()
	if target == nil {
		return nil
	}

	if target.Kind == "Pod" && target.Name == "satellite" {
		return admission.Warnings{fmt.Sprintf("Patch %s is targeting Pod 'satellite': consider targeting the DaemonSet 'linstor-satellite' instead", path)}
	}

	return nil
}
