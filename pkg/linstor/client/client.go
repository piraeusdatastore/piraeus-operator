package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	linstor "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"
	"github.com/sirupsen/logrus"
	ini "gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	kubeSpec "github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
)

// Various lapi consts yet to be defined in golinstor.
const (
	Controller                 = "CONTROLLER"
	Satellite                  = "SATELLITE"
	Online                     = "ONLINE"
	Offline                    = "OFFLINE"
	DefaultHTTPPort            = 3370
	DefaultHTTPSPort           = 3371
	ControllerReachableTimeout = 10 * time.Second
)

// Global linstor client configuration, like controllers and connection settings
type GlobalLinstorClientConfig struct {
	// Comma separated list of of LINSTOR REST API endpoints
	Controllers []string `ini:"controllers" delim:"|"`
	// Path to the PEM encoded root certificates used for HTTPS connections
	CAFile string `ini:"cafile,omitempty"`
	// Path to the PEM encoded certificate to present when TLS authentication is required
	Certfile string `ini:"certfile,omitempty"`
	// Path to the PEM encoded private key used when TLS authentication is required
	Keyfile string `ini:"keyfile,omitempty"`
}

// LinstorClientConfig is the go representation of `/etc/linstor/linstor-client.conf`.
type LinstorClientConfig struct {
	Global GlobalLinstorClientConfig `ini:"global"`
}

// StorageNode is a linstor node with its respective storage pools.
type StorageNode struct {
	lapi.Node
	StoragePools []lapi.StoragePool
}

// HighLevelClient is a golinstor client with convience functions.
type HighLevelClient struct {
	lapi.Client
}

type SecretFetcher func(string) (map[string][]byte, error)

// NamedSecret returns a SecretFetcher for the named secret.
func NamedSecret(ctx context.Context, client client.Client, namespace string) SecretFetcher {
	return func(secretName string) (map[string][]byte, error) {
		secret := corev1.Secret{}

		err := client.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, &secret)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch secret: %w", err)
		}

		return secret.Data, nil
	}
}

// NewHighLevelLinstorClientFromConfig configures a HighLevelClient with an
// in-cluster url based on service naming convention.
func NewHighLevelLinstorClientFromConfig(endpoint string, config *shared.LinstorClientConfig, secretFetcher SecretFetcher) (*HighLevelClient, error) {
	tlsConfig, err := newTLSConfigFromConfig(config, secretFetcher)
	if err != nil {
		return nil, fmt.Errorf("unable to create TLSSecret for HTTP client: %w", err)
	}

	transport := http.Transport{
		TLSClientConfig: tlsConfig,
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("unable to create LINSTOR API client: %v", err)
	}

	c, err := NewHighLevelClient(
		lapi.BaseURL(u),
		lapi.Log(&logrus.Logger{
			Level:     logrus.DebugLevel,
			Out:       os.Stdout,
			Formatter: &logrus.TextFormatter{},
		}),
		lapi.HTTPClient(&http.Client{Transport: &transport}),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create LINSTOR API client: %v", err)
	}

	return c, nil
}

// Convert an ApiResource (i.e. secret name) into a go tls configration useable for HTTP clients.
func newTLSConfigFromConfig(cfg *shared.LinstorClientConfig, secretFetcher SecretFetcher) (*tls.Config, error) {
	if cfg.LinstorHttpsClientSecret == "" {
		return nil, nil
	}

	var clientCerts []tls.Certificate

	rootCA := x509.NewCertPool()

	secretData, err := secretFetcher(cfg.LinstorHttpsClientSecret)
	if err != nil {
		return nil, err
	}

	rootCaBytes, ok := secretData[SecretCARootName]
	if !ok {
		return nil, fmt.Errorf("did not find expected key '%s' in secret '%s'", SecretCARootName, cfg.LinstorHttpsClientSecret)
	}

	ok = rootCA.AppendCertsFromPEM(rootCaBytes)
	if !ok {
		return nil, fmt.Errorf("failed to set valid root certificate for linstor client")
	}

	clientKey, ok := secretData[SecretKeyName]
	if !ok {
		return nil, fmt.Errorf("did not find expected key '%s' in secret '%s'", SecretKeyName, cfg.LinstorHttpsClientSecret)
	}

	clientCert, ok := secretData[SecretCertName]
	if !ok {
		return nil, fmt.Errorf("did not find expected key '%s' in secret '%s'", SecretCertName, cfg.LinstorHttpsClientSecret)
	}

	key, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, err
	}

	clientCerts = []tls.Certificate{key}

	return &tls.Config{
		RootCAs:      rootCA,
		Certificates: clientCerts,
	}, nil
}

