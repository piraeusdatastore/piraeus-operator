package controller

import (
	"time"

	"k8s.io/client-go/util/workqueue"
)

// DefaultRateLimiter is a modified workqueue.DefaultControllerRateLimiter.
//
// It reduced the maximum delay between reconcile attempts from 1000 seconds to 30 seconds.
func DefaultRateLimiter[T comparable]() workqueue.TypedRateLimiter[T] {
	return workqueue.NewTypedWithMaxWaitRateLimiter[T](workqueue.DefaultTypedControllerRateLimiter[T](), 30*time.Second)
}
