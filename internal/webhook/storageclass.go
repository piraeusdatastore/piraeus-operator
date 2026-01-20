package webhook

import (
	"context"

	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	"github.com/piraeusdatastore/linstor-csi/pkg/volume"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-storage-k8s-io-v1-storageclass,mutating=false,failurePolicy=fail,sideEffects=None,groups=storage.k8s.io,resources=storageclasses,verbs=create;update,versions=v1,name=vstorageclass.kb.io,admissionReviewVersions=v1

type StorageClass struct{}

var _ admission.Validator[*storagev1.StorageClass] = &StorageClass{}

func (s *StorageClass) ValidateCreate(ctx context.Context, sc *storagev1.StorageClass) (admission.Warnings, error) {
	warnings, errs := s.validate(nil, sc)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(sc.GroupVersionKind().GroupKind(), sc.GetName(), errs)
	}

	return warnings, nil
}

func (s *StorageClass) ValidateUpdate(ctx context.Context, oldSC, newSC *storagev1.StorageClass) (admission.Warnings, error) {
	warnings, errs := s.validate(oldSC, newSC)
	if len(errs) != 0 {
		return warnings, apierrors.NewInvalid(newSC.GroupVersionKind().GroupKind(), newSC.GetName(), errs)
	}

	return warnings, nil
}

func (s *StorageClass) ValidateDelete(ctx context.Context, sc *storagev1.StorageClass) (admission.Warnings, error) {
	return nil, nil
}

func (s *StorageClass) validate(old, new *storagev1.StorageClass) (admission.Warnings, field.ErrorList) {
	var errs field.ErrorList

	if new.Provisioner != linstorcsi.DriverName {
		return nil, nil
	}

	_, err := volume.NewParameters(new.Parameters, "Aux/topology")
	if err != nil {
		errs = append(errs, field.Invalid(field.NewPath("parameters"), new.Parameters, err.Error()))
	}

	return nil, errs
}
