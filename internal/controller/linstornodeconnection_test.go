package controller_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/internal/controller"
)

var testLabelMap = map[string]map[string]string{
	"z1n1": {
		"topology.kubernetes.io/zone": "1",
		"topology.kubernetes.io/rack": "1",
		"kubernetes.io/hostname":      "z1n1",
		"extra":                       "1",
	},
	"z1n2": {
		"topology.kubernetes.io/zone": "1",
		"topology.kubernetes.io/rack": "2",
		"kubernetes.io/hostname":      "z1n2",
	},
	"z2n1": {
		"topology.kubernetes.io/zone": "2",
		"topology.kubernetes.io/rack": "1",
		"kubernetes.io/hostname":      "z2n1",
	},
	"z2n2": {
		"topology.kubernetes.io/zone": "2",
		"topology.kubernetes.io/rack": "2",
		"kubernetes.io/hostname":      "z2n2",
		"extra":                       "2",
	},
}

func TestNodeConnectionApplies(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name      string
		selectors []piraeusiov1.SelectorTerm
		expected  map[string]bool
	}{
		{
			name: "empty-selector-allow-all",
			expected: map[string]bool{
				"z1n1:z1n2": true,
				"z1n1:z2n1": true,
				"z1n1:z2n2": true,
				"z1n2:z2n1": true,
				"z1n2:z2n2": true,
				"z2n1:z2n2": true,
			},
		},
		{
			name: "exists-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{{
					Key: "extra",
					Op:  piraeusiov1.MatchLabelSelectorOpExists,
				}},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": false,
				"z1n1:z2n1": false,
				"z1n1:z2n2": true,
				"z1n2:z2n1": false,
				"z1n2:z2n2": false,
				"z2n1:z2n2": false,
			},
		},
		{
			name: "does-not-exists-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{{
					Key: "extra",
					Op:  piraeusiov1.MatchLabelSelectorOpDoesNotExist,
				}},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": false,
				"z1n1:z2n1": false,
				"z1n1:z2n2": false,
				"z1n2:z2n1": true,
				"z1n2:z2n2": false,
				"z2n1:z2n2": false,
			},
		},
		{
			name: "selector-terms-are-joined-by-or",
			selectors: []piraeusiov1.SelectorTerm{
				{
					MatchLabels: []piraeusiov1.MatchLabelSelector{{
						Key: "extra",
						Op:  piraeusiov1.MatchLabelSelectorOpExists,
					}},
				},
				{
					MatchLabels: []piraeusiov1.MatchLabelSelector{{
						Key: "extra",
						Op:  piraeusiov1.MatchLabelSelectorOpDoesNotExist,
					}},
				},
			},
			expected: map[string]bool{
				"z1n1:z1n2": false,
				"z1n1:z2n1": false,
				"z1n1:z2n2": true,
				"z1n2:z2n1": true,
				"z1n2:z2n2": false,
				"z2n1:z2n2": false,
			},
		},
		{
			name: "in-operator-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{{
					Key:    "topology.kubernetes.io/zone",
					Op:     piraeusiov1.MatchLabelSelectorOpIn,
					Values: []string{"5", "1"},
				}},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": true,
				"z1n1:z2n1": false,
				"z1n1:z2n2": false,
				"z1n2:z2n1": false,
				"z1n2:z2n2": false,
				"z2n1:z2n2": false,
			},
		},
		{
			name: "not-in-operator-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{{
					Key:    "topology.kubernetes.io/zone",
					Op:     piraeusiov1.MatchLabelSelectorOpNotIn,
					Values: []string{"5", "1"},
				}},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": false,
				"z1n1:z2n1": false,
				"z1n1:z2n2": false,
				"z1n2:z2n1": false,
				"z1n2:z2n2": false,
				"z2n1:z2n2": true,
			},
		},
		{
			name: "same-operator-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{
					{
						Key: "topology.kubernetes.io/zone",
						Op:  piraeusiov1.MatchLabelSelectorOpSame,
					},
				},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": true,
				"z1n1:z2n1": false,
				"z1n1:z2n2": false,
				"z1n2:z2n1": false,
				"z1n2:z2n2": false,
				"z2n1:z2n2": true,
			},
		},
		{
			name: "not-same-operator-works",
			selectors: []piraeusiov1.SelectorTerm{{
				MatchLabels: []piraeusiov1.MatchLabelSelector{
					{
						Key: "topology.kubernetes.io/zone",
						Op:  piraeusiov1.MatchLabelSelectorOpNotSame,
					},
				},
			}},
			expected: map[string]bool{
				"z1n1:z1n2": false,
				"z1n1:z2n1": true,
				"z1n1:z2n2": true,
				"z1n2:z2n1": true,
				"z1n2:z2n2": true,
				"z2n1:z2n2": false,
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			actual := map[string]bool{
				"z1n1:z1n2": controller.NodeConnectionApplies(tcase.selectors, "z1n1", "z1n2", testLabelMap),
				"z1n1:z2n1": controller.NodeConnectionApplies(tcase.selectors, "z1n1", "z2n1", testLabelMap),
				"z1n1:z2n2": controller.NodeConnectionApplies(tcase.selectors, "z1n1", "z2n2", testLabelMap),
				"z1n2:z2n1": controller.NodeConnectionApplies(tcase.selectors, "z1n2", "z2n1", testLabelMap),
				"z1n2:z2n2": controller.NodeConnectionApplies(tcase.selectors, "z1n2", "z2n2", testLabelMap),
				"z2n1:z2n2": controller.NodeConnectionApplies(tcase.selectors, "z2n1", "z2n2", testLabelMap),
			}
			assert.Equal(t, tcase.expected, actual)
		})
	}
}
