# How to Use Consistency Groups

This guide shows you how to use consistency groups to combine multiple volumes into a single LINSTOR® resource,
so that replicas and snapshots always present a consistent view of the whole set of volumes.

Workloads often spread their state over multiple volumes — for example, a virtual machine with multiple disks,
or a database with separate data and log volumes. By default, every PersistentVolume is a separate LINSTOR
resource, replicated independently of all others. Replication of one volume can pause, disconnect or fall
behind without affecting the other volumes. If the workload's node then crashes or is disconnected, a fail-over
node may see up-to-date data on one volume and outdated data on another: a combined state of the volume set
that never existed on the original node.

A consistency group avoids this: all PersistentVolumeClaims sharing a `linstor.csi.linbit.com/consistency-group`
label are placed as separate volumes of a single LINSTOR resource, sharing one replication connection and one
replication state. Whatever data a fail-over node sees is consistent across the whole group, exactly as it
existed on the original node at one point in time. All members of the group are also always placed on the same
set of nodes.

The same idea applies to backups: snapshots of consistency group members are always created for the whole
group at once, using the `VolumeGroupSnapshot` resource. The group is captured in a single LINSTOR snapshot,
with every volume at the same point in time.

To follow the steps in this guide, you should be familiar with creating and restoring snapshots of single
volumes. Learn about them in the [snapshots tutorial](../tutorial/snapshots.md).

## Prerequisites

* An installed and configured Piraeus Datastore. Learn how to get started in our
  [introduction tutorial](../tutorial/get-started.md).
* A storage pool supporting snapshots. LINSTOR supports snapshots for `LVM_THIN`, `FILE_THIN`, `ZFS` and
  `ZFS_THIN` pools.
