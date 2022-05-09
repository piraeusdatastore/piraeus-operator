package monitoring

import (
	"context"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MutateServiceMonitor = func(*monitoringv1.ServiceMonitor)

func Enabled(ctx context.Context, kubeClient client.Client, scheme *runtime.Scheme) bool {
	err := monitoringv1.AddToScheme(scheme)
	if err != nil {
		return false
	}

	var monitors monitoringv1.ServiceMonitorList

	err = kubeClient.List(ctx, &monitors, &client.ListOptions{Limit: 1})
	return err == nil
}

func MonitorForService(service *corev1.Service) *monitoringv1.ServiceMonitor {
	endpoints := make([]monitoringv1.Endpoint, len(service.Spec.Ports))

	for i, port := range service.Spec.Ports {
		endpoints[i] = monitoringv1.Endpoint{
			Port:     port.Name,
			Interval: "30s",
			Scheme:   string(corev1.URISchemeHTTP),
		}
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name,
			Namespace: service.Namespace,
			Labels:    service.Labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: service.Labels,
			},
			Endpoints: endpoints,
		},
	}
}
