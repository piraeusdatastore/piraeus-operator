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
	DevDir             = "/dev/"
	DevDirName         = "device-dir"
	LinstorConfDir     = "/etc/linstor"
	LinstorConfDirName = "linstor-conf"
	ModulesDir         = "/lib/modules/" // "/usr/lib/modules/"
	ModulesDirName     = "modules-dir"
	SrcDir             = "/usr/src"
	SrcDirName         = "src-dir"
	UdevDir            = "/run/udev"
	UdevDirName        = "udev"
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

const selectorPrefix = "linstor.linbit.com/"

// Kubernetes node labels that are an opt-in selector to run piraeus pods when
// set to "true".
const (
	// PiraeusSatelliteNode label to mark node eligible to run piraeus-node pods.
	PiraeusNode = selectorPrefix + "piraeus-node"
)

// PiraeusPriorityClassName is the name of the PriorityClass set up in the
// example yaml used by important piraeus components.
const PiraeusCSPriorityClassName = "piraeus-cs-priority-class"
const PiraeusNSPriorityClassName = "piraeus-ns-priority-class"

const (
	// PiraeusServerImage is the repo/tag for piraeus-server. The controller
	// and node use the same image with different commands.
	PiraeusServerImage = "quay.io/piraeusdatastore/piraeus-server"
	// PiraeusVersion must must match exactly in the ControllerSet and the NodeSet
	// since the linstor controller and satellite versions must also match exactly.
	PiraeusVersion = "v1.4.2"

	// PiraeusSatelliteImage is the repo/tag for LINSTOR Satellite contaier.
	PiraeusSatelliteImage = "quay.io/piraeusdatastore/piraeus-server"
	// PiraeusSatelliteVersion is the release tag for the above image
	PiraeusSatelliteVersion = "v1.4.2"

	// PiraeusKernelModImage is the worker (aka satellite) image for each node
	PiraeusKernelModImage = "quay.io/piraeusdatastore/drbd9-centos7"
	// PiraeusKernelModVersion is the release tag for the above image
	PiraeusKernelModVersion = "v9.0.21"

	// DrbdRepoCred is the name of the kubernetes secret that holds the repo
	// credentials for the DRBD related repositories
	DrbdRepoCred = "drbdiocred"
)
