package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

var _ = Describe("LinstorSatelliteReconciler", func() {
	TypeMeta := metav1.TypeMeta{Kind: "LinstorSatellite", APIVersion: piraeusiov1.GroupVersion.String()}

	Context("When creating LinstorSatellite resources", func() {
		BeforeEach(func(ctx context.Context) {
			err := k8sClient.Create(ctx, &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						Architecture:  "amd64",
						KernelVersion: "5.14.0-70.26.1.el9_0.x86_64",
						OSImage:       "AlmaLinux 9.0 (Emerald Puma)",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Create(ctx, &piraeusiov1.LinstorSatellite{
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					ClusterRef: piraeusiov1.ClusterReference{Name: "example"},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func(ctx context.Context) {
			err := k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorSatellite{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []piraeusiov1.LinstorSatellite {
				var satellites piraeusiov1.LinstorSatelliteList
				err = k8sClient.List(ctx, &satellites)
				Expect(err).NotTo(HaveOccurred())
				return satellites.Items
			}, DefaultTimeout, DefaultCheckInterval).Should(BeEmpty())

			err = k8sClient.DeleteAllOf(ctx, &corev1.Node{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should select loader image, apply resources, setting finalizer and condition", func(ctx context.Context) {
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
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())

			Expect(satellite.Finalizers).To(ContainElement(vars.SatelliteFinalizer))

			var pod corev1.Pod
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ExampleNodeName}, &pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.InitContainers).To(HaveLen(2))
			Expect(pod.Spec.InitContainers[0].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd9-almalinux9:"))
			Expect(pod.Spec.InitContainers[1].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd-shutdown-guard:"))
		})

		It("should create pod with TLS secret", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{},
				},
			}, client.Apply, client.FieldOwner("test"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ExampleNodeName}, &pod)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pod.Spec.Volumes).To(ContainElement(HaveField("Secret.SecretName", ExampleNodeName+"-tls")))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
		})

		It("should mount host directory for file storage", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					StoragePools: []piraeusiov1.LinstorStoragePool{
						{
							Name:         "pool1",
							FileThinPool: &piraeusiov1.LinstorStoragePoolFile{},
						},
					},
				},
			}, client.Apply, client.FieldOwner("test"))
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var pod corev1.Pod
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ExampleNodeName}, &pod)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pod.Spec.Volumes).To(ContainElement(HaveField("HostPath.Path", "/var/lib/linstor-pools/pool1")))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
		})

		Context("with additional finalizer", func() {
			BeforeEach(func(ctx context.Context) {
				err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
					TypeMeta:   TypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName, Finalizers: []string{"piraeus.io/test"}},
				}, client.Apply, client.FieldOwner("test"))
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx context.Context) {
				err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
					TypeMeta:   TypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				}, client.Apply, client.FieldOwner("test"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should continue to reconcile after deleting a k8s node", func(ctx context.Context) {
				err := k8sClient.Delete(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName}})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() metav1.ConditionStatus {
					var satellite piraeusiov1.LinstorSatellite
					err := k8sClient.Get(ctx, types.NamespacedName{Name: ExampleNodeName}, &satellite)
					if err != nil {
						return metav1.ConditionUnknown
					}

					condition := meta.FindStatusCondition(satellite.Status.Conditions, "EvacuationCompleted")
					if condition == nil || condition.ObservedGeneration != satellite.Generation {
						return metav1.ConditionUnknown
					}
					return condition.Status
				}, DefaultTimeout, DefaultCheckInterval).Should(Equal(metav1.ConditionTrue))
			})
		})
	})
})
