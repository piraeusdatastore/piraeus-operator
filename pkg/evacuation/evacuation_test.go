package evacuation_test

import (
	"fmt"
	"net/url"
	"testing"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	affinity "github.com/piraeusdatastore/linstor-affinity-controller/pkg/version"
	linstorcsidriver "github.com/piraeusdatastore/linstor-csi/pkg/driver"
	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	linstorcsitopology "github.com/piraeusdatastore/linstor-csi/pkg/topology"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/clusterapi"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/conditions"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/evacuation"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/fakelinstor"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

var (
	TestNodeA = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-a",
			Annotations: map[string]string{
				clusterapiv1beta1.ClusterNamespaceAnnotation: "cluster-api-ns",
				clusterapiv1beta1.ClusterNameAnnotation:      "test-cluster",
				clusterapiv1beta1.MachineAnnotation:          "machine-a",
			},
		},
	}
	TestNodeB = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-b",
			Annotations: map[string]string{
				clusterapiv1beta1.ClusterNamespaceAnnotation: "cluster-api-ns",
				clusterapiv1beta1.ClusterNameAnnotation:      "test-cluster",
				clusterapiv1beta1.MachineAnnotation:          "machine-b",
			},
		},
	}
	MachineA = &clusterapiv1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-a",
			Namespace: "cluster-api-ns",
			Annotations: map[string]string{
				vars.MachinePreDrainHookAnnotation:     "",
				vars.MachinePreTerminateHookAnnotation: "",
			},
		},
	}
	MachineB = &clusterapiv1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "machine-b",
			Namespace: "cluster-api-ns",
			Annotations: map[string]string{
				vars.MachinePreDrainHookAnnotation:     "",
				vars.MachinePreTerminateHookAnnotation: "",
			},
		},
	}
)

