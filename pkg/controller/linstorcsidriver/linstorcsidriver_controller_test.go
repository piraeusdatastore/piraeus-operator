package linstorcsidriver

import (
	"context"
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis"
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"testing"
)

var (
	OperatorName         = "foo"
	OperatorNamespace    = "bar"
	DefaultNodeDaemonSet = appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-node-daemonset",
			Namespace: "bar",
		},
	}
	DefaultControllerDeployment = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-controller-deployment",
			Namespace: "bar",
		},
	}
	DefaultNodeServiceAccount = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-node-sa",
			Namespace: "bar",
		},
	}
	DefaulControllerServiceAccount = corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-controller-sa",
			Namespace: "bar",
		},
	}
	DefaultPriorityClass = schedv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo-csi-priority-class",
			Namespace: "bar",
		},
	}
)

func TestReconcileLinstorCSIDriver_Reconcile(t *testing.T) {
	type expectedResources struct {
		daemonsSets     []appsv1.DaemonSet
		deployments     []appsv1.Deployment
		serviceAccounts []corev1.ServiceAccount
		priorityClasses []schedv1.PriorityClass
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
					Spec: piraeusv1alpha1.LinstorCSIDriverSpec{
						LinstorControllerAddress: "http://fakeservice.svc:3700/",
					},
				},
			},
			expectedResources: expectedResources{
				daemonsSets:     []appsv1.DaemonSet{DefaultNodeDaemonSet},
				deployments:     []appsv1.Deployment{DefaultControllerDeployment},
				serviceAccounts: []corev1.ServiceAccount{DefaulControllerServiceAccount, DefaultNodeServiceAccount},
				priorityClasses: []schedv1.PriorityClass{DefaultPriorityClass},
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
			}
		})
	}
}

func compareDaemonSets(expectedItems []appsv1.DaemonSet, actualItems []appsv1.DaemonSet, t *testing.T) {
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

func compareDeployments(expectedItems []appsv1.Deployment, actualItems []appsv1.Deployment, t *testing.T) {
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

func compareServiceAccounts(expectedItems []corev1.ServiceAccount, actualItems []corev1.ServiceAccount, t *testing.T) {
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

func comparePriorityClass(expectedItems []schedv1.PriorityClass, actualItems []schedv1.PriorityClass, t *testing.T) {
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
