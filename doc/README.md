# Piraeus Operator Documentation

## Deployment

The [project README](../README.md) contains documentation on the initial deployment
process. A quick summary can be found here:

```
# Create an initial set of persistent volumes for etcd. Creates a hostpath volume on every control-plane node.
helm install piraeus-etcd-pv ./charts/pv-hostpath
# Deploy the piraeus operator chart. Replace <image> with the piraeus DRBD loader image matching your host OS.
helm install piraeus-op ./charts/piraeus --set operator.satelliteSet.kernelModuleInjectionImage=<image>
```

Then, wait for the deployment to finish:

```
kubectl wait  --for=condition=Ready --timeout=10m pod --all
```

## The LINSTOR client

Piraeus uses [LINSTOR](https://github.com/linbit/linstor-server) as the storage backend. Most
configuration needs can be handled by the Piraeus Operator by editing one of the
`LinstorController`, `LinstorSatelliteSet` or `LinstorCSIDriver` resources.

However, in some cases you might want to directly interface with the LINSTOR system using the `linstor` command.
There are two ways to achieve this:

* Use the [kubectl-linstor](https://github.com/piraeusdatastore/kubectl-linstor) plugin. The plugin interfaces
  with the resources used by the Piraeus Operator and enables you to use `kubectl linstor ...` to execute LINSTOR
  commands.
* Execute the `linstor` command directly in the controller pod:
  ```
  kubectl exec -it deployment/piraeus-op-cs-controller -- linstor ...
  ```

## Configuring storage pools

To provision volumes, you need to configure storage pools. The LINSTOR backend supports
a range of different storage providers. For some common providers, the Piraeus Operator
provides convenient configuration via the `LinstorSatelliteSet` resource. You can read more
on how to configure storage [here](./storage.md).

## Creating volumes

Once you have storage pools configured (confirm by running `kubectl linstor storage-pool list`), you
can almost start creating Persistent Volumes (PV) with Piraeus. First you will need to create
a new storage class in Kubernetes.

The following example storage class configures piraeus to:
* use 2 replicas for every persistent volume
* use the `xfs` filesystem
* use storage pools named `ssd`
* allow volume expansion by resizing the Persistent Volume Claim (PVC).
* wait for the first Pod to create the volume, placing a replica on the same node if possible.
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-ssd
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  autoPlace: "2"
  storagePool: ssd
  csi.storage.k8s.io/fstype: xfs
```

You can find a full list of supported options [here](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-kubernetes-sc-parameters).

Using this storage class, you can provision volumes by applying a Persistent Volume Claim and waiting
for Piraeus to provision the PV. The following PVC creates a 5GiB volume using the above storage class:

```yaml
apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    name: piraeus-pvc-1
  spec:
    storageClassName: piraeus-ssd
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 5Gi
```

## Snapshots

Piraeus supports snapshots via the CSI snapshotting feature. To enable this feature in your
cluster, you need to add a [Snapshot Controller](https://github.com/kubernetes-csi/external-snapshotter/) to your cluster.

Some Kubernetes distributions (for example: OpenShift) already bundle this snapshot controller. On distributions
without a bundled snapshot controller, you can use our guide [here](./optional-components.md#snapshot-support-components).

# Additional information

Should you require further information check out the following links:

* Additional documentation on [LINSTOR](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-kubernetes-basic-configuration-and-deployment).
* You can join our [community slack channel](https://piraeus-datastore.slack.com/join/shared_invite/enQtOTM4OTk3MDcxMTIzLTM4YTdiMWI2YWZmMTYzYTg4YjQ0MjMxM2MxZDliZmEwNDA0MjBhMjIxY2UwYmY5YWU0NDBhNzFiNDFiN2JkM2Q).
* Professional support is available to [LINBIT customers](https://linbit.com/kubernetes).
