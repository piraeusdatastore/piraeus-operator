//go:build !ignore_autogenerated

/*
Copyright 2022.

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
	"encoding/json"
	metav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	apismetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterReference) DeepCopyInto(out *ClusterReference) {
	*out = *in
	if in.ExternalController != nil {
		in, out := &in.ExternalController, &out.ExternalController
		*out = new(LinstorExternalControllerRef)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterReference.
func (in *ClusterReference) DeepCopy() *ClusterReference {
	if in == nil {
		return nil
	}
	out := new(ClusterReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ComponentSpec) DeepCopyInto(out *ComponentSpec) {
	*out = *in
	if in.PodTemplate != nil {
		in, out := &in.PodTemplate, &out.PodTemplate
		*out = make(json.RawMessage, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ComponentSpec.
func (in *ComponentSpec) DeepCopy() *ComponentSpec {
	if in == nil {
		return nil
	}
	out := new(ComponentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorCluster) DeepCopyInto(out *LinstorCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorCluster.
func (in *LinstorCluster) DeepCopy() *LinstorCluster {
	if in == nil {
		return nil
	}
	out := new(LinstorCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorClusterApiTLS) DeepCopyInto(out *LinstorClusterApiTLS) {
	*out = *in
	if in.CertManager != nil {
		in, out := &in.CertManager, &out.CertManager
		*out = new(metav1.ObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorClusterApiTLS.
func (in *LinstorClusterApiTLS) DeepCopy() *LinstorClusterApiTLS {
	if in == nil {
		return nil
	}
	out := new(LinstorClusterApiTLS)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorClusterList) DeepCopyInto(out *LinstorClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorClusterList.
func (in *LinstorClusterList) DeepCopy() *LinstorClusterList {
	if in == nil {
		return nil
	}
	out := new(LinstorClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorClusterSpec) DeepCopyInto(out *LinstorClusterSpec) {
	*out = *in
	if in.ExternalController != nil {
		in, out := &in.ExternalController, &out.ExternalController
		*out = new(LinstorExternalControllerRef)
		**out = **in
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(corev1.NodeSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make([]LinstorControllerProperty, len(*in))
		copy(*out, *in)
	}
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InternalTLS != nil {
		in, out := &in.InternalTLS, &out.InternalTLS
		*out = new(TLSConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ApiTLS != nil {
		in, out := &in.ApiTLS, &out.ApiTLS
		*out = new(LinstorClusterApiTLS)
		(*in).DeepCopyInto(*out)
	}
	if in.Controller != nil {
		in, out := &in.Controller, &out.Controller
		*out = new(ComponentSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.CSIController != nil {
		in, out := &in.CSIController, &out.CSIController
		*out = new(ComponentSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.CSINode != nil {
		in, out := &in.CSINode, &out.CSINode
		*out = new(ComponentSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.HighAvailabilityController != nil {
		in, out := &in.HighAvailabilityController, &out.HighAvailabilityController
		*out = new(ComponentSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorClusterSpec.
func (in *LinstorClusterSpec) DeepCopy() *LinstorClusterSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorClusterStatus) DeepCopyInto(out *LinstorClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]apismetav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorClusterStatus.
func (in *LinstorClusterStatus) DeepCopy() *LinstorClusterStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorControllerProperty) DeepCopyInto(out *LinstorControllerProperty) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorControllerProperty.
func (in *LinstorControllerProperty) DeepCopy() *LinstorControllerProperty {
	if in == nil {
		return nil
	}
	out := new(LinstorControllerProperty)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorExternalControllerRef) DeepCopyInto(out *LinstorExternalControllerRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorExternalControllerRef.
func (in *LinstorExternalControllerRef) DeepCopy() *LinstorExternalControllerRef {
	if in == nil {
		return nil
	}
	out := new(LinstorExternalControllerRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeConnection) DeepCopyInto(out *LinstorNodeConnection) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeConnection.
func (in *LinstorNodeConnection) DeepCopy() *LinstorNodeConnection {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorNodeConnection) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeConnectionList) DeepCopyInto(out *LinstorNodeConnectionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorNodeConnection, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeConnectionList.
func (in *LinstorNodeConnectionList) DeepCopy() *LinstorNodeConnectionList {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeConnectionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorNodeConnectionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeConnectionPath) DeepCopyInto(out *LinstorNodeConnectionPath) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeConnectionPath.
func (in *LinstorNodeConnectionPath) DeepCopy() *LinstorNodeConnectionPath {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeConnectionPath)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeConnectionSpec) DeepCopyInto(out *LinstorNodeConnectionSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make([]SelectorTerm, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make([]LinstorControllerProperty, len(*in))
		copy(*out, *in)
	}
	if in.Paths != nil {
		in, out := &in.Paths, &out.Paths
		*out = make([]LinstorNodeConnectionPath, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeConnectionSpec.
func (in *LinstorNodeConnectionSpec) DeepCopy() *LinstorNodeConnectionSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeConnectionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeConnectionStatus) DeepCopyInto(out *LinstorNodeConnectionStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]apismetav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeConnectionStatus.
func (in *LinstorNodeConnectionStatus) DeepCopy() *LinstorNodeConnectionStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeConnectionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodeProperty) DeepCopyInto(out *LinstorNodeProperty) {
	*out = *in
	if in.ValueFrom != nil {
		in, out := &in.ValueFrom, &out.ValueFrom
		*out = new(LinstorNodePropertyValueFrom)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodeProperty.
func (in *LinstorNodeProperty) DeepCopy() *LinstorNodeProperty {
	if in == nil {
		return nil
	}
	out := new(LinstorNodeProperty)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorNodePropertyValueFrom) DeepCopyInto(out *LinstorNodePropertyValueFrom) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorNodePropertyValueFrom.
func (in *LinstorNodePropertyValueFrom) DeepCopy() *LinstorNodePropertyValueFrom {
	if in == nil {
		return nil
	}
	out := new(LinstorNodePropertyValueFrom)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatellite) DeepCopyInto(out *LinstorSatellite) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatellite.
func (in *LinstorSatellite) DeepCopy() *LinstorSatellite {
	if in == nil {
		return nil
	}
	out := new(LinstorSatellite)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatellite) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteConfiguration) DeepCopyInto(out *LinstorSatelliteConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteConfiguration.
func (in *LinstorSatelliteConfiguration) DeepCopy() *LinstorSatelliteConfiguration {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatelliteConfiguration) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteConfigurationList) DeepCopyInto(out *LinstorSatelliteConfigurationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorSatelliteConfiguration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteConfigurationList.
func (in *LinstorSatelliteConfigurationList) DeepCopy() *LinstorSatelliteConfigurationList {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteConfigurationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatelliteConfigurationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteConfigurationSpec) DeepCopyInto(out *LinstorSatelliteConfigurationSpec) {
	*out = *in
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(corev1.NodeSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.StoragePools != nil {
		in, out := &in.StoragePools, &out.StoragePools
		*out = make([]LinstorStoragePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make([]LinstorNodeProperty, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InternalTLS != nil {
		in, out := &in.InternalTLS, &out.InternalTLS
		*out = new(TLSConfigWithHandshakeDaemon)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteConfigurationSpec.
func (in *LinstorSatelliteConfigurationSpec) DeepCopy() *LinstorSatelliteConfigurationSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteConfigurationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteConfigurationStatus) DeepCopyInto(out *LinstorSatelliteConfigurationStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]apismetav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteConfigurationStatus.
func (in *LinstorSatelliteConfigurationStatus) DeepCopy() *LinstorSatelliteConfigurationStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteConfigurationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteList) DeepCopyInto(out *LinstorSatelliteList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]LinstorSatellite, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteList.
func (in *LinstorSatelliteList) DeepCopy() *LinstorSatelliteList {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *LinstorSatelliteList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteSpec) DeepCopyInto(out *LinstorSatelliteSpec) {
	*out = *in
	in.ClusterRef.DeepCopyInto(&out.ClusterRef)
	if in.Patches != nil {
		in, out := &in.Patches, &out.Patches
		*out = make([]Patch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.StoragePools != nil {
		in, out := &in.StoragePools, &out.StoragePools
		*out = make([]LinstorStoragePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make([]LinstorNodeProperty, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InternalTLS != nil {
		in, out := &in.InternalTLS, &out.InternalTLS
		*out = new(TLSConfigWithHandshakeDaemon)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteSpec.
func (in *LinstorSatelliteSpec) DeepCopy() *LinstorSatelliteSpec {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorSatelliteStatus) DeepCopyInto(out *LinstorSatelliteStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]apismetav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorSatelliteStatus.
func (in *LinstorSatelliteStatus) DeepCopy() *LinstorSatelliteStatus {
	if in == nil {
		return nil
	}
	out := new(LinstorSatelliteStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorStoragePool) DeepCopyInto(out *LinstorStoragePool) {
	*out = *in
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make([]LinstorNodeProperty, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LvmPool != nil {
		in, out := &in.LvmPool, &out.LvmPool
		*out = new(LinstorStoragePoolLvm)
		**out = **in
	}
	if in.LvmThinPool != nil {
		in, out := &in.LvmThinPool, &out.LvmThinPool
		*out = new(LinstorStoragePoolLvmThin)
		**out = **in
	}
	if in.FilePool != nil {
		in, out := &in.FilePool, &out.FilePool
		*out = new(LinstorStoragePoolFile)
		**out = **in
	}
	if in.FileThinPool != nil {
		in, out := &in.FileThinPool, &out.FileThinPool
		*out = new(LinstorStoragePoolFile)
		**out = **in
	}
	if in.Source != nil {
		in, out := &in.Source, &out.Source
		*out = new(LinstorStoragePoolSource)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorStoragePool.
func (in *LinstorStoragePool) DeepCopy() *LinstorStoragePool {
	if in == nil {
		return nil
	}
	out := new(LinstorStoragePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorStoragePoolFile) DeepCopyInto(out *LinstorStoragePoolFile) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorStoragePoolFile.
func (in *LinstorStoragePoolFile) DeepCopy() *LinstorStoragePoolFile {
	if in == nil {
		return nil
	}
	out := new(LinstorStoragePoolFile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorStoragePoolLvm) DeepCopyInto(out *LinstorStoragePoolLvm) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorStoragePoolLvm.
func (in *LinstorStoragePoolLvm) DeepCopy() *LinstorStoragePoolLvm {
	if in == nil {
		return nil
	}
	out := new(LinstorStoragePoolLvm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorStoragePoolLvmThin) DeepCopyInto(out *LinstorStoragePoolLvmThin) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorStoragePoolLvmThin.
func (in *LinstorStoragePoolLvmThin) DeepCopy() *LinstorStoragePoolLvmThin {
	if in == nil {
		return nil
	}
	out := new(LinstorStoragePoolLvmThin)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LinstorStoragePoolSource) DeepCopyInto(out *LinstorStoragePoolSource) {
	*out = *in
	if in.HostDevices != nil {
		in, out := &in.HostDevices, &out.HostDevices
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LinstorStoragePoolSource.
func (in *LinstorStoragePoolSource) DeepCopy() *LinstorStoragePoolSource {
	if in == nil {
		return nil
	}
	out := new(LinstorStoragePoolSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatchLabelSelector) DeepCopyInto(out *MatchLabelSelector) {
	*out = *in
	if in.Values != nil {
		in, out := &in.Values, &out.Values
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatchLabelSelector.
func (in *MatchLabelSelector) DeepCopy() *MatchLabelSelector {
	if in == nil {
		return nil
	}
	out := new(MatchLabelSelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Patch) DeepCopyInto(out *Patch) {
	*out = *in
	if in.Target != nil {
		in, out := &in.Target, &out.Target
		*out = new(Selector)
		**out = **in
	}
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]bool, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Patch.
func (in *Patch) DeepCopy() *Patch {
	if in == nil {
		return nil
	}
	out := new(Patch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Selector) DeepCopyInto(out *Selector) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Selector.
func (in *Selector) DeepCopy() *Selector {
	if in == nil {
		return nil
	}
	out := new(Selector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SelectorTerm) DeepCopyInto(out *SelectorTerm) {
	*out = *in
	if in.MatchLabels != nil {
		in, out := &in.MatchLabels, &out.MatchLabels
		*out = make([]MatchLabelSelector, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SelectorTerm.
func (in *SelectorTerm) DeepCopy() *SelectorTerm {
	if in == nil {
		return nil
	}
	out := new(SelectorTerm)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSConfig) DeepCopyInto(out *TLSConfig) {
	*out = *in
	if in.CertManager != nil {
		in, out := &in.CertManager, &out.CertManager
		*out = new(metav1.ObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSConfig.
func (in *TLSConfig) DeepCopy() *TLSConfig {
	if in == nil {
		return nil
	}
	out := new(TLSConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TLSConfigWithHandshakeDaemon) DeepCopyInto(out *TLSConfigWithHandshakeDaemon) {
	*out = *in
	in.TLSConfig.DeepCopyInto(&out.TLSConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TLSConfigWithHandshakeDaemon.
func (in *TLSConfigWithHandshakeDaemon) DeepCopy() *TLSConfigWithHandshakeDaemon {
	if in == nil {
		return nil
	}
	out := new(TLSConfigWithHandshakeDaemon)
	in.DeepCopyInto(out)
	return out
}
