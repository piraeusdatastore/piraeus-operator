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

package spec

import corev1 "k8s.io/api/core/v1"

// Shared consts common to container volumes.
const (
	DevDir                      = "/dev/"
	DevDirName                  = "device-dir"
	LinstorConfDir              = "/etc/linstor"
	LinstorCertDir              = "/etc/linstor/certs"
	LinstorClientDir            = "/etc/linstor/client"
	LinstorClientDirName        = "linstor-client"
	LinstorHttpsCertDir         = "/etc/linstor/https"
	LinstorHttpsCertDirName     = "linstor-https"
	LinstorHttpsCertPassword    = "linstor"
	LinstorHttpsCertPemDir      = "/etc/linstor/https-pem"
	LinstorHttpsCertPemDirName  = "linstor-https-pem"
	LinstorSslDir               = "/etc/linstor/ssl"
	LinstorSslDirName           = "linstor-ssl"
	LinstorSslPemDir            = "/etc/linstor/ssl-pem"
	LinstorSslPemDirName        = "linstor-ssl-pem"
	LinstorConfDirName          = "linstor-conf"
	LinstorCertDirName          = "linstor-certs"
	ModulesDir                  = "/lib/modules/" // "/usr/lib/modules/"
	ModulesDirName              = "modules-dir"
	UsrLibDir                   = "/usr/lib"
	UsrLibDirMountPath          = "/host/usr/lib"
	UsrLibDirName               = "usr-lib-dir"
	SrcDir                      = "/usr/src"
	SrcDirName                  = "src-dir"
	SysDir                      = "/sys/"
	SysDirName                  = "sys-dir"
	LinstorControllerConfigFile = "linstor.toml"
	LinstorSatelliteConfigFile  = "linstor_satellite.toml"
	LinstorClientConfigFile     = "linstor-client.conf"
	LinstorControllerGID        = 1000
	DrbdPrometheuscConfName     = "drbd-reactor-config"
	MonitorungPortNumber        = 9942
	MonitoringPortName          = "prometheus"
)

// Special strings for communicating with the module injector
const (
	LinstorKernelModHow                = "LB_HOW"
	LinstorKernelModCompile            = "compile"
	LinstorKernelModShippedModules     = "shipped_modules"
	LinstorKernelModDepsOnly           = "deps_only"
	LinstorKernelModHelperCheck        = "LB_FAIL_IF_USERMODE_HELPER_NOT_DISABLED"
	LinstorKernelModHelperCheckEnabled = "yes"
)

// Special strings when configuring Linstor
const (
	LinstorLUKSPassphraseEnvName = "MASTER_PASSPHRASE"
	JavaOptsName                 = "JAVA_OPTS"
	LinstorRegistrationProperty  = "Aux/registered-by"
)

// k8s constants: Special names for k8s APIs.
const (
	SystemNamespace                 = "kube-system"
	SystemCriticalPriorityClassName = "system-node-critical"
	DefaultTopologyKey              = "kubernetes.io/hostname"
	LinstorControllerServiceAccount = "linstor-controller"
	LinstorSatelliteServiceAccount  = "linstor-satellite"
)

// ocp: special environment variables for certified operators
const (
	ImageLinstorControllerEnv     = "RELATED_IMAGE_LINSTOR_CONTROLLER"
	ImageLinstorSatelliteEnv      = "RELATED_IMAGE_LINSTOR_SATELLITE"
	ImageKernelModuleInjectionEnv = "RELATED_IMAGE_KERNEL_MODULE_INJECTION"
	ImageMonitoringEnv            = "RELATED_IMAGE_MONITORING"
	ImageCSIPluginEnv             = "RELATED_IMAGE_CSI_PLUGIN"
	ImageCSIAttacherEnv           = "RELATED_IMAGE_CSI_ATTACHER"
	ImageCSILivenessProbeEnv      = "RELATED_IMAGE_CSI_LIVENESSPROBE"
	ImageCSINodeRegistrarEnv      = "RELATED_IMAGE_CSI_NODE_REGISTRAR"
	ImageCSIProvisionerEnv        = "RELATED_IMAGE_CSI_PROVISIONER"
	ImageCSISnapshotterEnv        = "RELATED_IMAGE_CSI_SNAPSHOTTER"
	ImageCSIResizerEnv            = "RELATED_IMAGE_CSI_RESIZER"
)

// Shared consts common to container volumes. These need to be vars, so they
// are addressible.
var (
	HostPathDirectoryType         = corev1.HostPathDirectory
	HostPathDirectoryOrCreateType = corev1.HostPathDirectoryOrCreate
	MountPropagationBidirectional = corev1.MountPropagationBidirectional
	FileType = corev1.HostPathFile
)

// Shared consts common to container security. These need to be vars, so they
// are addressible.
var (
	Privileged = true
)
