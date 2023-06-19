/*
Copyright 2023.

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

package main

import (
	"context"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var enableLeaderElection bool
	var probeAddr string
	var namespace string
	var webhookConfigurationName string
	var webhookServiceName string
	var webhookTlsSecretName string
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active instance.")
	flag.StringVar(&namespace, "namespace", os.Getenv("NAMESPACE"), "The namespace to create resources in.")
	flag.StringVar(&webhookConfigurationName, "webhook-configuration-name", os.Getenv("WEBHOOK_CONFIGURATION_NAME"), "The name of the ValidatingWebhookConfiguration.")
	flag.StringVar(&webhookServiceName, "webhook-service-name", os.Getenv("WEBHOOK_SERVICE_NAME"), "The name of the service used to route the webhook.")
	flag.StringVar(&webhookTlsSecretName, "webhook-tls-secret-name", os.Getenv("WEBHOOK_TLS_SECRET_NAME"), "The name of the tls-type secret used for serving the webhooks.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	if namespace == "" {
		setupLog.Info("No namespace specified, defaulting to operator namespace")
		raw, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			setupLog.Error(err, "unable to detect operator namespace")
			os.Exit(1)
		}
		namespace = string(raw)
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       vars.GenCertLeaderElectionID,
		Namespace:              namespace,
		NewClient: func(cache cache.Cache, config *rest.Config, options client.Options, uncachedObjects ...client.Object) (client.Client, error) {
			return client.New(config, options)
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	err = mgr.Add(&GenCertRunner{
		Client:                   mgr.GetClient(),
		Logger:                   mgr.GetLogger(),
		Namespace:                namespace,
		WebhookConfigurationName: webhookConfigurationName,
		WebhookServiceName:       webhookServiceName,
		WebhookTlsSecretName:     webhookTlsSecretName,
	})
	if err != nil {
		setupLog.Error(err, "unable to add gen cert runner")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", vars.Version)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

type GenCertRunner struct {
	Client                   client.Client
	Logger                   logr.Logger
	Namespace                string
	WebhookConfigurationName string
	WebhookServiceName       string
	WebhookTlsSecretName     string
}

func (g *GenCertRunner) NeedLeaderElection() bool {
	return true
}

func (g *GenCertRunner) Start(ctx context.Context) error {
	g.Logger.Info("starting cert renew")
	for {
		c, err := g.reconcile(ctx)
		if err != nil {
			return err
		}

		renewIn := NeedsRenewIn(c, g.dnsNames(), time.Now())
		g.Logger.Info("sleeping until cert renewal", "renewIn", renewIn)
		t := time.NewTimer(renewIn)

		select {
		case <-t.C:
			// continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (g *GenCertRunner) reconcile(ctx context.Context) (*x509.Certificate, error) {
	certBytes, keyBytes, err := g.getCert(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get initial cert: %w", err)
	}

	cs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificates: %w", err)
	}

	err = g.reconcileCertSecret(ctx, certBytes, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile certificate secret: %w", err)
	}

	err = g.reconcileWebhookCA(ctx, certBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile webhook ca config: %w", err)
	}

	return cs[0], nil
}

func (g *GenCertRunner) getCert(ctx context.Context) ([]byte, []byte, error) {
	g.Logger.Info("checking for existing secret")
	var secret corev1.Secret
	err := g.Client.Get(ctx, types.NamespacedName{Name: g.WebhookTlsSecretName, Namespace: g.Namespace}, &secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, nil, err
	}

	if err == nil {
		certs, _ := cert.ParseCertsPEM(secret.Data[corev1.TLSCertKey])
		if len(certs) > 0 && NeedsRenewIn(certs[0], g.dnsNames(), time.Now()) > 0 {
			g.Logger.Info("existing secret does not need to be renewed")
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		}
	}

	g.Logger.Info("creating new self signed secret")
	return cert.GenerateSelfSignedCertKey(g.dnsNames()[0], nil, g.dnsNames())
}

func (g *GenCertRunner) reconcileCertSecret(ctx context.Context, certBytes, keyBytes []byte) error {
	g.Logger.Info("applying cert as secret")
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.WebhookTlsSecretName,
			Namespace: g.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certBytes,
			corev1.TLSPrivateKeyKey: keyBytes,
		},
	}

	return g.Client.Patch(ctx, secret, client.Apply, client.ForceOwnership, client.FieldOwner(vars.FieldOwner))
}

func (g *GenCertRunner) reconcileWebhookCA(ctx context.Context, caBytes []byte) error {
	g.Logger.Info("checking webhook configuration")
	var webhookConfiguration v1.ValidatingWebhookConfiguration
	err := g.Client.Get(ctx, types.NamespacedName{Name: g.WebhookConfigurationName}, &webhookConfiguration)
	if err != nil {
		return err
	}

	changed := false
	for i := range webhookConfiguration.Webhooks {
		whc := &webhookConfiguration.Webhooks[i].ClientConfig
		if whc.Service == nil {
			continue
		}

		if whc.Service.Name != g.WebhookServiceName || whc.Service.Namespace != g.Namespace {
			continue
		}

		if string(whc.CABundle) != string(caBytes) {
			changed = true
			whc.CABundle = caBytes
		}
	}

	if changed {
		g.Logger.Info("applying changed webhook configuration")
		return g.Client.Update(ctx, &webhookConfiguration)
	}

	return nil
}

func (g *GenCertRunner) dnsNames() []string {
	return []string{
		fmt.Sprintf("%s.%s.svc", g.WebhookServiceName, g.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", g.WebhookServiceName, g.Namespace),
	}
}

func NeedsRenewIn(c *x509.Certificate, expectedNames []string, now time.Time) time.Duration {
	// There may be repeats in the cert, so we use a set to compare here
	if !sets.NewString(c.DNSNames...).Equal(sets.NewString(expectedNames...)) {
		return 0
	}

	if now.Before(c.NotBefore) {
		return 0
	}

	// Renew two weeks before actually expiring. Our certificates are always valid for a year, so using a fixed offset
	// should be good enough.
	return c.NotAfter.Sub(now) - (14 * 24 * time.Hour)
}
