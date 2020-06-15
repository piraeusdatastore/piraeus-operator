package reconcileutil

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// an error implementing the error interface, that has multiple sources.
type CombinedError struct {
	Sources []error
}

func (e *CombinedError) Error() string {
	return "multiple errors: " + strings.Join(ErrorStrings(e.Sources...), "|")
}

// convert a list of errors into a list of string.
func ErrorStrings(errs ...error) []string {
	s := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			s = append(s, err.Error())
		}
	}

	return s
}

// convert a list of errors into a result as returned by reconcile.Reconciler.
func ToReconcileResult(errs ...error) (reconcile.Result, error) {
	var filteredErrors []error = nil
	var reconcileResults []reconcile.Result = nil

	for _, item := range errs {
		if item != nil {
			filteredErrors = append(filteredErrors, item)
		}

		if err, ok := item.(*TemporaryError); ok {
			reconcileResults = append(reconcileResults, err.ToReconcileResult())
		}
	}

	if len(reconcileResults) != 0 {
		return CombineReconcileResults(reconcileResults...), nil
	}

	if len(filteredErrors) != 0 {
		return reconcile.Result{}, &CombinedError{Sources: filteredErrors}
	}

	return reconcile.Result{}, nil
}

// wrapper around an error that may be solved after waiting some time.
type TemporaryError struct {
	Source       error
	RequeueAfter time.Duration
}

// Methods for the error (and errors.Is and errors.Unwrap) interface on TemporaryError

func (e *TemporaryError) Error() string {
	return fmt.Sprintf("temporary error: timeout=%d, error=%v", e.RequeueAfter, e.Source)
}

func (e *TemporaryError) Is(target error) bool {
	return errors.Is(e.Source, target)
}

func (e *TemporaryError) Unwrap() error {
	return e.Source
}

func (e *TemporaryError) ToReconcileResult() reconcile.Result {
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: e.RequeueAfter,
	}
}

// Given a list of reconcile results, returns the one with the closest restart time.
func CombineReconcileResults(results ...reconcile.Result) reconcile.Result {
	combined := reconcile.Result{}

	for _, res := range results {
		if res.Requeue && res.RequeueAfter == 0 {
			// immediate requeue, no need to check the other results, this will always trump the rest
			return res
		}

		if res.RequeueAfter == 0 {
			// no requeue from this result, can't change our combined result
			continue
		}

		if combined.RequeueAfter == 0 || res.RequeueAfter < combined.RequeueAfter {
			// new result with shorter requeue time
			combined.RequeueAfter = res.RequeueAfter
		}
	}

	return combined
}
