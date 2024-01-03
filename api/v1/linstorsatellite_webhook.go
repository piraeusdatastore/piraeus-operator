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
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var linstorsatellitelog = logf.Log.WithName("linstorsatellite-resource")

func (r *LinstorSatellite) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstorsatellite,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstorsatellites,verbs=create;update,versions=v1,name=vlinstorsatellite.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &LinstorSatellite{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatellite) ValidateCreate() (admission.Warnings, error) {
	linstorsatellitelog.Info("validate create", "name", r.Name)

	warnings, errs := r.validate(nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorSatellite"}, r.Name, errs)
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatellite) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linstorsatellitelog.Info("validate update", "name", r.Name)

	warnings, errs := r.validate(old.(*LinstorSatellite))
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorSatellite"}, r.Name, errs)
	}

	return warnings, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorSatellite) ValidateDelete() (admission.Warnings, error) {
	linstorsatellitelog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r *LinstorSatellite) validate(old *LinstorSatellite) (admission.Warnings, field.ErrorList) {
	var oldSPs []LinstorStoragePool
	if old != nil {
		oldSPs = old.Spec.StoragePools
	}

	var warnings admission.Warnings

	errs := ValidateExternalController(r.Spec.ClusterRef.ExternalController, field.NewPath("spec", "clusterRef", "externalController"))
	errs = append(errs, ValidateStoragePools(r.Spec.StoragePools, oldSPs, field.NewPath("spec", "storagePools"))...)
	errs = append(errs, ValidateNodeProperties(r.Spec.Properties, field.NewPath("spec", "properties"))...)
	for i := range r.Spec.Patches {
		path := field.NewPath("spec", "patches", strconv.Itoa(i))
		errs = append(errs, r.Spec.Patches[i].validate(path)...)
		warnings = append(warnings, WarnOnBareSatellitePodPatch(&r.Spec.Patches[i], path)...)
	}

	return warnings, errs
}

func WarnOnBareSatellitePodPatch(patch *Patch, path *field.Path) admission.Warnings {
	target := patch.GetTarget()
	if target == nil {
		return nil
	}

	if target.Kind == "Pod" && target.Name == "satellite" {
		return admission.Warnings{fmt.Sprintf("Patch %s is targeting Pod 'satellite': consider targeting the DaemonSet 'linstor-satellite' instead", path)}
	}

	return nil
}
