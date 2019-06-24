/*
* A REST client to interact with LINSTOR's REST API
* Copyright © 2019 LINBIT HA-Solutions GmbH
* Author: Roland Kammerer <roland.kammerer@linbit.com>
*
* This program is free software; you can redistribute it and/or modify
* it under the terms of the GNU General Public License as published by
* the Free Software Foundation; either version 2 of the License, or
* (at your option) any later version.
*
* This program is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License
* along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package client

import "context"

// copy & paste from generated code

// Node represents a node in LINSTOR
type Node struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Flags []string `json:"flags,omitempty"`
	// A string to string property map.
	Props         map[string]string `json:"props,omitempty"`
	NetInterfaces []NetInterface    `json:"net_interfaces,omitempty"`
	// Enum describing the current connection status.
	ConnectionStatus string `json:"connection_status,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

type NodeModify struct {
	NodeType string `json:"node_type,omitempty"`
	// A string to string property map.
	GenericPropsModify
}

// NetInterface represents a node's network interface.
type NetInterface struct {
	Name                    string `json:"name"`
	Address                 string `json:"address"`
	SatellitePort           int32  `json:"satellite_port,omitempty"`
	SatelliteEncryptionType string `json:"satellite_encryption_type,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

// StoragePool represents a nodes storage pool as defined in LINSTOR.
type StoragePool struct {
	StoragePoolName string       `json:"storage_pool_name"`
	NodeName        string       `json:"node_name,omitempty"`
	ProviderKind    ProviderKind `json:"provider_kind"`
	// A string to string property map.
	Props map[string]string `json:"props,omitempty"`
	// read only map of static storage pool traits
	StaticTraits map[string]string `json:"static_traits,omitempty"`
	// Kibi - read only
	FreeCapacity int64 `json:"free_capacity,omitempty"`
	// Kibi - read only
	TotalCapacity int64 `json:"total_capacity,omitempty"`
	// read only
	FreeSpaceMgrName string `json:"free_space_mgr_name,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
	// Currently known report messages for this storage pool
	Reports []ApiCallRc `json:"reports,omitempty"`
}

// ProviderKind is a type that represents various types of storage.
type ProviderKind string

// List of ProviderKind
const (
	DISKLESS            ProviderKind = "DISKLESS"
	LVM                 ProviderKind = "LVM"
	LVM_THIN            ProviderKind = "LVM_THIN"
	ZFS                 ProviderKind = "ZFS"
	ZFS_THIN            ProviderKind = "ZFS_THIN"
	SWORDFISH_TARGET    ProviderKind = "SWORDFISH_TARGET"
	SWORDFISH_INITIATOR ProviderKind = "SWORDFISH_INITIATOR"
)

// custom code

// NodeService is the service that deals with node related tasks.
type NodeService struct {
	client *Client
}

// GetAll gets information for all registered nodes.
func (n *NodeService) GetAll(ctx context.Context, opts ...*ListOpts) ([]Node, error) {
	var nodes []Node
	_, err := n.client.doGET(ctx, "/v1/nodes", &nodes, opts...)
	return nodes, err
}

// Get gets information for a particular node.
func (n *NodeService) Get(ctx context.Context, nodeName string, opts ...*ListOpts) (Node, error) {
	var node Node
	_, err := n.client.doGET(ctx, "/v1/nodes/"+nodeName, &node, opts...)
	return node, err
}

// Create creates a new node object.
func (n *NodeService) Create(ctx context.Context, node Node) error {
	_, err := n.client.doPOST(ctx, "/v1/nodes", node)
	return err
}

// Modify modifies the given node and sets/deletes the given properties.
func (n *NodeService) Modify(ctx context.Context, nodeName string, props NodeModify) error {
	_, err := n.client.doPUT(ctx, "/v1/nodes/"+nodeName, props)
	return err
}

// Delete deletes the given node.
func (n *NodeService) Delete(ctx context.Context, nodeName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/nodes/"+nodeName, nil)
	return err
}

// Lost marks the given node as lost to delete an unrecoverable node.
func (n *NodeService) Lost(ctx context.Context, nodeName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/nodes/"+nodeName+"/lost", nil)
	return err
}

// Reconnect reconnects a node to the controller.
func (n *NodeService) Reconnect(ctx context.Context, nodeName string) error {
	_, err := n.client.doPUT(ctx, "/v1/nodes/"+nodeName+"/reconnect", nil)
	return err
}

// GetNetInterfaces gets information about all network interfaces of a given node.
func (n *NodeService) GetNetInterfaces(ctx context.Context, nodeName string, opts ...*ListOpts) ([]NetInterface, error) {
	var nifs []NetInterface
	_, err := n.client.doGET(ctx, "/v1/nodes/"+nodeName+"/net-interfaces", &nifs, opts...)
	return nifs, err
}

// GetNetInterface gets information about a particular network interface on a given node.
func (n *NodeService) GetNetInterface(ctx context.Context, nodeName, nifName string, opts ...*ListOpts) (NetInterface, error) {
	var nif NetInterface
	_, err := n.client.doGET(ctx, "/v1/nodes/"+nodeName+"/net-interfaces/"+nifName, nif, opts...)
	return nif, err
}

// CreateNetInterface creates the given network interface on a given node.
func (n *NodeService) CreateNetInterface(ctx context.Context, nodeName string, nif NetInterface) error {
	_, err := n.client.doPOST(ctx, "/v1/nodes/"+nodeName+"/net-interfaces", nif)
	return err
}

// ModifyNetInterface modifies the given network interface on a given node.
func (n *NodeService) ModifyNetInterface(ctx context.Context, nodeName, nifName string, nif NetInterface) error {
	_, err := n.client.doPUT(ctx, "/v1/nodes/"+nodeName+"/net-interfaces/"+nifName, nif)
	return err
}

// DeleteNetinterface deletes the given network interface on a given node.
func (n *NodeService) DeleteNetinterface(ctx context.Context, nodeName, nifName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/nodes/"+nodeName+"/net-interfaces/"+nifName, nil)
	return err
}

// GetStoragePoolView gets information about all storage pools in the cluster.
func (n *NodeService) GetStoragePoolView(ctx context.Context, opts ...*ListOpts) ([]StoragePool, error) {
	var sps []StoragePool
	_, err := n.client.doGET(ctx, "/v1/view/storage-pools", &sps, opts...)
	return sps, err
}

// GetStoragePools gets information about all storage pools on a given node.
func (n *NodeService) GetStoragePools(ctx context.Context, nodeName string, opts ...*ListOpts) ([]StoragePool, error) {
	var sps []StoragePool
	_, err := n.client.doGET(ctx, "/v1/nodes/"+nodeName+"/storage-pools", &sps, opts...)
	return sps, err
}

// GetStoragePool gets information about a specific storage pool on a given node.
func (n *NodeService) GetStoragePool(ctx context.Context, nodeName, spName string, opts ...*ListOpts) (StoragePool, error) {
	var sp StoragePool
	_, err := n.client.doGET(ctx, "/v1/nodes/"+nodeName+"/storage-pools/"+spName, &sp, opts...)
	return sp, err
}

// CreateStoragePool creates a storage pool on a given node.
func (n *NodeService) CreateStoragePool(ctx context.Context, nodeName string, sp StoragePool) error {
	_, err := n.client.doPOST(ctx, "/v1/nodes/"+nodeName+"/storage-pools", sp)
	return err
}

// ModifyStoragePool modifies a storage pool on a given node.
func (n *NodeService) ModifyStoragePool(ctx context.Context, nodeName, spName string, sp StoragePool) error {
	_, err := n.client.doPOST(ctx, "/v1/nodes/"+nodeName+"/storage-pools/"+spName, sp)
	return err
}

// DeleteStoragePool deletes a storage pool on a given node.
func (n *NodeService) DeleteStoragePool(ctx context.Context, nodeName, spName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/nodes/"+nodeName+"/storage-pools/"+spName, nil)
	return err
}
