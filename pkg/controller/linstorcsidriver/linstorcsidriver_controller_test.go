package linstorcsidriver

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis"
	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
)

var customLabels = map[string]string{
	"piraeus": "test",
}

var (
	yes                  = true
	DefaultNodeDaemonSet = appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo-csi-node",
			Namespace:   "bar",
			Annotations: customLabels,
			Labels:      customLabels,
		},
	}
	DefaultControllerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo-csi-controller",
			Namespace:   "bar",
			Annotations: customLabels,
			Labels:      customLabels,
		},
	}
	DefaultCSIDriver = storagev1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "linstor.csi.linbit.com",
			Annotations: customLabels,
			Labels:      customLabels,
		},
		Spec: storagev1.CSIDriverSpec{
			AttachRequired:  &yes,
			PodInfoOnMount:  &yes,
			StorageCapacity: &yes,
		},
	}
)

func TestReconcileLinstorCSIDriver_Reconcile(t *testing.T) {
	t.Parallel()

	type expectedResources struct {
		daemonsSets []appsv1.DaemonSet
		deployments []appsv1.Deployment
		csiDrivers  []storagev1.CSIDriver
	}

	testcases := []struct {
		name              string
		initialResources  []client.Object
		expectedResources expectedResources
		withError         bool
	}{
		{
			name:              "no-resource-no-reconcile",
			initialResources:  []client.Object{},
			expectedResources: expectedResources{},
			withError:         false,
		},
		{
			name: "default-config-creates-everything",
			initialResources: []client.Object{
				&piraeusv1.LinstorCSIDriver{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "foo",
						Namespace:   "bar",
						Annotations: customLabels,
						Labels:      customLabels,
					},
					Spec: piraeusv1.LinstorCSIDriverSpec{},
				},
			},
			expectedResources: expectedResources{
				daemonsSets: []appsv1.DaemonSet{DefaultNodeDaemonSet},
				deployments: []appsv1.Deployment{DefaultControllerDeployment},
				csiDrivers:  []storagev1.CSIDriver{DefaultCSIDriver},
			},
		},
	}

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("Could not prepare test: %v", err)
	}

	for i := range testcases {
		testcase := &testcases[i]

		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			// Create controller fake client.
			controllerClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testcase.initialResources...).Build()

			reconciler := ReconcileLinstorCSIDriver{controllerClient, scheme.Scheme}

			_, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "bar"}})
			if testcase.withError {
				if err == nil {
					t.Errorf("expected error, got no error")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got error: %v", err)
				}

				// TO CHECK: DaemonSets, Deployments, ServiceAccounts, PriorityClasses
				daemonSets := appsv1.DaemonSetList{}
				err = controllerClient.List(context.Background(), &daemonSets)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				compareDaemonSets(testcase.expectedResources.daemonsSets, daemonSets.Items, t)

				deployments := appsv1.DeploymentList{}
				err = controllerClient.List(context.Background(), &deployments)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				compareDeployments(testcase.expectedResources.deployments, deployments.Items, t)

				drivers := storagev1.CSIDriverList{}
				err = controllerClient.List(context.Background(), &drivers)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				compareCSIDrivers(t, testcase.expectedResources.csiDrivers, drivers.Items)
			}
		})
	}
}

func compareDaemonSets(expectedItems, actualItems []appsv1.DaemonSet, t *testing.T) {
	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected daemonsets to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *appsv1.DaemonSet = nil
		for _, candidate := range expectedItems {
			if actual.Name == candidate.Name && actual.Namespace == candidate.Namespace {
				expected = &candidate
				break
			}
		}

		if expected == nil {
			t.Errorf("unexpected daemonset: %s/%s", actual.Namespace, actual.Name)
			continue
		}

		// TODO: deeper comparison
	}
}

func compareDeployments(expectedItems, actualItems []appsv1.Deployment, t *testing.T) {
	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected deployments to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *appsv1.Deployment

		for _, candidate := range expectedItems {
			if actual.Name == candidate.Name && actual.Namespace == candidate.Namespace {
				expected = &candidate
				break
			}
		}

		if expected == nil {
			t.Errorf("unexpected deployment: %s/%s", actual.Namespace, actual.Name)
			continue
		}

		// TODO: deeper comparison
	}
}

func compareCSIDrivers(t *testing.T, expectedItems, actualItems []storagev1.CSIDriver) {
	t.Helper()

	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected daemonsets to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *storagev1.CSIDriver

		for _, candidate := range expectedItems {
			if actual.Name == candidate.Name && actual.Namespace == candidate.Namespace {
				expected = &candidate
				break
			}
		}

		if expected == nil {
			t.Errorf("unexpected csi driver: %s/%s", actual.Namespace, actual.Name)
			continue
		}

		if !reflect.DeepEqual(actual.Spec.AttachRequired, expected.Spec.PodInfoOnMount) {
			t.Errorf("driver %s/%s differs in .Spec.PodInfoOnMount", actual.Namespace, actual.Name)
		}

		if !reflect.DeepEqual(actual.Spec.AttachRequired, expected.Spec.AttachRequired) {
			t.Errorf("driver %s/%s differs in .Spec.AttachRequired", actual.Namespace, actual.Name)
		}
	}
}
