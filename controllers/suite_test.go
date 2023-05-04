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

package controllers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/controllers"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/k8sgc"
	//+kubebuilder:scaffold:imports
)

const (
	DefaultTimeout       = 30 * time.Second
	DefaultCheckInterval = 5 * time.Second
	Namespace            = "piraeus-datastore"
	ImageConfigMapName   = "image-config"
	ExampleNodeName      = "node1.example.com"
)

var (
	cancelManager context.CancelFunc
	cfg           *rest.Config
	k8sClient     client.Client
	testEnv       *envtest.Environment
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	imageConfig, err := os.ReadFile(filepath.Join("..", "config", "manager", "images.yaml"))
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = certmanagerv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = piraeusiov1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	ctx, cancel := context.WithCancel(context.Background())
	cancelManager = cancel

	err = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: Namespace}})
	Expect(err).NotTo(HaveOccurred())

	err = k8sClient.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ImageConfigMapName,
			Namespace: Namespace,
		},
		Data: map[string]string{
			"images.yaml": string(imageConfig),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	// Increase the requeue rate limit to make tests faster and more stable
	opts := controller.Options{
		RateLimiter: workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(5*time.Millisecond, 1*time.Second),
			&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(10), 100)},
		),
	}

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	unlimiter := rate.NewLimiter(rate.Inf, 0)

	err = (&controllers.LinstorClusterReconciler{
		Client:             k8sManager.GetClient(),
		Scheme:             k8sManager.GetScheme(),
		Namespace:          Namespace,
		ImageConfigMapName: ImageConfigMapName,
		LinstorApiLimiter:  unlimiter,
	}).SetupWithManager(k8sManager, opts)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.LinstorSatelliteReconciler{
		Client:             k8sManager.GetClient(),
		Scheme:             k8sManager.GetScheme(),
		Namespace:          Namespace,
		ImageConfigMapName: ImageConfigMapName,
		LinstorApiLimiter:  unlimiter,
	}).SetupWithManager(k8sManager, opts)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err := k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	go func() {
		defer GinkgoRecover()
		gc, err := k8sgc.New(ctx, k8sClient)
		Expect(err).ToNot(HaveOccurred(), "failed to create GC")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err = gc.Run(ctx)
				if ctx.Err() != nil {
					return
				}

				Expect(err).ToNot(HaveOccurred(), "failed to run GC")
			}
		}
	}()
})

var _ = AfterSuite(func() {
	if cancelManager != nil {
		cancelManager()
	}

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
