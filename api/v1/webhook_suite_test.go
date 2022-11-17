/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//+kubebuilder:scaffold:imports
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Webhook Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
		},
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme := runtime.NewScheme()
	err = AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&LinstorSatellite{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
}, 60)

var _ = Describe("LinstorSatellite webhook", func() {
	typeMeta := metav1.TypeMeta{
		Kind:       "LinstorSatellite",
		APIVersion: GroupVersion.String(),
	}
	complexSatellite := &LinstorSatellite{
		TypeMeta:   typeMeta,
		ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"},
		Spec: LinstorSatelliteSpec{
			ClusterRef: ClusterReference{Name: "cluster"},
			Repository: "",
			Patches: []Patch{
				{
					Target: &Selector{
						Name: "satellite",
						Kind: "Pod",
					},
					Patch: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: satellite\n  annotations:\n    k8s.v1.cni.cncf.io/networks: eth1",
				},
			},
			StoragePools: []LinstorStoragePool{
				{
					Name: "thinpool",
					LvmThin: &LinstorStoragePoolLvmThin{
						VolumeGroup: "linstor_thinpool",
						ThinPool:    "thinpool",
					},
					Source: &LinstorStoragePoolSource{
						HostDevices: []string{"/dev/vdb"},
					},
				},
			},
			Properties: []LinstorNodeProperty{
				{
					Name:      "Aux/topology/linbit.com/hostname",
					ValueFrom: &LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.name"},
				},
				{
					Name:      "Aux/topology/kubernetes.io/hostname",
					ValueFrom: &LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['kubernetes.io/hostname']"},
				},
				{
					Name:      "Aux/topology/topology.kubernetes.io/region",
					ValueFrom: &LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/region']"},
				},
				{
					Name:      "Aux/topology/topology.kubernetes.io/zone",
					ValueFrom: &LinstorNodePropertyValueFrom{NodeFieldRef: "metadata.labels['topology.kubernetes.io/zone']"},
				},
				{
					Name:  "PrefNic",
					Value: "default-ipv4",
				},
			},
		},
	}

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &LinstorSatellite{})
		Expect(err).NotTo(HaveOccurred())
	})

	It("should allow empty satellite", func() {
		satellite := &LinstorSatellite{TypeMeta: typeMeta, ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"}}
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
		satellite := &LinstorSatellite{
			TypeMeta:   typeMeta,
			ObjectMeta: metav1.ObjectMeta{Name: "node-1.example.com"},
			Spec: LinstorSatelliteSpec{
				StoragePools: []LinstorStoragePool{
					{Name: "missing-type"},
					{Name: "multiple-types", Lvm: &LinstorStoragePoolLvm{}, LvmThin: &LinstorStoragePoolLvmThin{}},
					{Name: "valid-pool", Lvm: &LinstorStoragePoolLvm{}},
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

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
