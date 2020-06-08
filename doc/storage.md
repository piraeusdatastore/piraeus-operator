# Storage

The piraeus operator can automate some basic storage set up for LINSTOR. This includes:

* Preparing physical devices. [Read LINSTOR guide](https://www.linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-a_storage_pool_per_backend_device)
* Creating storage pools. [Read LINSTOR guide](https://www.linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-storage_pools)

## Preparing physical devices

The piraeus operator can automatically configure devices on satellites. Eligible for automatic configuration
are block devices that:

* do not contain partition information
* have more than 1 GiB

Any such device will be prepared according to the value of `operator.nodeSet.automaticStorageType`. Devices are
added to a storage pool based on the device name (i.e. all `/dev/nvme1` devices will be part of the pool
`nvme1`)

The possible values for `operator.nodeSet.automaticStorageType`:

* `None` no automatic set up (default)
* `LVM` create a LVM (thick) storage pool
* `LVMTHIN` create a LVM thin storage pool
* `ZFS` create a ZFS based storage pool

## Configuring storage pool creation

The piraeus operator installed by helm can be used to create storage pools. Creation is under control of the
LinstorNodeSet resource:

```
$ kubectl get LinstorNodeSet.piraeus.linbit.com piraeus-op-ns -o yaml                                                                                       [24/1880]
kind: LinstorNodeSet
metadata:
..
spec:
  ..
  storagePools:
    lvmPools:
    - name: lvm-thick
      volumeGroup: drbdpool
    lvmThinPools:
    - name: lvm-thin
      thinVolume: thinpool
      volumeGroup: drbdpool

```

There are two ways to configure storage pools

### At install time

At install time, by setting the value of `operator.nodeSet.storagePools` when running helm install.

First create a file with the storage configuration like:

```yaml
operator:
  nodeSet:
    storagePools:
      lvmPools:
      - name: lvm-thick
        volumeGroup: drbdpool
    ..
```

This file can be passed to the helm installation like this:

```
helm install -f <file> charts/piraeus-op
```

### After install

On a cluster with the operator already configured (i.e. after `helm install`),
you can edit the nodeset configuration like this:

```
$ kubectl edit LinstorNodeSet.piraeus.linbit.com <nodesetname>
```

The storage pool configuration can be updated like in the example above.
