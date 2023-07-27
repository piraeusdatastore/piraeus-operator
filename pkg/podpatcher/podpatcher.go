package podpatcher

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Patch patches a Pod.
//
// The differences to the normal client.Patch method are:
//   - We recreate the Pod if the patch failed to apply, as Pods generally can't be altered dynamically.
//   - We also recreate for image changes. Normally, these would be applied by kubelet eventually, but will cause a
//     "restarted container" event, which many users will have alerts for. It also can't be nicely controlled by the
//     operator. So we just have to make sure to detect any changes.
//   - We additionally recreate the Pod if the old one is in a terminated state. This can happen if a node shuts down
//     even with a restartPolicy: Always.
func Patch(ctx context.Context, cl client.Client, pod client.Object, patch client.Patch, opts ...client.PatchOption) error {
	var oldPod corev1.Pod
	err := cl.Get(ctx, types.NamespacedName{Name: pod.GetName(), Namespace: pod.GetNamespace()}, &oldPod)
	if apierrors.IsNotFound(err) {
		return cl.Patch(ctx, pod, patch, opts...)
	}

	if err != nil {
		return err
	}

	// For some reason, Pods allow updating the images for a Pod. While this could work for our case a lot of users will
	// have alerts for restarted container pods configured. So it is better to delete + recreate the pod. But only if
	// something substantial actually changed.
	var newPod corev1.Pod
	newEncoded, err := json.Marshal(pod)
	if err != nil {
		return err
	}

	err = json.Unmarshal(newEncoded, &newPod)
	if err != nil {
		return err
	}

	if !PodNeedsRestart(&oldPod, newPod.Spec.RestartPolicy) && EqualImages(&oldPod, &newPod) {
		err := cl.Patch(ctx, pod, patch, opts...)
		// IsInvalid gets returned if the patch fails to apply. In that case we want to continue with delete+recreate
		// below.
		if !apierrors.IsInvalid(err) {
			return err
		}
	}

	// If we reached this point, either the images have changed between old and new pod, the old pod has failed for some
	// reason or some other unpatchable item needs changing. The solution is simple: delete the pod and try again.
	err = cl.Delete(ctx, &oldPod)
	if err != nil {
		return err
	}

	return cl.Patch(ctx, pod, patch, opts...)
}

func EqualImages(a, b *corev1.Pod) bool {
	if len(a.Spec.InitContainers) != len(b.Spec.InitContainers) {
		return false
	}

	for i := range a.Spec.InitContainers {
		if a.Spec.InitContainers[i].Image != b.Spec.InitContainers[i].Image {
			return false
		}
	}

	if len(a.Spec.Containers) != len(b.Spec.Containers) {
		return false
	}

	for i := range a.Spec.Containers {
		if a.Spec.Containers[i].Image != b.Spec.Containers[i].Image {
			return false
		}
	}

	return true
}

func PodNeedsRestart(pod *corev1.Pod, restartPolicy corev1.RestartPolicy) bool {
	switch restartPolicy {
	case corev1.RestartPolicyNever:
		return false
	case corev1.RestartPolicyOnFailure:
		return pod.Status.Phase == corev1.PodFailed
	default:
		return pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded
	}
}
