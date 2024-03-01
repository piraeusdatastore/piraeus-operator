package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"strings"

	cmmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/resid"
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

func ClusterLinstorControllerNodeSelector(selector map[string]string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/linstor-controller-selector.yaml",
		map[string]any{
			"NODE_SELECTOR": selector,
		})
}

func ClusterLinstorControllerNodeAffinityPatch(affinity *corev1.NodeSelector) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/linstor-controller-node-affinity.yaml",
		map[string]any{
			"NODE_AFFINITY": affinity,
		})
}

func ClusterCSIControllerNodeSelector(selector map[string]string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/csi-controller-selector.yaml",
		map[string]any{
			"NODE_SELECTOR": selector,
		})
}

func ClusterCSIControllerNodeAffinityPatch(affinity *corev1.NodeSelector) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/csi-controller-node-affinity.yaml",
		map[string]any{
			"NODE_AFFINITY": affinity,
		})
}

func ClusterCSINodeSelectorPatch(selector map[string]string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/csi-node-selector.yaml",
		map[string]any{
			"NODE_SELECTOR": selector,
		})
}

func ClusterCSINodeNodeAffinityPatch(affinity *corev1.NodeSelector) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/csi-node-node-affinity.yaml",
		map[string]any{
			"NODE_AFFINITY": affinity,
		})
}

func ClusterHAControllerNodeSelectorPatch(selector map[string]string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/ha-controller-node-selector.yaml",
		map[string]any{
			"NODE_SELECTOR": selector,
		})
}

func ClusterHAControllerNodeAffinityPatch(affinity *corev1.NodeSelector) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/ha-controller-node-affinity.yaml",
		map[string]any{
			"NODE_AFFINITY": affinity,
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

func ClusterCSIDriverSeLinuxPatch(apiVersion *utils.APIVersion) ([]kusttypes.Patch, error) {
	if apiVersion.Compare(&utils.APIVersion{Major: 1, Minor: 25}) >= 0 {
		return nil, nil
	}

	return render(
		cluster.Resources,
		"patches/csi-driver-no-selinux.yaml",
		nil,
	)
}

func ClusterApiEndpointPatch(url string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-endpoint.yaml",
		map[string]any{
			"LINSTOR_CONTROLLER_URL": url,
		})
}

func ClusterCSIControllerApiTLSPatch(controllerSecret string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls-csi-controller.yaml",
		map[string]any{
			"LINSTOR_CSI_CONTROLLER_API_TLS_SECRET_NAME": controllerSecret,
		})
}

func ClusterCSINodeApiTLSPatch(nodeSecret string) ([]kusttypes.Patch, error) {
	return render(
		cluster.Resources,
		"patches/api-tls-csi-node.yaml",
		map[string]any{
			"LINSTOR_CSI_NODE_API_TLS_SECRET_NAME": nodeSecret,
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

func SatelliteLinstorHandshakeDaemonPatch() ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/tlshd.yaml",
		nil,
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

func SatellitePrecompiledModulePatch() ([]kusttypes.Patch, error) {
	return render(
		satellite.Resources,
		"patches/precompiled-module.yaml",
		nil,
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

func ComponentPodTemplate(kind, name string, template json.RawMessage) ([]kusttypes.Patch, error) {
	patches, err := render(
		cluster.Resources,
		"patches/pod-template.yaml",
		map[string]any{
			"KIND":     kind,
			"NAME":     name,
			"TEMPLATE": template,
		},
	)
	if err != nil {
		return nil, err
	}

	for i := range patches {
		patches[i].Target = &kusttypes.Selector{
			ResId: resid.NewResIdKindOnly(kind, name),
		}
	}

	return patches, nil
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
