package resources_test

import (
	"embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/test"
)

func TestNewKustomizer(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name      string
		fs        *embed.FS
		kustomize *types.Kustomization
		expected  string
	}{
		{
			name:      "empty",
			fs:        &test.EmptyResources,
			kustomize: &types.Kustomization{Resources: []string{"empty"}},
		},
		{
			name:      "basic",
			fs:        &test.BasicResources,
			kustomize: &types.Kustomization{Resources: []string{"basic"}},
			expected: `apiVersion: v1
kind: Namespace
metadata:
  name: example
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: example-sa
  namespace: example
`,
		},
		{
			name: "basic-patch",
			fs:   &test.BasicResources,
			kustomize: &types.Kustomization{
				Resources:  []string{"basic"},
				NamePrefix: "patch-",
				Namespace:  "patched",
			},
			expected: `apiVersion: v1
kind: Namespace
metadata:
  name: patched
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: patch-example-sa
  namespace: patched
`,
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			kstmzr, err := resources.NewKustomizer(tcase.fs, krusty.MakeDefaultOptions())
			assert.NoError(t, err)

			resmap, err := kstmzr.Kustomize(tcase.kustomize)
			assert.NoError(t, err)
			actual, err := resmap.AsYaml()
			assert.NoError(t, err)
			assert.Equal(t, tcase.expected, string(actual))
		})
	}
}

func TestOpenApiInit(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		Kind       string
		APIVersion string
	}{
		{Kind: "LinstorCluster", APIVersion: piraeusv1.GroupVersion.String()},
		{Kind: "LinstorSatelliteConfiguration", APIVersion: piraeusv1.GroupVersion.String()},
		{Kind: "LinstorSatellite", APIVersion: piraeusv1.GroupVersion.String()},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.Kind, func(t *testing.T) {
			t.Parallel()

			actual := openapi.IsCertainlyClusterScoped(yaml.TypeMeta{Kind: tcase.Kind, APIVersion: tcase.APIVersion})
			assert.True(t, actual)
		})
	}
}
