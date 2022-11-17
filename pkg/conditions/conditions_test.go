package conditions_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
)

func TestConditions(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name       string
		mutate     func(c conditions.Conditions)
		generation int64
		expected   []metav1.Condition
	}{
		{
			name: "empty-default-unknown",
			expected: []metav1.Condition{
				{Type: string(conditions.Applied), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: string(conditions.Available), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: string(conditions.Configured), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
			},
		},
		{
			name:       "with-observations-and-generation",
			generation: 2,
			mutate: func(c conditions.Conditions) {
				c.AddSuccess(conditions.Applied, "Applied success")
				c.AddUnknown(conditions.Available, "Not checked")
				c.AddError(conditions.Configured, errors.New("not available"))
			},
			expected: []metav1.Condition{
				{Type: string(conditions.Applied), Status: metav1.ConditionTrue, Reason: string(conditions.ReasonAsExpected), Message: "Applied success", ObservedGeneration: 2},
				{Type: string(conditions.Available), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonUnknown), Message: "Not checked", ObservedGeneration: 2},
				{Type: string(conditions.Configured), Status: metav1.ConditionFalse, Reason: string(conditions.ReasonError), Message: "Error: not available", ObservedGeneration: 2},
			},
		},
		{
			name: "add-new-conditions",
			mutate: func(c conditions.Conditions) {
				c.AddSuccess("NewCond", "success")
			},
			expected: []metav1.Condition{
				{Type: string(conditions.Applied), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: string(conditions.Available), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: string(conditions.Configured), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: "NewCond", Status: metav1.ConditionTrue, Reason: string(conditions.ReasonAsExpected), Message: "success"},
			},
		},
		{
			name: "condition-set-multiple",
			mutate: func(c conditions.Conditions) {
				c.AddSuccess(conditions.Applied, "success")
				c.AddError(conditions.Applied, errors.New("actually an error"))
			},
			expected: []metav1.Condition{
				{Type: string(conditions.Applied), Status: metav1.ConditionFalse, Reason: string(conditions.ReasonError), Message: "success\nError: actually an error"},
				{Type: string(conditions.Available), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
				{Type: string(conditions.Configured), Status: metav1.ConditionUnknown, Reason: string(conditions.ReasonNotObserved)},
			},
		},
	}

	for i := range testcases {
		tcase := &testcases[i]
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			c := conditions.New()
			if tcase.mutate != nil {
				tcase.mutate(c)
			}
			actual := c.ToConditions(tcase.generation)
			assert.Equal(t, tcase.expected, actual)
		})
	}
}
