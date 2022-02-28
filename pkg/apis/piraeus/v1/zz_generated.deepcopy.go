//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"github.com/piraeusdatastore/piraeus-operator/pkg/apis/piraeus/shared"
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorCSIDriver) DeepCopyInto(out *LinstorCSIDriver) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorCSIDriver.
func (in *LinstorCSIDriver) DeepCopy() *LinstorCSIDriver {
	if in == nil {
		return nil
	}
	out := new(LinstorCSIDriver)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorCSIDriver) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorCSIDriverList) DeepCopyInto(out *LinstorCSIDriverList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorCSIDriver, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorCSIDriverList.
func (in *LinstorCSIDriverList) DeepCopy() *LinstorCSIDriverList {
	if in == nil {
		return nil
	}
	out := new(LinstorCSIDriverList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorCSIDriverList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorCSIDriverSpec) DeepCopyInto(out *LinstorCSIDriverSpec) {
	*out = *in
	if in.ControllerReplicas != nil {
		in, out := &in.ControllerReplicas, &out.ControllerReplicas
		*out = new(int32)
		**out = **in
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeTolerations != nil {
		in, out := &in.NodeTolerations, &out.NodeTolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ControllerAffinity != nil {
		in, out := &in.ControllerAffinity, &out.ControllerAffinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerTolerations != nil {
		in, out := &in.ControllerTolerations, &out.ControllerTolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ControllerSidecars != nil {
		in, out := &in.ControllerSidecars, &out.ControllerSidecars
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ControllerExtraVolumes != nil {
		in, out := &in.ControllerExtraVolumes, &out.ControllerExtraVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NodeSidecars != nil {
		in, out := &in.NodeSidecars, &out.NodeSidecars
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NodeExtraVolumes != nil {
		in, out := &in.NodeExtraVolumes, &out.NodeExtraVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.LinstorClientConfig = in.LinstorClientConfig
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorCSIDriverSpec.
func (in *LinstorCSIDriverSpec) DeepCopy() *LinstorCSIDriverSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorCSIDriverSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorCSIDriverStatus) DeepCopyInto(out *LinstorCSIDriverStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorCSIDriverStatus.
func (in *LinstorCSIDriverStatus) DeepCopy() *LinstorCSIDriverStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorCSIDriverStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorController) DeepCopyInto(out *LinstorController) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorController.
func (in *LinstorController) DeepCopy() *LinstorController {
	if in == nil {
		return nil
	}
	out := new(LinstorController)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorController) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorControllerList) DeepCopyInto(out *LinstorControllerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorController, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorControllerList.
func (in *LinstorControllerList) DeepCopy() *LinstorControllerList {
	if in == nil {
		return nil
	}
	out := new(LinstorControllerList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorControllerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorControllerSpec) DeepCopyInto(out *LinstorControllerSpec) {
	*out = *in
	if in.SslConfig != nil {
		in, out := &in.SslConfig, &out.SslConfig
		*out = new(shared.LinstorSSLConfig)
		**out = **in
	}
	in.Resources.DeepCopyInto(&out.Resources)
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Replicas != nil {
		in, out := &in.Replicas, &out.Replicas
		*out = new(int32)
		**out = **in
	}
	if in.AdditionalEnv != nil {
		in, out := &in.AdditionalEnv, &out.AdditionalEnv
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AdditionalProperties != nil {
		in, out := &in.AdditionalProperties, &out.AdditionalProperties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Sidecars != nil {
		in, out := &in.Sidecars, &out.Sidecars
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExtraVolumes != nil {
		in, out := &in.ExtraVolumes, &out.ExtraVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.LinstorClientConfig = in.LinstorClientConfig
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorControllerSpec.
func (in *LinstorControllerSpec) DeepCopy() *LinstorControllerSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorControllerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorControllerStatus) DeepCopyInto(out *LinstorControllerStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ControllerStatus != nil {
		in, out := &in.ControllerStatus, &out.ControllerStatus
		*out = new(shared.NodeStatus)
		**out = **in
	}
	if in.SatelliteStatuses != nil {
		in, out := &in.SatelliteStatuses, &out.SatelliteStatuses
		*out = make([]*shared.SatelliteStatus, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(shared.SatelliteStatus)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.ControllerProperties != nil {
		in, out := &in.ControllerProperties, &out.ControllerProperties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorControllerStatus.
func (in *LinstorControllerStatus) DeepCopy() *LinstorControllerStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorControllerStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteSet) DeepCopyInto(out *LinstorSatelliteSet) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteSet.
func (in *LinstorSatelliteSet) DeepCopy() *LinstorSatelliteSet {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatelliteSet) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteSetList) DeepCopyInto(out *LinstorSatelliteSetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorSatelliteSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteSetList.
func (in *LinstorSatelliteSetList) DeepCopy() *LinstorSatelliteSetList {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteSetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatelliteSetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteSetSpec) DeepCopyInto(out *LinstorSatelliteSetSpec) {
	*out = *in
	if in.StoragePools != nil {
		in, out := &in.StoragePools, &out.StoragePools
		*out = new(shared.StoragePools)
		(*in).DeepCopyInto(*out)
	}
	if in.SslConfig != nil {
		in, out := &in.SslConfig, &out.SslConfig
		*out = new(shared.LinstorSSLConfig)
		**out = **in
	}
	in.Resources.DeepCopyInto(&out.Resources)
	in.KernelModuleInjectionResources.DeepCopyInto(&out.KernelModuleInjectionResources)
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AdditionalEnv != nil {
		in, out := &in.AdditionalEnv, &out.AdditionalEnv
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	out.LinstorClientConfig = in.LinstorClientConfig
	if in.Sidecars != nil {
		in, out := &in.Sidecars, &out.Sidecars
		*out = make([]corev1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExtraVolumes != nil {
		in, out := &in.ExtraVolumes, &out.ExtraVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteSetSpec.
func (in *LinstorSatelliteSetSpec) DeepCopy() *LinstorSatelliteSetSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteSetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteSetStatus) DeepCopyInto(out *LinstorSatelliteSetStatus) {
	*out = *in
	if in.Errors != nil {
		in, out := &in.Errors, &out.Errors
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SatelliteStatuses != nil {
		in, out := &in.SatelliteStatuses, &out.SatelliteStatuses
		*out = make([]*shared.SatelliteStatus, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(shared.SatelliteStatus)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteSetStatus.
func (in *LinstorSatelliteSetStatus) DeepCopy() *LinstorSatelliteSetStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteSetStatus)
	in.DeepCopyInto(out)
	return out
}
