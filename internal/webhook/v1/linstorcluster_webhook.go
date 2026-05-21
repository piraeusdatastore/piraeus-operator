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
	"strconv"
	"strings"

	lapi "github.com/LINBIT/golinstor/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var linstorclusterlog = logf.Log.WithName("linstorcluster-resource")

func SetupLinstorClusterWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &piraeusiov1.LinstorCluster{}).
		WithValidator(&LinstorClusterCustomValidator{}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-piraeus-io-v1-linstorcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=piraeus.io,resources=linstorclusters,verbs=create;update,versions=v1,name=vlinstorcluster.kb.io,admissionReviewVersions=v1

type LinstorClusterCustomValidator struct{}

var _ admission.Validator[*piraeusiov1.LinstorCluster] = &LinstorClusterCustomValidator{}

func (r *LinstorClusterCustomValidator) ValidateCreate(ctx context.Context, linstorCluster *piraeusiov1.LinstorCluster) (admission.Warnings, error) {
	linstorclusterlog.Info("validate create", "name", linstorCluster.GetName())

	warnings, errs := r.validate(linstorCluster, nil)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(linstorCluster.GroupVersionKind().GroupKind(), linstorCluster.GetName(), errs)
	}

	return warnings, nil
}

func (r *LinstorClusterCustomValidator) ValidateUpdate(ctx context.Context, oldLinstorCluster, linstorCluster *piraeusiov1.LinstorCluster) (admission.Warnings, error) {
	linstorclusterlog.Info("validate update", "name", linstorCluster.GetName())

	warnings, errs := r.validate(linstorCluster, oldLinstorCluster)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(linstorCluster.GroupVersionKind().GroupKind(), linstorCluster.GetName(), errs)
	}

	return warnings, nil
}

func (r *LinstorClusterCustomValidator) ValidateDelete(ctx context.Context, linstorCluster *piraeusiov1.LinstorCluster) (admission.Warnings, error) {
	linstorclusterlog.Info("validate delete", "name", linstorCluster.GetName())

	return nil, nil
}

func (r *LinstorClusterCustomValidator) validate(current, old *piraeusiov1.LinstorCluster) (admission.Warnings, field.ErrorList) {
	errs := ValidateExternalController(current.Spec.ExternalController, field.NewPath("spec", "externalController"))
	errs = append(errs, ValidateNodeSelector(current.Spec.NodeSelector, field.NewPath("spec", "nodeSelector"))...)
	errs = append(errs, ValidateComponentSpec(current.Spec.Controller, field.NewPath("spec", "controller"))...)
	errs = append(errs, ValidateComponentSpec(current.Spec.CSINode, field.NewPath("spec", "csiNode"))...)
	errs = append(errs, ValidateComponentSpec(current.Spec.HighAvailabilityController, field.NewPath("spec", "highAvailabilityController"))...)
	errs = append(errs, ValidateDeploymentSpec(current.Spec.CSIController, field.NewPath("spec", "csiController"))...)
	errs = append(errs, ValidateDeploymentSpec(current.Spec.AffinityController, field.NewPath("spec", "affinityController"))...)

	for i := range current.Spec.Patches {
		errs = append(errs, ValidatePatch(&current.Spec.Patches[i], field.NewPath("spec", "patches", strconv.Itoa(i)))...)
	}

	return nil, errs
}

func ValidateExternalController(ref *piraeusiov1.LinstorExternalControllerRef, path *field.Path) field.ErrorList {
	var result field.ErrorList

	if ref != nil {
		_, err := lapi.NewClient(lapi.Controllers(strings.Split(ref.URL, ",")))
		if err != nil {
			result = append(result, field.Invalid(path.Child("url"), ref.URL, err.Error()))
		}
	}

	return result
}
