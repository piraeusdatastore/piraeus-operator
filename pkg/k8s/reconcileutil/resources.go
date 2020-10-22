package reconcileutil

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GCRuntimeObject interface {
	metav1.Object
	runtime.Object
}

// Create a resource at the cluster scope.
//
// cluster scoped resource are not allowed to have owner references, so these objects will not be cleaned up
// automatically.
func CreateOrReplace(ctx context.Context, kubeClient client.Client, obj runtime.Object) error {
	err := kubeClient.Create(ctx, obj)
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return err
	}

	// TODO: support update operation.
	// Updates automatically trigger reconciliation, which means we get an endless loop of .Reconcile() calls. To
	// support this properly we would need to check for spec equality in some way.
	return nil
}

// Create a resource at current owning resource scope.
//
// Once the owning resource is cleaned up, the created items will be removed as well.
func CreateOrReplaceWithOwner(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject, owner metav1.Object) error {
	err := controllerutil.SetControllerReference(owner, obj, scheme)
	// If it is already owned, we don't treat the SetControllerReference() call as a failure condition
	if err != nil {
		_, isAlreadyOwned := err.(*controllerutil.AlreadyOwnedError)
		if !isAlreadyOwned {
			return err
		}
	}

	return CreateOrReplace(ctx, kubeClient, obj)
}
