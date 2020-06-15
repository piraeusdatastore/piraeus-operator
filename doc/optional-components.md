# Optional components

The operator installs some additional components by default:

* [CSI Snapshot controller](#snapshot-support-components)
* [Stork scheduler](#scheduler-components)

These components are installed to show the full feature set of Piraeus. They can be disabled without affecting the other
components.

## Snapshot support components

LINSTOR supports Kubernetes volume snapshots, which is currently in beta. To use it, you need to install a cluster wide
snapshot controller. This is done either by the cluster provider, or you can use the piraeus chart.

By default, the piraeus chart will install its own snapshot controller. This can lead to conflict in some cases:

* the cluster already has a snapshot controller
* the cluster does not meet the minimal version requirements (>= 1.17)

In such a case, installation of the snapshot controller can be disabled:

```
--set csi-snapshotter.enabled=false
```

### Using snapshots

To use snapshots, you first need to create a `VolumeSnapshotClass`:

```yaml
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshotClass
metadata:
  name: my-first-linstor-snapshot-class
driver: linstor.csi.linbit.com
deletionPolicy: Delete
```

You can then use this snapshot class to create a snapshot from an existing LINSTOR PVC:

```yaml
apiVersion: snapshot.storage.k8s.io/v1beta1
kind: VolumeSnapshot
metadata:
  name: my-first-linstor-snapshot
spec:
  volumeSnapshotClassName: my-first-linstor-snapshot-class
  source:
    persistentVolumeClaimName: my-first-linstor-volume
```

After a short wait, the snapshot will be ready:

```yaml
$ kubectl describe volumesnapshots.snapshot.storage.k8s.io my-first-linstor-snapshot
...
Spec:
  Source:
    Persistent Volume Claim Name:  my-first-linstor-snapshot
  Volume Snapshot Class Name:      my-first-linstor-snapshot-class
Status:
  Bound Volume Snapshot Content Name:  snapcontent-b6072ab7-6ddf-482b-a4e3-693088136d2c
  Creation Time:                       2020-06-04T13:02:28Z
  Ready To Use:                        true
  Restore Size:                        500Mi
```

You can restore the content of this snaphost by creating a new PVC with the snapshot as source:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-first-linstor-volume-from-snapshot
spec:
  storageClassName: linstor-basic-storage-class
  dataSource:
    name: my-first-linstor-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

## Scheduler components

Stork is a scheduler extender plugin for Kubernetes which allows a storage driver to give the Kubernetes scheduler
hints about where to place a new pod so that it is optimally located for storage performance. You can learn more
about the project on its [GitHub page](https://github.com/libopenstorage/stork).

By default, the operator will install the components required for Stork, and register a new scheduler called `stork`
with Kubernetes. This new scheduler can be used to place pods near to their volumes.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  schedulerName: stork
  containers:
  - name: busybox
    image: busybox
    command: ["tail", "-f", "/dev/null"]
    volumeMounts:
    - name: my-first-linstor-volume
      mountPath: /data
    ports:
    - containerPort: 80
  volumes:
  - name: my-first-linstor-volume
    persistentVolumeClaim:
      claimName: "test-volume"
```

Deployment of the scheduler can be disabled using

```
--set stork.enabled=false
```
