# How to Use the Host Network for DRBD Replication

This guide shows you how to use the host network for DRBD® replication.

By default, DRBD will use the container network to replicate volume data. This ensures replication works on a wide
range of clusters without further configuration. It also enables use of `NetworkPolicy` to block unauthorized access
to DRBD traffic. Since the network interface of the Pod is tied to the lifecycle of the Pod, it also means DRBD will
temporarily disrupt replication when the LINSTOR® Satellite Pod is restarted.

In contrast, using the host network for DRBD replication will cause replication to work independent of the LINSTOR
Satellite Pod. The host network might also offer better performance than the container network. As a downside, you
will have to manually ensure connectivity between Nodes on the relevant ports.

To follow the steps in this guide, you should be familiar with editing `LinstorSatelliteConfiguration` resources.

## Switch from Container Network to Host Network

Switching from the default container network to host network is possible at any time. Existing DRBD resources will be
reconfigured to use the host network interface.

To configure the host network for the LINSTOR Satellite, apply the following configuration:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: host-network
spec:
  podTemplate:
    spec:
      hostNetwork: true
```

After the Satellite Pods are recreated, they will use the host network. Any existing DRBD resources are reconfigured
to use the new IP Address instead.

## Switch from Host Network to Container Network

Switching back from host network to container network involves manually resetting the configured peer addresses used by
DRBD. This can either be achieved by rebooting every node, or by manually resetting the address using `drbdadm`.

### Switch from Host Network to Container Network Using Reboots

First, you need to remove the `LinstorSatelliteConfiguration` that set `hostNetwork: true`:

```
$ kubectl delete linstorsatelliteconfigurations.piraeus.io host-network
linstorsatelliteconfiguration.piraeus.io "host-network" deleted
```

Then, reboot each cluster node, either one by one or multiple at once. In general, replication will not work between
rebooted nodes and non-rebooted nodes. The non-rebooted nodes will continue to use the host network addresses, which
are generally not reachable from the container network.

After all nodes are rebooted, all resources are configured to use the container network, and all DRBD connections
should be connected again.

### Switch from Host Network to Container Network Using `drbdadm`

During this procedure, ensure no new volumes or snapshots are created: otherwise the migration to the
container network might not be applied to all resources.

First, you need to temporarily stop all replication and suspend all DRBD volumes using `drbdadm suspend-io all`.
The command is executed once on each LINSTOR Satellite Pod.

```
$ kubectl exec node1.example.com -- drbdadm suspend-io all
$ kubectl exec node2.example.com -- drbdadm suspend-io all
$ kubectl exec node3.example.com -- drbdadm suspend-io all
```

Next, you will need to disconnect all DRBD connections on all nodes.

```
$ kubectl exec node1.example.com -- drbdadm disconnect --force all
$ kubectl exec node2.example.com -- drbdadm disconnect --force all
$ kubectl exec node3.example.com -- drbdadm disconnect --force all
```

Now, we can safely reset all DRBD connection paths, which frees the connection to be moved to the container network.

```
$ kubectl exec node1.example.com -- drbdadm del-path all
$ kubectl exec node2.example.com -- drbdadm del-path all
$ kubectl exec node3.example.com -- drbdadm del-path all
```

Finally, removing the `LinstorSatelliteConfiguration` that set `hostNetwork: true` will trigger the creation of new
LINSTOR Satellite Pods using the container network:

```
$ kubectl delete linstorsatelliteconfigurations.piraeus.io host-network
linstorsatelliteconfiguration.piraeus.io "host-network" deleted
```

After the Pods are recreated and the LINSTOR Satellites are `Online`, the DRBD resource will be reconfigured and resume
IO.
