package webhook

import (
	"context"

	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	"github.com/piraeusdatastore/linstor-csi/pkg/volume"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/validate-storage-k8s-io-v1-storageclass,mutating=false,failurePolicy=fail,sideEffects=None,groups=storage.k8s.io,resources=storageclasses,verbs=create;update,versions=v1,name=vstorageclass.kb.io,admissionReviewVersions=v1

type StorageClass struct{}

func (s *StorageClass) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	warnings, errs := s.validate(nil, obj.(*storagev1.StorageClass))
	if len(errs) != 0 {
		accessor, _ := meta.Accessor(obj)
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: storagev1.GroupName, Kind: "StorageClass"}, accessor.GetName(), errs)
	}

	return warnings, nil
}

func (s *StorageClass) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	warnings, errs := s.validate(oldObj.(*storagev1.StorageClass), newObj.(*storagev1.StorageClass))
	if len(errs) != 0 {
		accessor, _ := meta.Accessor(newObj)
		return warnings, apierrors.NewInvalid(schema.GroupKind{Group: storagev1.GroupName, Kind: "StorageClass"}, accessor.GetName(), errs)
	}

	return warnings, nil
}

func (s *StorageClass) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
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
