package v1_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

var _ = Describe("LinstorSatellite webhook", func() {
	typeMeta := metav1.TypeMeta{
		Kind:       "LinstorSatellite",
		APIVersion: piraeusv1.GroupVersion.String(),
	}
	complexSatellite := &piraeusv1.LinstorSatellite{
		TypeMeta:   typeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"},
		Spec: piraeusv1.LinstorSatelliteSpec{
			ClusterRef: piraeusv1.ClusterReference{Name: "cluster"},
			Repository: "",
			Patches: []piraeusv1.Patch{
				{
					Target: &piraeusv1.Selector{
						Name: "satellite",
						Kind: "Pod",
					},
					Patch: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: satellite\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1",
				},
			},
			StoragePools: []piraeusv1.LinstorStoragePool{
				{
					Name: "thinpool",
					LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{
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

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &piraeusv1.LinstorSatellite{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow empty satellite", func() {
		satellite := &piraeusv1.LinstorSatellite{TypeMeta: typeMeta, ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"}}
		err := k8sClient.Patch(ctx, satellite, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow complex satellite", func() {
		err := k8sClient.Patch(ctx, complexSatellite.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow updating a complex satellite", func() {
		err := k8sClient.Patch(ctx, complexSatellite.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())

		satelliteCopy := complexSatellite.DeepCopy()
		satelliteCopy.Spec.Patches[0].Patch = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: satellite\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1\nspec:\n  containers:\n  - name: linstor-satellite\n    volumeMounts:\n    - name: var-lib-drbd\n      mountPath: /var/lib/drbd\n  volumes:\n  - name: var-lib-drbd\n    emptyDir: {}\n"
		err = k8sClient.Patch(ctx, satelliteCopy, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should require exactly one pool type for storage pools", func() {
		satellite := &piraeusv1.LinstorSatellite{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"},
			Spec: piraeusv1.LinstorSatelliteSpec{
				StoragePools: []piraeusv1.LinstorStoragePool{
					{Name: "missing-type"},
					{Name: "multiple-types", Lvm: &piraeusv1.LinstorStoragePoolLvm{}, LvmThin: &piraeusv1.LinstorStoragePoolLvmThin{}},
					{Name: "valid-pool", Lvm: &piraeusv1.LinstorStoragePoolLvm{}},
				},
			},
		}
		err := k8sClient.Patch(ctx, satellite, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
		Expect(statusErr.ErrStatus.Details.Causes[0].Field).To(Equal("spec.storagePools.0"))
		Expect(statusErr.ErrStatus.Details.Causes[1].Field).To(Equal("spec.storagePools.1.lvm"))
	})
})
