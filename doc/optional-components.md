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

### CSI Volume Cloning

Based on the concept of snapshots LINSTOR also supports cloning of persistent volumes - or to be more precise: of existing
persistent volume claims (PVC). The CSI specification mentions some restrictions regarding namespace and storage classes
of a PVC clone (see [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/) for details).
In regard to LINSTOR a clone requires that the volume was created using a LINSTOR storage pool which supports snapshots
(i.e. a LVMTHIN pool). The new volume will be placed on the same nodes as the original (this can later change during
use, but you can't directly clone to a completely different node).

To clone a volume create a new PVC and define the origin PVC in the dataSource:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-cloned-pvc
spec:
  storageClassName: linstor-basic-storage-class
  dataSource:
    name: my-origin-linstor-pvc
    kind: PersistentVolumeClaim
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

## Monitoring with Prometheus

Starting with operator version 1.5.0, you can use [Prometheus](https://prometheus.io/) to monitor Piraeus components.
The operator will set up monitoring containers along the existing components and make them available as a `Service`.

If you use the [Prometheus Operator](https://prometheus-operator.dev/), the Piraeus Operator will also set up the `ServiceMonitor`
instances. The metrics will automatically be collected by the Prometheus instance associated to the operator, assuming
[watching the Piraeus namespace is enabled](https://prometheus-operator.dev/docs/kube/monitoring-other-namespaces/).

### Linstor Controller Monitoring

The Linstor Controller exports cluster-wide metrics. Metrics are exported on the existing controller service, using the
path [`/metrics`](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-linstor-monitoring).

### DRBD Resource Monitoring

All satellites are bundled with a secondary container that uses [`drbd-reactor`](https://github.com/LINBIT/drbd-reactor/)
to export metrics directly from DRBD. The metrics are available on port 9942, for convenience a headless service named
`<linstorsatelliteset-name>-monitoring` is provided.

If you want to disable the monitoring container, set `monitoringImage` to `""` in your LinstorSatelliteSet resource.

## High Availability Controller

The [Piraeus High Availability (HA) Controller] will speed up the fail over process for stateful workloads using Piraeus for
storage. Using the HA Controller reduces the time it takes for Kubernetes to reschedule a Pod using faulty storage from
15min to 45seconds (exact times depend on your Kubernetes set up).

[Piraeus High Availability (HA) Controller]: https://github.com/piraeusdatastore/piraeus-ha-controller

To mark your stateful applications as managed by Piraeus, use the `linstor.csi.linbit.com/on-storage-lost: remove` label.
For example, Pod Templates in a StatefulSet should look like:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: my-stateful-app
spec:
  serviceName: my-stateful-app
  selector:
    matchLabels:
      app.kubernetes.io/name: my-stateful-app
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-stateful-app
        linstor.csi.linbit.com/on-storage-lost: remove
    ...
```

This way, the Piraeus High Availability Controller will not interfere with applications that do not benefit or even
support it's primary use.

To disable deployment of the HA Controller use:

```
--set haController.enabled=false
```

### Usage with STORK

STORK is a scheduler extender plugin and storage health monitoring tool (see below). There is considerable overlap
between the functionality of STORK and the HA Controller.

Like the HA Controller, STORK will also delete Pods which use faulty volumes. In contrast to the HA Controller, STORK
does not discriminate based on labels on the Pod.

Another difference between the two is that the HA Controller reacts faster on storage failures, as it watches the
raw event stream from Piraeus, while STORK just periodically checks the volume status.

While they overlap in functionality, there are no known compatibility issues when running both STORK and the HA Controller.

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
