package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

func TestToEncodedPatch(t *testing.T) {
	t.Parallel()

	exampleSelector := &kusttypes.Selector{
		ResId:         resid.NewResIdKindOnly("Pod", "example"),
		LabelSelector: "controlled-by=test",
	}

	testcases := []struct {
		name     string
		selector *kusttypes.Selector
		patch    any
		result   *kusttypes.Patch
	}{
		{
			name:     "convert-string",
			patch:    "a-string-patch",
			selector: exampleSelector,
			result: &kusttypes.Patch{
				Target: exampleSelector,
				Patch:  "a-string-patch",
			},
		},
		{
			name: "convert-apply-obj",
			patch: applycorev1.Pod("example", "").
				WithSpec(applycorev1.PodSpec().
					WithHostNetwork(true)),
			selector: exampleSelector,
			result: &kusttypes.Patch{
				Target: exampleSelector,
				Patch:  "{\"kind\":\"Pod\",\"apiVersion\":\"v1\",\"metadata\":{\"name\":\"example\",\"namespace\":\"\"},\"spec\":{\"hostNetwork\":true}}",
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual, err := utils.ToEncodedPatch(tcase.selector, tcase.patch)
			assert.NoError(t, err)
			assert.Equal(t, tcase.result, actual)
		})
	}
}

func TestMakeKustPatches(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name    string
		patches []piraeusv1.Patch
		result  []kusttypes.Patch
	}{
		{
			name: "empty",
		},
		{
			name: "basic-patches",
			patches: []piraeusv1.Patch{
				{Patch: "patch-without-selector"},
				{Patch: "patch-with-selector", Target: &piraeusv1.Selector{Kind: "Deployment", Version: "v1", Group: "apps", Name: "foo", Namespace: "test-ns", AnnotationSelector: "annotation1=val1", LabelSelector: "label1=val2"}},
				{Patch: "patch-with-options", Options: map[string]bool{"opt1": true, "opt2": false}},
			},
			result: []kusttypes.Patch{
				{Patch: "patch-without-selector"},
				{Patch: "patch-with-selector", Target: &kusttypes.Selector{ResId: resid.ResId{Gvk: resid.Gvk{Kind: "Deployment", Version: "v1", Group: "apps"}, Name: "foo", Namespace: "test-ns"}, AnnotationSelector: "annotation1=val1", LabelSelector: "label1=val2"}},
				{Patch: "patch-with-options", Options: map[string]bool{"opt1": true, "opt2": false}},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := utils.MakeKustPatches(tcase.patches...)
			assert.Equal(t, tcase.result, actual)
		})
	}
}
