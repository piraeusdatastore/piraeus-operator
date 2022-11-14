package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

// PruneResources removes all resources that are controlled by the given controller, but that should (no longer) exist.
func PruneResources(ctx context.Context, cl client.Client, controller client.Object, namespace string, toKeep resmap.ResMap, kindsToPrune ...client.Object) error {
	for _, kind := range kindsToPrune {
		gvks, _, err := cl.Scheme().ObjectKinds(kind)
		if err != nil {
			return err
		}

		if len(gvks) < 1 {
			return fmt.Errorf("'%T' is not known in scheme", kind)
		}

		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(gvks[0])
		err = cl.List(ctx, u, client.InNamespace(namespace))
		if err != nil {
			if meta.IsNoMatchError(err) {
				// Nothing to prune, as there is not even a schema definition for that type.
				continue
			}

			return err
		}

		for _, item := range u.Items {
			_, err = toKeep.GetByCurrentId(resid.NewResIdWithNamespace(resid.NewGvk(gvks[0].Group, gvks[0].Version, gvks[0].Kind), item.GetName(), item.GetNamespace()))
			if err == nil {
				continue
			}

			if !metav1.IsControlledBy(&item, controller) {
				continue
			}

			err := cl.Delete(ctx, &item)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
