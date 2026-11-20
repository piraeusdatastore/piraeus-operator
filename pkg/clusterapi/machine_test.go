package clusterapi_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/clusterapi"
)

func TestGetMachineForNodeWithoutClusterAPI(t *testing.T) {
	t.Parallel()

	var lookup client.ObjectKey
	cl := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
			lookup = key
			return &meta.NoKindMatchError{GroupKind: clusterapiv1beta1.GroupVersion.WithKind("Machine").GroupKind()}
		},
	}).Build()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				clusterapiv1beta1.ClusterNamespaceAnnotation: "machines",
				clusterapiv1beta1.MachineAnnotation:          "machine-1",
			},
		},
	}

	machine, err := clusterapi.NewClient(cl).GetMachineForNode(t.Context(), node)
	require.NoError(t, err)
	assert.Nil(t, machine)
	assert.Equal(t, client.ObjectKey{Namespace: "machines", Name: "machine-1"}, lookup)
}
