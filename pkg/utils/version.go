package utils

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type APIDiscoveryClient struct {
	cl *discovery.DiscoveryClient
}

func NewAPIDiscoveryClient(conf *rest.Config) *APIDiscoveryClient {
	client, _ := discovery.NewDiscoveryClientForConfig(conf)

	return &APIDiscoveryClient{
		cl: client,
	}
}

func (ac *APIDiscoveryClient) HasGroupVersionResource(gvr schema.GroupVersionResource) bool {
	if ac.cl == nil {
		return false
	}

	resources, err := ac.cl.ServerResourcesForGroupVersion(gvr.GroupVersion().String())
	if err != nil {
		return false
	}

	for _, r := range resources.APIResources {
		if r.Name == gvr.Resource {
			return true
		}
	}

	return false
}
