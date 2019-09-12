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

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// ResourceService is a struct which contains the pointer of the client
type ResourceService struct {
	client *Client
}

// copy & paste from generated code

// Resource is a struct which holds the information of a resource
type Resource struct {
	Name     string `json:"name,omitempty"`
	NodeName string `json:"node_name,omitempty"`
	// A string to string property map.
	Props       map[string]string `json:"props,omitempty"`
	Flags       []string          `json:"flags,omitempty"`
	LayerObject ResourceLayer     `json:"layer_object,omitempty"`
	State       ResourceState     `json:"state,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

type ResourceWithVolumes struct {
	Resource
	Volumes []Volume `json:"volumes,omitempty"`
}

type ResourceDefinitionModify struct {
	// drbd port for resources
	DrbdPort int32 `json:"drbd_port,omitempty"`
	// drbd peer slot number
	DrbdPeerSlots int32       `json:"drbd_peer_slots,omitempty"`
	LayerStack    []LayerType `json:"layer_stack,omitempty"`
	GenericPropsModify
}

// ResourceCreate is a struct where the properties of a resource are stored to create it
type ResourceCreate struct {
	Resource   Resource    `json:"resource,omitempty"`
	LayerList  []LayerType `json:"layer_list,omitempty"`
	DrbdNodeId int32       `json:"drbd_node_id,omitempty"`
}

// ResourceLayer is a struct to store layer-information abour a resource
type ResourceLayer struct {
	Children           []ResourceLayer `json:"children,omitempty"`
	ResourceNameSuffix string          `json:"resource_name_suffix,omitempty"`
	Type               LayerType       `json:"type,omitempty"`
	Drbd               DrbdResource    `json:"drbd,omitempty"`
	Luks               LuksResource    `json:"luks,omitempty"`
	Storage            StorageResource `json:"storage,omitempty"`
	Nvme               NvmeResource    `json:"nvme,omitempty"`
}

// DrbdResource is a struct used to give linstor drbd properties for a resource
type DrbdResource struct {
	DrbdResourceDefinition DrbdResourceDefinitionLayer `json:"drbd_resource_definition,omitempty"`
	NodeId                 int32                       `json:"node_id,omitempty"`
	PeerSlots              int32                       `json:"peer_slots,omitempty"`
	AlStripes              int32                       `json:"al_stripes,omitempty"`
	AlSize                 int64                       `json:"al_size,omitempty"`
	Flags                  []string                    `json:"flags,omitempty"`
	DrbdVolumes            []DrbdVolume                `json:"drbd_volumes,omitempty"`
}

// DrbdVolume is a struct for linstor to get inormation about a drbd-volume
type DrbdVolume struct {
	DrbdVolumeDefinition DrbdVolumeDefinition `json:"drbd_volume_definition,omitempty"`
	// drbd device path e.g. '/dev/drbd1000'
	DevicePath string `json:"device_path,omitempty"`
	// block device used by drbd
	BackingDevice    string `json:"backing_device,omitempty"`
	MetaDisk         string `json:"meta_disk,omitempty"`
	AllocatedSizeKib int64  `json:"allocated_size_kib,omitempty"`
	UsableSizeKib    int64  `json:"usable_size_kib,omitempty"`
	// String describing current volume state
	DiskState string `json:"disk_state,omitempty"`
	// Storage pool name used for external meta data; null for internal
	ExtMetaStorPool string `json:"ext_meta_stor_pool,omitempty"`
}

// LuksResource is a struct to store storage-volumes for a luks-resource
type LuksResource struct {
	StorageVolumes []LuksVolume `json:"storage_volumes,omitempty"`
}

// LuksVolume is a struct used for information about a luks-volume
type LuksVolume struct {
	VolumeNumber int32 `json:"volume_number,omitempty"`
	// block device path
	DevicePath string `json:"device_path,omitempty"`
	// block device used by luks
	BackingDevice    string `json:"backing_device,omitempty"`
	AllocatedSizeKib int64  `json:"allocated_size_kib,omitempty"`
	UsableSizeKib    int64  `json:"usable_size_kib,omitempty"`
	// String describing current volume state
	DiskState string `json:"disk_state,omitempty"`
	Opened    bool   `json:"opened,omitempty"`
}

// StorageResource is a struct which contains the storage-volumes for a storage-resource
type StorageResource struct {
	StorageVolumes []StorageVolume `json:"storage_volumes,omitempty"`
}

// StorageVolume is a struct to store standard poperties of a Volume
type StorageVolume struct {
	VolumeNumber int32 `json:"volume_number,omitempty"`
	// block device path
	DevicePath       string `json:"device_path,omitempty"`
	AllocatedSizeKib int64  `json:"allocated_size_kib,omitempty"`
	UsableSizeKib    int64  `json:"usable_size_kib,omitempty"`
	// String describing current volume state
	DiskState string `json:"disk_state,omitempty"`
}

type NvmeResource struct {
	NvmeVolumes []NvmeVolume `json:"nvme_volumes,omitempty"`
}

type NvmeVolume struct {
	VolumeNumber int32 `json:"volume_number,omitempty"`
	// block device path
	DevicePath string `json:"device_path,omitempty"`
	// block device used by nvme
	BackingDevice    string `json:"backing_device,omitempty"`
	AllocatedSizeKib int64  `json:"allocated_size_kib,omitempty"`
	UsableSizeKib    int64  `json:"usable_size_kib,omitempty"`
	// String describing current volume state
	DiskState string `json:"disk_state,omitempty"`
}

// ResourceState is a struct for getting the status of a resource
type ResourceState struct {
	InUse bool `json:"in_use,omitempty"`
}

// Volume is a struct which holds the information about a linstor-volume
type Volume struct {
	VolumeNumber     int32        `json:"volume_number,omitempty"`
	StoragePool      string       `json:"storage_pool,omitempty"`
	ProviderKind     ProviderKind `json:"provider_kind,omitempty"`
	DevicePath       string       `json:"device_path,omitempty"`
	AllocatedSizeKib int64        `json:"allocated_size_kib,omitempty"`
	UsableSizeKib    int64        `json:"usable_size_kib,omitempty"`
	// A string to string property map.
	Props         map[string]string `json:"props,omitempty"`
	Flags         []string          `json:"flags,omitempty"`
	State         VolumeState       `json:"state,omitempty"`
	LayerDataList []VolumeLayer     `json:"layer_data_list,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

// VolumeLayer is a struct for storing the layer-properties of a linstor-volume
type VolumeLayer struct {
	Type LayerType                                        `json:"type,omitempty"`
	Data OneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume `json:"data,omitempty"`
}

// VolumeState is a struct which contains the disk-state for volume
type VolumeState struct {
	DiskState string `json:"disk_state,omitempty"`
}

// AutoPlaceRequest is a struct to store the paramters for the linstor auto-place command
type AutoPlaceRequest struct {
	DisklessOnRemaining bool             `json:"diskless_on_remaining,omitempty"`
	SelectFilter        AutoSelectFilter `json:"select_filter,omitempty"`
	LayerList           []LayerType      `json:"layer_list,omitempty"`
}

// AutoSelectFilter is a struct used to have information about the auto-select function
type AutoSelectFilter struct {
	PlaceCount           int32    `json:"place_count,omitempty"`
	StoragePool          string   `json:"storage_pool,omitempty"`
	NotPlaceWithRsc      []string `json:"not_place_with_rsc,omitempty"`
	NotPlaceWithRscRegex string   `json:"not_place_with_rsc_regex,omitempty"`
	ReplicasOnSame       []string `json:"replicas_on_same,omitempty"`
	ReplicasOnDifferent  []string `json:"replicas_on_different,omitempty"`
	LayerStack           []string `json:"layer_stack,omitempty"`
	ProviderList         []string `json:"provider_list,omitempty"`
	DisklessOnRemaining  bool     `json:"diskless_on_remaining,omitempty"`
}

// ResourceConnection is a struct which holds information about a connection between to nodes
type ResourceConnection struct {
	// source node of the connection
	NodeA string `json:"node_a,omitempty"`
	// target node of the connection
	NodeB string `json:"node_b,omitempty"`
	// A string to string property map.
	Props map[string]string `json:"props,omitempty"`
	Flags []string          `json:"flags,omitempty"`
	Port  int32             `json:"port,omitempty"`
}

// Snapshot is a struct for information about a snapshot
type Snapshot struct {
	Name         string   `json:"name,omitempty"`
	ResourceName string   `json:"resource_name,omitempty"`
	Nodes        []string `json:"nodes,omitempty"`
	// A string to string property map.
	Props             map[string]string          `json:"props,omitempty"`
	Flags             []string                   `json:"flags,omitempty"`
	VolumeDefinitions []SnapshotVolumeDefinition `json:"volume_definitions,omitempty"`
	// unique object id
	Uuid string `json:"uuid,omitempty"`
}

// SnapshotVolumeDefinition is a struct to store the properties of a volume from a snapshot
type SnapshotVolumeDefinition struct {
	VolumeNumber int32 `json:"volume_number,omitempty"`
	// Volume size in KiB
	SizeKib uint64 `json:"size_kib,omitempty"`
}

// SnapshotRestore is a struct used to hold the information about where a Snapshot has to be restored
type SnapshotRestore struct {
	// Resource where to restore the snapshot
	ToResource string `json:"to_resource"`
	// List of nodes where to place the restored snapshot
	Nodes []string `json:"nodes,omitempty"`
}

type DrbdProxyModify struct {
	// Compression type used by the proxy.
	CompressionType string `json:"compression_type,omitempty"`
	// A string to string property map.
	CompressionProps map[string]string `json:"compression_props,omitempty"`
	GenericPropsModify
}

// custom code

// volumeLayerIn is a struct for volume-layers
type volumeLayerIn struct {
	Type LayerType       `json:"type,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// UnmarshalJSON fulfills the unmarshal interface for the VolumeLayer type
func (v *VolumeLayer) UnmarshalJSON(b []byte) error {
	var vIn volumeLayerIn
	if err := json.Unmarshal(b, &vIn); err != nil {
		return err
	}

	v.Type = vIn.Type
	switch v.Type {
	case DRBD:
		dst := new(DrbdVolume)
		if vIn.Data != nil {
			if err := json.Unmarshal(vIn.Data, &dst); err != nil {
				return err
			}
		}
		v.Data = dst
	case LUKS:
		dst := new(LuksVolume)
		if vIn.Data != nil {
			if err := json.Unmarshal(vIn.Data, &dst); err != nil {
				return err
			}
		}
		v.Data = dst
	case STORAGE:
		dst := new(StorageVolume)
		if vIn.Data != nil {
			if err := json.Unmarshal(vIn.Data, &dst); err != nil {
				return err
			}
		}
		v.Data = dst
	case NVME:
		dst := new(NvmeVolume)
		if vIn.Data != nil {
			if err := json.Unmarshal(vIn.Data, &dst); err != nil {
				return err
			}
		}
		v.Data = dst
	default:
		return fmt.Errorf("'%+v' is not a valid type to Unmarshal", v.Type)
	}

	return nil
}

// OneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume  is used to prevent that other types than drbd- luks- and storage-volume are used for a VolumeLayer
type OneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume interface {
	isOneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume()
}

// Functions which are used if type is a correct VolumeLayer
func (d *DrbdVolume) isOneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume()    {}
func (d *LuksVolume) isOneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume()    {}
func (d *StorageVolume) isOneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume() {}
func (d *NvmeVolume) isOneOfDrbdVolumeLuksVolumeStorageVolumeNvmeVolume()    {}

// GetResourceView returns all resources in the cluster. Filters can be set via ListOpts.
func (n *ResourceService) GetResourceView(ctx context.Context, opts ...*ListOpts) ([]ResourceWithVolumes, error) {
	var reses []ResourceWithVolumes
	_, err := n.client.doGET(ctx, "/v1/view/resources", &reses, opts...)
	return reses, err
}

// GetAll returns all resources for a resource-definition
func (n *ResourceService) GetAll(ctx context.Context, resName string, opts ...*ListOpts) ([]Resource, error) {
	var reses []Resource
	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/resources", &reses, opts...)
	return reses, err
}

// Get returns information about a resource on a specific node
func (n *ResourceService) Get(ctx context.Context, resName, nodeName string, opts ...*ListOpts) (Resource, error) {
	var res Resource
	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/resources/"+nodeName, &res, opts...)
	return res, err
}

// Create is used to create a resource on a node
func (n *ResourceService) Create(ctx context.Context, res ResourceCreate) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+res.Resource.Name+"/resources/"+res.Resource.NodeName, res)
	return err
}

// Modify gives the ability to modify a resource on a node
func (n *ResourceService) Modify(ctx context.Context, resName, nodeName string, props ResourceDefinitionModify) error {
	_, err := n.client.doPUT(ctx, "/v1/resource-definitions/"+resName+"/resources/"+nodeName, props)
	return err
}

// Delete deletes a resource on a specific node
func (n *ResourceService) Delete(ctx context.Context, resName, nodeName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/resource-definitions/"+resName+"/resources/"+nodeName, nil)
	return err
}

// GetVolumes lists als volumes of a resource
func (n *ResourceService) GetVolumes(ctx context.Context, resName, nodeName string, opts ...*ListOpts) ([]Volume, error) {
	var vols []Volume

	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/resources/"+nodeName+"/volumes", &vols, opts...)
	return vols, err
}

// GetVolume returns information about a specific volume defined by it resource,node and volume-number
func (n *ResourceService) GetVolume(ctx context.Context, resName, nodeName string, volNr int, opts ...*ListOpts) (Volume, error) {
	var vol Volume

	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/resources/"+nodeName+"/volumes/"+strconv.Itoa(volNr), &vol, opts...)
	return vol, err
}

// ModifyVolume modifies an existing volume with the given props
func (n *ResourceService) ModifyVolume(ctx context.Context, resName, nodeName string, volNr int, props GenericPropsModify) error {
	u := fmt.Sprintf("/v1/resource-definitions/%s/resources/%s/volumes/%d", resName, nodeName, volNr)
	_, err := n.client.doPUT(ctx, u, props)
	return err
}

// Diskless toggles a resource on a node to diskless - the parameter disklesspool can be set if its needed
func (n *ResourceService) Diskless(ctx context.Context, resName, nodeName, disklessPoolName string) error {
	u := "/v1/resource-definitions/" + resName + "/resources/" + nodeName + "/toggle-disk/diskless"
	if disklessPoolName != "" {
		u += "/" + disklessPoolName
	}
	_, err := n.client.doPUT(ctx, u, nil)
	return err
}

// Diskful toggles a resource to diskful - the parameter storagepool can be set if its needed
func (n *ResourceService) Diskful(ctx context.Context, resName, nodeName, storagePoolName string) error {
	u := "/v1/resource-definitions/" + resName + "/resources/" + nodeName + "/toggle-disk/diskful"
	if storagePoolName != "" {
		u += "/" + storagePoolName
	}
	_, err := n.client.doPUT(ctx, u, nil)
	return err
}

// Migrate mirgates a resource from one node to another node
func (n *ResourceService) Migrate(ctx context.Context, resName, fromNodeName, toNodeName, storagePoolName string) error {
	u := "/v1/resource-definitions/" + resName + "/resources/" + toNodeName + "/migrate-disk/" + fromNodeName
	if storagePoolName != "" {
		u += "/" + storagePoolName
	}
	_, err := n.client.doPUT(ctx, u, nil)
	return err
}

// Autoplace places a resource on your nodes autmatically
func (n *ResourceService) Autoplace(ctx context.Context, resName string, apr AutoPlaceRequest) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+resName+"/autoplace", apr)
	return err
}

// GetConnections lists all resource connections if no node-names are given- if two node-names are given it shows the connection between them
func (n *ResourceService) GetConnections(ctx context.Context, resName, nodeAName, nodeBName string, opts ...*ListOpts) ([]ResourceConnection, error) {
	var resConns []ResourceConnection

	u := "/v1/resource-definitions/" + resName + "/resources-connections"
	if nodeAName != "" && nodeBName != "" {
		u += fmt.Sprintf("/%s/%s", nodeAName, nodeBName)
	}

	_, err := n.client.doGET(ctx, u, &resConns, opts...)
	return resConns, err
}

// ModifyConnection allows to modify the connection between two nodes
func (n *ResourceService) ModifyConnection(ctx context.Context, resName, nodeAName, nodeBName string, props GenericPropsModify) error {
	u := fmt.Sprintf("/v1/resource-definitions/%s/resource-connections/%s/%s", resName, nodeAName, nodeBName)
	_, err := n.client.doPUT(ctx, u, props)
	return err
}

// GetSnapshots lists all snapshots of a resource
func (n *ResourceService) GetSnapshots(ctx context.Context, resName string, opts ...*ListOpts) ([]Snapshot, error) {
	var snaps []Snapshot

	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/snapshots", &snaps, opts...)
	return snaps, err
}

// GetSnapshot returns information about a specific Snapshot by its name
func (n *ResourceService) GetSnapshot(ctx context.Context, resName, snapName string, opts ...*ListOpts) (Snapshot, error) {
	var snap Snapshot

	_, err := n.client.doGET(ctx, "/v1/resource-definitions/"+resName+"/snapshots/"+snapName, &snap, opts...)
	return snap, err
}

// CreateSnapshot creates a snapshot of a resource
func (n *ResourceService) CreateSnapshot(ctx context.Context, snapshot Snapshot) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+snapshot.ResourceName+"/snapshots", snapshot)
	return err
}

// DeleteSnapshot deletes a snapshot by its name
func (n *ResourceService) DeleteSnapshot(ctx context.Context, resName, snapName string) error {
	_, err := n.client.doDELETE(ctx, "/v1/resource-definitions/"+resName+"/snapshots/"+snapName, nil)
	return err
}

// RestoreSnapshot restores a snapshot on a resource
func (n *ResourceService) RestoreSnapshot(ctx context.Context, origResName, snapName string, snapRestoreConf SnapshotRestore) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+origResName+"/snapshot-restore-resource/"+snapName, snapRestoreConf)
	return err
}

// RestoreVolumeDefinitionSnapshot restores a volume-definition-snapshot on a resource
func (n *ResourceService) RestoreVolumeDefinitionSnapshot(ctx context.Context, origResName, snapName string, snapRestoreConf SnapshotRestore) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+origResName+"/snapshot-restore-volume-definition/"+snapName, snapRestoreConf)
	return err
}

// RollbackSnapshot rolls back a snapshot from a specific resource
func (n *ResourceService) RollbackSnapshot(ctx context.Context, resName, snapName string) error {
	_, err := n.client.doPOST(ctx, "/v1/resource-definitions/"+resName+"/snapshot-rollback/"+snapName, nil)
	return err
}

// ModifyDRBDProxy is used to modify drbd-proxy properties
func (n *ResourceService) ModifyDRBDProxy(ctx context.Context, resName string, props DrbdProxyModify) error {
	_, err := n.client.doPUT(ctx, "/v1/resource-definitions/"+resName+"/drbd-proxy", props)
	return err
}

// enableDisableDRBDProxy enables or disables drbd-proxy between two nodes
func (n *ResourceService) enableDisableDRBDProxy(ctx context.Context, what, resName, nodeAName, nodeBName string) error {
	u := fmt.Sprintf("/v1/resource-definitions/%s/drbd-proxy/%s/%s/%s", resName, what, nodeAName, nodeBName)
	_, err := n.client.doPOST(ctx, u, nil)
	return err
}

// EnableDRBDProxy is used to enable drbd-proxy with the rest-api call from the function enableDisableDRBDProxy
func (n *ResourceService) EnableDRBDProxy(ctx context.Context, resName, nodeAName, nodeBName string) error {
	return n.enableDisableDRBDProxy(ctx, "enable", resName, nodeAName, nodeBName)
}

// DisableDRBDProxy is used to disable drbd-proxy with the rest-api call from the function enableDisableDRBDProxy
func (n *ResourceService) DisableDRBDProxy(ctx context.Context, resName, nodeAName, nodeBName string) error {
	return n.enableDisableDRBDProxy(ctx, "disable", resName, nodeAName, nodeBName)
}
