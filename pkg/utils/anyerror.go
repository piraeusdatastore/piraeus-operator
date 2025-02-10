package utils

import (
	"errors"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AnyResult returns an error if any given error is not nil, otherwise it just returns the result
func AnyResult(result reconcile.Result, errs ...error) (reconcile.Result, error) {
	err := errors.Join(errs...)
	if err != nil {
		return reconcile.Result{}, err
	}

	return result, nil
}
