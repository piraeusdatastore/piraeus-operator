package utils

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

type APIDiscoveryClient struct {
	cl              *discovery.DiscoveryClient
	fallbackVersion *APIVersion
}

func NewAPIDiscoveryClient(conf *rest.Config, fallback *APIVersion) *APIDiscoveryClient {
	client, _ := discovery.NewDiscoveryClientForConfig(conf)

	return &APIDiscoveryClient{
		cl:              client,
		fallbackVersion: fallback,
	}
}

func (ac *APIDiscoveryClient) ServerVersion() *APIVersion {
	if ac.cl == nil {
		return ac.fallbackVersion
	}

	version, err := ac.cl.ServerVersion()
	if err != nil {
		return ac.fallbackVersion
	}

	major, err := parseKubernetesVersionPart(version.Major)
	if err != nil {
		return ac.fallbackVersion
	}

	minor, err := parseKubernetesVersionPart(version.Minor)
	if err != nil {
		return ac.fallbackVersion
	}

	return &APIVersion{
		Major: major,
		Minor: minor,
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

func parseKubernetesVersionPart(version string) (int, error) {
	return strconv.Atoi(strings.TrimSuffix(version, "+"))
}

type APIVersion struct {
	Major int
	Minor int
}

func (a *APIVersion) String() string {
	return fmt.Sprintf("v%d.%d", a.Major, a.Minor)
}

func (a *APIVersion) Compare(b *APIVersion) int {
	majorDiff := cmp.Compare(a.Major, b.Major)
	if majorDiff != 0 {
		return majorDiff
	}

	return cmp.Compare(a.Minor, b.Minor)
}
