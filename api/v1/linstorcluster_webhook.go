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
	"net/url"
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

var linstorclusterlog = logf.Log.WithName("linstorcluster-resource")

func (r *LinstorCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstorcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstorclusters,verbs=create;update,versions=v1,name=vlinstorcluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &LinstorCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorCluster) ValidateCreate() (admission.Warnings, error) {
	linstorclusterlog.Info("validate create", "name", r.Name)

	warnings, errs := r.validate(nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorCluster"}, r.Name, errs)
	}

	return warnings, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	linstorclusterlog.Info("validate update", "name", r.Name)

	warnings, errs := r.validate(nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorCluster"}, r.Name, errs)
	}

	return warnings, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorCluster) ValidateDelete() (admission.Warnings, error) {
	linstorclusterlog.Info("validate delete", "name", r.Name)

	return nil, nil
}

func (r *LinstorCluster) validate(old *LinstorCluster) (admission.Warnings, field.ErrorList) {
	errs := ValidateExternalController(r.Spec.ExternalController, field.NewPath("spec", "externalController"))
	errs = append(errs, ValidateNodeSelector(r.Spec.NodeSelector, field.NewPath("spec", "nodeSelector"))...)
	for i := range r.Spec.Patches {
		errs = append(errs, r.Spec.Patches[i].validate(field.NewPath("spec", "patches", strconv.Itoa(i)))...)
	}

	return nil, errs
}

func ValidateExternalController(ref *LinstorExternalControllerRef, path *field.Path) field.ErrorList {
	var result field.ErrorList

	if ref != nil {
		_, err := url.Parse(ref.URL)
		if err != nil {
			result = append(result, field.Invalid(path.Child("url"), ref.URL, err.Error()))
		}
	}

	return result
}
