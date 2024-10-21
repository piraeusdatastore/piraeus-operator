package linstorhelper_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/google/go-cmp/cmp"
	"github.com/piraeusdatastore/linstor-csi/pkg/client/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

func TestNewClientForCluster(t *testing.T) {
	t.Parallel()

	testScheme := runtime.NewScheme()

	err := corev1.AddToScheme(testScheme)
	assert.NoError(t, err)

	err = piraeusv1.AddToScheme(testScheme)
	assert.NoError(t, err)

	tlsConfig, k8sSecretData := testTlsConfig(t)

	testcases := []struct {
		name             string
		existingObjs     []client.Object
		existingSecret   string
		externalRef      *piraeusv1.LinstorExternalControllerRef
		expectedNoClient bool
		expectedOptions  []lapi.Option
	}{
		{
			name:             "no-cluster-nil-client",
			expectedNoClient: true,
		},
		{
			name: "wrong-service-label-client-nil",
			existingObjs: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-service",
						Namespace: "test",
						Labels: map[string]string{
							"app.kubernetes.io/instance":  "other-cluster",
							"app.kubernetes.io/component": "linstor-controller",
						},
					},
				},
			},
			expectedNoClient: true,
		},
		{
			name: "cluster-external-controller",
			externalRef: &piraeusv1.LinstorExternalControllerRef{
				URL: "http://other-cluster.example.com:3370",
			},
			expectedOptions: []lapi.Option{
				lapi.BaseURL(&url.URL{Scheme: "http", Host: "other-cluster.example.com:3370"}),
				lapi.UserAgent(vars.OperatorName + "/" + vars.Version),
			},
		},
		{
			name: "cluster-external-controller-with-tls",
			externalRef: &piraeusv1.LinstorExternalControllerRef{
				URL: "https://other-cluster.example.com:3371",
			},
			existingObjs: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "client-secret", Namespace: "test"},
					Type:       corev1.SecretTypeTLS,
					Data:       k8sSecretData,
				},
			},
			existingSecret: "client-secret",
			expectedOptions: []lapi.Option{
				lapi.BaseURL(&url.URL{Scheme: "https", Host: "other-cluster.example.com:3371"}),
				lapi.HTTPClient(&http.Client{Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
				}}),
				lapi.UserAgent(vars.OperatorName + "/" + vars.Version),
			},
		},
		{
			name: "cluster-with-service-without-port-client-nil",
			existingObjs: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-service",
						Namespace: "test",
						Labels: map[string]string{
							"app.kubernetes.io/instance":  "test-cluster",
							"app.kubernetes.io/component": "linstor-controller",
						},
					},
				},
			},
			expectedNoClient: true,
		},
		{
			name: "cluster-with-service-with-port",
			existingObjs: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-service",
						Namespace: "test",
						Labels: map[string]string{
							"app.kubernetes.io/instance":  "test-cluster",
							"app.kubernetes.io/component": "linstor-controller",
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{
							Name: "api",
							Port: 3370,
						}},
					},
				},
			},
			expectedOptions: []lapi.Option{
				lapi.BaseURL(&url.URL{Scheme: "http", Host: "test-cluster-service.test.svc:3370"}),
				lapi.UserAgent(vars.OperatorName + "/" + vars.Version),
			},
		},
		{
			name: "cluster-with-service-with-port-tls",
			existingObjs: []client.Object{
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-service",
						Namespace: "test",
						Labels: map[string]string{
							"app.kubernetes.io/instance":  "test-cluster",
							"app.kubernetes.io/component": "linstor-controller",
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{{
							Name: "secure-api",
							Port: 3371,
						}},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "client-secret", Namespace: "test"},
					Type:       corev1.SecretTypeTLS,
					Data:       k8sSecretData,
				},
			},
			existingSecret: "client-secret",
			expectedOptions: []lapi.Option{
				lapi.BaseURL(&url.URL{Scheme: "https", Host: "test-cluster-service.test.svc:3371"}),
				lapi.HTTPClient(&http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}),
				lapi.UserAgent(vars.OperatorName + "/" + vars.Version),
			},
		},
	}

	for i := range testcases {
		testcase := &testcases[i]
		t.Run(testcase.name, func(t *testing.T) {
			t.Parallel()

			k8scl := fake.NewClientBuilder().WithObjects(testcase.existingObjs...).WithScheme(testScheme).Build()
			actual, err := linstorhelper.NewClientForCluster(context.Background(), k8scl, "test", "test-cluster", testcase.existingSecret, nil, testcase.externalRef)
			assert.NoError(t, err)

			if testcase.expectedNoClient {
				assert.Nil(t, actual)
			} else {
				expected, err := lapi.NewClient(testcase.expectedOptions...)
				require.NoError(t, err)

				// need to use go-cmp here, as that can handle the embedded x509.CertPool comparison.
				diff := cmp.Diff(*expected, actual.Client,
					// Compare all unexported fields, too
					cmp.Exporter(func(r reflect.Type) bool {
						return true
					}),
					// But ignore all logging
					cmp.Comparer(func(a, b lapi.Logger) bool {
						return true
					}),
				)
				if diff != "" {
					assert.Fail(t, diff)
				}
			}
		})
	}
}

