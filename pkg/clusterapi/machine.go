package clusterapi

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

type Machine = clusterapiv1beta1.Machine

const (
	SatelliteAllowedToBeDrainedEvent = "SatelliteAllowedToBeDrained"
	SatelliteAllowedToBeRemovedEvent = "SatelliteAllowedToBeRemoved"
)

// GetMachineForNode tries to get the Machine that controls the Node.
// Returns "nil, nil" if:
// * ClusterAPI is not deployed in this cluster
// * the Node is not managed by ClusterAPI
// * the Machine could not be found
func (cl *Client) GetMachineForNode(ctx context.Context, node *corev1.Node) (*Machine, error) {
	clusterNs := node.Annotations[clusterapiv1beta1.ClusterNamespaceAnnotation]
	machineName := node.Annotations[clusterapiv1beta1.MachineAnnotation]

	if cl == nil || clusterNs == "" || machineName == "" {
		return nil, nil
	}

	var machine clusterapiv1beta1.Machine
	err := cl.client.Get(ctx, client.ObjectKey{Name: machineName, Namespace: clusterNs}, &machine)
	if err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}

		return nil, err
	}

	return &machine, nil
}

// ShouldEvacuateNode returns true if the machine is being deleted
func ShouldEvacuateNode(machine *Machine) bool {
	return machine != nil && machine.GetDeletionTimestamp() != nil
}

// PreventMachineDeletion sets hooks that prevent ClusterAPI from terminating the node and thus the Satellite.
// The hooks are only set when the Machine is not already in the process of being deleted.
func (cl *Client) PreventMachineDeletion(ctx context.Context, machine *Machine) error {
	if cl == nil || machine == nil {
		return nil
	}

	if machine.GetDeletionTimestamp() != nil {
		return nil
	}

	_, hasPreDrainHookAnnotation := machine.Annotations[vars.MachinePreDrainHookAnnotation]
	_, hasPreTerminateHookAnnotation := machine.Annotations[vars.MachinePreTerminateHookAnnotation]
	if !hasPreDrainHookAnnotation || !hasPreTerminateHookAnnotation {
		if machine.Annotations == nil {
			machine.Annotations = make(map[string]string)
		}
		machine.Annotations[vars.MachinePreDrainHookAnnotation] = ""
		machine.Annotations[vars.MachinePreTerminateHookAnnotation] = ""
		return cl.client.Update(ctx, machine)
	}

	return nil
}

// AllowMachineDrain removes the annotation that prevents draining the Node.
func (cl *Client) AllowMachineDrain(ctx context.Context, recorder record.EventRecorder, machine *Machine) error {
	if cl == nil || machine == nil {
		return nil
	}

	_, ok := machine.Annotations[vars.MachinePreDrainHookAnnotation]
	if ok {
		cl.MachineEventf(recorder, machine, corev1.EventTypeNormal, SatelliteAllowedToBeDrainedEvent, "Satellite is ready to be drained")

		delete(machine.Annotations, vars.MachinePreDrainHookAnnotation)
		err := cl.client.Update(ctx, machine)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// AllowMachineTermination removes the annotation that prevents removal of the Node.
func (cl *Client) AllowMachineTermination(ctx context.Context, recorder record.EventRecorder, machine *Machine) error {
	if cl == nil || machine == nil {
		return nil
	}

	_, ok := machine.Annotations[vars.MachinePreTerminateHookAnnotation]
	if ok {
		cl.MachineEventf(recorder, machine, corev1.EventTypeNormal, SatelliteAllowedToBeRemovedEvent, "All volumes evacuated, Satellite is ready to be removed")

		delete(machine.Annotations, vars.MachinePreTerminateHookAnnotation)
		err := cl.client.Update(ctx, machine)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// MachineEventf records an event for the Machine.
// The default scheme used by record.EventRecorder does not recognize a Machine, so we manually convert it to a
// reference first.
func (cl *Client) MachineEventf(recorder record.EventRecorder, machine *Machine, eventtype, reason, messageFmt string, args ...interface{}) {
	ref, err := reference.GetReference(cl.client.Scheme(), machine)
	if err == nil {
		recorder.Eventf(ref, eventtype, reason, messageFmt, args...)
	}
}