// NewHighLevelClient returns a pointer to a golinstor client with convience.
func NewHighLevelClient(options ...lapi.Option) (*HighLevelClient, error) {
	c, err := lapi.NewClient(options...)
	if err != nil {
		return nil, err
	}
	return &HighLevelClient{*c}, nil
}

// GetNodeOrCreate gets a linstor node, creating it if it is not already present.
func (c *HighLevelClient) GetNodeOrCreate(ctx context.Context, node lapi.Node) (*lapi.Node, error) {
	existingNode, err := c.Nodes.Get(ctx, node.Name)
	if err != nil {
		// For 404
		if err != lapi.NotFoundError {
			return nil, fmt.Errorf("unable to get node %s: %w", node.Name, err)
		}

		// Node doesn't exist, create it.
		if err := c.Nodes.Create(ctx, node); err != nil {
			return nil, fmt.Errorf("unable to create node %s: %w", node.Name, err)
		}

		newNode, err := c.Nodes.Get(ctx, node.Name)
		if err != nil {
			return nil, fmt.Errorf("unable to get newly created node %s: %w", node.Name, err)
		}

		existingNode = newNode
	}

	upToDate := true

	for k, v := range node.Props {
		existing, ok := existingNode.Props[k]
		if !ok || existing != v {
			upToDate = false

			break
		}
	}

	var propsToDelete []string

	for k := range existingNode.Props {
		if !strings.HasPrefix(k, linstor.NamespcAuxiliary+"/") {
			// We only want to manage auxiliary properties
			continue
		}

		_, ok := node.Props[k]
		if !ok {
			propsToDelete = append(propsToDelete, k)
		}
	}

	if !upToDate || len(propsToDelete) != 0 {
		err := c.Nodes.Modify(ctx, node.Name, lapi.NodeModify{GenericPropsModify: lapi.GenericPropsModify{OverrideProps: node.Props, DeleteProps: propsToDelete}})
		if err != nil {
			return nil, fmt.Errorf("unable to update node properties: %w", err)
		}
	}

	for _, nic := range node.NetInterfaces {
		err = c.ensureWantedInterface(ctx, existingNode, nic)
		if err != nil {
			return nil, fmt.Errorf("failed to update network interface: %w", err)
		}
	}

	return &existingNode, nil
}

func (c *HighLevelClient) ensureWantedInterface(ctx context.Context, node lapi.Node, wanted lapi.NetInterface) error {
	for _, nodeIf := range node.NetInterfaces {
		if nodeIf.Name != wanted.Name {
			continue
		}

		if nodeIf.Address == wanted.Address && nodeIf.SatelliteEncryptionType == wanted.SatelliteEncryptionType && nodeIf.SatellitePort == wanted.SatellitePort {
			return nil
		}

		return c.Nodes.ModifyNetInterface(ctx, node.Name, wanted.Name, wanted)
	}

	// Interface was not found, creating it now
	return c.Nodes.CreateNetInterface(ctx, node.Name, wanted)
}

// GetAllResourcesOnNode returns a list of all resources on the specified node.
func (c *HighLevelClient) GetAllResourcesOnNode(ctx context.Context, nodeName string) ([]lapi.ResourceWithVolumes, error) {
	resList, err := c.Resources.GetResourceView(ctx) //, &lapi.ListOpts{Node: []string{nodeName}}) : not working
	if err != nil && err != lapi.NotFoundError {
		return resList, fmt.Errorf("unable to check for resources on node %s: %v", nodeName, err)
	}

	return filterNodes(resList, nodeName), nil
}

// ControllerReachable returns true if the LINSTOR controller is up.
func (c *HighLevelClient) ControllerReachable(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, ControllerReachableTimeout)
	defer cancel()

	_, err := c.Controller.GetVersion(ctx)

	return err == nil
}

