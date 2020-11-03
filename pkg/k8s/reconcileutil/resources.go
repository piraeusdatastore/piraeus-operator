package reconcileutil

import (
	"context"
	"fmt"

	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GCRuntimeObject interface {
	metav1.Object
	runtime.Object
}

const (
	lastAppliedAnnotation = kubeSpec.APIGroup + "/last-applied-configuration"
	fieldOwner            = kubeSpec.APIGroup + "/pkg/k8s/reconcileutil"
)

var defaultPreconditions = []mergepatch.PreconditionFunc{
	mergepatch.RequireMetadataKeyUnchanged("name"),
	mergepatch.RequireMetadataKeyUnchanged("namespace"),
	mergepatch.RequireKeyUnchanged("status"),
}

// Creates or updates a resource to be in line with the given resource spec.
//
// `kubectl apply` for go. First, will try to create the resource. If it already exists, it will try to compute the
// changes based on the one previously applied and patch the resource.
func CreateOrUpdate(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject) (bool, error) {
	modifiedEncoded, err := ensureAppliedConfigAnnotation(scheme, obj)
	if err != nil {
		return false, err
	}

	err = kubeClient.Create(ctx, obj, client.FieldOwner(fieldOwner))
	if err == nil {
		return true, nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return false, err
	}

	current, err := getCurrentResource(ctx, kubeClient, scheme, obj)
	if err != nil {
		return false, err
	}

	patchMeta, err := strategicpatch.NewPatchMetaFromStruct(current)
	if err != nil {
		return false, fmt.Errorf("failed to patch metadata from empty struct: %w", err)
	}

	originalEncoded := current.GetAnnotations()[lastAppliedAnnotation]

	// We reset the creation timestamp here.
	// This is done because the object is not recognized as "empty" in the json encoder. Unless we reset the creation
	// time of the current resource, it will show up in any patch we want to submit.
	current.SetCreationTimestamp(metav1.Time{})
	currentEncoded, err := runtime.Encode(unstructured.UnstructuredJSONScheme, current)
	if err != nil {
		return false, fmt.Errorf("failed to re-encoded current resource: %w", err)
	}

	patch, err := strategicpatch.CreateThreeWayMergePatch([]byte(originalEncoded), modifiedEncoded, currentEncoded, patchMeta, true, defaultPreconditions...)
	if err != nil {
		return false, fmt.Errorf("failed to generate patch data: %w", err)
	}

	if string(patch) == "{}" {
		return false, nil
	}

	return true, kubeClient.Patch(ctx, obj, client.ConstantPatch(types.StrategicMergePatchType, patch), client.FieldOwner(fieldOwner))
}

// Creates or updates a resource, ensuring the controller reference is set.
//
// Sets the owner reference, ensuring that once the owning resource is cleaned up, the created items will be removed as
// well. Then calls CreateOrUpdate.
func CreateOrUpdateWithOwner(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject, owner metav1.Object) (bool, error) {
	err := controllerutil.SetControllerReference(owner, obj, scheme)
	// If it is already owned, we don't treat the SetControllerReference() call as a failure condition
	if err != nil {
		_, isAlreadyOwned := err.(*controllerutil.AlreadyOwnedError)
		if !isAlreadyOwned {
			return false, err
		}
	}

	return CreateOrUpdate(ctx, kubeClient, scheme, obj)
}

// Returns the current state of the given object, as stored in Kubernetes.
func getCurrentResource(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject) (GCRuntimeObject, error) {
	gvks, unversioned, err := scheme.ObjectKinds(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to determine object kinds: %w", err)
	}

	if unversioned {
		return nil, fmt.Errorf("cannot update unversioned type")
	}

	emptyObj, err := scheme.New(gvks[0])
	if err != nil {
		return nil, fmt.Errorf("failed to create new instance based on GVK: %w", err)
	}

	err = kubeClient.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, emptyObj)
	if err != nil {
		return nil, fmt.Errorf("failed to feetch current resource state: %w", err)
	}

	current, ok := emptyObj.(GCRuntimeObject)
	if !ok {
		return nil, fmt.Errorf("failed to cast cloned object to original type")
	}

	return current, nil
}

// Ensure the resource has the current config stored in an annotation.
func ensureAppliedConfigAnnotation(scheme *runtime.Scheme, obj GCRuntimeObject) ([]byte, error) {
	// Instead of encoding the json directly, we first convert it to "Unstructured", i.e. a map[string]interface{}
	// We do this to remove a "status" item if there is any. Not all status fields are marked as `json:",omitempty`,
	// so their default value will be serialized too. The status field itself is marked as `json:",omitempty`, which
	// is useless, as struct values are never empty in golang.
	objUnstructured := unstructured.Unstructured{}

	err := scheme.Convert(obj, &objUnstructured, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured item: %w", err)
	}

	delete(objUnstructured.Object, "status")

	objEncoded, err := runtime.Encode(unstructured.UnstructuredJSONScheme, &objUnstructured)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare applied configuration metadata: %w", err)
	}

	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[lastAppliedAnnotation] = string(objEncoded)
	obj.SetAnnotations(annotations)

	return objEncoded, nil
}
