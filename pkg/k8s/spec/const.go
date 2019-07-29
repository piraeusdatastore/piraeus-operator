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
	DevDir                 = "/dev/"
	DevDirName             = "device-dir"
	LinstorDatabaseDir     = "/var/lib/linstor"
	LinstorDatabaseDirName = "linstor-db"
	ModulesDir             = "/usr/lib/modules/"
	ModulesDirName         = "modules-dir"
	SrcDir                 = "/usr/src"
	SrcDirName             = "src-dir"
	UdevDir                = "/run/udev"
	UdevDirName            = "udev"
)

// Shared consts common to container volumes. These need to be vars, so they
// are addressible.
var (
	HostPathDirectoryType         = corev1.HostPathDirectory
	MountPropagationBidirectional = corev1.MountPropagationBidirectional
)

// Shared consts common to container security. These need to be vars, so they
// are addressible.
var (
	Privileged = true
)

// NodeSelectorKey corresponds to a kubernetes node label that is an opt-in
// selector to run piraeus pods.
const NodeSelectorKey = "linstor.linbit.com/linstor-node-type"
