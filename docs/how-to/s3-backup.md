# How to Back Up Volumes to S3

This guide shows you how to store snapshots of your volumes in S3 compatible object storage, and how to restore
them.

LINSTOR® can upload snapshots to an S3 compatible bucket, registered as a "remote" in LINSTOR. In Kubernetes,
this is triggered by creating a `VolumeSnapshot` using a specially configured `VolumeSnapshotClass`: in addition
to creating the snapshot in the storage pool, LINSTOR uploads the snapshot content to the bucket. Because the
uploaded data is stored outside the cluster, it remains available even if a volume, a node, or the whole cluster
is lost, making it suitable for backups and disaster recovery.

To follow the steps in this guide, you should be familiar with creating and restoring snapshots of single
volumes. Learn about them in the [snapshots tutorial](../tutorial/snapshots.md).

## Prerequisites

* An installed and configured Piraeus Datastore. Learn how to get started in our
  [introduction tutorial](../tutorial/get-started.md).
* A storage pool supporting snapshots. LINSTOR supports snapshots for `LVM_THIN`, `FILE_THIN`, `ZFS` and
  `ZFS_THIN` pools.
* A cluster with the [`snapshot-controller`](https://github.com/kubernetes-csi/external-snapshotter/) deployed.
  The [snapshots tutorial](../tutorial/snapshots.md#prerequisites) shows how to check for and deploy it.
* A LINSTOR passphrase, configured through
  [`LinstorCluster.spec.linstorPassphraseSecret`](../reference/linstorcluster.md#speclinstorpassphrasesecret).
  LINSTOR uses the passphrase to encrypt the S3 credentials before storing them in its database.
* An S3 compatible bucket, for example on Amazon S3, Ceph Object Gateway or MinIO, along with an access key and
  secret key that allow reading and writing the bucket content.

## Storing the S3 Credentials

The access key and secret key for the bucket are stored in a Secret. The Secret can be placed in any namespace:
in this guide, it is placed alongside the other Piraeus Datastore resources in the `piraeus-datastore`
namespace.

```
$ kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: s3-backup-credentials
  namespace: piraeus-datastore
type: linstor.csi.linbit.com/s3-credentials.v1
immutable: true
stringData:
  access-key: ACCESS_KEY
  secret-key: SECRET_KEY
EOF
```

## Granting Access to the Credentials

Backups are created by the LINSTOR CSI driver, which receives the credentials from the `csi-snapshotter` sidecar
of the `linstor-csi-controller` Deployment. As a security precaution, the sidecar is deployed **without** any
permission to read Secrets: you choose which Secrets it can read by explicitly granting access.

Create a Role in the namespace containing the Secret, allowing read access to only this Secret, and bind it to
the `linstor-csi-controller` ServiceAccount:

```
$ kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: read-s3-backup-credentials
  # Must be the namespace containing the Secret.
  namespace: piraeus-datastore
rules:
  - apiGroups: [ "" ]
    resources: [ "secrets" ]
    resourceNames: [ "s3-backup-credentials" ]
    verbs: [ "get" ]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: read-s3-backup-credentials
  # Must be the namespace containing the Secret.
  namespace: piraeus-datastore
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: read-s3-backup-credentials
subjects:
  - kind: ServiceAccount
    name: linstor-csi-controller
    # Must be the namespace of the Piraeus Datastore deployment.
    namespace: piraeus-datastore
EOF
```

The credentials are used when creating a backup, and again when deleting it. Keep the Secret and this grant in
place for as long as any backup created with these credentials exists.

## Creating the VolumeSnapshotClass

A `VolumeSnapshotClass` configures where and how snapshots are shipped, using parameters prefixed with
`snap.linstor.csi.linbit.com/`, and references the credentials Secret created above:

```
$ kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: piraeus-s3-backup
driver: linstor.csi.linbit.com
deletionPolicy: Delete
parameters:
  snap.linstor.csi.linbit.com/type: S3
  snap.linstor.csi.linbit.com/remote-name: backup-remote
  snap.linstor.csi.linbit.com/allow-incremental: "false"
  snap.linstor.csi.linbit.com/s3-bucket: my-piraeus-backups
  snap.linstor.csi.linbit.com/s3-endpoint: s3.us-west-1.amazonaws.com
  snap.linstor.csi.linbit.com/s3-signing-region: us-west-1
  snap.linstor.csi.linbit.com/s3-use-path-style: "false"
  csi.storage.k8s.io/snapshotter-secret-name: s3-backup-credentials
  csi.storage.k8s.io/snapshotter-secret-namespace: piraeus-datastore
EOF
```

The parameters control the following behavior:

* `type: S3` uploads every snapshot created with this class to the S3 remote.
* `remote-name` sets the name under which the remote is registered in LINSTOR. The LINSTOR CSI driver registers
  the remote automatically, using the endpoint, bucket, region and credentials from the remaining parameters.
* `allow-incremental` chooses between full backups for every snapshot, or uploading only the changes since the
  previous snapshot. Incremental backups form a chain: use `max-increments` or `full-snapshot-after` to bound
  the chain, so that older backups eventually become reclaimable.
* `delete-local: "true"` optionally removes the snapshot from the local storage pool once the upload completes,
  keeping only the copy in the bucket.
* `s3-bucket`, `s3-endpoint`, `s3-signing-region` and `s3-use-path-style` describe the S3 endpoint. Learn more
  about them in the
  [LINSTOR User's Guide](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-shipping_snapshots).
* `csi.storage.k8s.io/snapshotter-secret-name` and `csi.storage.k8s.io/snapshotter-secret-namespace` reference
  the Secret created above. These parameters support
  [templating](https://kubernetes-csi.github.io/docs/secrets-and-credentials-volume-snapshot-class.html), for
  example to choose a Secret based on the namespace of the `VolumeSnapshot`. When using templated names, widen
  the Role from the previous section to cover the expected Secrets.

## Creating a Backup

We use the same example workload as the [snapshots tutorial](../tutorial/snapshots.md#creating-an-example-workload),
which logs a message to the `data-volume` PersistentVolumeClaim. Creating a backup works exactly like creating a
regular snapshot, only using the `VolumeSnapshotClass` created above:

```
$ kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: data-volume-backup-1
spec:
  volumeSnapshotClassName: piraeus-s3-backup
  source:
    persistentVolumeClaimName: data-volume
EOF
```

The snapshot becomes ready once the upload to the bucket is complete:

```
$ kubectl wait volumesnapshot --for=jsonpath='{.status.readyToUse}'=true data-volume-backup-1
volumesnapshot.snapshot.storage.k8s.io/data-volume-backup-1 condition met
```

In LINSTOR, we can verify that the remote was registered and the backup uploaded:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor remote list
+-------------------------------------------------------------------------+
| Name          | Type | Info                                             |
|=========================================================================|
| backup-remote | S3   | us-west-1.s3.us-west-1.amazonaws.com/my-piraeus- |
|               |      | backups                                          |
+-------------------------------------------------------------------------+
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor backup list backup-remote
+------------------------------------------------------------------------------------------------------------------------+
| Resource                                 | Snapshot                                      | Finished at         | Status |
|==========================================================================================================================|
| pvc-9c04b307-d22d-454f-8f24-ed5837fe4426 | snapshot-a8757c1d-cd37-42d2-9557-a24b1222d118 | 2026-08-27 09:15:41 | Success |
+------------------------------------------------------------------------------------------------------------------------+
```

## Restoring a Backup

Within the same cluster, restoring a backup works exactly like restoring a regular snapshot: create a new
PersistentVolumeClaim referencing the `VolumeSnapshot` as its data source, as shown in the
[snapshots tutorial](../tutorial/snapshots.md#restoring-from-a-snapshot). If the local snapshot was removed with
`delete-local`, LINSTOR automatically downloads the snapshot data from the bucket.

Because the bucket content is independent of the cluster, backups can also be restored into a **different**
LINSTOR cluster, for example when recovering from the loss of the original cluster. This requires registering
the remote and restoring the backup using the LINSTOR client, as described in the
[LINSTOR User's Guide](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-shipping_snapshots).

## Backing Up Consistency Groups

Volumes in a [consistency group](./consistency-groups.md) are backed up with a `VolumeGroupSnapshotClass` using
the same `snap.linstor.csi.linbit.com/` parameters as above. Only the parameters referencing the credentials
Secret are named differently:

```yaml
  csi.storage.k8s.io/group-snapshotter-secret-name: s3-backup-credentials
  csi.storage.k8s.io/group-snapshotter-secret-namespace: piraeus-datastore
```

The Secret is read by the same ServiceAccount, so the grant from
[Granting Access to the Credentials](#granting-access-to-the-credentials) applies without changes.

Uploading a `VolumeGroupSnapshot` to S3 works only for members of a consistency group: they share a single
LINSTOR resource, which is uploaded as one backup. A `VolumeGroupSnapshot` of volumes that are not in a
consistency group spans multiple LINSTOR resources and can not be uploaded to a remote.

## Troubleshooting

If access to the credentials Secret is missing, the `VolumeSnapshot` never becomes ready, and describing it
shows an error event similar to:

```
$ kubectl describe volumesnapshot data-volume-backup-1
...
Events:
  Type     Reason                  Age   From                 Message
  ----     ------                  ----  ----                 -------
  Warning  SnapshotContentCheckandUpdateFailed  10s  snapshot-controller  Failed to check and update snapshot content: error getting secret s3-backup-credentials in namespace piraeus-datastore: secrets "s3-backup-credentials" is forbidden: User "system:serviceaccount:piraeus-datastore:linstor-csi-controller" cannot get resource "secrets" in API group "" in the namespace "piraeus-datastore"
```

Verify that the Role and RoleBinding from [Granting Access to the Credentials](#granting-access-to-the-credentials)
exist in the namespace containing the Secret, that the Role names the Secret in `resourceNames`, and that the
RoleBinding subject references the `linstor-csi-controller` ServiceAccount in the namespace of the Piraeus
Datastore deployment.
