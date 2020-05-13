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
	DevDir                     = "/dev/"
	DevDirName                 = "device-dir"
	LinstorConfDir             = "/etc/linstor"
	LinstorCertDir             = "/etc/linstor/certs"
	LinstorSslDir              = "/etc/linstor/ssl"
	LinstorConfDirName         = "linstor-conf"
	LinstorCertDirName         = "linstor-certs"
	LinstorSslDirName          = "linstor-ssl"
	ModulesDir                 = "/lib/modules/" // "/usr/lib/modules/"
	ModulesDirName             = "modules-dir"
	SrcDir                     = "/usr/src"
	SrcDirName                 = "src-dir"
	LinstorSatelliteConfigFile = "linstor_satellite.toml"
)

// Special strings for communicating with the module injector
const (
	LinstorKernelModHow            = "LB_HOW"
	LinstorKernelModCompile        = "compile"
	LinstorKernelModShippedModules = "shipped_modules"
)

// Special strings when configuring Linstor
const (
	LinstorLUKSPassphraseEnvName = "MASTER_PASSPHRASE"
)

// Shared consts common to container volumes. These need to be vars, so they
// are addressible.
var (
	HostPathDirectoryType         = corev1.HostPathDirectory
	HostPathDirectoryOrCreateType = corev1.HostPathDirectoryOrCreate
	MountPropagationBidirectional = corev1.MountPropagationBidirectional
)

// Shared consts common to container security. These need to be vars, so they
// are addressible.
var (
	Privileged = true
)
