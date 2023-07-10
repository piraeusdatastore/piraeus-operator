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
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var linstornodeconnectionlog = logf.Log.WithName("linstornodeconnection-resource")

func (r *LinstorNodeConnection) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstornodeconnection,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstornodeconnections,verbs=create;update,versions=v1,name=vlinstornodeconnection.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &LinstorNodeConnection{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorNodeConnection) ValidateCreate() error {
	linstornodeconnectionlog.Info("validate create", "name", r.Name)

	errs := r.validate()
	if len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorNodeConnection"}, r.Name, errs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorNodeConnection) ValidateUpdate(old runtime.Object) error {
	linstornodeconnectionlog.Info("validate update", "name", r.Name)

	errs := r.validate()
	if len(errs) != 0 {
		return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "LinstorNodeConnection"}, r.Name, errs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *LinstorNodeConnection) ValidateDelete() error {
	linstornodeconnectionlog.Info("validate delete", "name", r.Name)

	return nil
}

func (r *LinstorNodeConnection) validate() field.ErrorList {
	errs := ValidateControllerProperties(r.Spec.Properties, field.NewPath("spec", "properties"))
	errs = append(errs, ValidateNodeConnectionPaths(r.Spec.Paths, field.NewPath("spec", "paths"))...)
	errs = append(errs, ValidateNodeConnectionSelectors(r.Spec.Selector, field.NewPath("spec", "selector"))...)

	return errs
}

func ValidateNodeConnectionPaths(paths []LinstorNodeConnectionPath, path *field.Path) field.ErrorList {
	var result field.ErrorList

	allNames := sets.New[string]()

	for i := range paths {
		p := &paths[i]
		if allNames.Has(p.Name) {
			result = append(result, field.Duplicate(path.Child(strconv.Itoa(i), "name"), p.Name))
		}

		allNames.Insert(p.Name)
	}

	return result
}

func ValidateNodeConnectionSelectors(selector []SelectorTerm, path *field.Path) field.ErrorList {
	var result field.ErrorList

	for i := range selector {
		for j := range selector[i].MatchLabels {
			switch selector[i].MatchLabels[j].Op {
			case MatchLabelSelectorOpExists, MatchLabelSelectorOpDoesNotExist, MatchLabelSelectorOpSame, MatchLabelSelectorOpNotSame:
				if len(selector[i].MatchLabels[j].Values) > 0 {
					result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "matchLabels", strconv.Itoa(j), "values"), selector[i].MatchLabels[j].Values, fmt.Sprintf("Chosen operator '%s' does not expect any values", selector[i].MatchLabels[j].Op)))
				}
			case MatchLabelSelectorOpIn, MatchLabelSelectorOpNotIn:
				// Nothing to check, empty values list is allowed
			default:
				result = append(result, field.NotSupported(path.Child(strconv.Itoa(i), "matchLabels", strconv.Itoa(j), "op"), selector[i].MatchLabels[j].Op, []string{
					string(MatchLabelSelectorOpExists),
					string(MatchLabelSelectorOpDoesNotExist),
					string(MatchLabelSelectorOpIn),
					string(MatchLabelSelectorOpNotIn),
					string(MatchLabelSelectorOpSame),
					string(MatchLabelSelectorOpNotSame),
				}))
			}
		}
	}

	return result
}
