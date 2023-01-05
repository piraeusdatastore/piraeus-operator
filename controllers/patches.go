package controllers

import (
	"bytes"
	"io"
	"io/fs"
	"strings"

	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/cluster"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/satellite"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils"
)

func ClusterLinstorPassphrasePatch(secretName string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/passphrase.yaml",
		map[string]any{
			"LINSTOR_PASSPHRASE_SECRET_NAME": secretName,
		},
	)
}

func ClusterLinstorInternalTLSPatch(secretName string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/internal-tls.yaml",
		map[string]any{
			"LINSTOR_INTERNAL_TLS_SECRET_NAME": secretName,
		},
	)
}

func ClusterLinstorInternalTLSCertManagerPatch(secretName string, issuer *cmmetav1.ObjectReference) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/internal-tls-cert-manager.yaml",
		map[string]any{
			"LINSTOR_INTERNAL_TLS_SECRET_NAME": secretName,
			"LINSTOR_INTERNAL_TLS_CERT_ISSUER": issuer,
		},
	)
}

func ClusterCSINodeSelectorPatch(selector map[string]string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/csi-node-selector.yaml",
		map[string]any{
			"NODE_SELECTOR": selector,
		})
}

func ClusterApiTLSPatch(apiSecretName, clientSecretName string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls.yaml",
		map[string]any{
			"LINSTOR_API_TLS_SECRET_NAME":        apiSecretName,
			"LINSTOR_API_TLS_CLIENT_SECRET_NAME": clientSecretName,
		})
}

func ClusterApiTLSCertManagerPatch(secretName string, issuer *cmmetav1.ObjectReference, dnsNames []string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls-cert-manager.yaml",
		map[string]any{
			"LINSTOR_API_TLS_SECRET_NAME": secretName,
			"LINSTOR_API_TLS_CERT_ISSUER": issuer,
			"LINSTOR_API_TLS_DNS_NAMES":   dnsNames,
		})
}

func ClusterCSIApiTLSPatch(controllerSecret, nodeSecret string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls-csi.yaml",
		map[string]any{
			"LINSTOR_CSI_CONTROLLER_API_TLS_SECRET_NAME": controllerSecret,
			"LINSTOR_CSI_NODE_API_TLS_SECRET_NAME":       nodeSecret,
		})
}

func ClusterApiTLSClientCertManagerPatch(certName, secretName string, issuer *cmmetav1.ObjectReference) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls-client-cert-manager.yaml",
		map[string]any{
			"CERTIFICATE_NAME":                   certName,
			"LINSTOR_API_TLS_CLIENT_SECRET_NAME": secretName,
			"LINSTOR_API_TLS_CLIENT_CERT_ISSUER": issuer,
		})
}

func PullSecretPatch(secretName string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/pull-secret.yaml",
		map[string]any{
			"IMAGE_PULL_SECRET": secretName,
		})
}

func SatelliteLinstorInternalTLSPatch(secretName string) ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/internal-tls.yaml",
		map[string]any{
			"LINSTOR_INTERNAL_TLS_SECRET_NAME": secretName,
		},
	)
}

func SatelliteLinstorInternalTLSCertManagerPatch(secretName string, issuer *cmmetav1.ObjectReference) ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/internal-tls-cert-manager.yaml",
		map[string]any{
			"LINSTOR_INTERNAL_TLS_SECRET_NAME": secretName,
			"LINSTOR_INTERNAL_TLS_CERT_ISSUER": issuer,
		},
	)
}

func SatelliteCommonNodePatch(nodeName string) ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/common-node.yaml",
		map[string]any{
			"NODE_NAME": nodeName,
		},
	)
}

func SatelliteHostPathVolumePatch(volumeName, hostPath string) ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/host-path-volume.yaml",
		map[string]any{
			"VOLUME_NAME": volumeName,
			"HOST_PATH":   hostPath,
		},
	)
}

func SatelliteHostPathVolumeEnvPatch(hostPaths []string) ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/host-path-volume-env.yaml",
		map[string]any{
			"HOST_PATHS": strings.Join(hostPaths, ":"),
		},
	)
}

func render(f fs.FS, fileName string, params map[string]any) ([]kusttypes.Patch, error) {
	raw, err := f.Open(fileName)
	if err != nil {
		return nil, err
	}

	defer raw.Close()

	buf := bytes.Buffer{}
	_, err = io.Copy(&buf, raw)
	if err != nil {
		return nil, err
	}

	var patches []kusttypes.Patch
	err = yaml.Unmarshal(buf.Bytes(), &patches)
	if err != nil {
		return nil, err
	}

	return utils.RenderPatches(params, patches...)
}
