package linstorhelper

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	lapi "github.com/LINBIT/golinstor/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	piraeusv1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// Client is a LINSTOR client with convenience functions.
type Client struct {
	lapi.Client
}

// NewClientForCluster returns a LINSTOR client for a LINSTOR Controller managed by the operator.
func NewClientForCluster(ctx context.Context, cl client.Client, namespace, clusterName, clientSecretName string, externalCluster *piraeusv1.LinstorExternalControllerRef, options ...lapi.Option) (*Client, error) {
	var clientUrl *url.URL
	if externalCluster != nil {
		u, err := url.Parse(externalCluster.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse external controller URL: %w", err)
		}

		clientUrl = u
	} else {
		services := corev1.ServiceList{}
		err := cl.List(ctx, &services, client.InNamespace(namespace), client.MatchingLabels{
			"app.kubernetes.io/instance":  clusterName,
			"app.kubernetes.io/component": "linstor-controller",
		})
		if err != nil {
			return nil, err
		}

		if len(services.Items) != 1 {
			return nil, nil
		}

		s := services.Items[0]

		scheme, port, ok := extractSchemeAndPort(&s)
		if !ok {
			return nil, nil
		}

		clientUrl = &url.URL{
			Scheme: scheme,
			Host:   fmt.Sprintf("%s.%s.svc:%d", s.Name, s.Namespace, port),
		}
	}

	if clientSecretName != "" {
		var secret corev1.Secret
		err := cl.Get(ctx, types.NamespacedName{Name: clientSecretName, Namespace: namespace}, &secret)
		if err != nil {
			return nil, err
		}

		tlsConfig, err := secretToTlsConfig(&secret)
		if err != nil {
			return nil, err
		}

		options = append(options, lapi.HTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}))
	}

	options = append(options, lapi.BaseURL(clientUrl), lapi.UserAgent(vars.OperatorName+"/"+vars.Version))

	c, err := lapi.NewClient(options...)
	if err != nil {
		return nil, err
	}

	return &Client{*c}, nil
}

// extractSchemeAndPort returns the preferred connection scheme and port from the service.
// It prefers HTTPS connections when it finds a "secure-api" port and falls back to the "api" service otherwise.
// If no suitable port was found, the last argument will return false.
func extractSchemeAndPort(svc *corev1.Service) (string, int32, bool) {
	var port int32
	var scheme string
	found := false

	for _, p := range svc.Spec.Ports {
		if p.Name == "secure-api" {
			return "https", p.Port, true
		}

		if p.Name == "api" {
			port = p.Port
			scheme = "http"
			found = true
		}
	}

	return scheme, port, found
}

func secretToTlsConfig(secret *corev1.Secret) (*tls.Config, error) {
	if secret.Type != corev1.SecretTypeTLS {
		return nil, fmt.Errorf("secret '%s/%s' of type '%s', expected '%s'", secret.Namespace, secret.Name, secret.Type, corev1.SecretTypeTLS)
	}

	caRoot := secret.Data["ca.crt"]
	key := secret.Data[corev1.TLSPrivateKeyKey]

	cert := secret.Data[corev1.TLSCertKey]

	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM(caRoot)
	if !ok {
		return nil, fmt.Errorf("failed to parse CA root: %s", caRoot)
	}

	keyPair, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: []tls.Certificate{keyPair},
	}, nil
}

// CreateOrUpdateNode ensures a node in LINSTOR matches the given node object.
func (c *Client) CreateOrUpdateNode(ctx context.Context, node lapi.Node) (*lapi.Node, error) {
	existingNode, err := c.Nodes.Get(ctx, node.Name)
	if err != nil {
		// For 404
		if err != lapi.NotFoundError {
			return nil, fmt.Errorf("unable to get node %s: %w", node.Name, err)
		}

		// Node doesn't exist, create it.
		node.Props = UpdateLastApplyProperty(node.Props)
		if err := c.Nodes.Create(ctx, node); err != nil {
			return nil, fmt.Errorf("unable to create node %s: %w", node.Name, err)
		}

		newNode, err := c.Nodes.Get(ctx, node.Name)
		if err != nil {
			return nil, fmt.Errorf("unable to get newly created node %s: %w", node.Name, err)
		}

		existingNode = newNode
	}

	modification := MakePropertiesModification(existingNode.Props, node.Props)
	if modification != nil {
		err := c.Nodes.Modify(ctx, node.Name, lapi.NodeModify{GenericPropsModify: *modification})
		if err != nil {
			return nil, err
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

func (c *Client) ensureWantedInterface(ctx context.Context, node lapi.Node, wanted lapi.NetInterface) error {
	for _, nodeIf := range node.NetInterfaces {
		if nodeIf.Name != wanted.Name {
			continue
		}

		// LINSTOR is sadly inconsistent with using "Plain" vs "PLAIN" in encryption types. Fixing it in linstor-common
		// (which is used to generate the constants in golinstor) was deemed too much effort, so we do the next best
		// thing: just ignore casing while comparing.
		if nodeIf.Address.Equal(wanted.Address) && strings.EqualFold(nodeIf.SatelliteEncryptionType, wanted.SatelliteEncryptionType) && nodeIf.SatellitePort == wanted.SatellitePort {
			return nil
		}

		return c.Nodes.ModifyNetInterface(ctx, node.Name, wanted.Name, wanted)
	}

	// Interface was not found, creating it now
	return c.Nodes.CreateNetInterface(ctx, node.Name, wanted)
}
