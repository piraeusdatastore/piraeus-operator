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

package v1_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	//+kubebuilder:scaffold:imports
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg            *rest.Config
	k8sClient      client.Client
	testEnv        *envtest.Environment
	cancelManager  context.CancelFunc
	warningHandler WarningHandler
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Webhook Suite")
}

var _ = BeforeSuite(func(_ context.Context) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

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
	err = piraeusv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	cfg.WarningHandler = &warningHandler
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme, WarningHandler: client.WarningHandlerOptions{SuppressWarnings: true}})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// start webhook server using Manager
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    webhookInstallOptions.LocalServingHost,
			Port:    webhookInstallOptions.LocalServingPort,
			CertDir: webhookInstallOptions.LocalServingCertDir,
		}),
		LeaderElection: false,
		Metrics:        metricsserver.Options{BindAddress: "0"},
	})
	Expect(err).NotTo(HaveOccurred())

	err = (&piraeusv1.LinstorCluster{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&piraeusv1.LinstorSatellite{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&piraeusv1.LinstorSatelliteConfiguration{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	err = (&piraeusv1.LinstorNodeConnection{}).SetupWebhookWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:webhook

	// We create our own context here, as the API server needs to run in the background
	ctx, cancel := context.WithCancel(context.Background())
	cancelManager = cancel

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
}, NodeTimeout(60*time.Second))

var _ = AfterSuite(func() {
	if cancelManager != nil {
		cancelManager()
	}

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

type Warning struct {
	code  int
	agent string
	text  string
}

type WarningHandler []Warning

func (w *WarningHandler) HandleWarningHeader(code int, agent string, text string) {
	if code == 0 {
		return
	}

	*w = append(*w, Warning{
		code:  code,
		agent: agent,
		text:  text,
	})
}

func (w *WarningHandler) Clear() {
	*w = (*w)[:0]
}
