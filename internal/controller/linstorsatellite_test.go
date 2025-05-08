package controller_test

import (
	"context"
	"net"
	"net/http/httptest"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/fakelinstor"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

var _ = Describe("LinstorSatelliteReconciler", func() {
	TypeMeta := metav1.TypeMeta{Kind: "LinstorSatellite", APIVersion: piraeusiov1.GroupVersion.String()}

	Context("When creating LinstorSatellite resources", func() {
		var clusterRef *piraeusiov1.ClusterReference
		var satellite *piraeusiov1.LinstorSatellite
		var linstorController *httptest.Server

		BeforeEach(func(ctx context.Context) {
			linstorController = httptest.NewServer(fakelinstor.New())

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
					ClusterRef: piraeusiov1.ClusterReference{
						Name: "example",
						ExternalController: &piraeusiov1.LinstorExternalControllerRef{
							URL: linstorController.URL,
						},
					},
				},
			}
			err = k8sClient.Create(ctx, satellite)
			Expect(err).NotTo(HaveOccurred())

			clusterRef = &satellite.Spec.ClusterRef
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

			linstorController.Close()
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
			Expect(ds.Spec.Template.Spec.InitContainers).To(HaveLen(3))
			Expect(ds.Spec.Template.Spec.InitContainers[0].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd9-almalinux9:"))
			Expect(ds.Spec.Template.Spec.InitContainers[1].Image).To(ContainSubstring("quay.io/piraeusdatastore/drbd-shutdown-guard:"))
			Expect(ds.Spec.Template.Spec.InitContainers[2].Image).To(ContainSubstring("quay.io/piraeusdatastore/piraeus-server:"))
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(2))
			Expect(ds.Spec.Template.Spec.Containers[0].Name).To(Equal("linstor-satellite"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].Name).To(Equal("linstor"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(3366)))
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

			var ds appsv1.DaemonSet
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", ExampleNodeName+"-tls")))))
			}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

			Expect(ds.Spec.Template.Spec.Containers[0].Name).To(Equal("linstor-satellite"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].Name).To(Equal("linstor"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(3367)))
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

		Context("with created Pod resource", func() {
			var linstorClient *linstorhelper.Client

			BeforeEach(func(ctx context.Context) {
				var ds *appsv1.DaemonSet
				Eventually(func() *appsv1.DaemonSet {
					var current appsv1.DaemonSet
					err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &current)
					if err != nil {
						return nil
					}
					ds = &current
					return ds
				}).Should(Not(BeNil()))

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName:    "linstor-satellite-" + ExampleNodeName,
						Namespace:       Namespace,
						Labels:          ds.Spec.Template.Labels,
						Annotations:     ds.Spec.Template.Annotations,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(ds, schema.FromAPIVersionAndKind("apps/v1", "DaemonSet"))},
					},
					Spec: ds.Spec.Template.Spec,
				}

				pod.Spec.NodeName = ExampleNodeName
				err := k8sClient.Create(ctx, pod)
				Expect(err).NotTo(HaveOccurred())

				pod.Status = corev1.PodStatus{
					Phase: corev1.PodRunning,
					PodIP: "10.0.0.147",
					PodIPs: []corev1.PodIP{{
						IP: "10.0.0.147",
					}},
				}

				err = k8sClient.Status().Update(ctx, pod)
				Expect(err).NotTo(HaveOccurred())

				linstorClient, err = linstorhelper.NewClientForCluster(ctx, k8sClient, Namespace, clusterRef)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx context.Context) {
				err := k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(Namespace))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should register the satellite", func(ctx context.Context) {
				Eventually(func(g Gomega) {
					node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(node.Name).To(Equal(ExampleNodeName))
					g.Expect(node.NetInterfaces).To(HaveExactElements(lapi.NetInterface{
						Name:                    "default-ipv4",
						Address:                 net.ParseIP("10.0.0.147"),
						SatellitePort:           linstor.DfltStltPortPlain,
						SatelliteEncryptionType: linstor.ValNetcomTypePlain,
					}))
				}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
			})

			Context("with additional finalizer and resource", func() {
				BeforeEach(func(ctx context.Context) {
					err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
						TypeMeta:   TypeMeta,
						ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName, Finalizers: []string{"piraeus.io/test"}},
					}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
					Expect(err).NotTo(HaveOccurred())

					Eventually(func(g Gomega) {
						node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(node.Name).To(Equal(ExampleNodeName))
						g.Expect(node.ConnectionStatus).To(Equal("ONLINE"))
					}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

					err = linstorClient.ResourceDefinitions.Create(ctx, lapi.ResourceDefinitionCreate{
						ResourceDefinition: lapi.ResourceDefinition{Name: "resource1"},
					})
					Expect(err).NotTo(HaveOccurred())

					err = linstorClient.Resources.Create(ctx, lapi.ResourceCreate{
						Resource: lapi.Resource{Name: "resource1", NodeName: ExampleNodeName},
					})
					Expect(err).NotTo(HaveOccurred())
				})

				AfterEach(func(ctx context.Context) {
					err := linstorClient.ResourceDefinitions.Delete(ctx, "resource1")
					Expect(err).NotTo(HaveOccurred())

					Eventually(func(g Gomega) {
						var satellite piraeusiov1.LinstorSatellite
						err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: ExampleNodeName}, &satellite)
						if errors.IsNotFound(err) {
							return
						}
						g.Expect(err).NotTo(HaveOccurred())

						controllerutil.RemoveFinalizer(&satellite, "piraeus.io/test")
						err = k8sClient.Update(ctx, &satellite)
						g.Expect(err).NotTo(HaveOccurred())
					}).Should(Succeed())
				})

				It("should evacuate the node after deleting the satellite", func(ctx context.Context) {
					err := k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName}})
					Expect(err).NotTo(HaveOccurred())

					GinkgoWriter.Println("checking that Satellite is in evacuation")

					Eventually(func(g Gomega) {
						node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(node.Flags).To(ContainElement(linstor.FlagEvacuate))
					}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

					GinkgoWriter.Println("checking that Satellite status reports evacuation progress")

					Eventually(func() *metav1.Condition {
						var satellite piraeusiov1.LinstorSatellite
						err := k8sClient.Get(ctx, types.NamespacedName{Name: ExampleNodeName}, &satellite)
						if err != nil {
							return nil
						}

						condition := meta.FindStatusCondition(satellite.Status.Conditions, "EvacuationCompleted")
						if condition == nil || condition.ObservedGeneration != satellite.Generation {
							return nil
						}

						return condition
					}, DefaultTimeout, DefaultCheckInterval).Should(And(
						Not(BeNil()),
						HaveField("Status", metav1.ConditionFalse),
						HaveField("Message", ContainSubstring("resource1"))),
					)

					GinkgoWriter.Println("by deleting resources, evacuation should complete")

					err = linstorClient.ResourceDefinitions.Delete(ctx, "resource1")
					Expect(err).NotTo(HaveOccurred())

					Eventually(func(g Gomega) {
						_, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						g.Expect(err).To(Equal(lapi.NotFoundError))
					}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

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
})
