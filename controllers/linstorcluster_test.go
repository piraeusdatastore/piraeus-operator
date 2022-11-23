package controllers_test

import (
	"context"
	"time"

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
	const (
		DefaultTimeout       = 30 * time.Second
		DefaultCheckInterval = 5 * time.Second
	)

	Context("When creating an empty LinstorCluster", func() {
		BeforeEach(func(ctx context.Context) {
			err := k8sClient.Create(ctx, &piraeusiov1.LinstorCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
		})
		AfterEach(func(ctx context.Context) {
			err := k8sClient.Delete(ctx, &piraeusiov1.LinstorCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should set the available condition", func(ctx context.Context) {
			Eventually(func() bool {
				cluster := &piraeusiov1.LinstorCluster{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "default"}, cluster)
				if err != nil {
					return false
				}

				return meta.FindStatusCondition(cluster.Status.Conditions, string(conditions.Applied)) != nil
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
		})
		It("Should create controller resources", func(ctx context.Context) {
			Eventually(func() bool {
				deploy := appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: "linstor-controller", Namespace: "piraeus-datastore"}, &deploy)

				return err == nil
			}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
		})

		Describe("With cluster nodes present", func() {
			BeforeEach(func(ctx context.Context) {
				err := k8sClient.Create(ctx, &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: "node-1a", Labels: map[string]string{"topology.kubernetes.io/zone": "a"}},
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

			It("Should create LinstorSatellite resources", func(ctx context.Context) {
				Eventually(func() bool {
					var satellites piraeusiov1.LinstorSatelliteList
					err := k8sClient.List(ctx, &satellites)
					Expect(err).NotTo(HaveOccurred())

					return len(satellites.Items) == 3
				}, DefaultTimeout, DefaultCheckInterval).Should(BeTrue())
			})

			It("Should apply LinstorSatelliteConfigs to matching nodes", func(ctx context.Context) {
				err := k8sClient.Create(ctx, &piraeusiov1.LinstorSatelliteConfiguration{
					ObjectMeta: metav1.ObjectMeta{Name: "00-all-satellites"},
					Spec: piraeusiov1.LinstorSatelliteConfigurationSpec{
						Properties: []piraeusiov1.LinstorNodeProperty{
							{Name: "prop1", Value: "val1"},
							{Name: "prop2", Value: "val2"},
						},
						StoragePools: []piraeusiov1.LinstorStoragePool{
							{Name: "pool1", Lvm: &piraeusiov1.LinstorStoragePoolLvm{}},
							{Name: "pool2", LvmThin: &piraeusiov1.LinstorStoragePoolLvmThin{}},
						},
						Patches: []piraeusiov1.Patch{
							{Target: &piraeusiov1.Selector{Kind: "ServiceAccount"}, Patch: "sa-patch1"},
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
							{Name: "pool2", LvmThin: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}, Source: &piraeusiov1.LinstorStoragePoolSource{HostDevices: []string{"/dev/vdb"}}},
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
						{Name: "pool1", Lvm: &piraeusiov1.LinstorStoragePoolLvm{}},
						{Name: "pool2", LvmThin: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}, Source: &piraeusiov1.LinstorStoragePoolSource{HostDevices: []string{"/dev/vdb"}}},
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
						{Name: "pool1", Lvm: &piraeusiov1.LinstorStoragePoolLvm{}},
						{Name: "pool2", LvmThin: &piraeusiov1.LinstorStoragePoolLvmThin{}},
					},
				}

				Expect(&satNode1A.Spec).To(Equal(specZoneA))
				Expect(&satNode1B.Spec).To(Equal(specZoneB))
				Expect(&satNode2A.Spec).To(Equal(specZoneA))
			})
		})
	})
})
