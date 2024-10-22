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

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var linstornodeconnectionlog = logf.Log.WithName("linstornodeconnection-resource")

func SetupLinstorNodeConnectionWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&piraeusv1.LinstorNodeConnection{}).
		WithValidator(&LinstorNodeConnectionCustomValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstornodeconnection,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstornodeconnections,verbs=create;update,versions=v1,name=vlinstornodeconnection.kb.io,admissionReviewVersions=v1

type LinstorNodeConnectionCustomValidator struct{}

var _ webhook.CustomValidator = &LinstorNodeConnectionCustomValidator{}

func (r *LinstorNodeConnectionCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	nodeConnection, ok := obj.(*piraeusv1.LinstorNodeConnection)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected LinstorNodeConnection, got %T", obj))
	}

	linstornodeconnectionlog.Info("validate create", "name", nodeConnection.GetName())

	warnings, errs := r.validate(nodeConnection, nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(nodeConnection.GroupVersionKind().GroupKind(), nodeConnection.GetName(), errs)
	}

	return warnings, nil
}

func (r *LinstorNodeConnectionCustomValidator) ValidateUpdate(ctx context.Context, obj, old runtime.Object) (admission.Warnings, error) {
	nodeConnection, ok := obj.(*piraeusv1.LinstorNodeConnection)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected LinstorNodeConnection, got %T", obj))
	}

	linstornodeconnectionlog.Info("validate update", "name", nodeConnection.GetName())

	warnings, errs := r.validate(nodeConnection, old.(*piraeusv1.LinstorNodeConnection))
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(nodeConnection.GroupVersionKind().GroupKind(), nodeConnection.GetName(), errs)
	}

	return warnings, nil
}

func (r *LinstorNodeConnectionCustomValidator) ValidateDelete(ctx context.Context, old runtime.Object) (admission.Warnings, error) {
	nodeConnection, ok := old.(*piraeusv1.LinstorNodeConnection)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected LinstorNodeConnection, got %T", old))
	}

	linstornodeconnectionlog.Info("validate delete", "name", nodeConnection.GetName())

	return nil, nil
}

func (r *LinstorNodeConnectionCustomValidator) validate(new, old *piraeusv1.LinstorNodeConnection) (admission.Warnings, field.ErrorList) {
	return nil, ValidateNodeConnectionSelectors(new.Spec.Selector, field.NewPath("spec", "selector"))
}

func ValidateNodeConnectionSelectors(selector []piraeusv1.SelectorTerm, path *field.Path) field.ErrorList {
	var result field.ErrorList

	for i := range selector {
		for j := range selector[i].MatchLabels {
			switch selector[i].MatchLabels[j].Op {
			case piraeusv1.MatchLabelSelectorOpExists, piraeusv1.MatchLabelSelectorOpDoesNotExist, piraeusv1.MatchLabelSelectorOpSame, piraeusv1.MatchLabelSelectorOpNotSame:
				if len(selector[i].MatchLabels[j].Values) > 0 {
					result = append(result, field.Invalid(path.Child(strconv.Itoa(i), "matchLabels", strconv.Itoa(j), "values"), selector[i].MatchLabels[j].Values, fmt.Sprintf("Chosen operator '%s' does not expect any values", selector[i].MatchLabels[j].Op)))
				}
			case piraeusv1.MatchLabelSelectorOpIn, piraeusv1.MatchLabelSelectorOpNotIn:
				// Nothing to check, empty values list is allowed
			default:
				result = append(result, field.NotSupported(path.Child(strconv.Itoa(i), "matchLabels", strconv.Itoa(j), "op"), selector[i].MatchLabels[j].Op, []string{
					string(piraeusv1.MatchLabelSelectorOpExists),
					string(piraeusv1.MatchLabelSelectorOpDoesNotExist),
					string(piraeusv1.MatchLabelSelectorOpIn),
					string(piraeusv1.MatchLabelSelectorOpNotIn),
					string(piraeusv1.MatchLabelSelectorOpSame),
					string(piraeusv1.MatchLabelSelectorOpNotSame),
				}))
			}
		}
	}

	return result
}
