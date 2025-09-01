package clusterapi

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1" //nolint:staticcheck
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

type Machine = clusterapiv1beta1.Machine

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
		if errors.IsNotFound(err) {
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
func (cl *Client) AllowMachineDrain(ctx context.Context, machine *Machine) error {
	if cl == nil || machine == nil {
		return nil
	}

	_, ok := machine.Annotations[vars.MachinePreDrainHookAnnotation]
	if ok {
		delete(machine.Annotations, vars.MachinePreDrainHookAnnotation)
		err := cl.client.Update(ctx, machine)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// AllowMachineTermination removes the annotation that prevents removal of the Node.
func (cl *Client) AllowMachineTermination(ctx context.Context, machine *Machine) error {
	if cl == nil || machine == nil {
		return nil
	}

	_, ok := machine.Annotations[vars.MachinePreTerminateHookAnnotation]
	if ok {
		delete(machine.Annotations, vars.MachinePreTerminateHookAnnotation)
		err := cl.client.Update(ctx, machine)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
