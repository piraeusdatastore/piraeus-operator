package client

import (
	"context"
	"fmt"

	lapi "github.com/LINBIT/golinstor/client"
)

// Various lapi consts yet to be defined in golinstor.
const (
	Controller = "CONTROLLER"
	Satellite  = "SATELLITE"
	Online     = "ONLINE"
)

// StorageNode is a linstor node with its respective storage pools.
type StorageNode struct {
	lapi.Node
	StoragePools []lapi.StoragePool
}

// HighLevelClient is a golinstor client with convience fucntions.
type HighLevelClient struct {
	lapi.Client
}

// NewHighLevelClient returns a pointer to a golinstor client with convience.
func NewHighLevelClient(options ...func(*lapi.Client) error) (*HighLevelClient, error) {
	c, err := lapi.NewClient(options...)
	if err != nil {
		return nil, err
	}
	return &HighLevelClient{*c}, nil
}

// GetStoragePoolOrCreateOnNode gets a linstor storage pool, creating it on the
// node if it is not already present.
func (c *HighLevelClient) GetStoragePoolOrCreateOnNode(ctx context.Context, pool lapi.StoragePool, nodeName string) (lapi.StoragePool, error) {
	foundPool, err := c.Nodes.GetStoragePool(ctx, nodeName, pool.StoragePoolName)
	// StoragePool doesn't exists, create it.
	if err != nil && err == lapi.NotFoundError {
		if err := c.Nodes.CreateStoragePool(ctx, nodeName, pool); err != nil {
			return pool, fmt.Errorf("unable to create storage pool %s on node %s: %v", pool.StoragePoolName, nodeName, err)
		}
		return c.Nodes.GetStoragePool(ctx, nodeName, pool.StoragePoolName)
	}
	// Other error.
	if err != nil {
		return pool, fmt.Errorf("unable to get storage pool %s on node %s: %v", pool.StoragePoolName, nodeName, err)
	}

	return foundPool, nil
}

// GetNodeOrCreate gets a linstor node, creating it if it is not already present.
func (c *HighLevelClient) GetNodeOrCreate(ctx context.Context, node lapi.Node) (lapi.Node, error) {
	n, err := c.Nodes.Get(context.TODO(), node.Name)
	if err != nil {
		if err != lapi.NotFoundError {
			return n, fmt.Errorf("unable to get node %s: %v", node.Name, err)
		}

		// Node doesn't exist, create it.
		if len(node.NetInterfaces) != 1 {
			return n, fmt.Errorf("only able to create a new node with a single interface")
		}

		if err := c.Nodes.Create(context.TODO(), node); err != nil {
			return n, fmt.Errorf("unable to create node %s: %v", node.Name, err)
		}

		newNode, err := c.Nodes.Get(context.TODO(), node.Name)
		if err != nil {
			return newNode, fmt.Errorf("unable to get newly created node %s: %v", node.Name, err)
		}

		return newNode, c.insureWantedInterface(ctx, newNode, node.NetInterfaces[0])
	}

	return n, nil
}

func (c *HighLevelClient) insureWantedInterface(ctx context.Context, node lapi.Node, wanted lapi.NetInterface) error {
	// Make sure default network interface is to spec.
	for _, nodeIf := range node.NetInterfaces {
		if nodeIf.Name == wanted.Name {

			// TODO: Maybe we should error out here.
			if nodeIf.Address != wanted.Address {
				if err := c.Nodes.ModifyNetInterface(ctx, node.Name, wanted.Name, wanted); err != nil {
					return fmt.Errorf("unable to modify default network interface on %s: %v", node.Name, err)
				}
			}
			break
		}

		if err := c.Nodes.CreateNetInterface(ctx, node.Name, wanted); err != nil {
			return fmt.Errorf("unable to create default network interface on %s: %v", node.Name, err)
		}
	}
	return nil
}

// GetAllResourcesOnNode returns a list of all resources on the specified node.
func (c *HighLevelClient) GetAllResourcesOnNode(ctx context.Context, nodeName string) ([]lapi.Resource, error) {
	resList, err := c.Resources.GetResourceView(ctx) //, &lapi.ListOpts{Node: []string{nodeName}}) : not working
	if err != nil && err != lapi.NotFoundError {
		return resList, fmt.Errorf("unable to check for resources on node %s: %v", nodeName, err)
	}

	return filterNodes(resList, nodeName), nil
}

func filterNodes(resources []lapi.Resource, nodeName string) []lapi.Resource {
	var nodeRes = make([]lapi.Resource, 0)
	for _, r := range resources {
		if r.NodeName == nodeName {
			nodeRes = append(nodeRes, r)
		}
	}
	return nodeRes
}

// GetAllStorageNodes returns a list of all Satellite nodes with a list of their
// storage pools.
func (c *HighLevelClient) GetAllStorageNodes(ctx context.Context) ([]StorageNode, error) {
	var storageNodes = make([]StorageNode, 0)

	// TODO: Expand LINSTOR API for an all nodes plus storage pools view?
	nodes, err := c.Nodes.GetAll(ctx)
	if err != nil {
		return storageNodes, fmt.Errorf("unable to get cluster nodes: %v", err)
	}
	pools, err := c.Nodes.GetStoragePoolView(ctx)
	if err != nil {
		return storageNodes, fmt.Errorf("unable to get cluster storage pools: %v", err)
	}

	for _, node := range nodes {
		if node.Type == Satellite {
			sn := StorageNode{node, make([]lapi.StoragePool, 0)}
			for _, pool := range pools {
				if pool.NodeName == sn.Name {
					sn.StoragePools = append(sn.StoragePools, pool)
				}
			}
			storageNodes = append(storageNodes, sn)
		}
	}
	return storageNodes, nil
}
