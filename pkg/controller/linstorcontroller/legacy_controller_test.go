package linstorcontroller_test

import (
	"context"
	"testing"
	"time"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"

	"github.com/piraeusdatastore/piraeus-operator/pkg/controller/linstorcontroller"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis"
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
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

	TestMetaNewOwner = metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "piraeus.linbit.com/v1alpha1",
				Kind:       "LinstorController",
				Name:       "example",
			},
		},
	}

	TestMetaWithOldOwner = metav1.ObjectMeta{
		Name:      "example",
		Namespace: "default",
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "piraeus.linbit.com/v1alpha1",
				Kind:       "LinstorControllerSet",
				Name:       "example",
			},
		},
	}

	TestControllerSpec = v1alpha1.LinstorControllerSpec{
		DrbdRepoCred: "super-secret",
	}

	TestControllerStatus = v1alpha1.LinstorControllerStatus{
		SatelliteStatuses: []*shared.SatelliteStatus{{NodeStatus: shared.NodeStatus{NodeName: "example"}}},
		Errors:            []string{},
	}

	TestDeploymentSpec = appsv1.DeploymentSpec{
		MinReadySeconds: 32,
	}

	TestServiceSpec = corev1.ServiceSpec{
		ClusterIP: "10.1.1.1",
	}
)

func TestReconcileLegacyLinstorNodeSet_Reconcile(t *testing.T) {
	type ExpectedResources struct {
		deployment    *appsv1.Deployment
		configMap     *corev1.ConfigMap
		service       *corev1.Service
		controllerSet *v1alpha1.LinstorControllerSet
		controller    *v1alpha1.LinstorController
	}

	testcases := []struct {
		name              string
		initialResources  []runtime.Object
		expectedResources ExpectedResources
	}{
		{
			name: "create-satellite-set-no-dependants",
			initialResources: []runtime.Object{
				&v1alpha1.LinstorControllerSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorControllerSetSpec{LinstorControllerSpec: TestControllerSpec},
					Status: v1alpha1.LinstorControllerSetStatus{
						LinstorControllerStatus: TestControllerStatus,
					},
				},
			},
			expectedResources: ExpectedResources{
				controllerSet: &v1alpha1.LinstorControllerSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorControllerSetSpec{LinstorControllerSpec: TestControllerSpec},
					Status: v1alpha1.LinstorControllerSetStatus{
						ResourceMigrated:        true,
						DependantsMigrated:      true,
						LinstorControllerStatus: TestControllerStatus,
					},
				},
				controller: &v1alpha1.LinstorController{
					ObjectMeta: TestMeta,
					Spec:       TestControllerSpec,
					Status:     TestControllerStatus,
				},
			},
		},
		{
			name: "create-satellite-set-with-dependants",
			initialResources: []runtime.Object{
				&v1alpha1.LinstorControllerSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorControllerSetSpec{LinstorControllerSpec: TestControllerSpec},
					Status: v1alpha1.LinstorControllerSetStatus{
						LinstorControllerStatus: TestControllerStatus,
					},
				},
				&appsv1.Deployment{
					ObjectMeta: TestMetaWithOldOwner,
					Spec:       TestDeploymentSpec,
				},
				&corev1.ConfigMap{
					ObjectMeta: TestMetaWithOldOwner,
					Data:       map[string]string{"foo": "bar "},
				},
				&corev1.Service{
					ObjectMeta: TestMetaWithOldOwner,
					Spec:       TestServiceSpec,
				},
			},
			expectedResources: ExpectedResources{
				controllerSet: &v1alpha1.LinstorControllerSet{
					ObjectMeta: TestMeta,
					Spec:       v1alpha1.LinstorControllerSetSpec{LinstorControllerSpec: TestControllerSpec},
					Status: v1alpha1.LinstorControllerSetStatus{
						ResourceMigrated:        true,
						DependantsMigrated:      true,
						LinstorControllerStatus: TestControllerStatus,
					},
				},
				controller: &v1alpha1.LinstorController{
					ObjectMeta: TestMeta,
					Spec:       TestControllerSpec,
					Status:     TestControllerStatus,
				},
				deployment: &appsv1.Deployment{
					ObjectMeta: TestMetaNewOwner,
					Spec:       TestDeploymentSpec,
				},
				configMap: &corev1.ConfigMap{
					ObjectMeta: TestMetaNewOwner,
					Data:       map[string]string{"foo": "bar "},
				},
				service: &corev1.Service{
					ObjectMeta: TestMetaNewOwner,
					Spec:       TestServiceSpec,
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

			reconciler := linstorcontroller.ReconcileLegacyController{Client: controllerClient, Scheme: scheme.Scheme}

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: TestKey})
			assert.NoError(t, err)

			actualControllerSet := &v1alpha1.LinstorControllerSet{}
			err = controllerClient.Get(ctx, TestKey, actualControllerSet)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expectedResources.controllerSet.Spec, actualControllerSet.Spec)
			assert.Equal(t, testcase.expectedResources.controllerSet.Status, actualControllerSet.Status)

			actualController := &v1alpha1.LinstorController{}
			err = controllerClient.Get(ctx, TestKey, actualController)
			assert.NoError(t, err)
			assert.Equal(t, testcase.expectedResources.controller.Spec, actualController.Spec)
			assert.Equal(t, testcase.expectedResources.controller.Status, actualController.Status)

			actualDaemonSet := &appsv1.Deployment{}
			err = controllerClient.Get(ctx, TestKey, actualDaemonSet)
			if testcase.expectedResources.deployment != nil {
				assert.NoError(t, err)
				assert.Equal(t, testcase.expectedResources.deployment.Spec, actualDaemonSet.Spec)
				assert.Equal(t, testcase.expectedResources.deployment.OwnerReferences[0].APIVersion, actualDaemonSet.OwnerReferences[0].APIVersion)
				assert.Equal(t, testcase.expectedResources.deployment.OwnerReferences[0].Kind, actualDaemonSet.OwnerReferences[0].Kind)
				assert.Equal(t, testcase.expectedResources.deployment.OwnerReferences[0].Name, actualDaemonSet.OwnerReferences[0].Name)
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

			actualService := &corev1.Service{}
			err = controllerClient.Get(ctx, TestKey, actualService)
			if testcase.expectedResources.service != nil {
				assert.NoError(t, err)
				assert.Equal(t, testcase.expectedResources.service.Spec, actualService.Spec)
				assert.Equal(t, testcase.expectedResources.service.OwnerReferences[0].APIVersion, actualService.OwnerReferences[0].APIVersion)
				assert.Equal(t, testcase.expectedResources.service.OwnerReferences[0].Kind, actualService.OwnerReferences[0].Kind)
				assert.Equal(t, testcase.expectedResources.service.OwnerReferences[0].Name, actualService.OwnerReferences[0].Name)
			}
		})
	}
}
