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

package shared

import (
	"fmt"

	lapiconst "github.com/LINBIT/golinstor"
	lapi "github.com/LINBIT/golinstor/client"

	"github.com/piraeusdatastore/piraeus-operator/pkg/k8s/spec"
)

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
func NewStoragePoolStatus(pool *lapi.StoragePool) *StoragePoolStatus {
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
	// LVMPools for LinstorSatelliteSet to manage.
	// +optional
	// +nullable
	LVMPools []*StoragePoolLVM `json:"lvmPools"`

	// LVMThinPools for LinstorSatelliteSet to manage.
	// +optional
	// +nullable
	LVMThinPools []*StoragePoolLVMThin `json:"lvmThinPools"`

	// ZFSPools for LinstorSatelliteSet to manage
	// +optional
	// +nullable
	ZFSPools []*StoragePoolZFS `json:"zfsPools"`
}

func (in *StoragePools) All() []StoragePool {
	all := make([]StoragePool, 0)
	for _, p := range in.LVMPools {
		all = append(all, p)
	}

	for _, p := range in.LVMThinPools {
		all = append(all, p)
	}

	for _, p := range in.ZFSPools {
		all = append(all, p)
	}

	return all
}

func (in *StoragePools) AllPhysicalStorageCreators() []PhysicalStorageCreator {
	all := make([]PhysicalStorageCreator, 0)
	for _, p := range in.LVMPools {
		all = append(all, p)
	}

	for _, p := range in.LVMThinPools {
		all = append(all, p)
	}

	return all
}

// StoragePool is the generalized type of storage pools.
type StoragePool interface {
	GetName() string
	ToLinstorStoragePool() lapi.StoragePool
}

type PhysicalStorageCreator interface {
	StoragePool
	GetDevicePaths() []string
	ToPhysicalStorageCreate() lapi.PhysicalStorageCreate
}

type CommonStoragePoolOptions struct {
	// Name of the storage pool.
	Name string `json:"name"`
}

type CommonPhysicalStorageOptions struct {
	// List of device paths that should make up the VG
	// +optional
	DevicePaths []string `json:"devicePaths,omitempty"`
}

// StoragePoolLVM represents LVM storage pool to be managed by a
// LinstorSatelliteSet
type StoragePoolLVM struct {
	CommonStoragePoolOptions     `json:",inline"`
	CommonPhysicalStorageOptions `json:",inline"`

	// Name of underlying lvm group
	VolumeGroup string `json:"volumeGroup"`

	// Enable the Virtual Data Optimizer (VDO) on the volume group.
	// +optional
	VDO bool `json:"vdo,omitempty"`

	// Set LVM RaidLevel
	// +optional
	RaidLevel string `json:"raidLevel,omitempty"`

	// Set VDO logical volume size
	VdoLogicalSizeKib int32 `json:"vdoLogicalSizeKib,omitempty"`

	// Set VDO slab size
	VdoSlabSizeKib int32 `json:"vdoSlabSizeKib,omitempty"`
}

// StoragePoolLVMThin represents LVM Thin storage pool to be
// managed by a LinstorSatelliteSet.
type StoragePoolLVMThin struct {
	CommonStoragePoolOptions     `json:",inline"`
	CommonPhysicalStorageOptions `json:",inline"`

	// Name of underlying lvm group
	VolumeGroup string `json:"volumeGroup"`

	// Name of underlying lvm thin volume
	ThinVolume string `json:"thinVolume"`

	// Set LVM RaidLevel
	// +optional
	RaidLevel string `json:"raidLevel,omitempty"`
}

//  StoragePoolZFS represents
type StoragePoolZFS struct {
	CommonStoragePoolOptions `json:",inline"`

	// Name of the zpool to use.
	ZPool string `json:"zPool"`

	// use thin provisioning
	Thin bool `json:"thin"`
}

func (in *CommonStoragePoolOptions) GetName() string {
	return in.Name
}

func (in *CommonPhysicalStorageOptions) GetDevicePaths() []string {
	return in.DevicePaths
}

func (in *StoragePoolLVM) props() map[string]string {
	return map[string]string{
		"StorDriver/LvmVg":               in.VolumeGroup,
		spec.LinstorRegistrationProperty: spec.Name,
	}
}

// ToLinstorStoragePool returns lapi.StoragePool presentation of the StoragePoolLVM
func (in *StoragePoolLVM) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: in.Name,
		ProviderKind:    lapi.LVM,
		Props:           in.props(),
	}
}

