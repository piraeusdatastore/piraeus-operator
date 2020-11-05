// +build !custom

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

const (
	// APIGroup specifies the group for the custom resource definition APIs
	APIGroup = "piraeus.linbit.com"

	// ControllerRole is the role for the controller set
	ControllerRole = "piraeus-controller"

	// NodeRole is the role for the node set
	NodeRole = "piraeus-node"

	// CSIControllerRole is the role for the CSI Controller Workloads
	CSIControllerRole = "csi-controller"

	// CSINodeRole is the role for the CSI Node Workloads
	CSINodeRole = "csi-node"

	// Name is the name of the operator
	Name = "piraeus-operator"

	// LockName is the name of the lock for leader election
	LockName = "piraeus-operator-lock"
)
