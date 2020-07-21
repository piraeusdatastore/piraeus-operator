package reconcileutil

import (
	"context"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type TransferableObject interface {
	metav1.Object
	runtime.Object
}

type OwnershipTransferer struct {
	c        client.Client
	scheme   *runtime.Scheme
	oldOwner TransferableObject
	newOwner TransferableObject
}

func NewOwnershipTransferer(c client.Client, scheme *runtime.Scheme, oldOwner, newOwner TransferableObject) *OwnershipTransferer {
	return &OwnershipTransferer{
		c:        c,
		scheme:   scheme,
		oldOwner: oldOwner,
		newOwner: newOwner,
	}
}

func (ot *OwnershipTransferer) TransferOwnershipOfAll(ctx context.Context, list runtime.Object) error {
	nsSelector := &client.ListOptions{Namespace: ot.oldOwner.GetNamespace()}
	err := ot.c.List(ctx, list, nsSelector)
	if err != nil {
		return err
	}

	oldKind := ot.oldOwner.GetObjectKind().GroupVersionKind().Kind
	oldApiVersion := ot.oldOwner.GetObjectKind().GroupVersionKind().GroupVersion().String()

	for _, item := range asMetaSlice(list) {
		update := false
		for _, ref := range item.GetOwnerReferences() {
			if ref.Kind == oldKind && ref.APIVersion == oldApiVersion && ref.Name == ot.oldOwner.GetName() {
				update = true
				break
			}
		}

		if !update {
			continue
		}

		item.SetOwnerReferences(nil)
		err := controllerutil.SetControllerReference(ot.newOwner, item, ot.scheme)
		if err != nil {
			return err
		}

		err = ot.c.Update(ctx, item)
		if err != nil {
			return err
		}
	}

	return nil
}

func asMetaSlice(list runtime.Object) []TransferableObject {
	switch v := list.(type) {
	case *appsv1.DeploymentList:
		ret := make([]TransferableObject, len(v.Items))
		for i := range v.Items {
			ret[i] = &v.Items[i]
		}
		return ret
	case *appsv1.DaemonSetList:
		ret := make([]TransferableObject, len(v.Items))
		for i := range v.Items {
			ret[i] = &v.Items[i]
		}
		return ret
	case *corev1.ConfigMapList:
		ret := make([]TransferableObject, len(v.Items))
		for i := range v.Items {
			ret[i] = &v.Items[i]
		}
		return ret
	case *corev1.ServiceList:
		ret := make([]TransferableObject, len(v.Items))
		for i := range v.Items {
			ret[i] = &v.Items[i]
		}
		return ret
	default:
		logrus.Errorf("unsupported type %v", v)
		return nil
	}
}
