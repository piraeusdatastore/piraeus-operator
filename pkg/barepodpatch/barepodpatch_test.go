package barepodpatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/barepodpatch"
)

func TestConvertBarePodPatch(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name     string
		patch    piraeusiov1.Patch
		expected piraeusiov1.Patch
	}{
		{
			name: "ignore-non-pod-patch",
			patch: piraeusiov1.Patch{
				Patch: `apiVersion: v1
kind: ConfigMap
metadata:
  name: reactor-config
data:
  foo: bar
`,
			},
			expected: piraeusiov1.Patch{
				Patch: `apiVersion: v1
kind: ConfigMap
metadata:
  name: reactor-config
data:
  foo: bar
`,
			},
		},
		{
			name: "convert-strategic-merge-patch",
			patch: piraeusiov1.Patch{
				Patch: `apiVersion: v1
kind: Pod
metadata:
  annotations:
    foobar: baz
  labels:
    example.com/foo: bar
  name: satellite
spec:
  hostNetwork: true
  initContainers:
  - $patch: delete
    name: drbd-shutdown-guard
`,
			},
			expected: piraeusiov1.Patch{
				Target: &piraeusiov1.Selector{
					Group:   "apps",
					Version: "v1",
					Kind:    "DaemonSet",
					Name:    "linstor-satellite",
				},
				Patch: `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: linstor-satellite
spec:
  template:
    metadata:
      annotations:
        foobar: baz
      labels:
        example.com/foo: bar
    spec:
      hostNetwork: true
      initContainers:
      - $patch: delete
        name: drbd-shutdown-guard
`,
			},
		},
		{
			name: "convert-json-patch",
			patch: piraeusiov1.Patch{
				Target: &piraeusiov1.Selector{
					Kind: "Pod",
					Name: "satellite",
				},
				Patch: `- op: add
  path: /metadata/labels/foo
  value: bar
- from: /metadata/labels/bar
  op: move
  path: /metadata/labels/baz
`,
			},
			expected: piraeusiov1.Patch{
				Target: &piraeusiov1.Selector{
					Group:   "apps",
					Version: "v1",
					Kind:    "DaemonSet",
					Name:    "linstor-satellite",
				},
				Patch: `- op: add
  path: /spec/template/metadata/labels/foo
  value: bar
- from: /spec/template/metadata/labels/bar
  op: move
  path: /spec/template/metadata/labels/baz
`,
			},
		},
	}
	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual, err := barepodpatch.ConvertBarePodPatch(tcase.patch)
			assert.NoError(t, err)
			assert.Equal(t, &tcase.expected, &actual[0])
		})
	}
}