func testTlsConfig(t *testing.T) (*tls.Config, map[string][]byte) {
	priv, err := rsa.GenerateKey(rand.Reader, 2049)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: t.Name(),
		},
	}

	rawCert, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	require.NoError(t, err)

	cert := tls.Certificate{
		Certificate: [][]byte{rawCert},
		PrivateKey:  priv,
	}

	caCert, err := x509.ParseCertificate(rawCert)
	require.NoError(t, err)

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rawCert})
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
		}, map[string][]byte{
			"ca.crt":  pemCert,
			"tls.key": pemKey,
			"tls.crt": pemCert,
		}
}

func TestCreateOrUpdateNode(t *testing.T) {
	t.Parallel()

	sampleNode := lapi.Node{
		Name: "node1",
		Props: map[string]string{
			"ExampleProp1":                      "val1",
			linstorhelper.LastApplyProperty:     `["` + linstorhelper.NodeInterfaceProperty + `","ExampleProp1"]`,
			linstorhelper.NodeInterfaceProperty: `["default-ipv4"]`,
		},
		NetInterfaces: []lapi.NetInterface{{
			Name:          "default-ipv4",
			Address:       net.IPv4(127, 0, 0, 1),
			SatellitePort: linstor.DfltStltPortPlain,
		}},
	}

	testcases := []struct {
		name       string
		node       lapi.Node
		setupCalls func(t *testing.T) lapi.NodeProvider
	}{
		{
			name: "new-node",
			node: sampleNode,
			setupCalls: func(t *testing.T) lapi.NodeProvider {
				m := mocks.NewNodeProvider(t)
				m.On("Get", mock.Anything, "node1").Return(lapi.Node{}, lapi.NotFoundError).Once()
				m.On("Create", mock.Anything, sampleNode).Return(nil)
				m.On("Get", mock.Anything, "node1").Return(sampleNode, nil)
				return m
			},
		},
		{
			name: "existing-node",
			node: sampleNode,
			setupCalls: func(t *testing.T) lapi.NodeProvider {
				m := mocks.NewNodeProvider(t)
				m.On("Get", mock.Anything, "node1").Return(sampleNode, nil)
				return m
			},
		},
		{
			name: "existing-node-with-updated-props-and-interfaces",
			node: lapi.Node{
				Name: "node1",
				Props: map[string]string{
					"ExampleProp1": "val2",
				},
				NetInterfaces: []lapi.NetInterface{{
					Name:                    "default-ipv6",
					Address:                 net.IPv6loopback,
					SatelliteEncryptionType: "SSL",
					SatellitePort:           linstor.DfltStltPortSsl,
				}},
			},
			setupCalls: func(t *testing.T) lapi.NodeProvider {
				m := mocks.NewNodeProvider(t)
				m.On("Get", mock.Anything, "node1").Return(sampleNode, nil)
				m.On("Modify", mock.Anything, "node1", lapi.NodeModify{
					GenericPropsModify: lapi.GenericPropsModify{
						OverrideProps: map[string]string{
							"ExampleProp1":                      "val2",
							linstorhelper.NodeInterfaceProperty: `["default-ipv6"]`,
						},
					},
				}).Return(nil)
				m.On("CreateNetInterface", mock.Anything, "node1", lapi.NetInterface{
					Name:                    "default-ipv6",
					Address:                 net.IPv6loopback,
					SatelliteEncryptionType: "SSL",
					SatellitePort:           linstor.DfltStltPortSsl,
				}).Return(nil)
				m.On("DeleteNetinterface", mock.Anything, "node1", "default-ipv4").Return(nil)
				return m
			},
		},
		{
			name: "existing-node-without-interface-props",
			node: sampleNode,
			setupCalls: func(t *testing.T) lapi.NodeProvider {
				m := mocks.NewNodeProvider(t)
				m.On("Get", mock.Anything, "node1").Return(lapi.Node{
					Name:  "node1",
					Props: nil,
					NetInterfaces: []lapi.NetInterface{
						{
							Name:          "default-ipv4",
							Address:       net.IPv4(127, 0, 0, 1),
							SatellitePort: linstor.DfltStltPortPlain,
						},
						{
							Name:                    "default-ipv6",
							Address:                 net.IPv6loopback,
							SatelliteEncryptionType: "SSL",
							SatellitePort:           linstor.DfltStltPortSsl,
						},
					},
				}, nil)
				m.On("Modify", mock.Anything, "node1", lapi.NodeModify{
					GenericPropsModify: lapi.GenericPropsModify{
						OverrideProps: map[string]string{
							"ExampleProp1":                      "val1",
							linstorhelper.LastApplyProperty:     `["` + linstorhelper.NodeInterfaceProperty + `","ExampleProp1"]`,
							linstorhelper.NodeInterfaceProperty: `["default-ipv4"]`,
						},
					},
				}).Return(nil)
				m.On("DeleteNetinterface", mock.Anything, "node1", "default-ipv6").Return(nil)
				return m
			},
		},
	}

	for i := range testcases {
		test := &testcases[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			lc := linstorhelper.Client{Client: lapi.Client{
				Nodes: test.setupCalls(t),
			}}

			_, err := lc.CreateOrUpdateNode(context.Background(), test.node)
			assert.NoError(t, err)
		})
	}
}
