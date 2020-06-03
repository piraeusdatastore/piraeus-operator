package linstorcsidriver

import (
	"context"
	"reflect"
	"testing"

	schedv1 "k8s.io/api/scheduling/v1"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis"
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	CSIDriverAttachRequired = true
	CSIDriverPodInfoOnMount = true
	DefaultNodeDaemonSet    = appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-node",
			Namespace: "bar",
		},
	}
	DefaultControllerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-controller",
			Namespace: "bar",
		},
	}
	DefaultNodeServiceAccount = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-node",
			Namespace: "bar",
		},
	}
	DefaulControllerServiceAccount = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-controller",
			Namespace: "bar",
		},
	}
	DefaultPriorityClass = schedv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi",
			Namespace: "bar",
		},
	}
	DefaultCSIDriver = storagev1beta1.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "linstor.csi.linbit.com",
			Namespace: "bar",
		},
		Spec: storagev1beta1.CSIDriverSpec{
			AttachRequired: &CSIDriverAttachRequired,
			PodInfoOnMount: &CSIDriverPodInfoOnMount,
		},
	}
)

func TestReconcileLinstorCSIDriver_Reconcile(t *testing.T) {
	type expectedResources struct {
		daemonsSets     []appsv1.DaemonSet
		deployments     []appsv1.Deployment
		serviceAccounts []corev1.ServiceAccount
		priorityClasses []schedv1.PriorityClass
		csiDrivers      []storagev1beta1.CSIDriver
	}

	testcases := []struct {
		name              string
		initialResources  []runtime.Object
		expectedResources expectedResources
		withError         bool
	}{
		{
			name:              "no-resource-no-reconcile",
			initialResources:  []runtime.Object{},
			expectedResources: expectedResources{},
			withError:         false,
		},
		{
			name: "default-config-creates-everything",
			initialResources: []runtime.Object{
				&piraeusv1alpha1.LinstorCSIDriver{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
					Spec: piraeusv1alpha1.LinstorCSIDriverSpec{},
				},
			},
			expectedResources: expectedResources{
				daemonsSets:     []appsv1.DaemonSet{DefaultNodeDaemonSet},
				deployments:     []appsv1.Deployment{DefaultControllerDeployment},
				serviceAccounts: []corev1.ServiceAccount{DefaulControllerServiceAccount, DefaultNodeServiceAccount},
				priorityClasses: []schedv1.PriorityClass{DefaultPriorityClass},
				csiDrivers:      []storagev1beta1.CSIDriver{DefaultCSIDriver},
			},
		},
	}

	err := apis.AddToScheme(scheme.Scheme)
	if err != nil {
		t.Fatalf("Could not prepare test: %v", err)
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			// Create controller fake client.
			controllerClient := fake.NewFakeClientWithScheme(scheme.Scheme, testcase.initialResources...)

			reconciler := ReconcileLinstorCSIDriver{controllerClient, scheme.Scheme}

			_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "bar"}})
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

				serviceAccounts := corev1.ServiceAccountList{}
				err = controllerClient.List(context.Background(), &serviceAccounts)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				compareServiceAccounts(testcase.expectedResources.serviceAccounts, serviceAccounts.Items, t)

				priorityClasses := schedv1.PriorityClassList{}
				err = controllerClient.List(context.Background(), &priorityClasses)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				comparePriorityClass(testcase.expectedResources.priorityClasses, priorityClasses.Items, t)

				drivers := storagev1beta1.CSIDriverList{}
				err = controllerClient.List(context.Background(), &drivers)
				if err != nil {
					t.Fatalf("Failed to fetch items: %v", err)
				}
				compareCSIDrivers(testcase.expectedResources.csiDrivers, drivers.Items, t)
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
		var expected *appsv1.Deployment = nil
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

func compareServiceAccounts(expectedItems, actualItems []corev1.ServiceAccount, t *testing.T) {
	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected serviceaccounts to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *corev1.ServiceAccount = nil
		for _, candidate := range expectedItems {
			if actual.Name == candidate.Name && actual.Namespace == candidate.Namespace {
				expected = &candidate
				break
			}
		}

		if expected == nil {
			t.Errorf("unexpected serviceaccount: %s/%s", actual.Namespace, actual.Name)
			continue
		}
	}
}

func comparePriorityClass(expectedItems, actualItems []schedv1.PriorityClass, t *testing.T) {
	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected priorityclasses to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *schedv1.PriorityClass = nil
		for _, candidate := range expectedItems {
			if actual.Name == candidate.Name && actual.Namespace == candidate.Namespace {
				expected = &candidate
				break
			}
		}

		if expected == nil {
			t.Errorf("unexpected priorityclass: %s/%s", actual.Namespace, actual.Name)
			continue
		}
	}
}

func compareCSIDrivers(expectedItems, actualItems []storagev1beta1.CSIDriver, t *testing.T) {
	if len(expectedItems) != len(actualItems) {
		t.Errorf("expected daemonsets to contain %d items, got %d instead", len(expectedItems), len(actualItems))
	}

	for _, actual := range actualItems {
		var expected *storagev1beta1.CSIDriver = nil
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
