package controller_test

import (
	"testing"

	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	kusttypes "sigs.k8s.io/kustomize/api/types"

	"github.com/piraeusdatastore/piraeus-operator/v2/internal/controller"
)

func TestPatches(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name string
		call func() ([]kusttypes.Patch, error)
	}{
		{
			name: "ClusterLinstorPassphrasePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterLinstorPassphrasePatch("secret")
			},
		},
		{
			name: "ClusterLinstorInternalTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterLinstorInternalTLSPatch("secret")
			},
		},
		{
			name: "ClusterLinstorInternalTLSCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterLinstorInternalTLSCertManagerPatch("secret", &cmmetav1.ObjectReference{
					Name: "issuer",
				})
			},
		},
		{
			name: "ClusterCSINodeSelectorPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterCSINodeSelectorPatch(map[string]string{"foo": "bar"})
			},
		},
		{
			name: "PullSecretPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.PullSecretPatch("secret")
			},
		},
		{
			name: "SatelliteLinstorInternalTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.SatelliteLinstorInternalTLSPatch("secret")
			},
		},
		{
			name: "SatelliteCommonNodePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.SatelliteCommonNodePatch("node")
			},
		},
		{
			name: "SatelliteHostPathVolumePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.SatelliteHostPathVolumePatch("vol-name", "/host/path")
			},
		},
		{
			name: "SatellitePrecompiledModulePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.SatellitePrecompiledModulePatch()
			},
		},
		{
			name: "SatelliteHostPathVolumeEnvPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.SatelliteHostPathVolumeEnvPatch([]string{"/path1", "/path2"})
			},
		},
		{
			name: "ClusterApiTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterApiTLSPatch("apiSecret", "clientSecret")
			},
		},
		{
			name: "ClusterApiTLSCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterApiTLSCertManagerPatch("secret", &cmmetav1.ObjectReference{
					Name: "issuer",
				}, []string{"api.ns.svc"})
			},
		},
		{
			name: "ClusterApiEndpointPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterApiEndpointPatch("https://example.com:8888")
			},
		},
		{
			name: "ClusterCSIApiTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterCSIApiTLSPatch("controller", "node")
			},
		},
		{
			name: "ClusterApiTLSClientCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controller.ClusterApiTLSClientCertManagerPatch("cert", "secret", &cmmetav1.ObjectReference{
					Name: "issuer",
				})
			},
		},
	}

	for i := range testcases {
		testcase := &testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()
			_, err := testcase.call()
			assert.NoError(t, err)
		})
	}
}
