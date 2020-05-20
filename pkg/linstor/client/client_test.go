/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"testing"

	lapi "github.com/LINBIT/golinstor/client"
)

func TestFilterNode(t *testing.T) {
	var tableTest = []struct {
		raw        []lapi.ResourceWithVolumes
		filterNode string
		filtered   []lapi.ResourceWithVolumes
	}{
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node3"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
			"node4",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node4"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node3"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
			"node1",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node2"}},
			},
			"node2",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test4", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test3", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test2", NodeName: "node2"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node2"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"node1",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{},
			"node4",
			[]lapi.ResourceWithVolumes{},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"node0",
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
			},
		},
		{
			[]lapi.ResourceWithVolumes{
				{Resource: lapi.Resource{Name: "test0", NodeName: "node0"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test1", NodeName: "node1"}},
				{Resource: lapi.Resource{Name: "test0", NodeName: "node2"}},
			},
			"fake-node",
			[]lapi.ResourceWithVolumes{},
		},
	}

	for _, tt := range tableTest {
		actual := filterNodes(tt.raw, tt.filterNode)

		if !reflect.DeepEqual(tt.filtered, actual) {
			// Structs are printed without field names for a more compact comparison.
			t.Errorf("\nexpected\n\t%v\nto filter into\n\t%v\ngot\n\t%v",
				tt.raw, tt.filtered, actual)
		}
	}
}

func TestNewClientConfigForApiResource(t *testing.T) {
	testcases := []struct {
		name           string
		clientConfig   piraeusv1alpha1.LinstorClientConfig
		expectedConfig LinstorClientConfig
	}{
		{
			name:         "default",
			clientConfig: piraeusv1alpha1.LinstorClientConfig{},
			expectedConfig: LinstorClientConfig{
				Global: GlobalLinstorClientConfig{
					Controllers: []string{"http://default.test.svc:3370"},
				},
			},
		},
		{
			name: "with-https-client-auth",
			clientConfig: piraeusv1alpha1.LinstorClientConfig{
				LinstorHttpsClientSecret:  "secret",
			},
			expectedConfig: LinstorClientConfig{
				Global: GlobalLinstorClientConfig{
					Controllers: []string{"https://with-https-client-auth.test.svc:3371"},
					CAFile:      "/etc/linstor/client/ca.pem",
					Keyfile:     "/etc/linstor/client/client.key",
					Certfile:    "/etc/linstor/client/client.cert",
				},
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			serviceName := types.NamespacedName{Name: testcase.name, Namespace: "test"}
			actual := NewClientConfigForApiResource(serviceName, &testcase.clientConfig)

			if !reflect.DeepEqual(actual, &testcase.expectedConfig) {
				t.Fatalf("client configs not equal. expected: %v, actual: %v", testcase.expectedConfig, *actual)
			}
		})
	}
}

func TestClientConfigAsEnvVars(t *testing.T) {
	expectedHttpControllerVar := corev1.EnvVar{
		Name: "LS_CONTROLLERS",
		Value: "http://controller.test.svc:3370",
	}

	expectedHttpsControllerVar := corev1.EnvVar{
		Name: "LS_CONTROLLERS",
		Value: "https://controller.test.svc:3371",
	}

	expectedRootCaVar := corev1.EnvVar{
		Name: "LS_ROOT_CA",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret",
				},
				Key: "ca.pem",
			},
		},
	}

	expectedUserCertVar := corev1.EnvVar{
		Name: "LS_USER_CERTIFICATE",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret",
				},
				Key: "client.cert",
			},
		},
	}

	expectedUserKeyVar := corev1.EnvVar{
		Name: "LS_USER_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "secret",
				},
				Key: "client.key",
			},
		},
	}

	testcases := []struct {
		name           string
		clientConfig   piraeusv1alpha1.LinstorClientConfig
		expectedConfig []corev1.EnvVar
	}{
		{
			name:           "default",
			clientConfig:   piraeusv1alpha1.LinstorClientConfig{},
			expectedConfig: []corev1.EnvVar{
				expectedHttpControllerVar,
			},
		},
		{
			name: "with-https-client-auth",
			clientConfig: piraeusv1alpha1.LinstorClientConfig{
				LinstorHttpsClientSecret:  "secret",
			},
			expectedConfig: []corev1.EnvVar{
				expectedHttpsControllerVar,
				expectedRootCaVar,
				expectedUserCertVar,
				expectedUserKeyVar,
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			serviceName := types.NamespacedName{Name: "controller", Namespace: "test"}
			actual := ApiResourceAsEnvVars(serviceName, &testcase.clientConfig)

			if !reflect.DeepEqual(actual, testcase.expectedConfig) {
				t.Fatalf("client configs not equal. expected: %v, actual: %v", testcase.expectedConfig, actual)
			}
		})
	}
}
