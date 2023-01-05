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
)

// Client is a LINSTOR client with convenience functions.
type Client struct {
	lapi.Client
}

// NewClientForCluster returns a LINSTOR client for a LINSTOR Controller managed by the operator.
func NewClientForCluster(ctx context.Context, cl client.Client, namespace, clusterName, clientSecretName string, options ...lapi.Option) (*Client, error) {
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

	scheme, port := extractSchemeAndPort(&s)

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

	options = append(options, lapi.BaseURL(&url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s.%s.svc:%d", s.Name, s.Namespace, port),
	}))

	c, err := lapi.NewClient(options...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
}

func extractSchemeAndPort(svc *corev1.Service) (string, int32) {
	port := int32(3370)
	for _, p := range svc.Spec.Ports {
		if p.Name == "secure-api" {
			return "https", p.Port
		}

		if p.Name == "api" {
			port = p.Port
		}
	}

	return "http", port
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
