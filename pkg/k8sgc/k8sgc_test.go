package k8sgc_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/k8sgc"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "K8s-gc Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = piraeusiov1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("K8s-gc", func() {
	var gc k8sgc.GC
	var namespace string

	BeforeEach(func(ctx context.Context) {
		var err error
		gc, err = k8sgc.New(ctx, k8sClient)
		Expect(err).NotTo(HaveOccurred())

		namespace = fmt.Sprintf("gc-%04d", rand.Intn(10_000))
		err = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should not be needed for simple deletions", func(ctx context.Context) {
		simpleSA := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: "simple", Namespace: namespace},
		}

		err := k8sClient.Create(ctx, &simpleSA)
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Delete(ctx, &simpleSA)
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: simpleSA.Name, Namespace: simpleSA.Namespace}, &corev1.ServiceAccount{})
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should delete objects when owners not present", func(ctx context.Context) {
		simpleSA := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "with-owner",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{Name: "non-existent-owner-sa", APIVersion: "v1", Kind: "ServiceAccount", UID: "00000000-0000-0000-0000-000000000000"},
				},
			},
		}

		err := k8sClient.Create(ctx, &simpleSA)
		Expect(err).NotTo(HaveOccurred())

		deleted, err := gc.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeTrue())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: simpleSA.Name, Namespace: simpleSA.Namespace}, &corev1.ServiceAccount{})
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should delete objects when owners not present in a cascade", func(ctx context.Context) {
		simpleSA := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "with-owner",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{Name: "non-existent-owner-sa", APIVersion: "v1", Kind: "ServiceAccount", UID: "00000000-0000-0000-0000-000000000000"},
				},
			},
		}
		err := k8sClient.Create(ctx, &simpleSA)
		Expect(err).NotTo(HaveOccurred())

		subOwnerSA := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "with-owned-owner",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					{Name: simpleSA.Name, APIVersion: "v1", Kind: "ServiceAccount", UID: simpleSA.UID},
				},
			},
		}
		err = k8sClient.Create(ctx, &subOwnerSA)
		Expect(err).NotTo(HaveOccurred())

		deleted, err := gc.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeTrue())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: simpleSA.Name, Namespace: simpleSA.Namespace}, &corev1.ServiceAccount{})
		Expect(errors.IsNotFound(err)).To(BeTrue())

		err = k8sClient.Get(ctx, client.ObjectKey{Name: subOwnerSA.Name, Namespace: subOwnerSA.Namespace}, &corev1.ServiceAccount{})
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should not delete objects with finalizer", func(ctx context.Context) {
		withFinalizer := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "with-owner",
				Namespace:  namespace,
				Finalizers: []string{"piraeus.io/test"},
			},
		}
		err := k8sClient.Create(ctx, &withFinalizer)
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Delete(ctx, &withFinalizer)
		Expect(err).NotTo(HaveOccurred())

		var currentSA corev1.ServiceAccount
		err = k8sClient.Get(ctx, client.ObjectKey{Name: withFinalizer.Name, Namespace: withFinalizer.Namespace}, &currentSA)
		Expect(err).NotTo(HaveOccurred())
		Expect(currentSA.DeletionTimestamp).NotTo(BeNil())

		deleted, err := gc.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeFalse())

		cleanupWithFinalizer(ctx, withFinalizer.Name, withFinalizer.Namespace)
	})

	It("should break cycles", func(ctx context.Context) {
		withFinalizer1 := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "with-owner-cycle1",
				Namespace:  namespace,
				Finalizers: []string{"piraeus.io/test"},
			},
		}
		err := k8sClient.Create(ctx, &withFinalizer1)
		Expect(err).NotTo(HaveOccurred())

		withFinalizer2 := corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "with-owner-cycle2",
				Namespace:  namespace,
				Finalizers: []string{"piraeus.io/test"},
				OwnerReferences: []metav1.OwnerReference{
					{Name: withFinalizer1.Name, APIVersion: "v1", Kind: "ServiceAccount", UID: withFinalizer1.UID},
				},
			},
		}
		err = k8sClient.Create(ctx, &withFinalizer2)
		Expect(err).NotTo(HaveOccurred())

		withFinalizer1.OwnerReferences = []metav1.OwnerReference{
			{Name: withFinalizer2.Name, APIVersion: "v1", Kind: "ServiceAccount", UID: withFinalizer2.UID},
		}
		err = k8sClient.Update(ctx, &withFinalizer1)
		Expect(err).NotTo(HaveOccurred())

		err = k8sClient.Delete(ctx, &withFinalizer2)
		Expect(err).NotTo(HaveOccurred())

		deleted, err := gc.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeTrue())

		cleanupWithFinalizer(ctx, withFinalizer1.Name, withFinalizer1.Namespace)
		cleanupWithFinalizer(ctx, withFinalizer2.Name, withFinalizer2.Namespace)
	})
})

func cleanupWithFinalizer(ctx context.Context, name, namespace string) {
	var currentSA corev1.ServiceAccount
	err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &currentSA)
	Expect(err).NotTo(HaveOccurred())
	Expect(currentSA.DeletionTimestamp).NotTo(BeNil())

	currentSA.Finalizers = nil
	err = k8sClient.Update(ctx, &currentSA)
	Expect(err).NotTo(HaveOccurred())

	err = k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &currentSA)
	Expect(err).To(HaveOccurred())
	Expect(errors.IsNotFound(err)).To(BeTrue())
}
