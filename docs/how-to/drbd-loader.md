# How to Configure the DRBD Module Loader

This guide shows you how to configure various aspects of the DRBD® Module Loader.

To follow the steps in this guide, you should be familiar with editing `LinstorSatelliteConfiguration` resources.

DRBD Module Loader is the component responsible for making the DRBD kernel module available, as well as loading other
useful kernel modules for Piraeus.

The following modules are loaded if available:

| Module                    | Purpose                                          |
|---------------------------|--------------------------------------------------|
| `libcrc32c`               | dependency for DRBD                              |
| `nvmet_rdma`, `nvme_rdma` | LINSTOR NVME layer                               |
| `loop`                    | LINSTOR when using loop devices as backing disks |
| `dm_writecache`           | LINSTOR writecache layer                         |
| `dm_cache`                | LINSTOR cache layer                              |
| `dm_thin_pool`            | LINSTOR thin-provisioned storage                 |
| `dm_snapshot`             | LINSTOR Snapshots                                |
| `dm_crypt`                | LINSTOR encrypted volumes                        |

## Disable the DRBD Module Loader

In some circumstances it might be necessary to disable the DRBD Module Loader entirely. For example, you are using an
immutable operating system, and DRBD and other modules are loaded as part of the host configuration.

To disable the DRBD Module Loader completely, use the following configuration:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: no-loader
spec:
  patches:
    - target:
        kind: Pod
        name: satellite
      patch: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: satellite
        spec:
          initContainers:
          - name: drbd-module-loader
            $patch: delete
```

## Select a Different DRBD Loader Version

By default, the Operator will try to find a DRBD Module Loader matching the host operating system. The host distribution
is determined by inspecting the `.status.nodeInfo.osImage` field of the Kubernetes Node resource. A user-defined image
can be used if the automatic mapping does not succeed or if you have different module loading requirements.

This configuration overrides the chosen DRBD Module Loader image with a user-defined image `example.com/drbd-loader:v9`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: custom-drbd-module-loader-image
spec:
  patches:
    - target:
        kind: Pod
        name: satellite
      patch: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: satellite
        spec:
          initContainers:
          - name: drbd-module-loader
            image: example.com/drbd-loader:v9
```

Piraeus maintains the following images:

| Image                                       | Distribution                       |
|---------------------------------------------|------------------------------------|
| `quay.io/piraeusdatastore/drbd9-almalinux9` | RedHat Enterprise Linux 9 rebuilds |
| `quay.io/piraeusdatastore/drbd9-almalinux8` | RedHat Enterprise Linux 8 rebuilds |
| `quay.io/piraeusdatastore/drbd9-centos7`    | RedHat Enterprise Linux 7 rebuilds |
| `quay.io/piraeusdatastore/drbd9-jammy`      | Ubuntu 22.04                       |
| `quay.io/piraeusdatastore/drbd9-focal`      | Ubuntu 20.04                       |
| `quay.io/piraeusdatastore/drbd9-bionic`     | Ubuntu 18.04                       |
| `quay.io/piraeusdatastore/drbd9-flatcar`    | Flatcar Container Linux            |

You can create a loader image for your own distribution using the [Piraeus sources](https://github.com/piraeusdatastore/piraeus/tree/master/dockerfiles/drbd-driver-loader)
as reference.

## Change the Way DRBD Module Loader Loads the DRBD Module

By default, DRBD Module Loader will try to build the kernel module from source. The loader can also be configured to load the
module from a Debian or RPM package included in the image, or skip loading DRBD entirely.

To change the behaviour of the DRBD Loader, set the `LB_HOW` environment variable:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: no-drbd-module-loader
spec:
  patches:
    - target:
        kind: Pod
        name: satellite
      patch: |
        apiVersion: v1
        kind: Pod
        metadata:
          name: satellite
        spec:
          initContainers:
          - name: drbd-module-loader
            env:
            - name: LB_HOW
              value: deps_only
```

| `LB_HOW`          | Loader behaviour                                                                                                                                   |
|-------------------|----------------------------------------------------------------------------------------------------------------------------------------------------|
| `compile`         | The default. Build the DRBD module from source and try to load all optional modules from the host.                                                 |
| `shipped_modules` | Searches for `.rpm` or `.deb` packages at  `/pkgs` and inserts contained the DRBD modules. Optional modules are loaded from the host if available. |
| `deps_only`       | Only try loading the optional modules. No DRBD module will be loaded.                                                                              |
