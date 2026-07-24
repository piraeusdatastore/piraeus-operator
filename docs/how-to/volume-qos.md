# How to Configure Volume IO Limits (QoS)

Piraeus Datastore can enforce per-volume IO limits, restricting the read and write bandwidth and IOPS available to
the containers using a volume. This prevents a single noisy workload from saturating the storage of a node.

Two components work together to enforce the limits:

* The LINSTOR CSI driver reads the limits from the StorageClass parameters and stores them with the volume. Whenever
  the volume is attached to a node, the driver publishes the limits and the device path in the `VolumeAttachment`
  metadata.
* The [nri-volume-qos] plugin runs on every node and hooks into the container runtime using the [Node Resource
  Interface (NRI)][NRI]. When a container using the volume starts, the plugin applies the limits to the container's
  cgroup using the Linux cgroupv2 `io.max` interface. Both filesystem volumes and raw block volumes are supported.

To complete this guide, you should be familiar with:

* Deploying workloads in Kubernetes using [`helm`](https://helm.sh/)
* Deploying resources using [`kubectl`](https://kubernetes.io/docs/tasks/tools/)

## Prerequisites

* Piraeus Datastore v2.10.8 or later, which deploys LINSTOR CSI v1.11.3 or later.
* Kubernetes 1.26 or later.
* A container runtime with NRI support: containerd 1.7 or later, or CRI-O 1.26 or later.

    In containerd 1.7, NRI support is disabled by default. Enable it by adding the following snippet to
    `/etc/containerd/config.toml` and restarting containerd on every node:

    ```toml
    [plugins."io.containerd.nri.v1.nri"]
      disable = false
    ```

    Starting with containerd 2.0, and in all supported CRI-O releases, NRI is enabled by default.

* A Linux kernel with the cgroupv2 IO controller, version 5.14 or later.

## Deploying the NRI Plugin

=== "Helm"

    Deploy the `nri-volume-qos` plugin using the Helm chart published on the GitHub Container Registry:

    ```
    $ helm install nri-volume-qos oci://ghcr.io/piraeusdatastore/nri-volume-qos --namespace kube-system
    ```

=== "kubectl"

    Every [release][nri-volume-qos-releases] includes a `manifest.yaml`, containing the Helm chart rendered with
    default values. Apply it to deploy the plugin in the `kube-system` namespace:

    ```
    $ kubectl apply --server-side -f https://github.com/piraeusdatastore/nri-volume-qos/releases/latest/download/manifest.yaml
    ```

Then, wait for the plugin to be ready on all nodes:

```
$ kubectl --namespace kube-system rollout status daemonset nri-volume-qos
daemon set "nri-volume-qos" successfully rolled out
```

> 🗒️ NOTE: Some distributions place the kubelet directory somewhere other than `/var/lib/kubelet`, for example k0s
> and MicroK8s. Set the `hostPaths.kubeletPodsDir` Helm value to the pods directory used by your distribution. See
> the [chart documentation][nri-volume-qos-chart] for all available values.

## Configuring IO Limits in a StorageClass

IO limits are configured as StorageClass parameters. Create a StorageClass, adding the limits that should apply to
every volume provisioned from it:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-storage-qos
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  linstor.csi.linbit.com/storagePool: pool1
  qos.linbit.com/rbps: "40Mi"
  qos.linbit.com/wbps: "20Mi"
  qos.linbit.com/riops: "4000"
  qos.linbit.com/wiops: "2000"
```

The following parameters are available. All of them are optional: leaving out a parameter leaves that dimension
unlimited.

| Parameter             | Description                                     |
|-----------------------|-------------------------------------------------|
| `qos.linbit.com/rbps` | Maximum read bandwidth, in bytes per second     |
| `qos.linbit.com/wbps` | Maximum write bandwidth, in bytes per second    |
| `qos.linbit.com/riops`| Maximum read operations per second              |
| `qos.linbit.com/wiops`| Maximum write operations per second             |

Values are plain integers (`"41943040"`) or [Kubernetes quantities](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/)
(`"40Mi"`, `"500k"`).

> 🗒️ NOTE: The limits are stored with the volume when it is provisioned. Changing the parameters of a StorageClass
> only affects newly provisioned volumes. See [Changing the Limits of an Existing Volume](#changing-the-limits-of-an-existing-volume)
> for existing volumes.

The limits apply to each container individually: every container using the volume receives its own `io.max` entry,
so two containers sharing a volume can each use the full configured limit.

## Verifying the Limits

Create a volume using the StorageClass and a Job running the [`fio`](https://github.com/axboe/fio) disk benchmark:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: qos-test-volume
spec:
  storageClassName: piraeus-storage-qos
  accessModes: [ ReadWriteOnce ]
  resources:
    requests:
      storage: 1Gi
---
apiVersion: batch/v1
kind: Job
metadata:
  name: qos-test
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: fio
          image: nixery.dev/fio
          command:
            - fio
            - --name=qos-test
            - --filename=/volume/test.dat
            - --size=256m
            - --rw=randrw
            - --bs=128k
            - --direct=1
            - --ioengine=libaio
            - --iodepth=8
            - --runtime=30
            - --time_based
          volumeMounts:
            - name: volume
              mountPath: /volume
      volumes:
        - name: volume
          persistentVolumeClaim:
            claimName: qos-test-volume
```

While the Job is running, verify that the CSI driver published the limits in the `VolumeAttachment` resource:

```
$ kubectl get volumeattachments -o yaml | grep "qos.linbit.com"
      qos.linbit.com/device: /dev/drbd1000
      qos.linbit.com/rbps: "41943040"
      qos.linbit.com/riops: "4000"
      qos.linbit.com/wbps: "20971520"
      qos.linbit.com/wiops: "2000"
```

Once the Job completes, the reported bandwidth matches the configured limits of 40 MiB/s read and 20 MiB/s write:

```
$ kubectl logs job/qos-test | grep -E "READ|WRITE"
   READ: bw=40.1MiB/s (42.0MB/s), 40.1MiB/s-40.1MiB/s (42.0MB/s-42.0MB/s), io=1204MiB (1262MB), run=30020-30020msec
  WRITE: bw=20.0MiB/s (21.0MB/s), 20.0MiB/s-20.0MiB/s (21.0MB/s-21.0MB/s), io=601MiB (630MB), run=30020-30020msec
```

## Changing the Limits of an Existing Volume

The limits are stored as auxiliary properties on the LINSTOR resource definition. To change them for an existing
volume, set the properties using the LINSTOR client, referencing the volume by its PersistentVolume name:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- \
    linstor resource-definition set-property --aux pvc-1e24339c-ee8d-4c0e-a68e-cbc85c866704 qos.linbit.com/wbps 10Mi
```

The new limits take effect the next time the volume is attached: delete the Pods using the volume, wait for the
volume to be detached from the node, and start the Pods again.

[nri-volume-qos]: https://github.com/piraeusdatastore/nri-volume-qos
[nri-volume-qos-chart]: https://github.com/piraeusdatastore/nri-volume-qos/tree/main/charts/nri-volume-qos
[nri-volume-qos-releases]: https://github.com/piraeusdatastore/nri-volume-qos/releases
[NRI]: https://github.com/containerd/nri
