package utils

import (
	"cmp"
	"fmt"
	"strconv"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func NewAPIVersionFromConfigWithFallback(conf *rest.Config, fallback *APIVersion) *APIVersion {
	client, err := discovery.NewDiscoveryClientForConfig(conf)
	if err != nil {
		return fallback
	}

	version, err := client.ServerVersion()
	if err != nil {
		return fallback
	}

	major, err := strconv.Atoi(version.Major)
	if err != nil {
		return fallback
	}

	minor, err := strconv.Atoi(version.Minor)
	if err != nil {
		return fallback
	}

	return &APIVersion{
		Major: major,
		Minor: minor,
	}
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
