package controllers

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// DefaultRateLimiter is a modified workqueue.DefaultControllerRateLimiter.
//
// It reduced the maximum delay between reconcile attempts from 1000 seconds to 30 seconds.
func DefaultRateLimiter() workqueue.RateLimiter {
	return workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 30*time.Second),
		&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
	)
}
