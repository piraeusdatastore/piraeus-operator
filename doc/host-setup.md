# Preparing the host for DRBD

Piraeus depends on DRBD for data replication across multiple nodes. [DRBD] is a kernel module that must be loaded on
every node that should access the storage. Depending on your exact host OS and set up, you can choose **one** from the
following options to load DRBD:

[DRBD]: https://github.com/LINBIT/drbd/

## Build and load DRBD using the Kernel Module Injection Image (Preferred)

Every satellite container created by the operator will use an [InitContainer] to ensure DRBD is available. This init
step can also compile and load DRBD directly. To build DRBD, the container needs access to the kernel sources. It will
look for the sources on the host under `/usr/src` and `/lib/modules/$(uname -r)`. Most distributions provide a convenient package
for the kernel source:

* Ubuntu: `apt-get install linux-headers-$(uname -r)`
* CentOS: `yum install kernel-devel-$(uname -r)`

Install these packages and use the injector image matching your distribution:

* CentOS 7: `quay.io/piraeusdatastore/drbd9-centos7`
* CentOS 8: `quay.io/piraeusdatastore/drbd9-centos8` (See [#137] for CentOS 8 Stream)
* Ubuntu 20.04: `quay.io/piraeusdatastore/drbd9-focal`
* Ubuntu 18.04: `quay.io/piraeusdatastore/drbd9-bionic`

You can set the image by passing `--set operator.satelliteSet.kernelModuleInjectionImage=<image>` to Helm.

[InitContainer]: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
[#137]: https://github.com/piraeusdatastore/piraeus-operator/issues/137

## Install DRBD on the host

Some users may prefer to install the DRBD directly on the host. While a DRBD package is available on some distributions,
they are often out of date and can't be used for Piraeus. Instead, you should follow the [link to DRBDs user guide] to
ensure you install a recent enough version. After that, please read the instructions below to make DRBD ready for
containerized workloads.

[link to DRBDs user guide]: https://www.linbit.com/drbd-user-guide/users-guide-9-0/#p-build-install-configure

You have to disable DRBD's user mode helper. This feature of the DRBD module enables running user configured commands on
changes in DRBDs state. When using it with containers, it could confuse programs that expect to know of all configured
DRBD resources, such as the default `drbdadm`. This command then reports an error, which is interpreted by DRBD to abort
the current transition. In the end, you are left with a lot of `StandAlone` resources that can't be used.

To prevent this issue, you have to set the DRBD module parameter `usermode_helper` to `disabled`. To check the current
value, run:

```
# cat /sys/module/drbd/parameters/usermode_helper
/sbin/drbdadm
```

To disable the helper without reloading the module _AND_ ensure it persist after reboots use:

```
# echo -n "disabled" > /sys/module/drbd/parameters/usermode_helper
# echo "options drbd usermode_helper=disabled" > /etc/modprobe.d/drbd.conf
```

Checking the current value again will reveal that the helper is now disabled

```
# cat /sys/module/drbd/parameters/usermode_helper
disabled
```
