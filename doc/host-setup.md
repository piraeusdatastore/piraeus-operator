# Preparing the host for DRBD

Piraeus depends on DRBD for data replication across multiple nodes. [DRBD] is a kernel module that must be loaded on
every node that should access the storage. Depending on your exact host OS and set up, you can choose **one** from the
following options to load DRBD:

[DRBD]: https://github.com/LINBIT/drbd/

## Software compatibility

Certain pieces of software conflict with Piraeus and either need to be configured to not interfere with Piraeus or they need
to be disabled to ensure Piraeus can function properly.

### Multipathd

If you are using `multipathd`, you will most likely need to change some configuration settings to ensure that Piraeus can function
properly as `multipathd` by default will interfere with Piraeus. `multipathd` by default opens up DRBD devices which prevents Piraeus
from working properly. To configure `multipathd`, put the following in the file `/etc/multipath/conf.d/drbd.conf`:

```
blacklist {
        devnode "^drbd[0-9]+"
}
```

You will most likely need to create the directory `/etc/multipath/conf.d/` if you have not made any other configuration changes to
`multipathd` before. Note that `multipathd` is enabled on Ubuntu 20.04 by default and this configuration change will need to be made
on fresh installations of Ubuntu 20.04 before Piraeus will be able to function properly.

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

### Flatcar Container Linux

Flatcar Container Linux requires building DRBD from source. Piraeus by default bind mounts `/usr/src` into the injection
container, as that is required on most distributions. This will fail on Flatcar Container Linux, as that directory
does not exist there, and can't easily be created (`/usr` is always read-only).

For this case, you can disable mounting of `/usr/src` by passing
`--set operator.satelliteSet.kernelModuleInjectionAdditionalSourceDirectory=none` to Helm.

### Injector image for compiling without headers on host

Installing the kernel headers is not always feasible on the host. A prime example is Fedora CoreOS (FCOS), which uses
older versions of the mainline Fedora kernel. Often, the matching kernel-headers are no longer available to install and
manually manipulating the `ostree` is cumbersome. In this section we will show you how to build your own injector image
that automatically fetches the right kernel headers using FCOS as example.

**Note**: While DRBD tries to stay compatible with the mainline kernel, there is no guarantee that a released version
will compile for every kernel.

For our custom injector image for FCOS we will need two files, a `entry-override.sh` script and a `Dockerfile`. We will
use `entry-override.sh` as our entrypoint in the injector image, i.e. the script that gets executed when the container
starts running. It will download the kernel headers and invoke the DRBD build process:
```shell
# download package matching the current kernel from build artifacts
koji download-build --rpm --arch=x86_64 kernel-devel-$(uname -r)
# unpack the kernel headers in the /opt directory. The default /usr/src directory is mounted read only at this point and cannot be used
rpm2cpio ./kernel-devel-$(uname -r).rpm | cpio -idD /opt

# set the KDIR variable, telling DRBD to search for the headers in /opt
export KDIR=/opt/usr/src/kernels/$(uname -r)
exec /entry.sh "$@"
```

The `/entry.sh` script is taken from the existing injector image. It will invoke the right build steps and ensure all
kernel modules are loaded. We only influence where it searches for the kernel headers by setting the `KDIR` variable.

For our `Dockerfile`, we first pull in an existing injector image to access the DRBD source and the regular injector
image entrypoint. We base our actual injector image on Fedora 33 as the FCOS kernel (currently) also comes from that
distribution.
```dockerfile
# Pull in DRBD sources and entrypoint
FROM quay.io/piraeusdatastore/drbd9-centos8:latest as SOURCE

FROM fedora:33

# Packages needed for the DRBD build process.
RUN yum install -y gcc make coccinelle koji cpio patch perl-interpreter diffutils kmod \
  && yum clean all

# Our script from above
COPY entry-override.sh /
# The DRBD sources and build script from an existing injector image
COPY --from=SOURCE /entry.sh /drbd.tar.gz /

ENV LB_HOW=compile
# Use our download script as entrypoint
ENTRYPOINT ["sh", "-e", "/entry-override.sh"]
```

Build the image, tag it and push it to a registry accessible to your cluster. You can start using your custom injector
image by passing `--set operator.satelliteSet.kernelModuleInjectionImage=<your-custom-image>` to Helm.

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

## Disable Lvmetad on CentOS 7 and Ubuntu 18.04

Reference: https://access.redhat.com/solutions/2053483

CentOS 7 and Ubuntu 18.04 by default use `lvmetad`,  a metadata caching daemon for LVM. The daemon improves performance of LVM commands by avoiding rescanning the disks on every execution. `piraeus-ns-node` pods run LVM commands inside a container without access to lvmetad. Since that means  `lvmetad` would get out of sync, it is recommended to disable `lvmetad` in CentOS 7 and Ubuntu 18.04 Bionic.

Note: Newer distributions no longer include with `lvmetad`, no changes necessary.

Follow below steps to disable lvmetad completely: 
```bash
$ systemctl stop lvm2-lvmetad.socket lvm2-lvmetad.service

$ systemctl disable lvm2-lvmetad.service

$ systemctl mask lvm2-lvmetad.socket

$ sed -i 's/use_lvmetad = 1/use_lvmetad = 0/' /etc/lvm/lvm.conf

$ cat /etc/lvm/lvm.conf | grep use_lvmetad
use_lvmetad = 0
```
If lvm is used for root, initial RAM has to be updated:
```bash
# CentOS 7
$ cp -vf /boot/initramfs-$(uname -r).img /boot/initramfs-$(uname -r).img.$(date +%m-%d-%H%M%S).bak
$ dracut -f -v

# Ubuntu 18.04
$ cp -vf /boot/initrd.img-$(uname -r) /boot/initd.img.$(uname -r).$(date +%m-%d-%H%M%S).bak
$ update-initramfs -c -k $(uname -r)
$ update-grub
```