func (in *StoragePoolLVM) ToPhysicalStorageCreate() lapi.PhysicalStorageCreate {
	return lapi.PhysicalStorageCreate{
		DevicePaths:       in.DevicePaths,
		PoolName:          in.VolumeGroup,
		ProviderKind:      lapi.LVM,
		VdoEnable:         in.VDO,
		RaidLevel:         in.RaidLevel,
		VdoLogicalSizeKib: int64(in.VdoLogicalSizeKib),
		VdoSlabSizeKib:    int64(in.VdoSlabSizeKib),
		WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
			Name:  in.Name,
			Props: in.props(),
		},
	}
}

func (in *StoragePoolLVMThin) CreatedVolumeGroup() string {
	if len(in.DevicePaths) != 0 {
		return fmt.Sprintf("linstor_%s", in.ThinVolume)
	}

	return in.VolumeGroup
}

func (in *StoragePoolLVMThin) props() map[string]string {
	vg := in.CreatedVolumeGroup()
	return map[string]string{
		"StorDriver/LvmVg":               vg,
		"StorDriver/ThinPool":            in.ThinVolume,
		"StorDriver/StorPoolName":        fmt.Sprintf("%s/%s", vg, in.ThinVolume),
		spec.LinstorRegistrationProperty: spec.Name,
	}
}

// ToLinstorStoragePool returns lapi.StoragePool presentation of the StoragePoolLVMThin
func (in *StoragePoolLVMThin) ToLinstorStoragePool() lapi.StoragePool {
	return lapi.StoragePool{
		StoragePoolName: in.Name,
		ProviderKind:    lapi.LVM_THIN,
		Props:           in.props(),
	}
}

func (in *StoragePoolLVMThin) ToPhysicalStorageCreate() lapi.PhysicalStorageCreate {
	return lapi.PhysicalStorageCreate{
		DevicePaths:  in.DevicePaths,
		PoolName:     in.ThinVolume,
		ProviderKind: lapi.LVM_THIN,
		RaidLevel:    in.RaidLevel,
		WithStoragePool: lapi.PhysicalStorageStoragePoolCreate{
			Name:  in.Name,
			Props: in.props(),
		},
	}
}

func (in *StoragePoolZFS) props() map[string]string {
	return map[string]string{
		"StorDriver/StorPoolName":        in.ZPool,
		spec.LinstorRegistrationProperty: spec.Name,
	}
}

func (in *StoragePoolZFS) ToLinstorStoragePool() lapi.StoragePool {
	kind := lapi.ZFS
	if in.Thin {
		kind = lapi.ZFS_THIN
	}

	return lapi.StoragePool{
		StoragePoolName: in.Name,
		ProviderKind:    kind,
		Props:           in.props(),
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
	// (a) `ca.crt`: root certificate used to validate HTTPS connections with Linstor (PEM format, without password)
	// (b) `tls.key`: client key used by the linstor client (PEM format, without password)
	// (c) `tls.crt`: client certificate matching the client key (PEM format, without password)
	// If set, HTTPS is used for connecting and authenticating with linstor
	// +optional
	LinstorHttpsClientSecret string `json:"linstorHttpsClientSecret"`
}

// PriorityClassName is the name PriorityClass associated with all pods for this resource.
// Can be left empty to use the default PriorityClass.
type PriorityClassName string

func (pcn *PriorityClassName) GetName(namespace string) string {
	if *pcn != "" {
		return string(*pcn)
	}

	if namespace == spec.SystemNamespace {
		return spec.SystemCriticalPriorityClassName
	}

	return ""
}

// KernelModuleInjectionMode describes the source for injecting a kernel module
type KernelModuleInjectionMode string

const (
	// ModuleInjectionNone means that no module will be injected
	ModuleInjectionNone = "None"
	// ModuleInjectionCompile means that the module will be compiled from sources available on the host
	ModuleInjectionCompile = "Compile"
	// ModuleInjectionShippedModules means that a module included in the injector image will be used
	ModuleInjectionShippedModules = "ShippedModules"
	// ModuleInjectionDepsOnly means we only inject already present modules on the host for LINSTOR layers
	ModuleInjectionDepsOnly = "DepsOnly"
)

type LogLevel string

const (
	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

func (l LogLevel) ToLinstor() lapi.LogLevel {
	switch l {
	case LogLevelTrace:
		return lapi.TRACE
	case LogLevelDebug:
		return lapi.DEBUG
	case LogLevelInfo:
		return lapi.INFO
	case LogLevelWarn:
		return lapi.WARN
	case LogLevelError:
		return lapi.ERROR
	default:
		// Keep LINSTOR default
		return ""
	}
}
