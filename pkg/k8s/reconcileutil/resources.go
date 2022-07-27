package reconcileutil

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
)

type GCRuntimeObject interface {
	metav1.Object
	runtime.Object
}

const (
	lastAppliedAnnotation = kubeSpec.APIGroup + "/last-applied-configuration"
	restartAnnotation     = kubeSpec.APIGroup + "/restarted-at"
	fieldOwner            = kubeSpec.APIGroup + "/pkg/k8s/reconcileutil"
)

var defaultPreconditions = []mergepatch.PreconditionFunc{
	mergepatch.RequireMetadataKeyUnchanged("name"),
	mergepatch.RequireMetadataKeyUnchanged("namespace"),
	mergepatch.RequireKeyUnchanged("status"),
}

type OnPatchError = func(ctx context.Context, kubeClient client.Client, current, desired GCRuntimeObject) error

// OnPatchErrorReturn returns the error when applying the patch.
var OnPatchErrorReturn OnPatchError

// OnPatchErrorRecreate recreates a resource by deleting old resources before applying it again.
func OnPatchErrorRecreate(ctx context.Context, kubeClient client.Client, current, desired GCRuntimeObject) error {
	policy := metav1.DeletePropagationForeground
	resourceVersion := current.GetResourceVersion()
	uid := current.GetUID()
	deleteOptions := &client.DeleteOptions{
		Preconditions: &metav1.Preconditions{
			ResourceVersion: &resourceVersion,
			UID:             &uid,
		},
		PropagationPolicy: &policy,
	}

	err := kubeClient.Delete(ctx, current, deleteOptions)
	if err != nil {
		return fmt.Errorf("recreate failed: could not delete old resource: %w", err)
	}

	err = kubeClient.Create(ctx, desired)
	if err != nil {
		return fmt.Errorf("recreate failed: could not create new resource: %w", err)
	}

	return nil
}

// CreateOrUpdate reconciles a resource to be in line with the given resource spec.
//
// `kubectl apply` for go. First, will try to create the resource. If it already exists, it will try to compute the
// changes based on the one previously applied and patch the resource.
func CreateOrUpdate(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject, onPatchErr OnPatchError) (bool, error) {
	desiredEncoded, desired, err := ensureAppliedConfigAnnotation(scheme, obj)
	if err != nil {
		return false, err
	}

	err = kubeClient.Create(ctx, desired, client.FieldOwner(fieldOwner))
	if err == nil {
		return true, nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return false, err
	}

	current, err := findCurrentResource(ctx, kubeClient, obj)
	if err != nil {
		return false, err
	}

	if current == nil {
		return false, fmt.Errorf("could not find object, but it should exist")
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

	patch, err := strategicpatch.CreateThreeWayMergePatch([]byte(originalEncoded), desiredEncoded, currentEncoded, patchMeta, true, defaultPreconditions...)
	if err != nil {
		return false, fmt.Errorf("failed to generate patch data: %w", err)
	}

	if string(patch) == "{}" {
		return false, nil
	}

	err = kubeClient.Patch(ctx, obj, client.RawPatch(types.StrategicMergePatchType, patch), client.FieldOwner(fieldOwner))
	if err != nil {
		if (apierrors.IsInvalid(err) || apierrors.IsUnsupportedMediaType(err)) && onPatchErr != nil {
			err := onPatchErr(ctx, kubeClient, current, desired)

			return err == nil, err
		}

		return false, fmt.Errorf("failed to apply patch: %w", err)
	}

	return true, nil
}

// CreateOrUpdateWithOwner reconciles a resource, ensuring the controller reference is set.
//
// Sets the owner reference, ensuring that once the owning resource is cleaned up, the created items will be removed as
// well. Then calls CreateOrUpdate.
func CreateOrUpdateWithOwner(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme, obj GCRuntimeObject, owner metav1.Object, onPatchErr OnPatchError) (bool, error) {
	err := controllerutil.SetControllerReference(owner, obj, scheme)
	// If it is already owned, we don't treat the SetControllerReference() call as a failure condition
	if err != nil {
		_, isAlreadyOwned := err.(*controllerutil.AlreadyOwnedError)
		if !isAlreadyOwned {
			return false, err
		}
	}

	return CreateOrUpdate(ctx, kubeClient, scheme, obj, onPatchErr)
}

// RestartRollout is "kubectl rollout restart" in go
//
// Works for Deployments, StatefulSets, DaemonSets and maybe others. Restart is trigger by setting/updating an
// annotation on the pod template.
func RestartRollout(ctx context.Context, kubeClient client.Client, obj client.Object) error {
	nowString := time.Now().Format(time.RFC3339)

	patchData := fmt.Sprintf("{\"spec\":{\"template\":{\"metadata\":{\"annotations\":{\"%s\":\"%s\"}}}}}", restartAnnotation, nowString)

	err := kubeClient.Patch(ctx, obj, client.RawPatch(types.MergePatchType, []byte(patchData)))
	if err != nil {
		return fmt.Errorf("failed to restart workload: %w", err)
	}

	return nil
}

func DeleteIfOwned(ctx context.Context, kubeClient client.Client, obj GCRuntimeObject, owner metav1.Object) error {
	current, err := findCurrentResource(ctx, kubeClient, obj)
	if err != nil {
		return err
	}

	if current == nil {
		return nil
	}

	for i := range current.GetOwnerReferences() {
		ref := &current.GetOwnerReferences()[i]
		if ref.UID == owner.GetUID() {
			return kubeClient.Delete(ctx, obj)
		}
	}

	return nil
}

// Returns the current state of the given object, as stored in Kubernetes.
//
// Returns nil, nil if the resource doesn't exist.
func findCurrentResource(ctx context.Context, kubeClient client.Client, obj GCRuntimeObject) (GCRuntimeObject, error) {
	current, ok := obj.DeepCopyObject().(GCRuntimeObject)
	if !ok {
		return nil, fmt.Errorf("failed to cast cloned object to original type")
	}

	err := kubeClient.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, current)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to feetch current resource state: %w", err)
	}

	return current, nil
}

// Ensure the resource has the current config stored in an annotation.
func ensureAppliedConfigAnnotation(scheme *runtime.Scheme, obj GCRuntimeObject) ([]byte, *unstructured.Unstructured, error) {
	// Instead of encoding the json directly, we first convert it to "Unstructured", i.e. a map[string]interface{}
	// We do this to remove a "status" item if there is any. Not all status fields are marked as `json:",omitempty`,
	// so their default value will be serialized too. The status field itself is marked as `json:",omitempty`, which
	// is useless, as struct values are never empty in golang.
	objUnstructured := unstructured.Unstructured{}

	err := scheme.Convert(obj, &objUnstructured, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert to unstructured item: %w", err)
	}

	delete(objUnstructured.Object, "status")

	// First serialize the object without the annotation to prevent recursion,
	// then add that serialization to it as the annotation and serialize it again.
	annotations := objUnstructured.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	delete(annotations, lastAppliedAnnotation)

	objEncoded, err := runtime.Encode(unstructured.UnstructuredJSONScheme, &objUnstructured)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare applied configuration metadata: %w", err)
	}

	annotations[lastAppliedAnnotation] = string(objEncoded)
	objUnstructured.SetAnnotations(annotations)

	objEncoded, err = runtime.Encode(unstructured.UnstructuredJSONScheme, &objUnstructured)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare applied configuration metadata: %w", err)
	}

	return objEncoded, &objUnstructured, nil
}
