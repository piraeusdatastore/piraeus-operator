package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func AddFinalizer(obj metav1.Object, finalizer string) {
	if !SliceContains(obj.GetFinalizers(), finalizer) {
		obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	}
}

func DeleteFinalizer(obj metav1.Object, finalizer string) {
	obj.SetFinalizers(remove(obj.GetFinalizers(), finalizer))
}

func HasFinalizer(obj metav1.Object, finalizer string) bool {
	return SliceContains(obj.GetFinalizers(), finalizer)
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func SliceContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
