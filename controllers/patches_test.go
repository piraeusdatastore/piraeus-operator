package controllers_test

import (
	"testing"

	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/stretchr/testify/assert"
	kusttypes "sigs.k8s.io/kustomize/api/types"

	"github.com/piraeusdatastore/piraeus-operator/v2/controllers"
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
				return controllers.ClusterLinstorPassphrasePatch("secret")
			},
		},
		{
			name: "ClusterLinstorInternalTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterLinstorInternalTLSPatch("secret")
			},
		},
		{
			name: "ClusterLinstorInternalTLSCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterLinstorInternalTLSCertManagerPatch("secret", &cmmetav1.ObjectReference{
					Name: "issuer",
				})
			},
		},
		{
			name: "ClusterCSINodeSelectorPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterCSINodeSelectorPatch(map[string]string{"foo": "bar"})
			},
		},
		{
			name: "PullSecretPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.PullSecretPatch("secret")
			},
		},
		{
			name: "SatelliteLinstorInternalTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.SatelliteLinstorInternalTLSPatch("secret")
			},
		},
		{
			name: "SatelliteCommonNodePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.SatelliteCommonNodePatch("node")
			},
		},
		{
			name: "SatelliteHostPathVolumePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.SatelliteHostPathVolumePatch("vol-name", "/host/path")
			},
		},
		{
			name: "SatellitePrecompiledModulePatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.SatellitePrecompiledModulePatch()
			},
		},
		{
			name: "SatelliteHostPathVolumeEnvPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.SatelliteHostPathVolumeEnvPatch([]string{"/path1", "/path2"})
			},
		},
		{
			name: "ClusterApiTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterApiTLSPatch("apiSecret", "clientSecret")
			},
		},
		{
			name: "ClusterApiTLSCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterApiTLSCertManagerPatch("secret", &cmmetav1.ObjectReference{
					Name: "issuer",
				}, []string{"api.ns.svc"})
			},
		},
		{
			name: "ClusterCSIApiTLSPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterCSIApiTLSPatch("controller", "node")
			},
		},
		{
			name: "ClusterApiTLSClientCertManagerPatch",
			call: func() ([]kusttypes.Patch, error) {
				return controllers.ClusterApiTLSClientCertManagerPatch("cert", "secret", &cmmetav1.ObjectReference{
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
