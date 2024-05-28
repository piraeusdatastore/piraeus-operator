package utils_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

func MustResource(resmap map[string]any) *resource.Resource {
	factory := provider.NewDefaultDepProvider().GetResourceFactory()
	res, err := factory.FromMap(resmap)
	if err != nil {
		panic(err)
	}

	return res
}

var (
	Owner = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "example",
			UID:  types.UID("00000000-0000-0000-0000-000000000000"),
		},
	}
	NotOwner = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-owner",
			UID:  types.UID("00000000-0000-0000-0000-000000000001"),
		},
	}

	Initial = []client.Object{
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa1",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(Owner, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}),
				},
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa2",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(Owner, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}),
				},
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa3",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(NotOwner, schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}),
				},
			},
		},
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sa4",
			},
		},
	}

	AppliedS1 = MustResource(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": map[string]interface{}{
			"name": "sa1",
		},
	})

	AppliedS2 = MustResource(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": map[string]interface{}{
			"name": "sa2",
		},
	})
)

func TestPruneResources(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name      string
		owner     client.Object
		toKeep    []*resource.Resource
		remaining []string
	}{
		{
			name:      "nothing-to-delete",
			owner:     Owner,
			toKeep:    []*resource.Resource{AppliedS1, AppliedS2},
			remaining: []string{"sa1", "sa2", "sa3", "sa4"},
		},
		{
			name:      "delete-all-owned",
			owner:     Owner,
			remaining: []string{"sa3", "sa4"},
		},
		{
			name:      "delete-not-applied",
			owner:     Owner,
			toKeep:    []*resource.Resource{AppliedS1},
			remaining: []string{"sa1", "sa3", "sa4"},
		},
	}

	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	assert.NoError(t, err)

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			cl := fake.NewClientBuilder().
				WithObjects(Initial...).
				Build()

			resm := resmap.New()
			for _, r := range tcase.toKeep {
				err := resm.Append(r)
				assert.NoError(t, err)
			}

			err := utils.PruneResources(context.Background(), cl, tcase.owner, "", resm, &corev1.ServiceAccount{})
			assert.NoError(t, err)

			var accounts corev1.ServiceAccountList
			err = cl.List(context.Background(), &accounts)
			assert.NoError(t, err)
			AssertEqualResources(t, tcase.remaining, accounts.Items)
		})
	}
}

func AssertEqualResources(t *testing.T, expected []string, actualList []corev1.ServiceAccount) {
	var resourceNames []string
	for _, o := range actualList {
		resourceNames = append(resourceNames, o.Name)
	}

	assert.ElementsMatch(t, expected, resourceNames)
}
