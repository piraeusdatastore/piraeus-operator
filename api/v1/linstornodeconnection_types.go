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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LinstorNodeConnectionSpec defines the desired state of LinstorNodeConnection
type LinstorNodeConnectionSpec struct {
	// Selector selects which pair of Satellites the connection should apply to.
	// If not given, the connection will be applied to all connections.
	// +kubebuilder:validation:Optional
	Selector []SelectorTerm `json:"selector,omitempty"`

	// Properties to apply for the node connection.
	//
	// Use to create default settings for DRBD that should apply to all resources connections between a set of
	// cluster nodes.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// +patchMergeKey=name
	// +patchStrategy=merge
	Properties []LinstorControllerProperty `json:"properties,omitempty"`

	// Paths configure the network path used when connecting two nodes.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// +patchMergeKey=name
	// +patchStrategy=merge
	Paths []LinstorNodeConnectionPath `json:"paths,omitempty"`
}

// SelectorTerm matches pairs of nodes by checking that the nodes match all specified requirements.
type SelectorTerm struct {
	// MatchLabels is a list of match expressions that the node pairs must meet.
	//+kubebuilder:validation:Required
	MatchLabels []MatchLabelSelector `json:"matchLabels,omitempty"`
}

type MatchLabelSelector struct {
	// Key is the name of a node label.
	//+kubebuilder:validation:MinLength=1
	//+kubebuilder:validation:Required
	Key string `json:"key"`
	// Op to apply to the label.
	// Exists (default) checks for the presence of the label on both nodes in the pair.
	// DoesNotExist checks that the label is not present on either node in the pair.
	// In checks for the presence of the label value given by Values on both nodes in the pair.
	// NotIn checks that both nodes in the pair do not have any of the label values given by Values.
	// Same checks that the label value is equal in the node pair.
	// NotSame checks that the label value is not equal in the node pair.
	// +kubebuilder:default:=Exists
	// +kubebuilder:validation:Enum:=Exists;DoesNotExist;In;NotIn;Same;NotSame
	// +kubebuilder:validation:Optional
	Op MatchLabelSelectorOperator `json:"op,omitempty"`
	// Values to match on, using the provided Op.
	// +kubebuilder:validation:Optional
	Values []string `json:"values,omitempty"`
}

type MatchLabelSelectorOperator string

const (
	MatchLabelSelectorOpExists       MatchLabelSelectorOperator = "Exists"
	MatchLabelSelectorOpDoesNotExist MatchLabelSelectorOperator = "DoesNotExist"
	MatchLabelSelectorOpIn           MatchLabelSelectorOperator = "In"
	MatchLabelSelectorOpNotIn        MatchLabelSelectorOperator = "NotIn"
	MatchLabelSelectorOpSame         MatchLabelSelectorOperator = "Same"
	MatchLabelSelectorOpNotSame      MatchLabelSelectorOperator = "NotSame"
)

type LinstorNodeConnectionPath struct {
	// Name of the path.
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Interface to use on both nodes.
	// +required
	// +kubebuilder:validation:Required
	Interface string `json:"interface"`
}

// LinstorNodeConnectionStatus defines the observed state of LinstorNodeConnection
type LinstorNodeConnectionStatus struct {
	// Current LINSTOR Node Connection state
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// LinstorNodeConnection is the Schema for the linstornodeconnections API
type LinstorNodeConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LinstorNodeConnectionSpec   `json:"spec,omitempty"`
	Status LinstorNodeConnectionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LinstorNodeConnectionList contains a list of LinstorNodeConnection
type LinstorNodeConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LinstorNodeConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LinstorNodeConnection{}, &LinstorNodeConnectionList{})
}
