package controller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
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
			}, DefaultTimeout, DefaultCheckInterval).Should(BeEmpty())
		})

		It("should set the available condition", func(ctx context.Context) {
			Eventually(func() bool {
				cluster := &piraeusiov1.LinstorCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, cluster)
				if err != nil {
					return false
				}

				return meta.FindStatusCondition(cluster.Status.Conditions, string(conditions.Applied)) != nil
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
		})
		It("should create controller resources", func(ctx context.Context) {
			Eventually(func() bool {
				deploy := appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: "piraeus-datastore"}, &deploy)

				return err == nil
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
		})

		Describe("with cluster nodes present", func() {
			BeforeEach(func(ctx context.Context) {
				err := k8sClient.Create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1a", Labels: map[string]string{
						"topology.kubernetes.io/zone": "a",
						"example.com/exclude":         "yes",
					}},
				})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-2a", Labels: map[string]string{"topology.kubernetes.io/zone": "a"}},
				})
				Expect(err).NotTo(HaveOccurred())

				err = k8sClient.Create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1b", Labels: map[string]string{"topology.kubernetes.io/zone": "b"}},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx context.Context) {
				err := k8sClient.DeleteAllOf(ctx, &corev1.Node{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create LinstorSatellite resources", func(ctx context.Context) {
				Eventually(func() bool {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return len(satellites.Items) == 3
				}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
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
				}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())

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
					InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{},
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
					InternalTLS: &piraeusiov1.TLSConfigWithHandshakeDaemon{},
				}

				Expect(&satNode1A.Spec).To(Equal(specZoneA))
				Expect(&satNode1B.Spec).To(Equal(specZoneB))
				Expect(&satNode2A.Spec).To(Equal(specZoneA))
			})

			It("should apply changes made to the cluster resource", func(ctx context.Context) {
				Eventually(func() bool {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return len(satellites.Items) == 3
				}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())

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
				}, DefaultTimeout, DefaultCheckInterval).Should(ConsistOf("node-1a", "node-2a"))

				Eventually(func() string {
					var controllerDeployment appsv1.Deployment
					err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
					Expect(err).NotTo(HaveOccurred())
					controller := GetContainer(controllerDeployment.Spec.Template.Spec.Containers, "linstor-controller")
					Expect(controller).NotTo(BeNil())
					return controller.Image
				}, DefaultTimeout, DefaultCheckInterval).Should(HavePrefix("piraeus.io/test"))
			})

			It("should apply affinity set on the cluster resource", func(ctx context.Context) {
				Eventually(func() bool {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return len(satellites.Items) == 3
				}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())

				var cluster piraeusiov1.LinstorCluster
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, &cluster)
				Expect(err).NotTo(HaveOccurred())

				cluster.Spec.NodeAffinity = &corev1.NodeSelector{
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
				}

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
				}, DefaultTimeout, DefaultCheckInterval).Should(ConsistOf("node-2a"))
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
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Secret.SecretName", "my-controller-internal-tls")))
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
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
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiDaemonSet appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-node", Namespace: Namespace}, &csiDaemonSet)
			g.Expect(err).NotTo(HaveOccurred())
			container := GetContainer(csiDaemonSet.Spec.Template.Spec.Containers, "linstor-csi")
			g.Expect(container).NotTo(BeNil())
			g.Expect(container.Env[0]).To(Equal(corev1.EnvVar{Name: "LS_CONTROLLERS", Value: "http://linstor-controller.invalid:3370"}))
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

		var controllerDeployment appsv1.Deployment
		err = k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
		Expect(err).NotTo(BeNil())
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
					ApiSecretName:           "my-api-tls",
					ClientSecretName:        "my-client-tls",
					CsiControllerSecretName: "my-csi-controller-tls",
					CsiNodeSecretName:       "my-csi-node-tls",
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			var controllerDeployment appsv1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: Namespace}, &controllerDeployment)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Secret.SecretName", "my-api-tls")))
			g.Expect(controllerDeployment.Spec.Template.Spec.Volumes).To(ContainElement(HaveField("Secret.SecretName", "my-client-tls")))
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

		csiEnvCheck := func(g Gomega, container *corev1.Container, secretName string) {
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
			csiEnvCheck(g, linstorCsi, "my-csi-controller-tls")
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())

		Eventually(func(g Gomega) {
			var csiNodeDaemonSet appsv1.DaemonSet
			err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-csi-node", Namespace: Namespace}, &csiNodeDaemonSet)
			g.Expect(err).NotTo(HaveOccurred())

			linstorCsi := GetContainer(csiNodeDaemonSet.Spec.Template.Spec.Containers, "linstor-csi")
			csiEnvCheck(g, linstorCsi, "my-csi-node-tls")
		}, DefaultTimeout, DefaultCheckInterval).Should(Succeed())
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