func filterNodes(resources []lapi.ResourceWithVolumes, nodeName string) []lapi.ResourceWithVolumes {
	nodeRes := make([]lapi.ResourceWithVolumes, 0)
	for i := range resources {
		r := resources[i]

		if r.NodeName == nodeName {
			nodeRes = append(nodeRes, r)
		}
	}
	return nodeRes
}

// GetAllStorageNodes returns a list of all Satellite nodes with a list of their
// storage pools.
func (c *HighLevelClient) GetAllStorageNodes(ctx context.Context) ([]StorageNode, error) {
	storageNodes := make([]StorageNode, 0)

	nodes, err := c.Nodes.GetAll(ctx)
	if err != nil {
		return storageNodes, fmt.Errorf("unable to get cluster nodes: %v", err)
	}

	cached := true
	pools, err := c.Nodes.GetStoragePoolView(ctx, &lapi.ListOpts{Cached: &cached})
	if err != nil {
		return storageNodes, fmt.Errorf("unable to get cluster storage pools: %v", err)
	}

	for _, node := range nodes {
		if node.Type == Satellite {
			sn := StorageNode{node, make([]lapi.StoragePool, 0)}
			for i := range pools {
				pool := pools[i]

				if pool.NodeName == sn.Name {
					sn.StoragePools = append(sn.StoragePools, pool)
				}
			}
			storageNodes = append(storageNodes, sn)
		}
	}
	return storageNodes, nil
}

// Create a client config from an API resource.
func NewClientConfigForAPIResource(endpoint string, resource *shared.LinstorClientConfig) *LinstorClientConfig {
	clientCAPath := ""
	clientCertPath := ""
	clientKeyPath := ""

	if resource.LinstorHttpsClientSecret != "" {
		clientCAPath = kubeSpec.LinstorClientDir + "/ca.crt"
		clientCertPath = kubeSpec.LinstorClientDir + "/tls.crt"
		clientKeyPath = kubeSpec.LinstorClientDir + "/tls.key"
	}

	return &LinstorClientConfig{
		Global: GlobalLinstorClientConfig{
			Controllers: []string{endpoint},
			CAFile:      clientCAPath,
			Certfile:    clientCertPath,
			Keyfile:     clientKeyPath,
		},
	}
}

func DefaultControllerServiceEndpoint(serviceName types.NamespacedName, useHTTPS bool) string {
	if useHTTPS {
		return fmt.Sprintf("https://%s.%s.svc:%d", serviceName.Name, serviceName.Namespace, DefaultHTTPSPort)
	} else {
		return fmt.Sprintf("http://%s.%s.svc:%d", serviceName.Name, serviceName.Namespace, DefaultHTTPPort)
	}
}

func (clientConfig *LinstorClientConfig) ToConfigFile() (string, error) {
	cfg := ini.Empty()
	err := ini.ReflectFrom(cfg, clientConfig)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	_, err = cfg.WriteTo(&builder)
	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

// Consts for extracting TLS certificates from api resources
const (
	SecretCARootName = "ca.crt"
	SecretKeyName    = "tls.key"
	SecretCertName   = "tls.crt"
)

// Convert a LinstorClientConfig into env variables understood by the CSI plugins and golinstor client
// See also: https://pkg.go.dev/github.com/LINBIT/golinstor/client?tab=doc#NewClient
func APIResourceAsEnvVars(endpoint string, resource *shared.LinstorClientConfig) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "LS_CONTROLLERS",
			Value: endpoint,
		},
	}

	if resource.LinstorHttpsClientSecret != "" {
		env = append(env, corev1.EnvVar{
			Name: "LS_ROOT_CA",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: resource.LinstorHttpsClientSecret,
					},
					Key: SecretCARootName,
				},
			},
		}, corev1.EnvVar{
			Name: "LS_USER_CERTIFICATE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: resource.LinstorHttpsClientSecret,
					},
					Key: SecretCertName,
				},
			},
		}, corev1.EnvVar{
			Name: "LS_USER_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: resource.LinstorHttpsClientSecret,
					},
					Key: SecretKeyName,
				},
			},
		})
	}

	return env
}