* A cluster with the [`snapshot-controller`](https://github.com/kubernetes-csi/external-snapshotter/) deployed,
  including support for the `groupsnapshot.storage.k8s.io/v1` API (external-snapshotter v8.6.0 or newer). To
  check if the API is available, try running:
  ```
  $ kubectl api-resources --api-group=groupsnapshot.storage.k8s.io -oname
  volumegroupsnapshotclasses.groupsnapshot.storage.k8s.io
  volumegroupsnapshotcontents.groupsnapshot.storage.k8s.io
  volumegroupsnapshots.groupsnapshot.storage.k8s.io
  ```
  If your output is empty, deploy the snapshot controller and its CustomResourceDefinitions by using:
  ```
  kubectl apply -k https://github.com/kubernetes-csi/external-snapshotter//client/config/crd
  kubectl apply -k https://github.com/kubernetes-csi/external-snapshotter//deploy/kubernetes/snapshot-controller
  ```
  The snapshot controller processes `VolumeGroupSnapshot` resources only when the `CSIVolumeGroupSnapshot`
  feature gate is enabled, which is not the default. Verify that the `snapshot-controller` Deployment includes
  `--feature-gates=CSIVolumeGroupSnapshot=true` in its arguments. To add it, run:
  ```
  kubectl -n kube-system patch deployment snapshot-controller --type=json \
    -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--feature-gates=CSIVolumeGroupSnapshot=true"}]'
  ```

## Creating Volumes in a Consistency Group

Membership in a consistency group is controlled by a label on the PersistentVolumeClaim:

```yaml
metadata:
  labels:
    linstor.csi.linbit.com/consistency-group: my-group
```

All PersistentVolumeClaims in the same namespace carrying the same label value form one group. The label needs
to be present when the PersistentVolumeClaim is created: adding it to an existing volume has no effect. All
members of a group must use the same `StorageClass`, and because they share one LINSTOR resource, they are
always placed on the same set of nodes.

As an example workload, we deploy a Pod that appends the same log line to two separate volumes: a data volume
and a log volume. Both PersistentVolumeClaims carry the same `linstor.csi.linbit.com/consistency-group` label,
placing them in one consistency group named `volume-logger`:

```
$ kubectl apply -f - <<EOF
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-storage
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  linstor.csi.linbit.com/storagePool: pool1
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-volume
  labels:
    linstor.csi.linbit.com/consistency-group: volume-logger
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  accessModes:
    - ReadWriteOnce
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: log-volume
  labels:
    linstor.csi.linbit.com/consistency-group: volume-logger
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  accessModes:
    - ReadWriteOnce
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: volume-logger
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: volume-logger
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app.kubernetes.io/name: volume-logger
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - name: volume-logger
          image: busybox
          args:
            - sh
            - -c
            - |
              echo "Hello from \$HOSTNAME, started at \$(date)" | tee -a /data/hello /log/hello
              # We use this to keep the Pod running
              tail -f /dev/null
          volumeMounts:
            - mountPath: /data
              name: data-volume
            - mountPath: /log
              name: log-volume
      volumes:
        - name: data-volume
          persistentVolumeClaim:
            claimName: data-volume
        - name: log-volume
          persistentVolumeClaim:
            claimName: log-volume
EOF
```

Then, we wait for the Pod to start, and verify that the same line was logged to both volumes:

```
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-66f57d8bbd-jwmnh condition met
$ kubectl exec deploy/volume-logger -- cat /data/hello /log/hello
Hello from volume-logger-66f57d8bbd-jwmnh, started at Wed Jul 22 09:14:02 UTC 2026
Hello from volume-logger-66f57d8bbd-jwmnh, started at Wed Jul 22 09:14:02 UTC 2026
```

Instead of two separate resources, LINSTOR created a single resource named `cg-<uuid>`, holding both
PersistentVolumes as separate volume numbers:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor volume-definition list
+------------------------------------------------------------------------------------------+
| ResourceName                            | VolumeNr | VolumeMinor | Size  | Gross | State |
|==========================================================================================|
| cg-05b322f5-c522-45a0-9de4-3e3c1cca9be5 | 0        | 1000        | 1 GiB |       | ok    |
| cg-05b322f5-c522-45a0-9de4-3e3c1cca9be5 | 1        | 1001        | 1 GiB |       | ok    |
+------------------------------------------------------------------------------------------+
```

## Creating a Group Snapshot

Creating a group snapshot requires the creation of a `VolumeGroupSnapshotClass` first. Just like the
`VolumeSnapshotClass` for single-volume snapshots, it specifies our `linstor.csi.linbit.com` provisioner and the
clean-up policy for the snapshots:

```
$ kubectl apply -f - <<EOF
apiVersion: groupsnapshot.storage.k8s.io/v1
kind: VolumeGroupSnapshotClass
metadata:
  name: piraeus-group-snapshots
driver: linstor.csi.linbit.com
deletionPolicy: Delete
EOF
```

Next, we request the creation of a group snapshot using a `VolumeGroupSnapshot` resource. It selects its member
volumes using a label selector: here we simply select the same label that defines the consistency group:

```
$ kubectl apply -f - <<EOF
apiVersion: groupsnapshot.storage.k8s.io/v1
kind: VolumeGroupSnapshot
metadata:
  name: volume-logger-snap-1
spec:
  volumeGroupSnapshotClassName: piraeus-group-snapshots
  source:
    selector:
      matchLabels:
        linstor.csi.linbit.com/consistency-group: volume-logger
EOF
```

Now, we wait for the group snapshot to be created:

```
$ kubectl wait volumegroupsnapshot --for=jsonpath='{.status.readyToUse}'=true volume-logger-snap-1
volumegroupsnapshot.groupsnapshot.storage.k8s.io/volume-logger-snap-1 condition met
```

The snapshot controller created one `VolumeSnapshot` resource per member. The `SOURCEPVC` column shows which
member each snapshot belongs to — take note of these generated names, they are referenced when restoring:

```
$ kubectl get volumesnapshot
NAME                                                                        READYTOUSE   SOURCEPVC     SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS   SNAPSHOTCONTENT                                                                CREATIONTIME   AGE
snapshot-46f95430b32172e01d035ffed01a512f76dc317aeb15d689e40b26f0a13c76d1   true         data-volume                           1Gi                           snapcontent-46f95430b32172e01d035ffed01a512f76dc317aeb15d689e40b26f0a13c76d1   17s            17s
snapshot-c92e39e07c9e0d1801e0e26adcb2c337a0a4a45de53f3f8b98429aeb7db9a765   true         log-volume                            1Gi                           snapcontent-c92e39e07c9e0d1801e0e26adcb2c337a0a4a45de53f3f8b98429aeb7db9a765   17s            17s
```

While Kubernetes shows one `VolumeSnapshot` per member, LINSTOR created a single snapshot of the shared
resource, capturing both volumes at the same point in time:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor snapshot list
+-----------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ResourceName                            | SnapshotName                                       | NodeNames      | Volumes            | CreatedOn           | State      |
|=======================================================================================================================================================================|
| cg-05b322f5-c522-45a0-9de4-3e3c1cca9be5 | groupsnapshot-8f9e0e5c-3f04-4d5f-9f4e-b23a5c0efc21 | n1.example.com | 0: 1 GiB, 1: 1 GiB | 2026-07-22 09:15:41 | Successful |
+-----------------------------------------------------------------------------------------------------------------------------------------------------------------------+
```

## Restoring a Group Snapshot

To simulate the accidental removal of important data, we delete the files on both volumes:

```
$ kubectl exec deploy/volume-logger -- rm /data/hello /log/hello
```

Restoring mirrors the creation of the group: we create one new PersistentVolumeClaim per member, each
referencing the member's `VolumeSnapshot` as its data source, and each carrying the same **new**
consistency-group label value. The restored PersistentVolumeClaims again form one shared LINSTOR resource, with
every member claiming the volume number recorded in its snapshot.

A group must be restored completely: use the same new label value for all members, restore every member from
the same group snapshot, and do not add fresh volumes to the restored group. The requested sizes must match the
size of the snapshots.

First, we stop the Deployment and remove the existing PersistentVolumeClaims:

```
$ kubectl scale deploy/volume-logger --replicas=0
deployment.apps/volume-logger scaled
$ kubectl delete pvc data-volume log-volume
persistentvolumeclaim "data-volume" deleted
persistentvolumeclaim "log-volume" deleted
```

Then, we recreate them from the group snapshot. Replace the `dataSource` names with the generated
`VolumeSnapshot` names from the previous section, matching each new PersistentVolumeClaim with the snapshot of
the volume it replaces:

```
$ kubectl apply -f - <<EOF
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-volume
  labels:
    linstor.csi.linbit.com/consistency-group: volume-logger-restored
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  accessModes:
    - ReadWriteOnce
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: snapshot-46f95430b32172e01d035ffed01a512f76dc317aeb15d689e40b26f0a13c76d1
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: log-volume
  labels:
    linstor.csi.linbit.com/consistency-group: volume-logger-restored
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  accessModes:
    - ReadWriteOnce
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: snapshot-c92e39e07c9e0d1801e0e26adcb2c337a0a4a45de53f3f8b98429aeb7db9a765
EOF
```

Since the new PersistentVolumeClaims use the same names, we can simply scale the Deployment back up and verify
that both volumes contain the data from the time of the snapshot again:

```
$ kubectl scale deploy/volume-logger --replicas=1
deployment.apps/volume-logger scaled
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-66f57d8bbd-b6kfx condition met
$ kubectl exec deploy/volume-logger -- cat /data/hello /log/hello
Hello from volume-logger-66f57d8bbd-jwmnh, started at Wed Jul 22 09:14:02 UTC 2026
Hello from volume-logger-66f57d8bbd-b6kfx, started at Wed Jul 22 09:31:56 UTC 2026
Hello from volume-logger-66f57d8bbd-jwmnh, started at Wed Jul 22 09:14:02 UTC 2026
Hello from volume-logger-66f57d8bbd-b6kfx, started at Wed Jul 22 09:31:56 UTC 2026
```

## Restrictions

Because all members of a consistency group share one LINSTOR resource, some operations available on regular
volumes do not apply to group members:

* All members of a group must be in the same namespace and use the same `StorageClass`: they share one resource
  definition, resource group and placement. Groups with the same label value in different namespaces are
  independent.
* A member cannot be created from another volume or from a snapshot of a regular volume: LINSTOR restores whole
  resources only. The only supported data source for a member is a `VolumeSnapshot` created by a group
  snapshot, used when restoring the whole group. For the same reason, a single member snapshot cannot be
  restored into a standalone volume outside a group.
* A single member cannot be snapshotted on its own with a `VolumeSnapshot`: snapshots always cover the whole
  group, so use a `VolumeGroupSnapshot` instead.
* The `VolumeSnapshot` resources created by a group snapshot can only be deleted by deleting the owning
  `VolumeGroupSnapshot` resource.
* Members cannot use `ReadWriteMany` volumes in `Filesystem` mode, as the NFS export used to implement them is
  incompatible with consistency groups. `ReadWriteMany` in `Block` mode, as used for KubeVirt live migration,
  is supported: the driver temporarily allows the volume to be active on two nodes during the migration.
* Members can still be deleted individually: this removes only the member's volume, and the shared resource is
  removed together with its last member.
* Nothing applies the consistency-group label automatically: label the PersistentVolumeClaims yourself, or use
  a tool that propagates labels, such as CDI, which copies labels from a `DataVolume` to the created
  PersistentVolumeClaim.

For a complete example of running KubeVirt virtual machines with consistency groups — including cloning all
virtual machine disks from a shared golden image — see the
[consistency group examples in the LINSTOR CSI repository](https://github.com/piraeusdatastore/linstor-csi/tree/master/examples/consistency-groups).
