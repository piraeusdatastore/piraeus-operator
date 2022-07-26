# Optional components

The Piraeus Operator integrates with a number of optional external components. Not every cluster is configured to
provide these external components by default. Piraeus provides integration with:

* [Volume Snapshots](#snapshot-support-components)
* [Monitoring with Prometheus](#monitoring-with-prometheus)

The operator also installs some optional, piraeus-specific components by default:

* [High Availabiltiy Controller](#high-availability-controller)
* [Stork scheduler](#scheduler-components)

These components are installed to show the full feature set of Piraeus. They can be disabled without affecting the other
components.

## Snapshot support components

Snapshots in Kubernetes require 3 different components to work together. Not all Kubernetes distributions package these
components by default. Follow the steps below to find out how you can enable snapshots on your cluster.

1. The cluster needs to have the snapshot CRDs installed. To check whether your cluster has them installed or not, run:
   ```
   $ kubectl get crds volumesnapshots.snapshot.storage.k8s.io volumesnapshotclasses.snapshot.storage.k8s.io volumesnapshotcontents.snapshot.storage.k8s.io
   NAME                                             CREATED AT
   volumesnapshots.snapshot.storage.k8s.io          2021-07-13T07:53:02Z
   volumesnapshotclasses.snapshot.storage.k8s.io    2021-07-13T07:53:01Z
   volumesnapshotcontents.snapshot.storage.k8s.io   2021-07-13T07:53:04Z
   ```
   If your cluster doesn't have them installed, you can install them from [here](https://github.com/kubernetes-csi/external-snapshotter/tree/master/client/config/crd):
   ```
   kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v4.1.1/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
   kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v4.1.1/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
   kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v4.1.1/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
   ```

   ***NOTE***: you should replace `v4.1.1` in the above commands with the latest [release](https://github.com/kubernetes-csi/external-snapshotter/releases)
   recommended for your Kubernetes version.

2. Snapshot requests in Kubernetes are first processed by a cluster-wide snapshot controller. If you had to manually add the CRDs to the cluster in the step above,
   chances are you also need to deploy the snapshot controller.

   ***NOTE***: If in step 1 the CRDs where already pre-installed in your cluster, you almost certainly can skip this step. Your Kubernetes distribution
   should already include the snapshot controller.

   The Piraeus team provides 2 Helm charts to quickly deploy some [additional validation](https://artifacthub.io/packages/helm/piraeus-charts/snapshot-validation-webhook)
   for snapshot resource and the [snapshot controller](https://artifacthub.io/packages/helm/piraeus-charts/snapshot-controller) it self.

   Deployment should work out of the box on most clusters. Additional configuration options are available, please
   take a look at the chart documentation linked above.
   ```
   $ kubectl create namespace snapshot-controller
   $ helm repo add piraeus-charts https://piraeus.io/helm-charts/
   $ helm install validation-webhook piraeus-charts/snapshot-validation-webhook --namespace snapshot-controller
   $ helm install snapshot-controller piraeus-charts/snapshot-controller --namespace snapshot-controller
   ```

3. The last component is a driver-specific snapshot implementation. This is included in any Piraeus installation and
   requires no further steps. Every CSI Controller deployment of Piraeus also deploys the snapshotter sidecar, that
   ultimately triggers snapshot creation in LINSTOR.

   ***NOTE***: If the CRDs are not deployed, the snapshotter sidecar will continuously warn about the missing CRDs.
   This can be ignored.

### Using snapshots

To use snapshots, you first need to create a `VolumeSnapshotClass`:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: my-first-linstor-snapshot-class
driver: linstor.csi.linbit.com
deletionPolicy: Delete
```

You can then use this snapshot class to create a snapshot from an existing LINSTOR PVC:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
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

The HA Controller is packaged as a separate Helm chart available from our Chart Repository:

```
helm repo add piraeus-charts https://piraeus.io/helm-charts/
helm install piraeus-ha-controller piraeus-charts/piraeus-ha-controller
```

To take full advantage of the HA Controller, we recommend adding the following parameters to your Piraeus StorageClasses.
On loss of quorum, DRBD will freeze any IO requests until either quorum is restored or the HA Controller could trigger
a proper failover.

```yaml
parameters:
  property.linstor.csi.linbit.com/DrbdOptions/auto-quorum: suspend-io
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-no-data-accessible: suspend-io
  property.linstor.csi.linbit.com/DrbdOptions/Resource/on-suspended-primary-outdated: force-secondary
  property.linstor.csi.linbit.com/DrbdOptions/Net/rr-conflict: retry-connect
```

If you have Pods using Piraeus volumes that you want to exclude from automated fail-over, include the following annotation
on the Pod:

```yaml
metadata:
  annotations:
     drbd.linbit.com/ignore-fail-over: ""
  ...
```

### Legacy HA Controller

In previous versions of the Operator, we bundled the HA Controller directly with the Operator. Starting with Operator
1.9.0, this bundled deployment is disabled by default. We recommend using the stand-alone chart instead. If you want
to keep using the legacy version, you can enable setting `haController.enabled=true` when installing the Operator.

### Usage with STORK

STORK is a scheduler extender plugin and storage health monitoring tool (see below). There is considerable overlap
between the functionality of STORK and the HA Controller.

Like the HA Controller, STORK will also delete Pods which use faulty volumes. In contrast to the HA Controller, STORK
does not discriminate based on labels on the Pod.

Another difference between the two is that the HA Controller reacts faster on storage failures, as it watches the
raw event stream from Piraeus, while STORK just periodically checks the volume status.

While they overlap in functionality, there are no known compatibility issues when running both STORK and the HA Controller.

## Affinity controller

Affinity is used by Kubernetes to track on which node a specific resource can be accessed. For example, you can use
affinity to restrict access to a volume to a specific zone. While this is all supported by Piraeus and LINSTOR, and you
could tune your volumes to support almost any cluster topology, there was one important thing missing: updating affinity
after volume migration.

This is where the LINSTOR Affinity Controller comes in: it enables updating a PVs affinity, so that it always matches
the LINSTOR internal state. It enables strict affinity settings should you use ephemeral infrastructure: even if you
rotate out all nodes, your PV affinity will always match the actual volume placement in LINSTOR.

After installing the Operator, you can install the LINSTOR Affinity Controller using our Helm chart:

```
helm repo add piraeus-charts https://piraeus.io/helm-charts/
helm install linstor-affinity-controller piraeus-charts/linstor-affinity-controller
```

Detailed instructions are available on [artifacthub.io](https://artifacthub.io/packages/helm/piraeus-charts/linstor-affinity-controller)
and on the [project page](https://github.com/piraeusdatastore/linstor-affinity-controller).

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
