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

var _ = Describe("LinstorNodeConnection webhook", func() {
	typeMeta := metav1.TypeMeta{
		Kind:       "LinstorNodeConnection",
		APIVersion: piraeusv1.GroupVersion.String(),
	}

	AfterEach(func(ctx context.Context) {
		err := k8sClient.DeleteAllOf(ctx, &piraeusv1.LinstorNodeConnection{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow empty node connection configuration", func(ctx context.Context) {
		nodeConnection := &piraeusv1.LinstorNodeConnection{TypeMeta: typeMeta, ObjectMeta: metav1.ObjectMeta{Name: "empty"}}
		err := k8sClient.Patch(ctx, nodeConnection, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow complex node connection configuration", func(ctx context.Context) {
		nodeConnection := &piraeusv1.LinstorNodeConnection{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "complex-config"},
			Spec: piraeusv1.LinstorNodeConnectionSpec{
				Selector: []piraeusv1.SelectorTerm{
					{
						MatchLabels: []piraeusv1.MatchLabelSelector{
							{
								Key:    "kubernetes.io/hostname",
								Op:     piraeusv1.MatchLabelSelectorOpNotIn,
								Values: []string{"node-a", "node-b"},
							},
							{
								Key:    "topology.kubernetes.io/zone",
								Op:     piraeusv1.MatchLabelSelectorOpIn,
								Values: []string{"zone-a", "zone-b"},
							},
						},
					},
					{
						MatchLabels: []piraeusv1.MatchLabelSelector{
							{
								Key: "kubernetes.io/hostname",
								Op:  piraeusv1.MatchLabelSelectorOpExists,
							},
							{
								Key: "topology.kubernetes.io/region",
								Op:  piraeusv1.MatchLabelSelectorOpSame,
							},
							{
								Key: "topology.kubernetes.io/zone",
								Op:  piraeusv1.MatchLabelSelectorOpNotSame,
							},
						},
					},
				},
				Paths: []piraeusv1.LinstorNodeConnectionPath{
					{
						Name:      "path1",
						Interface: "if1",
					},
					{
						Name:      "path2",
						Interface: "if2",
					},
				},
				Properties: []piraeusv1.LinstorControllerProperty{
					{
						Name:  "Aux/prop1",
						Value: "val1",
					},
					{
						Name:  "Aux/prop2",
						Value: "val2",
					},
				},
			},
		}
		err := k8sClient.Patch(ctx, nodeConnection, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should reject duplicate properties and paths", func(ctx context.Context) {
		nodeConnection := &piraeusv1.LinstorNodeConnection{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "duplicate"},
			Spec: piraeusv1.LinstorNodeConnectionSpec{
				Paths: []piraeusv1.LinstorNodeConnectionPath{
					{
						Name:      "path1",
						Interface: "if1",
					},
					{
						Name:      "path1",
						Interface: "if2",
					},
				},
				Properties: []piraeusv1.LinstorControllerProperty{
					{
						Name:  "Aux/prop1",
						Value: "val1",
					},
					{
						Name:  "Aux/prop1",
						Value: "val2",
					},
				},
			},
		}
		err := k8sClient.Patch(ctx, nodeConnection, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
	})

	It("should reject improper node selectors", func(ctx context.Context) {
		nodeConnection := &piraeusv1.LinstorNodeConnection{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-selectors"},
			Spec: piraeusv1.LinstorNodeConnectionSpec{
				Selector: []piraeusv1.SelectorTerm{
					{
						MatchLabels: []piraeusv1.MatchLabelSelector{
							{
								Key:    "kubernetes.io/hostname",
								Op:     piraeusv1.MatchLabelSelectorOpIn,
								Values: []string{"node-a", "node-b"},
							},
							{
								Key:    "topology.kubernetes.io/zone",
								Op:     piraeusv1.MatchLabelSelectorOpDoesNotExist,
								Values: []string{"zone-a", "zone-b"},
							},
						},
					},
					{
						MatchLabels: []piraeusv1.MatchLabelSelector{
							{
								Key: "kubernetes.io/hostname",
								Op:  piraeusv1.MatchLabelSelectorOpExists,
							},
							{
								Key:    "topology.kubernetes.io/region",
								Op:     piraeusv1.MatchLabelSelectorOpSame,
								Values: []string{"region-1"},
							},
							{
								Key:    "topology.kubernetes.io/zone",
								Op:     piraeusv1.MatchLabelSelectorOpNotSame,
								Values: []string{"zone-a", "zone-b"},
							},
						},
					},
				},
			},
		}
		err := k8sClient.Patch(ctx, nodeConnection, client.Apply, client.FieldOwner("test"), client.ForceOwnership)
		Expect(err).To(HaveOccurred())
		statusErr := err.(*errors.StatusError)
		Expect(statusErr).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details).NotTo(BeNil())
		Expect(statusErr.ErrStatus.Details.Causes).To(HaveLen(3))
	})
})
