package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
		var satellite *piraeusiov1.LinstorSatellite
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

			satellite = &piraeusiov1.LinstorSatellite{
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					ClusterRef: piraeusiov1.ClusterReference{Name: "example"},
				},
			}
			err = k8sClient.Create(ctx, satellite)
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

			var ds appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
			Expect(err).NotTo(HaveOccurred())
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(2))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd9-almalinux9:"))
			Expect(ds.Spec.Template.Spec.InitContainers[1].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd-shutdown-guard:"))
		})

		It("should create pod with TLS secret", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{},
				},
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", ExampleNodeName+"-tls")))))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
		})

		It("should create pod with ktls-utils if enabled", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{
						TLSHandshakeDaemon: true,
					},
				},
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", ExampleNodeName+"-tls")))))
				container := GetContainer(ds.Spec.Template.Spec.Containers, "ktls-utils")
				g.Expect(container).NotTo(BeNil())
				g.Expect(container.VolumeMounts).To(ContainElement(HaveField("Name", "internal-tls")))
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
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("HostPath.Path", "/var/lib/linstor-pools/pool1")))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
		})

		It("should convert bare pod patches to daemonset patches", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					Patches: []piraeusiov1.Patch{
						{
							Target: &piraeusiov1.Selector{Kind: "Pod", Name: "satellite"},
							Patch:  `[{"op":"add","path":"/metadata/annotations/test1","value":"val1"}]`,
						},
						{
							Patch: `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"satellite","labels":{"example.com/foo":"bar"}},"spec":{"hostNetwork":true,"containers":[{"name":"drbd-reactor","$patch":"delete"}]}}`,
						},
					},
				},
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Annotations).To(HaveKeyWithValue("test1", "val1"))
				g.Expect(ds.Spec.Template.Labels).To(HaveKeyWithValue("example.com/foo", "bar"))
				g.Expect(ds.Spec.Template.Spec.HostNetwork).To(BeTrue())
				g.Expect(ds.Spec.Template.Spec.Containers).NotTo(ContainElement(HaveField("Name", "drbd-reactor")))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
		})

		Context("with additional finalizer", func() {
			BeforeEach(func(ctx context.Context) {
				err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
					TypeMeta:   TypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName, Finalizers: []string{"piraeus.io/test"}},
				}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx context.Context) {
				err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
					TypeMeta:   TypeMeta,
					ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
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
