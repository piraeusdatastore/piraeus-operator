package linstorhelper

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	lapi "github.com/LINBIT/golinstor/client"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Client is a LINSTOR client with convenience functions.
type Client struct {
	lapi.Client
}

// NewClientForCluster returns a LINSTOR client for a LINSTOR Controller managed by the operator.
func NewClientForCluster(ctx context.Context, cl client.Client, clusterName string, options ...lapi.Option) (*Client, error) {
	services := corev1.ServiceList{}
	err := cl.List(ctx, &services, client.MatchingLabels{
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
	port := 3370
	for _, p := range s.Spec.Ports {
		if p.Name == "api" {
			port = int(p.Port)
		}
	}

	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc:%d", s.Name, s.Namespace, port),
	}

	c, err := lapi.NewClient(append(options, lapi.BaseURL(u))...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
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
