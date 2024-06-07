package v1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var _ = Describe("LinstorSatelliteConfiguration webhook", func() {
	typeMeta := metav1.TypeMeta{
		Kind:       "LinstorSatelliteConfiguration",
		APIVersion: piraeusv1.GroupVersion.String(),
	}
	complexSatelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
		TypeMeta:   typeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: "example-nodes"},
		Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": "node-1.example.com",
			},
			Patches: []piraeusv1.Patch{
				{
					Target: &piraeusv1.Selector{
						Name: "linstor-satellite",
						Kind: "DaemonSet",
					},
					Patch: "apiVersion: apps/v1\nkind: DaemonSet\nmetadata:\n  name: linstor-satellite\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1",
				},
			},
			StoragePools: []piraeusv1.LinstorStoragePool{
				{
					Name: "thinpool",
					LvmThinPool: &piraeusv1.LinstorStoragePoolLvmThin{
						VolumeGroup: "linstor_thinpool",
						ThinPool:    "thinpool",
					},
					Source: &piraeusv1.LinstorStoragePoolSource{
						HostDevices: []string{"/dev/vdb"},
					},
				},
			},
			Properties: []piraeusv1.LinstorNodeProperty{
				{
					Name:      "Aux/topology/linbit.com/hostname",
					ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.name"},
				},
				{
					Name:      "Aux/topology/kubernetes.io/hostname",
					ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['kubernetes.io/hostname']"},
				},
				{
					Name:      "Aux/topology/topology.kubernetes.io/region",
					ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/region']"},
				},
				{
					Name:      "Aux/topology/topology.kubernetes.io/zone",
					ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/zone']"},
				},
				{
					Name:  "PrefNic",
					Value: "default-ipv4",
				},
			},
		},
	}

	AfterEach(func(ctx context.Context) {
		err := k8sClient.DeleteAllOf(ctx, &piraeusv1.LinstorSatelliteConfiguration{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow empty satellite configuration", func(ctx context.Context) {
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{TypeMeta: typeMeta, ObjectMeta: metav1.ObjectMeta{Name: "all-satellites"}}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow complex satellite", func(ctx context.Context) {
		err := k8sClient.Patch(ctx, complexSatelliteConfig.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow updating a complex satellite", func(ctx context.Context) {
		err := k8sClient.Patch(ctx, complexSatelliteConfig.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())

		satelliteConfigCopy := complexSatelliteConfig.DeepCopy()
		satelliteConfigCopy.Spec.Patches[0].Patch = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: satellite\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1\nspec:\n  containers:\n  - name: linstor-satellite\n    volumeMounts:\n    - name: var-lib-drbd\n      mountPath: /var/lib/drbd\n  volumes:\n  - name: var-lib-drbd\n    emptyDir: {}\n"
		err = k8sClient.Patch(ctx, satelliteConfigCopy, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should require exactly one pool type for storage pools", func(ctx context.Context) {
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "storage-pools"},
			Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
				StoragePools: []piraeusv1.LinstorStoragePool{
					{Name: "missing-type"},
					{Name: "multiple-types", LvmPool: &piraeusv1.LinstorStoragePoolLvm{}, LvmThinPool: &piraeusv1.LinstorStoragePoolLvmThin{}},
					{Name: "valid-pool", LvmPool: &piraeusv1.LinstorStoragePoolLvm{}},
				},
			},
		}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
		Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.storagePools.0"))
		Expect(statusErr.ErrStatus.Details.Causes[1].Field).To(Equal("spec.storagePools.1.lvmPool"))
	})

	It("should reject improper node selectors", func(ctx context.Context) {
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-labels"},
			Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
				NodeSelector: map[string]string{
					"example.com/key1":           "valid-label",
					"12.34.not+a+valid+key/key1": "valid-value",
					"example.com/key2":           "not a valid value",
				},
			},
		}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
	})

	It("should reject patches without target", func(ctx context.Context) {
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "bare-pod-patches"},
			Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
				Patches: []piraeusv1.Patch{
					{
						// This patch is fine, target information from strategic merge patch
						Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: reactor-config\n",
					},
					{
						// This patch is not: no target information
						Patch: "- op: add\n  path: /metadata/labels/foo\n  value: bar",
					},
				},
			},
		}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(1))
		Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.patches.1.target"))
	})

	It("should warn on using bare pod patches", func(ctx context.Context) {
		warningHandler.Clear()
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "bare-pod-patches"},
			Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
				Patches: []piraeusv1.Patch{{
					Patch: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: satellite\nspec:\n  hostNetwork: true",
				}},
			},
		}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
		Expect(warningHandler).To(HaveLen(1))
		Expect(warningHandler[0].text).To(ContainSubstring("consider targeting the DaemonSet 'linstor-satellite'"))
	})

	It("should reject wildcard properties without replacement target", func(ctx context.Context) {
		satelliteConfig := &piraeusv1.LinstorSatelliteConfiguration{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "wildcard-properties"},
			Spec: piraeusv1.LinstorSatelliteConfigurationSpec{
				Properties: []piraeusv1.LinstorNodeProperty{
					{
						Name:      "missing-target-name",
						ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.annotations['example.com/*']"},
					},
					{
						Name:      "invalid-reference",
						ValueFrom: &piraeusv1.LinstorNodePropertyValueFrom{NodeFieldRef: "something random"},
					},
				},
			},
		}
		err := k8sClient.Patch(ctx, satelliteConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
		Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.properties.0.name"))
		Expect(statusErr.ErrStatus.Details.Causes[1].Field).To(Equal("spec.properties.1.valueFrom.nodeFieldRef"))
	})
})
