package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources"
	clusterresources "github.com/piraeusdatastore/piraeus-operator/v2/pkg/resources/cluster"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/utils/tolerations"
)

func TestNodeDaemonSetsSchedulingConstraints(t *testing.T) {
	t.Parallel()

	kustomizer, err := resources.NewKustomizer(&clusterresources.Resources, krusty.MakeDefaultOptions())
	require.NoError(t, err)

	r := &LinstorClusterReconciler{
		Namespace:  "piraeus-datastore",
		Kustomizer: kustomizer,
	}

	lcluster := &piraeusiov1.LinstorCluster{
		Spec: piraeusiov1.LinstorClusterSpec{
			Tolerations: []corev1.Toleration{{
				Key:      "example.com/custom-taint",
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			}},
		},
	}

	testcases := []struct {
		name   string
		render func() (resmap.ResMap, error)
		dsName string
	}{
		{
			name: "CSI node",
			render: func() (resmap.ResMap, error) {
				return r.kustomizeCSINodeResources(lcluster, nil)
			},
			dsName: "linstor-csi-node",
		},
		{
			name: "HA controller",
			render: func() (resmap.ResMap, error) {
				return r.kustomizeHAControllerResources(lcluster, nil)
			},
			dsName: "ha-controller",
		},
		{
			name: "NFS server",
			render: func() (resmap.ResMap, error) {
				return r.kustomizeNFSServerResources(lcluster, nil)
			},
			dsName: "linstor-csi-nfs-server",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rendered, err := tc.render()
			require.NoError(t, err)

			ds := findDaemonSet(t, rendered, tc.dsName)
			require.Contains(t, ds.Spec.Template.Spec.Tolerations, tolerations.NoScheduleToleration[0])
			require.Contains(t, ds.Spec.Template.Spec.Tolerations, tolerations.HAControllerTolerations[0])
			require.Contains(t, ds.Spec.Template.Spec.Tolerations, tolerations.HAControllerTolerations[1])
			require.Contains(t, ds.Spec.Template.Spec.Tolerations, lcluster.Spec.Tolerations[0])

			require.NotNil(t, ds.Spec.Template.Spec.Affinity)
			require.NotNil(t, ds.Spec.Template.Spec.Affinity.PodAffinity)
			terms := ds.Spec.Template.Spec.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
			require.Len(t, terms, 1)
			require.Equal(t, "kubernetes.io/hostname", terms[0].TopologyKey)
			require.Equal(t, map[string]string{"app.kubernetes.io/component": "linstor-satellite"}, terms[0].LabelSelector.MatchLabels)
		})
	}
}

func findDaemonSet(t *testing.T, rendered resmap.ResMap, name string) appsv1.DaemonSet {
	t.Helper()

	for _, resource := range rendered.Resources() {
		if resource.GetKind() != "DaemonSet" || resource.GetName() != name {
			continue
		}

		raw, err := resource.MarshalJSON()
		require.NoError(t, err)

		var ds appsv1.DaemonSet
		require.NoError(t, json.Unmarshal(raw, &ds))

		return ds
	}

	t.Fatalf("daemonset %q not found", name)
	return appsv1.DaemonSet{}
}
