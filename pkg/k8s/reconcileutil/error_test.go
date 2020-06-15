package reconcileutil_test

import (
	"fmt"
	"testing"

	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/reconcileutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestErrorStrings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		errors   []error
		expected []string
	}{
		{
			name:     "empty",
			errors:   nil,
			expected: []string{},
		},
		{
			name: "multiple",
			errors: []error{
				fmt.Errorf("error #1"),
				fmt.Errorf("error #2"),
			},
			expected: []string{
				"error #1",
				"error #2",
			},
		},
	}
	for _, item := range cases {
		test := item
		t.Run(test.name, func(t *testing.T) {
			actual := reconcileutil.ErrorStrings(test.errors...)

			if len(test.expected) != len(actual) {
				t.Errorf("expected: %v, actual: %v", test.expected, actual)
			}

			for i := range test.expected {
				if test.expected[i] != actual[i] {
					t.Errorf("expected: %v, actual: %v", test.expected, actual)
				}
			}
		})
	}
}

func TestToReconcileResult(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		errors         []error
		expectedResult reconcile.Result
		hasError       bool
	}{
		{
			name:           "empty produces success",
			errors:         nil,
			expectedResult: reconcile.Result{},
			hasError:       false,
		},
		{
			name:           "nil errors are ignored",
			errors:         []error{nil},
			expectedResult: reconcile.Result{},
			hasError:       false,
		},
		{
			name:           "nil errors are ignored #2",
			errors:         []error{nil, fmt.Errorf("error"), nil},
			expectedResult: reconcile.Result{},
			hasError:       true,
		},
		{
			name: "temporary-error produces requeue",
			errors: []error{
				fmt.Errorf("error #1"),
				&reconcileutil.TemporaryError{
					Source:       fmt.Errorf("error #2"),
					RequeueAfter: 10,
				},
			},
			expectedResult: reconcile.Result{
				RequeueAfter: 10,
			},
			hasError: false,
		},
		{
			name: "only unknown-error produce error",
			errors: []error{
				fmt.Errorf("error #1"),
				fmt.Errorf("error #2"),
			},
			expectedResult: reconcile.Result{},
			hasError:       true,
		},
	}
	for _, item := range cases {
		test := item
		t.Run(test.name, func(t *testing.T) {
			actualResult, actualError := reconcileutil.ToReconcileResult(test.errors...)

			if test.expectedResult != actualResult {
				t.Errorf("expected: %v, actual: %v", test.expectedResult, actualResult)
			}

			if test.hasError != (actualError != nil) {
				t.Errorf("expected error: %t, actual: %v", test.hasError, actualError)
			}
		})
	}
}

func TestCombineReconcileResults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		results        []reconcile.Result
		expectedResult reconcile.Result
	}{
		{
			name:           "empty produces success",
			results:        nil,
			expectedResult: reconcile.Result{},
		},
		{
			name: "single entry returns itself",
			results: []reconcile.Result{
				{RequeueAfter: 10},
			},
			expectedResult: reconcile.Result{
				RequeueAfter: 10,
			},
		},
		{
			name: "select minimal requeue time",
			results: []reconcile.Result{
				{RequeueAfter: 100},
				{RequeueAfter: 20},
				{RequeueAfter: 50},
			},
			expectedResult: reconcile.Result{
				RequeueAfter: 20,
			},
		},
		{
			name: "immediate requeue works",
			results: []reconcile.Result{
				{RequeueAfter: 100},
				{RequeueAfter: 20},
				{RequeueAfter: 50},
				{Requeue: true},
			},
			expectedResult: reconcile.Result{
				Requeue:      true,
				RequeueAfter: 0,
			},
		},
	}
	for _, item := range cases {
		test := item
		t.Run(test.name, func(t *testing.T) {
			actualResult := reconcileutil.CombineReconcileResults(test.results...)

			if test.expectedResult != actualResult {
				t.Errorf("expected: %v, actual: %v", test.expectedResult, actualResult)
			}
		})
	}
}
