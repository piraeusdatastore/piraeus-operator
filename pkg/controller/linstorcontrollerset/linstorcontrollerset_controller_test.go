package linstorcontrollerset

import (
	piraeusv1alpha1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestNewConfigMapForPCS(t *testing.T) {
	testcases := []struct {
		name     string
		spec     *piraeusv1alpha1.LinstorControllerSet
		expected *corev1.ConfigMap
	}{
		{
			name: "default-settings",
			spec: &piraeusv1alpha1.LinstorControllerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1alpha1.LinstorControllerSetSpec{
					DBConnectionURL: "etcd://etcd.svc:5000/",
					DBCertSecret:    "",
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default-ns",
				},
				Data: map[string]string{
					"linstor.toml": `[config]

[debug]

[log]

[db]
  connection_url = "etcd://etcd.svc:5000/"
  [db.etcd]

[http]

[https]

[ldap]
`,
				},
			},
		},
		{
			name: "with-ssl-without-client-cert",
			spec: &piraeusv1alpha1.LinstorControllerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1alpha1.LinstorControllerSetSpec{
					DBConnectionURL: "etcd://secure.etcd.svc:443/",
					DBCertSecret:    "mysecret",
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default-ns",
				},
				Data: map[string]string{
					"linstor.toml": `[config]

[debug]

[log]

[db]
  connection_url = "etcd://secure.etcd.svc:443/"
  ca_certificate = "/etc/linstor/certs/ca.pem"
  [db.etcd]

[http]

[https]

[ldap]
`,
				},
			},
		},
		{
			name: "with-ssl-with-client-cert",
			spec: &piraeusv1alpha1.LinstorControllerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1alpha1.LinstorControllerSetSpec{
					DBConnectionURL: "etcd://secure.etcd.svc:443/",
					DBCertSecret:    "mysecret",
					DBUseClientCert: true,
				},
			},
			expected: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-config",
					Namespace: "default-ns",
				},
				Data: map[string]string{
					"linstor.toml": `[config]

[debug]

[log]

[db]
  connection_url = "etcd://secure.etcd.svc:443/"
  ca_certificate = "/etc/linstor/certs/ca.pem"
  client_certificate = "/etc/linstor/certs/client.cert"
  client_key_pkcs8_pem = "/etc/linstor/certs/client.key"
  [db.etcd]

[http]

[https]

[ldap]
`,
				},
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			actualCM, err := NewConfigMapForPCS(test.spec)
			if err != nil {
				t.Fatalf("config map creation failed: %v", err)
			}

			if actualCM.Name != test.expected.Name {
				t.Errorf("cm: name does not match, expected: '%s', actual: '%s'", test.expected.Name, actualCM.Name)
			}

			if actualCM.Namespace != test.expected.Namespace {
				t.Errorf("cm: namespace does not match, expected: '%s', actual: '%s'", test.expected.Namespace, actualCM.Namespace)
			}

			if !reflect.DeepEqual(actualCM.Data, test.expected.Data) {
				t.Errorf("cm: data does not match, expected: '%v', actual: '%v'", test.expected.Data, actualCM.Data)
			}
		})
	}
}