func TestEvacuateEmptySatellite(t *testing.T) {
	t.Parallel()
	cl, lc := setupTestCluster(t, TestNodeA, TestNodeB, MachineA, MachineB)

	// Satellite has no resources, no PVs, no nothing, so evacuation should just complete
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, "", msg)

	assert.Contains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteWithLocalPVs(t *testing.T) {
	t.Parallel()
	localPV := createPV("local-pv-attached", "false", nil, TestNodeA.Name)
	unattachedLocalPV := createPV("local-pv-unattached", "false", nil, TestNodeA.Name)
	remotePV := createPV("remote-pv-attached", "true", nil)
	unattachedRemotePV := createPV("remote-pv-unattached", "true", nil)

	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB,
		localPV, createAttachment(TestNodeA, localPV),
		unattachedLocalPV,
		remotePV, createAttachment(TestNodeA, remotePV),
		unattachedRemotePV,
	)

	// Satellite still has a local-only PV
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.False(t, cont)
	assert.Contains(t, msg, "Waiting for PVs to be schedulable on other nodes")
	assert.Contains(t, msg, "local-pv-attached")
	assert.NotContains(t, msg, "local-pv-unattached")
	assert.NotContains(t, msg, "remote-pv-attached")
	assert.NotContains(t, msg, "remote-pv-unattached")

	// Attached PVs should be marked for rescheduling, local only PV additionally with attach override
	assert.Contains(t, getPV(t, cl, "local-pv-attached").Annotations, affinity.OverrideAnnotationPrefix+"/node-a")
	assert.Contains(t, getPV(t, cl, "local-pv-attached").Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/node-a")
	assert.NotContains(t, getPV(t, cl, "local-pv-unattached").Annotations, affinity.OverrideAnnotationPrefix+"/node-a")
	assert.NotContains(t, getPV(t, cl, "local-pv-unattached").Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/node-a")
	assert.NotContains(t, getPV(t, cl, "remote-pv-attached").Annotations, affinity.OverrideAnnotationPrefix+"/node-a")
	assert.Contains(t, getPV(t, cl, "remote-pv-attached").Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/node-a")
	assert.NotContains(t, getPV(t, cl, "remote-pv-unattached").Annotations, affinity.OverrideAnnotationPrefix+"/node-a")
	assert.NotContains(t, getPV(t, cl, "remote-pv-unattached").Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/node-a")

	// No evacuation, no drain, no termination
	assert.NotContains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteWithEvacuationActionPVs(t *testing.T) {
	t.Parallel()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-ns"}}
	localPV := createPV("local-pv-attached", "false", nil, TestNodeA.Name)

	localActionDeletePV := createPV("local-pv-action-delete-on-pv", "false", map[string]string{
		vars.EvacuationActionAnnotation: "Delete",
	}, TestNodeA.Name)
	localActionDeletePV.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace: namespace.Name,
		Name:      "local-pv-action-delete-on-pv",
	}
	localActionDeletePVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-pv-action-delete-on-pv",
			Namespace: namespace.Name,
		},
	}
	localActionDeleteOnRDPV := createPV("local-pv-action-delete-on-rd", "true", nil, TestNodeA.Name)
	localActionDeleteOnRDPV.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace: namespace.Name,
		Name:      "local-pv-action-delete-on-rd",
	}
	localActionDeleteOnRDPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-pv-action-delete-on-rd",
			Namespace: namespace.Name,
		},
	}
	localActionDeleteOnRGPV := createPV("local-pv-action-delete-on-rg", "true", nil, TestNodeA.Name)
	localActionDeleteOnRGPV.Spec.ClaimRef = &corev1.ObjectReference{
		Namespace: namespace.Name,
		Name:      "local-pv-action-delete-on-rg",
	}
	localActionDeleteOnRGPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-pv-action-delete-on-rg",
			Namespace: namespace.Name,
		},
	}

	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB, namespace,
		localPV, createAttachment(TestNodeA, localPV),
		localActionDeletePVC, localActionDeletePV, createAttachment(TestNodeA, localActionDeletePV),
		localActionDeleteOnRDPVC, localActionDeleteOnRDPV, createAttachment(TestNodeA, localActionDeleteOnRDPV),
		localActionDeleteOnRGPVC, localActionDeleteOnRGPV, createAttachment(TestNodeA, localActionDeleteOnRGPV),
	)

	err := lc.ResourceGroups.Create(t.Context(), lapi.ResourceGroup{
		Name: "test-rg-default",
	})
	assert.NoError(t, err)

	err = lc.ResourceGroups.Create(t.Context(), lapi.ResourceGroup{
		Name: "test-rg-delete",
		Props: map[string]string{
			linstor.NamespcAuxiliary + "/" + vars.EvacuationActionAnnotation: "Delete",
		},
	})
	assert.NoError(t, err)

	err = lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{
		Name:              "local-pv-action-delete-on-pv",
		ResourceGroupName: "test-rg-default",
	}})
	assert.NoError(t, err)

	err = lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{
		Name:              "local-pv-action-delete-on-rd",
		ResourceGroupName: "test-rg-default",
		Props: map[string]string{
			linstor.NamespcAuxiliary + "/" + vars.EvacuationActionAnnotation: "Delete",
		},
	}})
	assert.NoError(t, err)

	err = lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{
		Name:              "local-pv-action-delete-on-rg",
		ResourceGroupName: "test-rg-delete",
	}})
	assert.NoError(t, err)

	// Satellite still has a local-only PV
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.False(t, cont)
	assert.Contains(t, msg, "Waiting for PVs to be schedulable on other nodes")
	assert.Contains(t, msg, "local-pv-attached")
	assert.NotContains(t, msg, "local-pv-action-delete-on-pv")
	assert.NotContains(t, msg, "local-pv-action-delete-on-rd")
	assert.NotContains(t, msg, "local-pv-action-delete-on-rg")

	// Attached PVs should be marked for rescheduling, local only PV additionally with attach override
	assert.Contains(t, getPV(t, cl, "local-pv-attached").Annotations, affinity.OverrideAnnotationPrefix+"/node-a")
	assert.Contains(t, getPV(t, cl, "local-pv-attached").Annotations, vars.PersistentVolumeWaitForReattachAnnotationPrefix+"/node-a")

	for _, pvName := range []string{"local-pv-action-delete-on-pv", "local-pv-action-delete-on-rd", "local-pv-action-delete-on-rg"} {
		var pvc corev1.PersistentVolumeClaim
		assert.Error(t, cl.Get(t.Context(), client.ObjectKey{Namespace: namespace.Name, Name: pvName}, &pvc))
		var pv corev1.PersistentVolume
		assert.Error(t, cl.Get(t.Context(), client.ObjectKey{Name: pvName}, &pv))
	}

	// No evacuation, no drain, no termination
	assert.NotContains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteWaitForOtherSatellites(t *testing.T) {
	t.Parallel()
	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB,
		createSatellite(TestNodeA, true),
		createSatellite(TestNodeB, false),
	)

	// Satellite still has a local-only PV
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.False(t, cont)
	assert.Contains(t, msg, "Waiting for LinstorSatellites to become ready")
	assert.Contains(t, msg, "node-b")
	assert.NotContains(t, msg, "node-a")

	// No evacuation, no drain, no termination
	assert.NotContains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteWaitForReattach(t *testing.T) {
	t.Parallel()
	localPV := createPV("local-pv-attached", "false", map[string]string{
		affinity.OverrideAnnotationPrefix + "/node-a":                    "true",
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedLocalPV := createPV("local-pv-unattached", "false", nil, TestNodeA.Name)
	remotePV := createPV("remote-pv-attached", "true", map[string]string{
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedRemotePV := createPV("remote-pv-unattached", "true", nil)

	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB,
		createSatellite(TestNodeA, true),
		createSatellite(TestNodeB, true),
		localPV, createAttachment(TestNodeA, localPV),
		unattachedLocalPV,
		remotePV, createAttachment(TestNodeA, remotePV),
		unattachedRemotePV,
	)

	// Node A still has a local-only PV attached
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.False(t, cont)
	assert.Contains(t, msg, "Waiting for PVs to reattach on other nodes")
	assert.Contains(t, msg, "local-pv-attached")
	assert.Contains(t, msg, "remote-pv-attached")
	assert.NotContains(t, msg, "local-pv-unattached")
	assert.NotContains(t, msg, "remote-pv-unattached")

	// No evacuation, allowed to drain, no termination
	assert.NotContains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteWaitForLinstorEvacuation(t *testing.T) {
	t.Parallel()
	localPV := createPV("local-pv-attached", "false", map[string]string{
		affinity.OverrideAnnotationPrefix + "/node-a":                    "true",
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedLocalPV := createPV("local-pv-unattached", "false", nil, TestNodeA.Name)
	remotePV := createPV("remote-pv-attached", "true", map[string]string{
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedRemotePV := createPV("remote-pv-unattached", "true", nil)

	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB,
		createSatellite(TestNodeA, true),
		createSatellite(TestNodeB, true),
		localPV, createAttachment(TestNodeB, localPV),
		unattachedLocalPV,
		remotePV, createAttachment(TestNodeB, remotePV),
		unattachedRemotePV,
	)

	// Create fake linstor resources, let the "unattached" ones be still on node A.
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "local-pv-attached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "local-pv-attached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "local-pv-unattached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "local-pv-unattached", NodeName: "node-a"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "local-pv-unattached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "remote-pv-attached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "remote-pv-attached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "remote-pv-unattached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "remote-pv-unattached", NodeName: "node-a"}}))

	// PVs already attached on other Nodes, now waiting for LINSTOR to clear the resource
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.False(t, cont)
	assert.Contains(t, msg, "Waiting on remaining resources and snapshots: resources")
	assert.NotContains(t, msg, "local-pv-attached")
	assert.NotContains(t, msg, "remote-pv-attached")
	assert.Contains(t, msg, "local-pv-unattached")
	assert.Contains(t, msg, "remote-pv-unattached")

	// evacuation, allowed to drain, no termination
	assert.Contains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.Contains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func TestEvacuateSatelliteComplete(t *testing.T) {
	t.Parallel()
	localPV := createPV("local-pv-attached", "false", map[string]string{
		affinity.OverrideAnnotationPrefix + "/node-a":                    "true",
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedLocalPV := createPV("local-pv-unattached", "false", nil, TestNodeA.Name)
	remotePV := createPV("remote-pv-attached", "true", map[string]string{
		vars.PersistentVolumeWaitForReattachAnnotationPrefix + "/node-a": "true",
	})
	unattachedRemotePV := createPV("remote-pv-unattached", "true", nil)

	cl, lc := setupTestCluster(t,
		TestNodeA, TestNodeB, MachineA, MachineB,
		createSatellite(TestNodeA, true),
		createSatellite(TestNodeB, true),
		localPV, createAttachment(TestNodeB, localPV),
		unattachedLocalPV,
		remotePV, createAttachment(TestNodeB, remotePV),
		unattachedRemotePV,
	)

	// Create fake linstor resources, everything is now on node B so A is fully evacuated.
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "local-pv-attached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "local-pv-attached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "local-pv-unattached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "local-pv-unattached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "remote-pv-attached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "remote-pv-attached", NodeName: "node-b"}}))
	assert.NoError(t, lc.ResourceDefinitions.Create(t.Context(), lapi.ResourceDefinitionCreate{ResourceDefinition: lapi.ResourceDefinition{Name: "remote-pv-unattached"}}))
	assert.NoError(t, lc.Resources.Create(t.Context(), lapi.ResourceCreate{Resource: lapi.Resource{Name: "remote-pv-unattached", NodeName: "node-b"}}))

	// PVs already attached on other Nodes, LINSTOR resources cleaned up
	msg, cont, err := runEvacuateSatellite(t, cl, lc)
	assert.NoError(t, err)
	assert.True(t, cont)
	assert.Equal(t, "", msg)

	// Check the PV annotations are cleared
	assert.Empty(t, getPV(t, cl, "local-pv-attached").Annotations)
	assert.Empty(t, getPV(t, cl, "local-pv-unattached").Annotations)
	assert.Empty(t, getPV(t, cl, "remote-pv-attached").Annotations)
	assert.Empty(t, getPV(t, cl, "remote-pv-unattached").Annotations)

	// evacuation, allowed to drain, no termination
	assert.Contains(t, getSatellite(t, lc, "node-a").Flags, linstor.FlagEvacuate)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreDrainHookAnnotation)
	assert.NotContains(t, getMachine(t, cl, "machine-a").Annotations, vars.MachinePreTerminateHookAnnotation)
}

func createPV(name, accessPolicy string, annotations map[string]string, nodes ...string) *corev1.PersistentVolume {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       linstorcsi.DriverName,
					VolumeHandle: name,
					VolumeAttributes: map[string]string{
						linstorcsidriver.VolumeContextMarker:    "true",
						linstorcsidriver.RemoteAccessPolicyOpts: accessPolicy,
					},
				},
			},
		},
	}

	for _, node := range nodes {
		if pv.Spec.NodeAffinity == nil {
			pv.Spec.NodeAffinity = &corev1.VolumeNodeAffinity{Required: &corev1.NodeSelector{}}
		}

		pv.Spec.NodeAffinity.Required.NodeSelectorTerms = append(pv.Spec.NodeAffinity.Required.NodeSelectorTerms, corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key:      linstorcsitopology.LinstorNodeKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   []string{node},
			}},
		})
	}

	return pv
}

func createAttachment(node *corev1.Node, pv *corev1.PersistentVolume) *storagev1.VolumeAttachment {
	return &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name + "-" + pv.Name,
		},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: node.Name,
			Attacher: linstorcsi.DriverName,
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pv.Name},
		},
	}
}

