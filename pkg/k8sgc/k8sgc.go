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

// Package k8sgc implements a basic for of garbage collection on the Kubernetes API.
//
// This job is normally performed by the controller-manager. The component is not available in a test setting, with only
// the basic API server deployed. This is where k8sgc comes in to fill the gap, so deletion of objects can be tested as
// expected.
package k8sgc

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GC offers a basic form of garbage collection for Kubernetes resources
type GC interface {
	// Run the garbage collector until all orphaned resources are collected.
	// Returns true if any resources where collected.
	Run(ctx context.Context) (bool, error)
}

type gc struct {
	client  client.Client
	toWatch []schema.GroupVersionKind
}

// New creates a new GC using the passed kubernetes client.
// The GC will watch all resources for which the client has a Scheme.
func New(ctx context.Context, cl client.Client) (GC, error) {
	var toWatch []schema.GroupVersionKind
	// The known types include types only used for specialised tasks, such as the status subfields and request options
	// for GET, POST, DELETE, etc. So we just see if we can list the resource: anything we can list we should also be
	// able to delete.
	for gvk := range cl.Scheme().AllKnownTypes() {
		u := &unstructured.UnstructuredList{}
		u.SetGroupVersionKind(gvk)
		err := cl.List(ctx, u)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}

			if _, ok := err.(*apierrors.StatusError); ok {
				continue
			}

			return nil, err
		}

		toWatch = append(toWatch, gvk)
	}

	return &gc{
		client:  cl,
		toWatch: toWatch,
	}, nil
}

func (g *gc) Run(ctx context.Context) (bool, error) {
	collected := false
	for {
		madeModification := false

		for _, gvk := range g.toWatch {
			u := &unstructured.UnstructuredList{}
			u.SetGroupVersionKind(gvk)
			err := g.client.List(ctx, u)
			if err != nil {
				return collected, err
			}

			for i := range u.Items {
				obj := &u.Items[i]
				collect, err := g.shouldCollect(ctx, obj)
				if err != nil {
					return collected, err
				}

				if collect {
					collected = true
					madeModification = true
					err := g.client.Delete(ctx, obj)
					if err != nil && !apierrors.IsNotFound(err) {
						return collected, err
					}
				}
			}
		}

		if !madeModification {
			break
		}
	}

	return collected, nil
}

func (g *gc) shouldCollect(ctx context.Context, obj client.Object) (bool, error) {
	owners := obj.GetOwnerReferences()
	if len(owners) == 0 {
		return false, nil
	}

	if obj.GetDeletionTimestamp() != nil {
		return false, nil
	}

	for _, owner := range owners {
		ownObj := &unstructured.Unstructured{}
		ownObj.SetGroupVersionKind(schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind))
		err := g.client.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: obj.GetNamespace()}, ownObj)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return false, err
		}

		if ownObj.GetDeletionTimestamp() != nil {
			continue
		}

		if ownObj.GetUID() == owner.UID {
			return false, nil
		}
	}

	return true, nil
}
