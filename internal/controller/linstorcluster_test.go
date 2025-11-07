package controller_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/piraeusdatastore/piraeus-ha-controller/pkg/metadata"
	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/tolerations"
)

var _ = Describe("LinstorCluster controller", func() {
	Context("when creating an empty LinstorCluster", func() {
		BeforeEach(func(ctx context.Context) {
			err := k8sClient.Create(ctx, &piraeusiov1.LinstorCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func(ctx context.Context) {
			err := k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorCluster{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []piraeusiov1.LinstorSatellite {
				var satellites piraeusiov1.LinstorSatelliteList
				err = k8sClient.List(ctx, &satellites)
				Expect(err).NotTo(HaveOccurred())
				return satellites.Items
			}).Should(BeEmpty())
		})

		It("should set the available condition", func(ctx context.Context) {
			Eventually(func() *metav1.Condition {
				cluster := &piraeusiov1.LinstorCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, cluster)
				if err != nil {
					return nil
				}

				condition := meta.FindStatusCondition(cluster.Status.Conditions, string(conditions.Applied))
				if condition == nil {
					return nil
				}

				if condition.ObservedGeneration != cluster.Generation {
					return nil
				}

				return condition
			}).Should(Not(BeNil()))
		})

		It("should create controller resources", func(ctx context.Context) {
			Eventually(func() error {
				deploy := appsv1.Deployment{}
				return k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: "piraeus-datastore"}, &deploy)
			}).Should(Not(HaveOccurred()))
		})

		It("should scale deployment resources", func(ctx context.Context) {
			var cluster piraeusiov1.LinstorCluster
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)
			Expect(err).NotTo(HaveOccurred())

			cluster.Spec.AffinityController = &piraeusiov1.DeploymentComponentSpec{
				Replicas: ptr.To(int32(2)),
			}
			cluster.Spec.CSIController = &piraeusiov1.DeploymentComponentSpec{
				Replicas: ptr.To(int32(3)),
			}

			err = k8sClient.Update(ctx, &cluster)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-affinity-controller"}, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(2))))

				err = k8sClient.Get(ctx, types.NamespacedName{Namespace: Namespace, Name: "linstor-csi-controller"}, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(deployment.Spec.Replicas).To(Equal(ptr.To(int32(3))))
			}).Should(Succeed())
		})

		Describe("with cluster nodes present", func() {
			BeforeEach(func(ctx context.Context) {
				nodes := []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-1a", Labels: map[string]string{
							"topology.kubernetes.io/zone": "a",
							"example.com/exclude":         "yes",
						}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-2a", Labels: map[string]string{"topology.kubernetes.io/zone": "a"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node-1b", Labels: map[string]string{"topology.kubernetes.io/zone": "b"}},
					},
				}

				for i := range nodes {
					err := k8sClient.Create(ctx, &nodes[i])
					Expect(err).NotTo(HaveOccurred())

					// Nodes automatically get the "not-ready" taint, we remove that so we can assume a "working"
					// cluster.
					nodes[i].Spec.Taints = nil
					err = k8sClient.Update(ctx, &nodes[i])
					Expect(err).NotTo(HaveOccurred())
				}
			})

			AfterEach(func(ctx context.Context) {
				err := k8sClient.DeleteAllOf(ctx, &corev1.Node{})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorSatelliteConfiguration{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create LinstorSatellite resources", func(ctx context.Context) {
				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return satellites.Items
				}).Should(HaveLen(3))
			})

			It("should apply LinstorSatelliteConfigs to matching nodes", func(ctx context.Context) {
				err := k8sClient.Create(ctx, &piraeusiov1.LinstorSatelliteConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "00-all-satellites"},
					Spec: piraeusiov1.LinstorSatelliteConfigurationSpec{
						Properties: []piraeusiov1.LinstorNodeProperty{
							{Name: "prop1", Value: "val1"},
							{Name: "prop2", Value: "val2"},
						},
						StoragePools: []piraeusiov1.LinstorStoragePool{
							{Name: "pool1", LvmPool: &piraeusiov1.LinstorStoragePoolLvm{}},
							{Name: "pool2", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{}},
						},
						Patches: []piraeusiov1.Patch{
							{Target: &piraeusiov1.Selector{Kind: "ServiceAccount"}, Patch: "sa-patch1"},
						},
						InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{},
						EvacuationStrategy: &piraeusiov1.EvacuationStrategy{
							AttachedVolumeReattachTimeout: metav1.Duration{Duration: 1 * time.Minute},
							UnattachedVolumeAttachTimeout: metav1.Duration{Duration: 2 * time.Minute},
						},
					},
				})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Create(ctx, &piraeusiov1.LinstorSatelliteConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "01-all-zone-a"},
					Spec: piraeusiov1.LinstorSatelliteConfigurationSpec{
						NodeSelector: map[string]string{"topology.kubernetes.io/zone": "a"},
						Properties: []piraeusiov1.LinstorNodeProperty{
							{Name: "prop2", Value: "new-val-2"},
							{Name: "prop3", Value: "val3"},
						},
						StoragePools: []piraeusiov1.LinstorStoragePool{
							{Name: "pool2", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}, Source: &piraeusiov1.LinstorStoragePoolSource{HostDevices: []string{"/dev/vdb"}}},
						},
						Patches: []piraeusiov1.Patch{
							{Target: &piraeusiov1.Selector{Kind: "Pod"}, Patch: "pod-patch1"},
						},
						DeletionPolicy: piraeusiov1.DeletionPolicyEvacuate,
					},
				})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					var satelliteConfigs piraeusiov1.LinstorSatelliteConfigurationList
					err := k8sClient.List(ctx, &satelliteConfigs)
					Expect(err).NotTo(HaveOccurred())
					Expect(satelliteConfigs.Items).To(HaveLen(2))

					for i := range satelliteConfigs.Items {
						cond := meta.FindStatusCondition(satelliteConfigs.Items[i].Status.Conditions, string(conditions.Applied))
						if cond == nil || cond.ObservedGeneration != satelliteConfigs.Items[i].Generation {
							return false
						}
					}

					return true
				}).Should(BeTrue())

				var satNode1A, satNode1B, satNode2A piraeusiov1.LinstorSatellite
				err = k8sClient.Get(ctx, types.NamespacedName{Name: "node-1a"}, &satNode1A)
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: "node-1b"}, &satNode1B)
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Get(ctx, types.NamespacedName{Name: "node-2a"}, &satNode2A)
				Expect(err).NotTo(HaveOccurred())

				defaultProps := []piraeusiov1.LinstorNodeProperty{
					{Name: "Aux/topology/linbit.com/hostname", ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.name"}},
					{Name: "Aux/topology/kubernetes.io/hostname", ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['kubernetes.io/hostname']"}},
					{Name: "Aux/topology/topology.kubernetes.io/region", ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/region']"}, Optional: true},
					{Name: "Aux/topology/topology.kubernetes.io/zone", ValueFrom: &piraeusiov1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/zone']"}, Optional: true},
				}

				specZoneA := &piraeusiov1.LinstorSatelliteSpec{
					ClusterRef: piraeusiov1.ClusterReference{Name: "default"},
					Patches: []piraeusiov1.Patch{
						{Target: &piraeusiov1.Selector{Kind: "ServiceAccount"}, Patch: "sa-patch1"},
						{Target: &piraeusiov1.Selector{Kind: "Pod"}, Patch: "pod-patch1"},
					},
					Properties: append(defaultProps,
						piraeusiov1.LinstorNodeProperty{Name: "prop1", Value: "val1"},
						piraeusiov1.LinstorNodeProperty{Name: "prop2", Value: "new-val-2"},
						piraeusiov1.LinstorNodeProperty{Name: "prop3", Value: "val3"},
					),
					StoragePools: []piraeusiov1.LinstorStoragePool{
						{Name: "pool1", LvmPool: &piraeusiov1.LinstorStoragePoolLvm{}},
						{Name: "pool2", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}, Source: &piraeusiov1.LinstorStoragePoolSource{HostDevices: []string{"/dev/vdb"}}},
					},
					InternalTLS:    &piraeusiov1.TLSConfigWithHandshakeDaemon{},
					DeletionPolicy: piraeusiov1.DeletionPolicyEvacuate,
					EvacuationStrategy: piraeusiov1.EvacuationStrategy{
						AttachedVolumeReattachTimeout: metav1.Duration{Duration: 1 * time.Minute},
						UnattachedVolumeAttachTimeout: metav1.Duration{Duration: 2 * time.Minute},
					},
				}

				specZoneB := &piraeusiov1.LinstorSatelliteSpec{
					ClusterRef: piraeusiov1.ClusterReference{Name: "default"},
					Patches: []piraeusiov1.Patch{
						{Target: &piraeusiov1.Selector{Kind: "ServiceAccount"}, Patch: "sa-patch1"},
					},
					Properties: append(defaultProps,
						piraeusiov1.LinstorNodeProperty{Name: "prop1", Value: "val1"},
						piraeusiov1.LinstorNodeProperty{Name: "prop2", Value: "val2"},
					),
					StoragePools: []piraeusiov1.LinstorStoragePool{
						{Name: "pool1", LvmPool: &piraeusiov1.LinstorStoragePoolLvm{}},
						{Name: "pool2", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{}},
					},
					InternalTLS:    &piraeusiov1.TLSConfigWithHandshakeDaemon{},
					DeletionPolicy: piraeusiov1.DeletionPolicyRetain,
					EvacuationStrategy: piraeusiov1.EvacuationStrategy{
						AttachedVolumeReattachTimeout: metav1.Duration{Duration: 1 * time.Minute},
						UnattachedVolumeAttachTimeout: metav1.Duration{Duration: 2 * time.Minute},
					},
				}

				// The first patch is always for tolerations. We ignore this here, as this is not related to
				// LinstorSatelliteConfigurations.
				satNode1A.Spec.Patches = satNode1A.Spec.Patches[1:]
				satNode1B.Spec.Patches = satNode1B.Spec.Patches[1:]
				satNode2A.Spec.Patches = satNode2A.Spec.Patches[1:]

				Expect(&satNode1A.Spec).To(Equal(specZoneA))
				Expect(&satNode1B.Spec).To(Equal(specZoneB))
				Expect(&satNode2A.Spec).To(Equal(specZoneA))
			})

			It("should apply changes made to the cluster resource", func(ctx context.Context) {
				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return satellites.Items
				}).Should(HaveLen(3))

				var cluster piraeusiov1.LinstorCluster
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)
				Expect(err).NotTo(HaveOccurred())

				cluster.Spec.Repository = "piraeus.io/test"
				cluster.Spec.NodeSelector = map[string]string{"topology.kubernetes.io/zone": "a"}

				err = k8sClient.Update(ctx, &cluster)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() []string {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					var result []string
					for i := range satellites.Items {
						if satellites.Items[i].DeletionTimestamp == nil {
							result = append(result, satellites.Items[i].Name)
						}
					}
					return result
				}).Should(ConsistOf("node-1a", "node-2a"))

				Eventually(func() string {
					var controllerDeployment appsv1.Deployment
					err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
					Expect(err).NotTo(HaveOccurred())
					controller := GetContainer(controllerDeployment.Spec.Template.Spec.Containers, "linstor-controller")
					Expect(controller).NotTo(BeNil())
					return controller.Image
				}).Should(HavePrefix("piraeus.io/test"))
			})

			It("should apply affinity set on the cluster resource", func(ctx context.Context) {
				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return satellites.Items
				}).Should(HaveLen(3))

				cluster := piraeusiov1.LinstorCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "default"},
					Spec: piraeusiov1.LinstorClusterSpec{
						NodeAffinity: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "topology.kubernetes.io/zone",
										Operator: corev1.NodeSelectorOpNotIn,
										Values:   []string{"b"},
									},
									{
										Key:      "example.com/exclude",
										Operator: corev1.NodeSelectorOpDoesNotExist,
									},
								},
							}},
						},
					},
				}

				err := k8sClient.Patch(ctx, &cluster, client.MergeFrom(&piraeusiov1.LinstorCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "default"},
				}))
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() []string {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					var result []string
					for i := range satellites.Items {
						if satellites.Items[i].DeletionTimestamp == nil {
							result = append(result, satellites.Items[i].Name)
						}
					}
					return result
				}).Should(ConsistOf("node-2a"))
			})

			It("should respect nodes taints", func(ctx context.Context) {
				var nodes corev1.NodeList
				err := k8sClient.List(ctx, &nodes)
				Expect(err).NotTo(HaveOccurred())
				Expect(nodes.Items).Should(HaveLen(3))

				taintsToAdd := []corev1.Taint{
					// A HA Controller taint we ignore by default.
					{Key: metadata.NodeForceIoErrorTaint, Effect: corev1.TaintEffectNoSchedule},
					// Another "core" taint we ignore by default.
					{Key: corev1.TaintNodeUnschedulable, Effect: corev1.TaintEffectNoSchedule},
					// A Taint we manually tolerate later.
					{Key: "example.com/manual-taint", Effect: corev1.TaintEffectNoExecute},
					// A Taint we never tolerate.
					{Key: "example.com/untolerated-taint", Effect: corev1.TaintEffectNoExecute},
				}

				for i := range nodes.Items {
					// Apply two taints to the first node, three to the second, all four to the third
					nodes.Items[i].Spec.Taints = taintsToAdd[:i+2]
					err := k8sClient.Update(ctx, &nodes.Items[i])
					Expect(err).NotTo(HaveOccurred())
				}

				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())
					return satellites.Items
				}).Should(ConsistOf(
					// Only the first node has taints we always tolerate
					HaveField("Name", nodes.Items[0].Name),
				))

				// Update the LinstorCluster to tolerate an additional taint
				Eventually(func() error {
					var cluster piraeusiov1.LinstorCluster
					err = k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)
					if err != nil {
						return err
					}

					cluster.Spec.Tolerations = append(cluster.Spec.Tolerations, corev1.Toleration{
						Key:      "example.com/manual-taint",
						Operator: corev1.TolerationOpExists,
						Effect:   corev1.TaintEffectNoExecute,
					})
					return k8sClient.Update(ctx, &cluster)
				}).Should(Succeed())

				Eventually(func() appsv1.DaemonSet {
					var csiNodes appsv1.DaemonSet
					err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-node", Namespace: Namespace}, &csiNodes)
					Expect(err).NotTo(HaveOccurred())
					return csiNodes
				}).Should(HaveField("Spec.Template.Spec.Tolerations", ContainElement(corev1.Toleration{
					Key:      "example.com/manual-taint",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				})))

				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())
					return satellites.Items
				}).Should(ConsistOf(
					// The first node has taints we always tolerate
					HaveField("Name", nodes.Items[0].Name),
					// The second node has taints we now tolerate
					HaveField("Name", nodes.Items[1].Name),
				))

				Eventually(func() []appsv1.Deployment {
					var deployments appsv1.DeploymentList
					err := k8sClient.List(ctx, &deployments)
					Expect(err).NotTo(HaveOccurred())
					return deployments.Items
				}).Should(And(
					HaveLen(3), // 1 LINSTOR Controller, 1 CSI Controller, 1 Affinity Controller
					// LINSTOR Controller has some additional tolerations, which we do not test for here.
					HaveEach(HaveField("Spec.Template.Spec.Tolerations", ContainElements(
						append(
							slices.Clone(tolerations.HAControllerTolerations),
							corev1.Toleration{
								Key:      "example.com/manual-taint",
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							}),
					))),
				))

				// The Satellites, CSI nodes, and HA Controller should have a patch updating their tolerations
				Eventually(func() []appsv1.DaemonSet {
					var daemonSets appsv1.DaemonSetList
					err := k8sClient.List(ctx, &daemonSets)
					Expect(err).NotTo(HaveOccurred())
					return daemonSets.Items
				}).Should(And(
					HaveLen(5), // 2 Satellites DS, 1 CSI Node, 1 HA Controller, 1 NFS Server.
					HaveEach(HaveField("Spec.Template.Spec.Tolerations", ConsistOf(
						append(slices.Clone(tolerations.HAControllerTolerations),
							corev1.Toleration{
								Key:      "example.com/manual-taint",
								Operator: corev1.TolerationOpExists,
								Effect:   corev1.TaintEffectNoExecute,
							})),
					)),
				))
			})

			It("should keep scheduled satellites on '*NoSchedule' tainted nodes", func(ctx context.Context) {
				var nodes corev1.NodeList
				err := k8sClient.List(ctx, &nodes)
				Expect(err).NotTo(HaveOccurred())
				Expect(nodes.Items).Should(HaveLen(3))

				By("Ensuring we have all Satellites scheduled")
				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return satellites.Items
				}).Should(HaveLen(3))

				By("Tainting nodes with NoExecute, NoSchedule and PreferNoSchedule effects")
				taintsToAdd := []corev1.Taint{
					{Key: "example.com/manual-taint", Effect: corev1.TaintEffectNoExecute},
					{Key: "example.com/manual-taint", Effect: corev1.TaintEffectNoSchedule},
					{Key: "example.com/manual-taint", Effect: corev1.TaintEffectPreferNoSchedule},
				}

				for i := range nodes.Items {
					// Apply one taint to each node, all having different effects.
					nodes.Items[i].Spec.Taints = append(nodes.Items[i].Spec.Taints, taintsToAdd[i])
					err := k8sClient.Update(ctx, &nodes.Items[i])
					Expect(err).NotTo(HaveOccurred())
				}

				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())
					return satellites.Items
				}).Should(ConsistOf(
					// The satellite of the first node should be removed, as it has a NoExecute taint.
					HaveField("Name", nodes.Items[1].Name),
					HaveField("Name", nodes.Items[2].Name),
				))

				By("Deleting the remaining Satellites, only the PreferNoSchedule node should be recreated")
				err = k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorSatellite{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() []piraeusiov1.LinstorSatellite {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())
					return satellites.Items
				}).Should(ConsistOf(
					// The satellite of the first node was already removed by the NoExecute taint.
					// The satellite of the second node was removed, and now has a NoSchedule taint.
					// The satellite of the third node should be recreated, as we ignore PreferNoSchedule taints.
					HaveField("Name", nodes.Items[2].Name),
				))
			})
		})
	})

	It("should add TLS secrets to the LINSTOR Controller", func(ctx context.Context) {
		DeferCleanup(func(ctx context.Context) {
			err := k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorCluster{})
			Expect(err).NotTo(HaveOccurred())
		})

		err := k8sClient.Create(ctx, &piraeusiov1.LinstorCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Spec: piraeusiov1.LinstorClusterSpec{
				InternalTLS: &piraeusiov1.TLSConfig{SecretName: "my-controller-internal-tls"},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var controllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", "my-controller-internal-tls")))))
		}).Should(Succeed())
	})

	It("should not deploy a controller when using external controller ref", func(ctx context.Context) {
		DeferCleanup(func(ctx context.Context) {
			err := k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorCluster{})
			Expect(err).NotTo(HaveOccurred())
		})

		err := k8sClient.Create(ctx, &piraeusiov1.LinstorCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Spec: piraeusiov1.LinstorClusterSpec{
				ExternalController: &piraeusiov1.LinstorExternalControllerRef{
					URL: "http://linstor-controller.invalid:3370",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var csiControllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-controller", Namespace: Namespace}, &csiControllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())
			container := GetContainer(csiControllerDeployment.Spec.Template.Spec.Containers, "linstor-csi")
			g.Expect(container).NotTo(BeNil())
			g.Expect(container.Env[0]).To(Equal(corev1.EnvVar{Name: "LS_CONTROLLERS", Value: "http://linstor-controller.invalid:3370"}))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiControllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-affinity-controller", Namespace: Namespace}, &csiControllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())
			container := GetContainer(csiControllerDeployment.Spec.Template.Spec.Containers, "linstor-affinity-controller")
			g.Expect(container).NotTo(BeNil())
			g.Expect(container.Env[0]).To(Equal(corev1.EnvVar{Name: "LS_CONTROLLERS", Value: "http://linstor-controller.invalid:3370"}))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiDaemonSet appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-node", Namespace: Namespace}, &csiDaemonSet)
			g.Expect(err).NotTo(HaveOccurred())
			container := GetContainer(csiDaemonSet.Spec.Template.Spec.Containers, "linstor-csi")
			g.Expect(container).NotTo(BeNil())
			g.Expect(container.Env[0]).To(Equal(corev1.EnvVar{Name: "LS_CONTROLLERS", Value: "http://linstor-controller.invalid:3370"}))
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var controllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
			g.Expect(err).To(MatchError(errors.IsNotFound, "IsNotFound"))
		}).Should(Succeed())
	})

	It("should add TLS secrets to the LINSTOR Components, configuring HTTPS access", func(ctx context.Context) {
		DeferCleanup(func(ctx context.Context) {
			err := k8sClient.DeleteAllOf(ctx, &piraeusiov1.LinstorCluster{})
			Expect(err).NotTo(HaveOccurred())
		})

		err := k8sClient.Create(ctx, &piraeusiov1.LinstorCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "default"},
			Spec: piraeusiov1.LinstorClusterSpec{
				ApiTLS: &piraeusiov1.LinstorClusterApiTLS{
					ApiSecretName:                "my-api-tls",
					ClientSecretName:             "my-client-tls",
					CsiControllerSecretName:      "my-csi-controller-tls",
					CsiNodeSecretName:            "my-csi-node-tls",
					NFSServerSecretName:          "my-nfs-server-tls",
					AffinityControllerSecretName: "my-affinity-controller-tls",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var controllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", "my-api-tls")))))
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Projected.Sources", ContainElement(HaveField("Secret.Name", "my-client-tls")))))
		}).Should(Succeed())

		envCheck := func(g Gomega, container *corev1.Container, secretName string) {
			g.Expect(container).NotTo(BeNil())
			g.Expect(container.Env).To(ContainElement(Equal(corev1.EnvVar{
				Name:  "LS_CONTROLLERS",
				Value: "https://linstor-controller:3371",
			})))
			g.Expect(container.Env).To(ContainElement(Equal(corev1.EnvVar{
				Name: "LS_ROOT_CA",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "ca.crt",
					},
				},
			})))
			g.Expect(container.Env).To(ContainElement(Equal(corev1.EnvVar{
				Name: "LS_USER_CERTIFICATE",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "tls.crt",
					},
				},
			})))
			g.Expect(container.Env).To(ContainElement(Equal(corev1.EnvVar{
				Name: "LS_USER_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "tls.key",
					},
				},
			})))
		}

		Eventually(func(g Gomega) {
			var csiControllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-controller", Namespace: Namespace}, &csiControllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())

			linstorCsi := GetContainer(csiControllerDeployment.Spec.Template.Spec.Containers, "linstor-csi")
			envCheck(g, linstorCsi, "my-csi-controller-tls")
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiNodeDaemonSet appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-node", Namespace: Namespace}, &csiNodeDaemonSet)
			g.Expect(err).NotTo(HaveOccurred())

			linstorCsi := GetContainer(csiNodeDaemonSet.Spec.Template.Spec.Containers, "linstor-csi")
			envCheck(g, linstorCsi, "my-csi-node-tls")
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var affinityControllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-affinity-controller", Namespace: Namespace}, &affinityControllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())

			linstorCsi := GetContainer(affinityControllerDeployment.Spec.Template.Spec.Containers, "linstor-affinity-controller")
			envCheck(g, linstorCsi, "my-affinity-controller-tls")
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiNodeDaemonSet appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-nfs-server", Namespace: Namespace}, &csiNodeDaemonSet)
			g.Expect(err).NotTo(HaveOccurred())

			linstorWaitNodeOnline := GetContainer(csiNodeDaemonSet.Spec.Template.Spec.InitContainers, "linstor-wait-node-online")
			envCheck(g, linstorWaitNodeOnline, "my-nfs-server-tls")
		}).Should(Succeed())
	})
})

func GetContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}

	return nil
}
