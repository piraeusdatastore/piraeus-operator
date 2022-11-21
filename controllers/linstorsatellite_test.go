package controllers_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

var _ = Describe("LinstorSatelliteReconciler", func() {
	Context("When creating LinstorSatellite resources", func() {
		ctx := context.Background()
		BeforeEach(func() {
			err := k8sClient.Create(ctx, &piraeusiov1.LinstorSatellite{
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					ClusterRef: piraeusiov1.ClusterReference{Name: "example"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				var satellite piraeusiov1.LinstorSatellite
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ExampleNodeName}, &satellite)
				return apierrors.IsNotFound(err)
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
		})

		It("should select loader image, apply resources, setting finalizer and condition", func() {
			var satellite piraeusiov1.LinstorSatellite
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ExampleNodeName}, &satellite)
				if err != nil {
					return false
				}

				condition := meta.FindStatusCondition(satellite.Status.Conditions, string(conditions.Applied))
				if condition == nil || condition.ObservedGeneration != satellite.Generation {
					return false
				}
				return condition.Status == metav1.ConditionTrue
			}).Should(BeTrue())

			Expect(satellite.Finalizers).To(ContainElement(vars.SatelliteFinalizer))

			var pod corev1.Pod
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ExampleNodeName}, &pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.InitContainers).To(HaveLen(1))
			Expect(pod.Spec.InitContainers[0].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd9-almalinux9:"))
		})
	})
})