func createSatellite(node *corev1.Node, available bool) *piraeusv1.LinstorSatellite {
	c := conditions.New()
	for k := range c {
		if available {
			c.AddSuccess(k, "Ok")
		} else {
			c.AddError(k, fmt.Errorf("error"))
		}
	}
	return &piraeusv1.LinstorSatellite{
		ObjectMeta: metav1.ObjectMeta{
			Name:       node.Name,
			Generation: 1,
		},
		Spec: piraeusv1.LinstorSatelliteSpec{},
		Status: piraeusv1.LinstorSatelliteStatus{
			Conditions: c.ToConditions(1),
		},
	}
}

func setupTestCluster(t *testing.T, objs ...client.Object) (client.Client, *lapi.Client) {
	scheme := runtime.NewScheme()
	assert.NoError(t, clientgoscheme.AddToScheme(scheme))
	assert.NoError(t, piraeusv1.AddToScheme(scheme))
	assert.NoError(t, clusterapiv1beta1.AddToScheme(scheme))

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	linstorController := fakelinstor.New()
	t.Cleanup(func() {
		linstorController.Server.Close()
	})

	baseUrl, err := url.Parse(linstorController.Server.URL)
	assert.NoError(t, err)

	lc, err := lapi.NewClient(
		lapi.BaseURL(baseUrl),
		lapi.HTTPClient(linstorController.Server.Client()),
	)
	assert.NoError(t, err)

	err = lc.Nodes.Create(t.Context(), lapi.Node{
		Name: "node-a",
	})
	assert.NoError(t, err)

	err = lc.Nodes.Create(t.Context(), lapi.Node{
		Name: "node-b",
	})
	assert.NoError(t, err)

	return cl, lc
}

