# How to Deploy Piraeus Datastore on Kubermatic Kubernetes Platform

This guide shows you how to deploy Piraeus Datastore on user clusters that [Kubermatic Kubernetes Platform] (KKP)
manages.

To complete this guide, you should be familiar with:

* creating user clusters and editing `MachineDeployment` resources in KKP.
* editing `LinstorSatelliteConfiguration` resources.
* using the `kubectl` command line tool to access the user cluster.

KKP runs the control plane of each user cluster in the seed cluster. The worker nodes are machines that the KKP
machine-controller creates. Piraeus Datastore runs on the worker nodes only. Most steps in this guide prepare the
worker nodes.

## Allow Privileged Pods in the Piraeus Datastore Namespace

KKP user clusters enforce [Pod Security Standards]. The `default` namespace uses the `baseline` policy, and new
namespaces get the cluster default. The LINSTOR Satellite and the DRBD® Module Loader need privileged access to the
host. Therefore, the namespace for Piraeus Datastore must use the `privileged` policy.

Before you deploy the Piraeus Operator, create and label the namespace:

```
$ kubectl create namespace piraeus-datastore
$ kubectl label namespace piraeus-datastore \
    pod-security.kubernetes.io/enforce=privileged \
    pod-security.kubernetes.io/audit=privileged \
    pod-security.kubernetes.io/warn=privileged
```

Without these labels, Kubernetes rejects the Satellite Pods with a `violates PodSecurity "baseline:latest"` error.

## Load DRBD on the Worker Nodes

KKP supports Ubuntu, Flatcar Container Linux, Red Hat Enterprise Linux, and Rocky Linux as operating systems for
worker nodes. By default, the DRBD Module Loader builds DRBD from source. To build DRBD, the DRBD Module Loader uses the
kernel headers that match the running kernel on the host:

* On Flatcar Container Linux, follow the [Flatcar guide](./flatcar.md).
* On Ubuntu, Red Hat Enterprise Linux, and Rocky Linux, install the kernel headers as described in
  [Installing Linux Kernel Headers](./install-kernel-headers.md). The Rocky Linux images that KKP uses do not include
  the `kernel-devel` package. Make sure that the package for the running kernel is installed on every node, for
  example with a custom `OperatingSystemProfile` for the `MachineDeployment`.

  The Rocky Linux repositories provide `kernel-devel` packages only for the current minor release. If a Rocky Linux 9
  node runs a kernel of an older minor release, the matching `kernel-devel` package is not available from these
  repositories. In this case, update the nodes to the current kernel, or use a DRBD Module Loader image with pre-built
  modules, as described in [How to Configure the DRBD Module Loader](./drbd-loader.md).

After the Satellites are running, verify that DRBD is loaded:

```
$ kubectl -n piraeus-datastore exec <satellite-pod> -c linstor-satellite -- cat /proc/drbd
version: 9.3.4 (api:2/proto:118-125)
```

## Provide Storage Devices

LINSTOR creates storage pools on block devices or LVM volume groups of the worker nodes. The way you add dedicated
disks to the worker nodes depends on the cloud provider that KKP uses for the user cluster.

### KubeVirt Provider

For the KubeVirt provider, the `MachineDeployment` has a `secondaryDisks` field. The KKP machine-controller creates
these disks from the same image as the primary disk. Therefore, the disks contain a copy of the operating system and
are not suitable as LINSTOR storage devices.

Instead, attach blank disks to the worker VMs in the KubeVirt infrastructure cluster. To attach disks to running VMs,
enable the `HotplugVolumes` feature gate in KubeVirt. For each worker VM, create a blank `DataVolume` in the namespace
of the user cluster (`cluster-<cluster-id>`) and attach it:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: <vm-name>-linstor
  namespace: cluster-<cluster-id>
spec:
  source:
    blank: {}
  storage:
    accessModes: [ReadWriteOnce]
    resources:
      requests:
        storage: 10Gi
```

```
$ virtctl addvolume <vm-name> -n cluster-<cluster-id> --volume-name=<vm-name>-linstor --serial=linstor0 --persist
```

The disk appears as a SCSI device on the worker node, for example `/dev/sda`.

### Other Providers

On providers that create VMs or bare-metal machines with additional disks, configure the disks in the provider-specific
part of the `MachineDeployment`, or attach them with the tools of your infrastructure.

## Configure the Storage Pool

After every worker node has a blank disk, create the storage pools with a `LinstorSatelliteConfiguration`. The
following example creates an LVM thin pool on `/dev/sda`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: storage-pool
spec:
  storagePools:
    - name: thinpool
      lvmThinPool:
        volumeGroup: linstor_thinpool
        thinPool: thinpool
      source:
        hostDevices:
          - /dev/sda
```

Verify the storage pools:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor storage-pool list
```

## Network Requirements

DRBD replicates data between the worker nodes. The default Cilium configuration of KKP user clusters does not restrict
traffic between nodes. If you use firewalls or security groups on your infrastructure, allow the following TCP ports
between all worker nodes:

* `3366-3367` for the LINSTOR Satellite
* `7000-7999` for DRBD

The Piraeus Operator uses an admission webhook. KKP routes requests from the control plane in the seed cluster to the
worker nodes through Konnectivity. You do not have to configure the webhook.

[Kubermatic Kubernetes Platform]: https://www.kubermatic.com/products/kubermatic-kubernetes-platform/
[Pod Security Standards]: https://kubernetes.io/docs/concepts/security/pod-security-standards/
