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

var _ = Describe("LinstorCluster webhook", func() {
	typeMeta := metav1.TypeMeta{
		Kind:       "LinstorCluster",
		APIVersion: piraeusv1.GroupVersion.String(),
	}
	complexCluster := &piraeusv1.LinstorCluster{
		TypeMeta:   typeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Spec: piraeusv1.LinstorClusterSpec{
			NodeSelector: map[string]string{
				"node-role.kubernetes.io/with-storage": "",
			},
			Properties: []piraeusv1.LinstorControllerProperty{
				{Name: "PrefNic", Value: "default-ipv6"},
				{Name: "Aux/foo", Value: "bar"},
			},
			Patches: []piraeusv1.Patch{
				{
					Target: &piraeusv1.Selector{
						Name: "linstor-csi-controller",
						Kind: "Deployment",
					},
					Patch: "[{\"op\": \"replace\", \"path\": \"/metadata/name\", \"value\": \"linstor-csi-controller-of-the-universe\"}]",
				},
			},
		},
	}

	AfterEach(func(ctx context.Context) {
		err := k8sClient.DeleteAllOf(ctx, &piraeusv1.LinstorCluster{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow empty cluster", func(ctx context.Context) {
		cluster := &piraeusv1.LinstorCluster{TypeMeta: typeMeta, ObjectMeta: metav1.ObjectMeta{Name: "cluster1"}}
		err := k8sClient.Patch(ctx, cluster, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow complex cluster", func(ctx context.Context) {
		err := k8sClient.Patch(ctx, complexCluster.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow updating a complex cluster", func(ctx context.Context) {
		err := k8sClient.Patch(ctx, complexCluster.DeepCopy(), client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())

		clusterCopy := complexCluster.DeepCopy()
		clusterCopy.Spec.Patches[0].Patch = "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: linstor-controller\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1\nspec:\n  containers:\n  - name: linstor-satellite\n    volumeMounts:\n    - name: var-lib-drbd\n      mountPath: /var/lib/drbd\n  volumes:\n  - name: var-lib-drbd\n    emptyDir: {}\n"
		err = k8sClient.Patch(ctx, clusterCopy, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should reject improper node selectors", func(ctx context.Context) {
		clusterConfig := &piraeusv1.LinstorCluster{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-labels"},
			Spec: piraeusv1.LinstorClusterSpec{
				NodeSelector: map[string]string{
					"example.com/key1":           "valid-label",
					"12.34.not+a+valid+key/key1": "valid-value",
					"example.com/key2":           "not a valid value",
				},
			},
		}
		err := k8sClient.Patch(ctx, clusterConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
	})

	It("should reject improper patches", func(ctx context.Context) {
		clusterConfig := &piraeusv1.LinstorCluster{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-labels"},
			Spec: piraeusv1.LinstorClusterSpec{
				Patches: []piraeusv1.Patch{
					{Patch: "# just some random yaml, not a partial k8s resource\nroses: red\nviolets: blue\n"},
					{Patch: "not even any structure at all\njust some random text"},
				},
			},
		}
		err := k8sClient.Patch(ctx, clusterConfig, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(2))
	})
})
