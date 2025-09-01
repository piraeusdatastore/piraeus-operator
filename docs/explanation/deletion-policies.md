# Understanding Satellite Deletion Policies

Piraeus Datastore supports different policies to handle removal of LINSTOR Satellites. Satellites are the local
configuration agent for LINSTOR managed storage. As such, their core responsibility is managing (one copy of) the
volume data. If a Satellite hosting the last copy of a volume is removed, that volume is rendered inaccessible and
data is likely lost.

## When are Satellites Deleted

Piraeus Datastore manages LINSTOR Satellites by creating [`LinstorSatellite`](../reference/linstorsatellite.md) resource
for every Satellite. From this resource, node resources, such as the `linstor-satellite` DaemonSet are spawned, the
Satellite is registered with the LINSTOR Controller and Storage Pools are configured.

Piraeus Datastore will create a `LinstorSatellite` resource for every Kubernetes `Node` that:

* Matches the [`nodeSelector`](../reference/linstorcluster.md#specnodeselector) on the `LinstorCluster` resource.
* Matches the [`nodeAffinity`](../reference/linstorcluster.md#specnodeaffinity) on the `LinstorCluster` resource.
* Tolerates the Node taints based on the [`tolerations`](../reference/linstorcluster.md#spectolerations) on the `LinstorCluster` resource.

An existing `LinstorSatellite` is deleted by when:

* The Node no longer matches the `nodeSelector`, either because the selector was updated or the Node labels changed.
* The Node no longer matches the `nodeAffinity`, either because the selector was updated or the Node labels changed.
* The Node has taints with effect `NoExecute` that are not tolerated, either because the Node taints changed or the
  tolerations changed.

In addition to the above reasons, a `LinstorSatellite` resource can also be manually deleted using the Kubernetes API,
for example by using `kubectl delete linstorsatellite <name>`. If the Node is matching the criteria listed above,
Piraeus Datastore will recreate it after deletion.

## Satellite Deletion Policies

Deletion policies govern the behaviour of Piraeus Datastore when a LINSTOR Satellite is to be deleted. There are
currently three deletion policies implemented:

* [`Retain`](#retain-policy) (Default): Piraeus Datastore will keep the Satellite registered, but remove
  the associated resources from Kubernetes.
* [`Evacuate`](#evacuate-policy): Piraeus Datastore will initiate an evacuation of the LINSTOR Satellite, ensuring
  the configured number of replicas is available after the Satellite is removed.
* [`Delete`](#delete-policy): Piraeus Datastore will forcefully remove the Satellite from LINSTOR, ignoring any safety
  checks, removing potentially the last replicas of volumes and snapshots.

Policies can be set by creating a [`LinstorSatelliteConfiguration`](../reference/linstorsatelliteconfiguration.md#specdeletionpolicy)
and setting the `spec.deletionPolicy` value.

A detailed description is given below:

### `Retain` Policy

When using the default `Retain` policy, the associated Kubernetes resources of the Satellite are removed without
removing the Satellite from the LINSTOR Cluster. This will cause the Satellite appear as OFFLINE in LINSTOR.

LINSTOR will keep the offline Satellite indefinitely. Resources assigned to the Satellite will still appear in the list
of LINSTOR resources and LINSTOR will keep necessary information such as device paths in its database to restore the
Satellite when it is recreated.

This is the default policy, as it is the least disruptive policies in Clusters that have a fixed set of Nodes, or
clusters where Nodes are only added but not removed.

### `Evacuate` Policy

Using the `Evacuate` deletion policy, Piraeus Datastore will first ensure the configured number of replicas for all
volumes remains available before removing the Satellite. The Satellite is only removed after all local replicas and
snapshots are removed.

In order to keep potential fail-over times short, Piraeus Datastore performs the following steps during evacuation:

1. Piraeus Datastore searches for PersistentVolumes (PVs) that:
    *   Are attached on the Node to be evacuated and marks them with `wait-for-reattach.evacuation.piraeus.io/<node-name>`
        annotation. It will later use this annotation to wait for the PV to be reattached.
    *   If the PV has a `linstor.csi.linbit.com/evacuation-action` annotation or the storage class has the
        `property.linstor.csi.linbit.com/Aux/linstor.csi.linbit.com/evacuation-action` parameter set, the value is used
        to determine the evacuation action. Possible actions are:
        * `AffinityOverride`: Annotate the PV with `override.piraeus.io/<nodename>`. This causes the LINSTOR Affinity
          Controller to temporarily mark the PV as accessible from any Node.
        * `Delete`: Delete the PVC and PV, causing the **existing data** to be **lost**. This is _only_ useful for PVCs
          storing caches or otherwise ephemeral data that get recreated automatically, for example using a StatefulSet
          with `volumeClaimTemplates`.
        * `None`: No special action is taken to ensure the PV can be evacuated.

        If no action is specified, PVs using the `allowRemoteVolumeAccess: "false"` policy, or PVs that are otherwise
        bound to the Node to be evacuated in their `.spec.nodeAffinity`, default to using the `AffinityOverride` action.
        All other PVs use the `None` action.

2. Piraeus Datastore will wait for all other `LinstorSatellite` resources to become available. This ensures that during
   rolling upgrades of the infrastructure, it waits until a replacement node for the node to be evacuated is available.
3. If using ClusterAPI, the `pre-drain.delete.hook.machine.cluster.x-k8s.io/linstor-prepare-for-drain` annotation on the
   Machine is removed. This will cause ClusterAPI to start draining the node.
4. Piraeus Datastore will wait for all PVs that were marked in Step 1 to be reattached on new nodes. This ensures that
   LINSTOR can choose the optimal replica placement for the next step.
5. Piraeus Datastore will signal LINSTOR to evacuate the node. LINSTOR will create replacement resources for all diskful
   and diskless resources on the node. Volumes that are currently in use on a diskless will get a local replica, if that
   is possible based on the available storage pools.
6. Piraeus Datastore will wait for LINSTOR to complete the evacuation. The evacuated Node no longer has any resources
   or snapshots.
7. All temporary annotations from PVs are removed. In particular, any temporary override that allowed a volume to be
   attached on a node violating the usual access policies is removed.
8. If using ClusterAPI, the `pre-terminate.delete.hook.machine.cluster.x-k8s.io/linstor-wait-for-complete-evacuation`
   annotation on the Machine is removed, letting ClusterAPI proceed with shutting down the node.
9. The Satellite is removed from LINSTOR.

### `Delete` Policy

When using the `Delete` policy, Piraeus Datastore will remove the Satellite from the LINSTOR Cluster in addition to
removing the associated Kubernetes resources. The Satellite will no longer appear in the list of LINSTOR nodes and
any resources on it are removed.

!!! warning "There are no checks preventing deletion of the last replica of a volume"

    Using this policy means that there are no checks the prevent the deletion of the last replica of a volume. While
    LINSTOR automatically tries to maintain the requested number of replicas, it does so only periodically.

    Using the `Delete` policy will cause the Satellite to be removed immediately, regardless of any current volume
    migration operations.

## Satellite Evacuation when using ClusterAPI

Piraeus Datastore integrates with ClusterAPI when it detects that Kubernetes is managed by ClusterAPI. Piraeus Operator
detects the presence of `cluster.x-k8s.io/machine` and `cluster.x-k8s.io/cluster-namespace` annotations and tries to
hook into the [Machine deletion process](https://cluster-api.sigs.k8s.io/tasks/automated-machine-management/machine_deletions).

In particular, if the Satellites are using the `Evacuate` deletion policy, Piraeus Datastore will instruct ClusterAPI
to wait at the appropriate times, keeping the Machine running until all resources have been evacuated.

By default, Piraeus Operator will search the Cluster it is deployed in for the Machine resources. If the Machine is
managed by a different Cluster, set the `CLUSTER_API_KUBECONFIG` environment variable in the Operator Deployment to
a file containing the configuration to access the Management Cluster.
