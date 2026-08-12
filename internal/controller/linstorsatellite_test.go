package controller_test

import (
	"context"
	"net"

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
		var linstorController *fakelinstor.FakeLinstor

		BeforeEach(func(ctx context.Context) {
			linstorController = fakelinstor.New()

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
							URL: linstorController.Server.URL,
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
			}).Should(BeEmpty())

			err = k8sClient.DeleteAllOf(ctx, &corev1.Node{})
			Expect(err).NotTo(HaveOccurred())

			linstorController.Server.Close()
		})

		It("should select loader image, apply resources, setting finalizer and condition", func(ctx context.Context) {
			Eventually(func() *metav1.Condition {
				return GetSatelliteCondition(ctx, k8sClient, ExampleNodeName, string(conditions.Applied))
			}).Should(HaveField("Status", metav1.ConditionTrue))

			var satellite piraeusiov1.LinstorSatellite
			err := k8sClient.Get(ctx, client.ObjectKey{Name: ExampleNodeName}, &satellite)
			Expect(err).NotTo(HaveOccurred())
			Expect(satellite.Finalizers).To(ContainElement(vars.SatelliteFinalizer))

			var ds appsv1.DaemonSet
			err = k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
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
			}).Should(Succeed())

			Expect(ds.Spec.Template.Spec.Containers[0].Name).To(Equal("linstor-satellite"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].Name).To(Equal("linstor"))
			Expect(ds.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(Equal(int32(3367)))
		})

		It("should use the configured resource name suffix separator", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					ResourceNameSuffixSeparator: "-",
				},
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite-" + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(errors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
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
			}).Should(Succeed())
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
			}).Should(Succeed())
		})

		It("should mount lvmlockd files for shared storage pools with external locking", func(ctx context.Context) {
			err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
				TypeMeta:   TypeMeta,
				ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName},
				Spec: piraeusiov1.LinstorSatelliteSpec{
					StoragePools: []piraeusiov1.LinstorStoragePool{
						{
							Name:    "shared-pool",
							LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1", SharedSpace: "space1", ExternalLocking: true},
						},
					},
				},
			}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var ds appsv1.DaemonSet
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ds.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("HostPath.Path", "/run/lvmlockd.pid")))
				container := GetContainer(ds.Spec.Template.Spec.Containers, "linstor-satellite")
				g.Expect(container).NotTo(BeNil())
				g.Expect(container.VolumeMounts).To(ContainElement(HaveField("Name", "run-lvmlockd-pid")))
			}).Should(Succeed())
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
			}).Should(Succeed())
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
					Conditions: []corev1.PodCondition{{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					}},
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
				}).Should(Succeed())
			})

			It("should restore an evacuated satellite", func(ctx context.Context) {
				Eventually(func(g Gomega) {
					_, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
					g.Expect(err).NotTo(HaveOccurred())
				}).Should(Succeed())

				err := linstorClient.Nodes.Evacuate(ctx, ExampleNodeName, lapi.NodeEvacuate{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func(g Gomega) {
					node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(node.Flags).NotTo(ContainElement(linstor.FlagEvacuate))
				}).Should(Succeed())
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
					}).Should(Succeed())

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

						satellite.ObjectMeta.Finalizers = []string{}
						err = k8sClient.Update(ctx, &satellite)
						g.Expect(err).NotTo(HaveOccurred())

						err = k8sClient.Delete(ctx, &satellite)
						g.Expect(err).NotTo(HaveOccurred())
					}).Should(Succeed())
				})

				It("should respect DeletionPolicy=Retain (Default policy)", func(ctx context.Context) {
					err := k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName}})
					Expect(err).NotTo(HaveOccurred())

					Eventually(func() *metav1.Condition {
						return GetSatelliteCondition(ctx, k8sClient, ExampleNodeName, "SatelliteDeleted")
					}).Should(HaveField("Status", metav1.ConditionTrue))

					node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
					Expect(err).NotTo(HaveOccurred())
					Expect(node.ConnectionStatus).To(Equal("ONLINE"))

					_, err = linstorClient.Resources.Get(ctx, "resource1", ExampleNodeName)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should respect DeletionPolicy=Evacuate", func(ctx context.Context) {
					err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
						TypeMeta:   TypeMeta,
						ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName, Finalizers: []string{"piraeus.io/test"}},
						Spec: piraeusiov1.LinstorSatelliteSpec{
							DeletionPolicy: piraeusiov1.DeletionPolicyEvacuate,
						},
					}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
					Expect(err).NotTo(HaveOccurred())

					err = k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName}})
					Expect(err).NotTo(HaveOccurred())

					GinkgoWriter.Println("checking that Satellite is in evacuation")
					Eventually(func(g Gomega) {
						node, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(node.Flags).To(ContainElement(linstor.FlagEvacuate))
					}).Should(Succeed())

					GinkgoWriter.Println("checking that Satellite status reports evacuation progress")
					Eventually(func() *metav1.Condition {
						return GetSatelliteCondition(ctx, k8sClient, ExampleNodeName, "SatelliteEvacuated")
					}).Should(And(
						HaveField("Status", metav1.ConditionFalse),
						HaveField("Reason", string(conditions.ReasonInProgress)),
						HaveField("Message", ContainSubstring("resource1"))),
					)

					GinkgoWriter.Println("by deleting resources, evacuation should complete")
					err = linstorClient.ResourceDefinitions.Delete(ctx, "resource1")
					Expect(err).NotTo(HaveOccurred())

					Eventually(func(g Gomega) {
						_, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						g.Expect(err).To(Equal(lapi.NotFoundError))
					}).Should(Succeed())

					Eventually(func() *metav1.Condition {
						return GetSatelliteCondition(ctx, k8sClient, ExampleNodeName, "SatelliteDeleted")
					}).Should(HaveField("Status", metav1.ConditionTrue))
				})

				It("should respect DeletionPolicy=Delete", func(ctx context.Context) {
					err := k8sClient.Patch(ctx, &piraeusiov1.LinstorSatellite{
						TypeMeta:   TypeMeta,
						ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName, Finalizers: []string{"piraeus.io/test"}},
						Spec: piraeusiov1.LinstorSatelliteSpec{
							DeletionPolicy: piraeusiov1.DeletionPolicyDelete,
						},
					}, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
					Expect(err).NotTo(HaveOccurred())

					err = k8sClient.Delete(ctx, &piraeusiov1.LinstorSatellite{ObjectMeta: metav1.ObjectMeta{Name: ExampleNodeName}})
					Expect(err).NotTo(HaveOccurred())

					GinkgoWriter.Println("checking that associated resources are deleted")
					Eventually(func(g Gomega) {
						var ds appsv1.DaemonSet
						err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-satellite." + ExampleNodeName}, &ds)
						if errors.IsNotFound(err) {
							return
						}
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(ds.ObjectMeta.DeletionTimestamp).NotTo(BeNil())
					}).Should(Succeed())

					Eventually(func() *metav1.Condition {
						return GetSatelliteCondition(ctx, k8sClient, ExampleNodeName, "SatelliteDeleted")
					}).Should(And(
						Not(BeNil()),
						HaveField("Status", metav1.ConditionFalse),
						HaveField("Message", ContainSubstring("node '%s' is not 'OFFLINE'", ExampleNodeName))),
					)

					linstorController.SetConnectionStatus(ExampleNodeName, "OFFLINE")

					GinkgoWriter.Println("checking that Satellite is forcefully removed from LINSTOR")
					Eventually(func() error {
						_, err := linstorClient.Nodes.Get(ctx, ExampleNodeName)
						return err
					}).Should(Equal(lapi.NotFoundError))
				})
			})
		})
	})
})

func GetSatelliteCondition(ctx context.Context, k8sClient client.Client, nodeName, ty string) *metav1.Condition {
	var satellite piraeusiov1.LinstorSatellite
	err := k8sClient.Get(ctx, types.NamespacedName{Name: ExampleNodeName}, &satellite)
	if err != nil {
		return nil
	}

	condition := meta.FindStatusCondition(satellite.Status.Conditions, ty)
	if condition == nil || condition.ObservedGeneration != satellite.Generation {
		return nil
	}

	return condition
}