func runEvacuateSatellite(t *testing.T, cl client.Client, lc *lapi.Client) (string, bool, error) {
	machineCl := clusterapi.NewClient(cl)

	var node corev1.Node
	err := cl.Get(t.Context(), types.NamespacedName{Name: "node-a"}, &node)
	assert.NoError(t, err)

	satellite, err := lc.Nodes.Get(t.Context(), TestNodeA.Name)
	assert.NoError(t, err)

	machine, err := machineCl.GetMachineForNode(t.Context(), &node)
	assert.NoError(t, err)

	return evacuation.EvacuateSatellite(t.Context(), cl, lc, &satellite, machineCl, machine)
}

func getSatellite(t *testing.T, lc *lapi.Client, name string) *lapi.Node {
	node, err := lc.Nodes.Get(t.Context(), name)
	assert.NoError(t, err)
	return &node
}

func getMachine(t *testing.T, cl client.Client, name string) *clusterapiv1beta1.Machine {
	var machine clusterapiv1beta1.Machine
	err := cl.Get(t.Context(), types.NamespacedName{Namespace: "cluster-api-ns", Name: name}, &machine)
	assert.NoError(t, err)
	return &machine
}

func getPV(t *testing.T, cl client.Client, name string) *corev1.PersistentVolume {
	var pv corev1.PersistentVolume
	err := cl.Get(t.Context(), types.NamespacedName{Name: name}, &pv)
	assert.NoError(t, err)
	return &pv
}
