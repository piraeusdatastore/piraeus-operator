package linstorcontroller

import (
	"reflect"
	"testing"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewConfigMapForPCS(t *testing.T) {
	testcases := []struct {
		name     string
		spec     *piraeusv1.LinstorController
		expected *corev1.ConfigMap
	}{
		{
			name: "default-settings",
			spec: &piraeusv1.LinstorController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1.LinstorControllerSpec{
					DBConnectionURL:     "etcd://etcd.svc:5000/",
					DBCertSecret:        "",
					LinstorClientConfig: shared.LinstorClientConfig{},
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
					"linstor-client.conf": `[global]
controllers = http://test.default-ns.svc:3370

`,
				},
			},
		},
		{
			name: "with-ssl-without-client-cert",
			spec: &piraeusv1.LinstorController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1.LinstorControllerSpec{
					DBConnectionURL:     "etcd://secure.etcd.svc:443/",
					DBCertSecret:        "mysecret",
					LinstorClientConfig: shared.LinstorClientConfig{},
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
					"linstor-client.conf": `[global]
controllers = http://test.default-ns.svc:3370

`,
				},
			},
		},
		{
			name: "with-ssl-with-client-cert",
			spec: &piraeusv1.LinstorController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1.LinstorControllerSpec{
					DBConnectionURL:     "etcd://secure.etcd.svc:443/",
					DBCertSecret:        "mysecret",
					DBUseClientCert:     true,
					LinstorClientConfig: shared.LinstorClientConfig{},
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
					"linstor-client.conf": `[global]
controllers = http://test.default-ns.svc:3370

`,
				},
			},
		},
		{
			name: "with-https-auth",
			spec: &piraeusv1.LinstorController{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default-ns",
				},
				Spec: piraeusv1.LinstorControllerSpec{
					DBConnectionURL:              "etcd://etcd.svc:5000/",
					DBCertSecret:                 "",
					LinstorHttpsControllerSecret: "controller-secret",
					LinstorClientConfig: shared.LinstorClientConfig{
						LinstorHttpsClientSecret: "secret",
					},
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
  enabled = true
  keystore = "/etc/linstor/https/keystore.jks"
  keystore_password = "linstor"
  truststore = "/etc/linstor/https/truststore.jks"
  truststore_password = "linstor"

[ldap]
`,
					"linstor-client.conf": `[global]
controllers = https://test.default-ns.svc:3371
cafile      = /etc/linstor/client/ca.pem
certfile    = /etc/linstor/client/client.cert
keyfile     = /etc/linstor/client/client.key

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
