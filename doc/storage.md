# Configuring Storage

The piraeus operator can automate some basic storage set up for LINSTOR.

## Configuring storage pool creation

The piraeus operator can be used to create LINSTOR storage pools. Creation is under control of the
LinstorSatelliteSet resource:

```
$ kubectl get LinstorSatelliteSet.piraeus.linbit.com piraeus-op-ns -o yaml
kind: LinstorSatelliteSet
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
    zfsPools:
    - name: my-linstor-zpool
      zPool: for-linstor
      thin: true
```

### At install time

At install time, by setting the value of `operator.satelliteSet.storagePools` when running helm install.

First create a file with the storage configuration like:

```yaml
operator:
  satelliteSet:
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
you can edit the LinstorSatelliteSet configuration like this:

```
$ kubectl edit LinstorSatelliteSet.piraeus.linbit.com <satellitesetname>
```

The storage pool configuration can be updated like in the example above.

## Preparing physical devices

By default, LINSTOR expects the referenced VolumeGroups, ThinPools and so on to be present. You can use the
`devicePaths: []` option to let LINSTOR automatically prepare devices for the pool. Eligible for automatic configuration
are block devices that:

* Are a root device (no partition)
* do not contain partition information
* have more than 1 GiB

To enable automatic configuration of devices, set the `devicePaths` key on `storagePools` entries:

```yaml
  storagePools:
    lvmPools:
    - name: lvm-thick
      volumeGroup: drbdpool
      devicePaths:
      - /dev/vdb
    lvmThinPools:
    - name: lvm-thin
      thinVolume: thinpool
      volumeGroup: linstor_thinpool
      devicePaths:
      - /dev/vdc
      - /dev/vdd
```

Currently, this method supports creation of LVM and LVMTHIN storage pools.

#### `lvmPools` configuration
* `name` name of the LINSTOR storage pool. Required
* `volumeGroup` name of the VG to create. Required
* `devicePaths` devices to configure for this pool. Must be empty and >= 1GiB to be recognized. Optional
* `raidLevel` LVM raid level. Optional
* `vdo` Enable [VDO] (requires VDO tools in the satellite). Optional
* `vdoLogicalSizeKib` Size of the created VG (expected to be bigger than the backing devices by using VDO). Optional
* `vdoSlabSizeKib` Slab size for VDO. Optional

[VDO]: https://www.redhat.com/en/blog/look-vdo-new-linux-compression-layer

#### `lvmThinPools` configuration
* `name` name of the LINSTOR storage pool. Required
* `volumeGroup` VG to use for the thin pool. If you want to use `devicePaths`, you must set this to `""`.
  This is required because LINSTOR does not allow configuration of the VG name when preparing devices.
* `thinVolume` name of the thinpool. Required
* `devicePaths` devices to configure for this pool. Must be empty and >= 1GiB to be recognized. Optional
* `raidLevel` LVM raid level. Optional

NOTE: The volume group created by LINSTOR for LVMTHIN pools will always follow the scheme "linstor_$THINPOOL".

#### `zfsPools` configuration
* `name` name of the LINSTOR storage pool. Required
* `zPool` name of the zpool to use. Must already be present on all machines. Required
* `thin` `true` to use thin provisioning, `false` otherwise. Required

## Using `automaticStorageType` (DEPRECATED)

_ALL_ eligible devices will be prepared according to the value of `operator.satelliteSet.automaticStorageType`, unless
they are already prepared using the `storagePools` section. Devices are added to a storage pool based on the device
name (i.e. all `/dev/nvme1` devices will be part of the pool `autopool-nvme1`)

The possible values for `operator.satelliteSet.automaticStorageType`:

* `None` no automatic set up (default)
* `LVM` create a LVM (thick) storage pool
* `LVMTHIN` create a LVM thin storage pool
* `ZFS` create a ZFS based storage pool (**UNTESTED**)
