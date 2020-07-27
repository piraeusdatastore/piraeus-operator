package linstorsatelliteset_test

import (
	"context"
	"testing"
	"time"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis"
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	"github.com/piraeusdatastore/piraeus-operator/pkg/controller/linstorsatelliteset"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	TestKey = types.NamespacedName{Name: "example", Namespace: "default"}

	TestMeta = metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
	}

	TestSatelliteSpec = v1alpha1.LinstorSatelliteSetSpec{
		DrbdRepoCred: "super-secret",
	}

	TestSatelliteStatus = v1alpha1.LinstorSatelliteSetStatus{
		SatelliteStatuses: []*shared.SatelliteStatus{{NodeStatus: shared.NodeStatus{NodeName: "example"}}},
		Errors:            []string{},
	}

	TestDaemonSetSpec = appsv1.DaemonSetSpec{
		MinReadySeconds: 32,
	}
)

func TestReconcileLegacyLinstorNodeSet_Reconcile(t *testing.T) {
	type ExpectedResources struct {
		daemonSet    *appsv1.DaemonSet
		configMap    *corev1.ConfigMap
		nodeSet      *v1alpha1.LinstorNodeSet
		satelliteSet *v1alpha1.LinstorSatelliteSet
	}

	testcases := []struct {
		name              string
		initialResources  []runtime.Object
		expectedResources ExpectedResources
	}{
		{
			name: "create-satellite-set-no-dependants",
			initialResources: []runtime.Object{
				&v1alpha1.LinstorNodeSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorNodeSetSpec{LinstorSatelliteSetSpec: TestSatelliteSpec},
					Status: v1alpha1.LinstorNodeSetStatus{
						LinstorSatelliteSetStatus: TestSatelliteStatus,
					},
				},
			},
			expectedResources: ExpectedResources{
				nodeSet: &v1alpha1.LinstorNodeSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorNodeSetSpec{LinstorSatelliteSetSpec: TestSatelliteSpec},
					Status: v1alpha1.LinstorNodeSetStatus{
						ResourceMigrated:          true,
						DependantsMigrated:        true,
						LinstorSatelliteSetStatus: TestSatelliteStatus,
					},
				},
				satelliteSet: &v1alpha1.LinstorSatelliteSet{
					ObjectMeta: TestMeta,
					Spec:       TestSatelliteSpec,
					Status:     TestSatelliteStatus,
				},
			},
		},
		{
			name: "create-satellite-set-with-dependants",
			initialResources: []runtime.Object{
				&v1alpha1.LinstorNodeSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorNodeSetSpec{LinstorSatelliteSetSpec: TestSatelliteSpec},
					Status: v1alpha1.LinstorNodeSetStatus{
						LinstorSatelliteSetStatus: TestSatelliteStatus,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "piraeus.linbit.com/v1alpha1",
								Kind:       "LinstorNodeSet",
								Name:       "example",
							},
						},
					},
					Spec: TestDaemonSetSpec,
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "piraeus.linbit.com/v1alpha1",
								Kind:       "LinstorNodeSet",
								Name:       "example",
							},
						},
					},
					Data: map[string]string{"foo": "bar "},
				},
			},
			expectedResources: ExpectedResources{
				nodeSet: &v1alpha1.LinstorNodeSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorNodeSetSpec{LinstorSatelliteSetSpec: TestSatelliteSpec},
					Status: v1alpha1.LinstorNodeSetStatus{
						ResourceMigrated:          true,
						DependantsMigrated:        true,
						LinstorSatelliteSetStatus: TestSatelliteStatus,
					},
				},
				satelliteSet: &v1alpha1.LinstorSatelliteSet{
					ObjectMeta: TestMeta,
					Spec:       TestSatelliteSpec,
					Status:     TestSatelliteStatus,
				},
				daemonSet: &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "piraeus.linbit.com/v1alpha1",
								Kind:       "LinstorSatelliteSet",
								Name:       "example",
							},
						},
					},
					Spec: TestDaemonSetSpec,
				},
				configMap: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "piraeus.linbit.com/v1alpha1",
								Kind:       "LinstorSatelliteSet",
								Name:       "example",
							},
						},
					},
					Data: map[string]string{"foo": "bar "},
				},
			},
		},
	}

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("Could not prepare test: %v", err)
	}

	t.Parallel()
	for _, testcase := range testcases {
		testcase := testcase

		t.Run(testcase.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)
			// Create controller fake Client.
			controllerClient := fake.NewFakeClientWithScheme(scheme.Scheme, testcase.initialResources...)

			reconciler := linstorsatelliteset.ReconcileLegacyLinstorNodeSet{Client: controllerClient, Scheme: scheme.Scheme}

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: TestKey})
			assert.NoError(t, err)

			actualNodeSet := &v1alpha1.LinstorNodeSet{}
			err = controllerClient.Get(ctx, TestKey, actualNodeSet)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expectedResources.nodeSet.Spec, actualNodeSet.Spec)
			assert.Equal(t, testcase.expectedResources.nodeSet.Status, actualNodeSet.Status)

			actualSatelliteSet := &v1alpha1.LinstorSatelliteSet{}
			err = controllerClient.Get(ctx, TestKey, actualSatelliteSet)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expectedResources.satelliteSet.Spec, actualSatelliteSet.Spec)
			assert.Equal(t, testcase.expectedResources.satelliteSet.Status, actualSatelliteSet.Status)

			actualDaemonSet := &appsv1.DaemonSet{}
			err = controllerClient.Get(ctx, TestKey, actualDaemonSet)
			if testcase.expectedResources.daemonSet != nil {
				assert.NoError(t, err)
				assert.Equal(t, testcase.expectedResources.daemonSet.Spec, actualDaemonSet.Spec)
				assert.Equal(t, testcase.expectedResources.daemonSet.OwnerReferences[0].APIVersion, actualDaemonSet.OwnerReferences[0].APIVersion)
				assert.Equal(t, testcase.expectedResources.daemonSet.OwnerReferences[0].Kind, actualDaemonSet.OwnerReferences[0].Kind)
				assert.Equal(t, testcase.expectedResources.daemonSet.OwnerReferences[0].Name, actualDaemonSet.OwnerReferences[0].Name)
			}

			actualConfigMap := &corev1.ConfigMap{}
			err = controllerClient.Get(ctx, TestKey, actualConfigMap)
			if testcase.expectedResources.configMap != nil {
				assert.NoError(t, err)
				assert.Equal(t, testcase.expectedResources.configMap.Data, actualConfigMap.Data)
				assert.Equal(t, testcase.expectedResources.configMap.OwnerReferences[0].APIVersion, actualConfigMap.OwnerReferences[0].APIVersion)
				assert.Equal(t, testcase.expectedResources.configMap.OwnerReferences[0].Kind, actualConfigMap.OwnerReferences[0].Kind)
				assert.Equal(t, testcase.expectedResources.configMap.OwnerReferences[0].Name, actualConfigMap.OwnerReferences[0].Name)
			}
		})
	}
}
