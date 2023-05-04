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

package main

import (
	"flag"
	"os"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/controllers"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))

	utilruntime.Must(piraeusiov1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var namespace string
	var pullSecret string
	var imageConfigMapName string
	var linstorApiQps float64
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&namespace, "namespace", os.Getenv("NAMESPACE"), "The namespace to create resources in.")
	flag.StringVar(&pullSecret, "pull-secret", os.Getenv("PULL_SECRET"), "The pull secret to use for all containers")
	flag.StringVar(&imageConfigMapName, "image-config-map-name", os.Getenv("IMAGE_CONFIG_MAP_NAME"), "Config map holding default images to use")
	flag.Float64Var(&linstorApiQps, "linstor-api-qps", 100.0, "Limit requests to the LINSTOR API to this many queries per second")
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

	linstorLimiter := rate.NewLimiter(rate.Limit(linstorApiQps), 1)

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       vars.OperatorName,
		Namespace:              namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.LinstorClusterReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Namespace:          namespace,
		ImageConfigMapName: imageConfigMapName,
		PullSecret:         pullSecret,
		LinstorApiLimiter:  linstorLimiter,
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinstorCluster")
		os.Exit(1)
	}
	if err = (&controllers.LinstorSatelliteReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Namespace:          namespace,
		ImageConfigMapName: imageConfigMapName,
		LinstorApiLimiter:  linstorLimiter,
	}).SetupWithManager(mgr, controller.Options{}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LinstorSatellite")
		os.Exit(1)
	}
	if err = (&piraeusiov1.LinstorCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinstorCluster")
		os.Exit(1)
	}
	if err = (&piraeusiov1.LinstorSatellite{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinstorSatellite")
		os.Exit(1)
	}
	if err = (&piraeusiov1.LinstorSatelliteConfiguration{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "LinstorSatelliteConfiguration")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

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
