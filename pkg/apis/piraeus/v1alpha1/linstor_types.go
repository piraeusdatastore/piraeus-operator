/*
Piraeus Operator
Copyright 2019 LINBIT USA, LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	lapi "github.com/LINBIT/golinstor/client"
)
import lapiconst "github.com/LINBIT/golinstor"

// NodeStatus simple status of the node in the linstor cluster.
type NodeStatus struct {
	// Indicates if the node has been created on the controller.
	RegisteredOnController bool `json:"registeredOnController"`
	// The hostname of the kubelet running the node
	NodeName string `json:"nodeName"`
}

// SatelliteStatus should provide all the information that the reconsile loop
// needs to manage the operation of the LINSTOR Satellite.
type SatelliteStatus struct {
	NodeStatus `json:",inline"`
	// As indicated by Linstor
	ConnectionStatus string `json:"connectionStatus"`
	// StoragePoolStatuses by storage pool name.
	StoragePoolStatuses []*StoragePoolStatus `json:"storagePoolStatus"`
}

// StoragePoolStatus reports basic information about storage pool state.
type StoragePoolStatus struct {
	// The name of the storage pool.
	Name string `json:"name"`
	// The hostname of the kubelet hosting the storage pool.
	NodeName string `json:"nodeName"`
	// Provider is the underlying storage, lvm, zfs, etc.
	Provider string `json:"provider"`
	// Usage reporting
	FreeCapacity  int64 `json:"freeCapacity"`
	TotalCapacity int64 `json:"totalCapacity"`
}

// NewStoragePoolStatus convert from golinstor StoragePool to StoragePoolStatus.
func NewStoragePoolStatus(pool lapi.StoragePool) *StoragePoolStatus {
	return &StoragePoolStatus{
		Name:          pool.StoragePoolName,
		NodeName:      pool.NodeName,
		Provider:      string(pool.ProviderKind),
		FreeCapacity:  pool.FreeCapacity,
		TotalCapacity: pool.TotalCapacity,
	}
}

// StoragePools hold lists of linstor storage pools supported by this operator.
type StoragePools struct {
	// LVMPools for LinstorNodeSet to manage.
	// +optional
	// +nullable
	LVMPools []*StoragePoolLVM `json:"lvmPools"`
	// LVMThinPools for LinstorNodeSet to manage.
	// +optional
	// +nullable
	LVMThinPools []*StoragePoolLVMThin `json:"lvmThinPools"`
}

// StoragePool is the generalized type of storage pools.
type StoragePool interface {
	ToLinstorStoragePool() lapi.StoragePool
}

// StoragePoolLVM represents LVM storage pool to be managed by a
// LinstorNodeSet
type StoragePoolLVM struct {
	// Name of the storage pool.
	Name string `json:"name"`
	// Name of underlying lvm group
	VolumeGroup string `json:"volumeGroup"`
}

// ToLinstorStoragePool returns lapi.StoragePool presentation of the StoragePoolLVM
func (s *StoragePoolLVM) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: s.Name,
		ProviderKind:    lapi.LVM,
		Props: map[string]string{
			"StorDriver/LvmVg": s.VolumeGroup,
		},
	}
}

// StoragePoolLVMThin represents LVM Thin storage pool to be
// managed by a LinstorNodeSet
type StoragePoolLVMThin struct {
	StoragePoolLVM `json:",inline"`
	// Name of underlying lvm thin volume
	ThinVolume string `json:"thinVolume"`
}

// ToLinstorStoragePool returns lapi.StoragePool presentation of the StoragePoolLVMThin
func (s *StoragePoolLVMThin) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: s.Name,
		ProviderKind:    lapi.LVM_THIN,
		Props: map[string]string{
			"StorDriver/LvmVg":    s.VolumeGroup,
			"StorDriver/ThinPool": s.ThinVolume,
		},
	}
}

// LinstorSSLConfig is the name of the k8s secret that holds the key (called `keystore.jks`) and
// the trusted certificates (called `certificates.jks`)
type LinstorSSLConfig string

func (lsc *LinstorSSLConfig) IsPlain() bool {
	return lsc == nil || *lsc == ""
}

func (lsc *LinstorSSLConfig) Port() int32 {
	if lsc.IsPlain() {
		return lapiconst.DfltStltPortPlain
	} else {
		return lapiconst.DfltStltPortSsl
	}
}

func (lsc *LinstorSSLConfig) Type() string {
	if lsc.IsPlain() {
		return lapiconst.ValNetcomTypePlain
	} else {
		return lapiconst.ValNetcomTypeSsl
	}
}

type LinstorClientConfig struct {
	// Name of the secret containing:
	// (a) `ca.pem`: root certificate used to validate HTTPS connections with Linstor (PEM format, without password)
	// (b) `client.key`: client key used by the linstor client (PEM format, without password)
	// (c) `client.cert`: client certificate matching the client key (PEM format, without password)
	// If set, HTTPS is used for connecting and authenticating with linstor
	// +optional
	LinstorHttpsClientSecret string `json:"linstorHttpsClientSecret"`
}
