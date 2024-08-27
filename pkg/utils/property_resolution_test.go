package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

func TestResolveNodeProperties(t *testing.T) {
	fakeNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"label1":                                "labelval1",
				"label2":                                "labelval2",
				"node-role.kubernetes.io/control-plane": "cp",
				"node-role.kubernetes.io/worker":        "w",
				"node-role.kubernetes.io/test":          "",
			},
			Annotations: map[string]string{
				"annotation1": "annotationval1",
				"annotation2": "annotationval2",
			},
		},
	}

	result, err := utils.ResolveNodeProperties(fakeNode,
		piraeusiov1.LinstorNodeProperty{
			Name:  "prop1",
			Value: "direct-val",
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "prop2",
			ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{
				NodeFieldRef: "metadata.labels['label1']",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "non-existing-non-optional",
			ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{
				NodeFieldRef: "metadata.labels['non-existent']",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name:     "non-existing-optional",
			Optional: true,
			ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{
				NodeFieldRef: "metadata.labels['non-existent']",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "prop3",
			ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{
				NodeFieldRef: "metadata.annotations['annotation2']",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "role/",
			ExpandFrom: &piraeusiov1.LinstorNodePropertyExpandFrom{
				LinstorNodePropertyValueFrom: piraeusiov1.LinstorNodePropertyValueFrom{
					NodeFieldRef: "metadata.labels['node-role.kubernetes.io/*']",
				},
				NameTemplate:  "$1",
				ValueTemplate: "$2",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "joined-role",
			ExpandFrom: &piraeusiov1.LinstorNodePropertyExpandFrom{
				LinstorNodePropertyValueFrom: piraeusiov1.LinstorNodePropertyValueFrom{
					NodeFieldRef: "metadata.labels['node-role.kubernetes.io/*']",
				},
				ValueTemplate: "$1=$2",
				Delimiter:     ",",
			},
		},
		piraeusiov1.LinstorNodeProperty{
			Name: "joined-role-without-delimiter",
			ExpandFrom: &piraeusiov1.LinstorNodePropertyExpandFrom{
				LinstorNodePropertyValueFrom: piraeusiov1.LinstorNodePropertyValueFrom{
					NodeFieldRef: "metadata.labels['node-role.kubernetes.io/*']",
				},
				ValueTemplate: "$1",
			},
		},
	)

	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"prop1":                         "direct-val",
		"prop2":                         "labelval1",
		"non-existing-non-optional":     "",
		"prop3":                         "annotationval2",
		"role/control-plane":            "cp",
		"role/worker":                   "w",
		"role/test":                     "",
		"joined-role":                   "control-plane=cp,test=,worker=w",
		"joined-role-without-delimiter": "control-planetestworker",
	}, result)
}

func TestResolveClusterProperties(t *testing.T) {
	t.Parallel()

	expected2 := maps.Clone(vars.DefaultControllerProperties)
	expected2["Aux/foo"] = "val2"
	expected2["Aux/bar"] = "val3"

	testcases := []struct {
		name   string
		props  []piraeusiov1.LinstorControllerProperty
		result map[string]string
	}{
		{
			name:   "default",
			result: vars.DefaultControllerProperties,
		},
		{
			name: "some-props",
			props: []piraeusiov1.LinstorControllerProperty{
				{Name: "Aux/foo", Value: "val1"},
				{Name: "Aux/foo", Value: "val2"},
				{Name: "Aux/bar", Value: "val3"},
			},
			result: expected2,
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := utils.ResolveClusterProperties(vars.DefaultControllerProperties, tcase.props...)
			assert.Equal(t, tcase.result, actual)
		})
	}
}
