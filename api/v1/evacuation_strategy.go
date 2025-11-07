package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EvacuationStrategy configures the evacuation of volumes from a Satellite when DeletionPolicy "Evacuate" is used.
type EvacuationStrategy struct {
	// AttachedVolumeReattachTimeout configures how long evacuation waits for attached volumes to reattach on
	// different nodes. Setting this to 0 disable this evacuation step.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="5m"
	AttachedVolumeReattachTimeout metav1.Duration `json:"attachedVolumeReattachTimeout"`
	// UnattachedVolumeAttachTimeout configures how long evacuation waits for unattached volumes to attach on
	// different nodes. Setting this to 0 disable this evacuation step.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="5m"
	UnattachedVolumeAttachTimeout metav1.Duration `json:"unattachedVolumeAttachTimeout"`
}
