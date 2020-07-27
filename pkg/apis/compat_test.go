package apis_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	v1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// Ensure that v1alpha1.LinstorController can be interpreted as v1.LinstorController
func Test_LinstorController(t *testing.T) {
	sslConfig := shared.LinstorSSLConfig("sslconfig")
	alphaSrc := &v1alpha1.LinstorController{
		Spec: v1alpha1.LinstorControllerSpec{
			PriorityClassName:            "priority",
			DBConnectionURL:              "test",
			DBCertSecret:                 "dbcert",
			DBUseClientCert:              true,
			LuksSecret:                   "luks",
			SslConfig:                    &sslConfig,
			DrbdRepoCred:                 "repo",
			ControllerImage:              "image",
			ImagePullPolicy:              "pullPolicy",
			LinstorHttpsControllerSecret: "https",
			Resources: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1000Gi")},
			},
			Affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{TopologyKey: "stuff"},
					},
				},
			},
			Tolerations:         []corev1.Toleration{{Key: "a", Value: "b"}},
			LinstorClientConfig: shared.LinstorClientConfig{LinstorHttpsClientSecret: "client-https"},
		},
		Status: v1alpha1.LinstorControllerStatus{
			Errors:            []string{"a", "b", "c"},
			ControllerStatus:  &shared.NodeStatus{NodeName: "control"},
			SatelliteStatuses: []*shared.SatelliteStatus{{ConnectionStatus: "down"}},
		},
	}

	data, err := json.Marshal(alphaSrc)
	assert.NoError(t, err, "couldn't convert v1alpha1.LinstorController to json")

	v1Dest := &v1.LinstorController{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(v1Dest)
	assert.NoError(t, err, "couldn't convert json to v1.LinstorController")
}

// Ensure that v1alpha1.LinstorCSIDriver can be interpreted as v1.LinstorCSIDriver
func Test_LinstorCSIDriver(t *testing.T) {
	replicas := int32(3)
	alphaSrc := &v1alpha1.LinstorCSIDriver{
		Spec: v1alpha1.LinstorCSIDriverSpec{
			PriorityClassName:  "prio",
			LinstorPluginImage: "plugin",
			Resources: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1000Gi")},
			},
			ControllerAffinity: &corev1.Affinity{
				PodAntiAffinity: &corev1.PodAntiAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{TopologyKey: "control-stuff"},
					},
				},
			},
			NodeAffinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{TopologyKey: "node-stuff"},
					},
				},
			},
			ControllerTolerations:           []corev1.Toleration{{Key: "d", Value: "e"}},
			NodeTolerations:                 []corev1.Toleration{{Key: "a", Value: "b"}},
			ImagePullPolicy:                 "pull",
			CSISnapshotterImage:             "snapshot",
			CSIProvisionerImage:             "provision",
			CSINodeDriverRegistrarImage:     "registrar",
			CSIAttacherImage:                "attach",
			ControllerEndpoint:              "endpoint",
			ControllerReplicas:              &replicas,
			CSIControllerServiceAccountName: "sa",
			CSINodeServiceAccountName:       "node-sa",
			CSIResizerImage:                 "resizer",
			ImagePullSecret:                 "secret",
			LinstorClientConfig:             shared.LinstorClientConfig{LinstorHttpsClientSecret: "client-https"},
		},
		Status: v1alpha1.LinstorCSIDriverStatus{
			Errors:          []string{"a", "b", "c"},
			ControllerReady: true,
			NodeReady:       true,
		},
	}

	data, err := json.Marshal(alphaSrc)
	assert.NoError(t, err, "couldn't convert v1alpha1.LinstorCSIDriver to json")

	v1Dest := &v1.LinstorCSIDriver{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(v1Dest)
	assert.NoError(t, err, "couldn't convert json to v1.LinstorCSIDriver")
}

// Ensure that v1alpha1.LinstorSatelliteSet can be interpreted as v1.LinstorSatelliteSet
func Test_LinstorSatelliteSet(t *testing.T) {
	sslConfig := shared.LinstorSSLConfig("sslconfig")

	alphaSrc := &v1alpha1.LinstorSatelliteSet{
		Spec: v1alpha1.LinstorSatelliteSetSpec{
			ControllerEndpoint: "ep",
			PriorityClassName:  "prio",
			DrbdRepoCred:       "repo",
			KernelModuleInjectionResources: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("120m")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("12000Mi")},
			},
			StoragePools: &shared.StoragePools{
				LVMPools:     []*shared.StoragePoolLVM{{VolumeGroup: "group"}},
				LVMThinPools: []*shared.StoragePoolLVMThin{{ThinVolume: "thin"}},
			},
			AutomaticStorageType:       "LVMTHIN",
			SslConfig:                  &sslConfig,
			KernelModuleInjectionImage: "injector",
			KernelModuleInjectionMode:  "mode",
			SatelliteImage:             "image",
			ImagePullPolicy:            "pullPolicy",
			Resources: corev1.ResourceRequirements{
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1000m")},
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("1000Gi")},
			},
			Affinity: &corev1.Affinity{
				PodAffinity: &corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{TopologyKey: "stuff"},
					},
				},
			},
			Tolerations:         []corev1.Toleration{{Key: "a", Value: "b"}},
			LinstorClientConfig: shared.LinstorClientConfig{LinstorHttpsClientSecret: "client-https"},
		},
		Status: v1alpha1.LinstorSatelliteSetStatus{},
	}

	data, err := json.Marshal(alphaSrc)
	assert.NoError(t, err, "couldn't convert v1alpha1.LinstorSatelliteSet to json")

	v1Dest := &v1.LinstorSatelliteSet{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	err = decoder.Decode(v1Dest)
	assert.NoError(t, err, "couldn't convert json to v1.LinstorSatelliteSet")
}
