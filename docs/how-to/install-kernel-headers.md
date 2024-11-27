# Installing Linux Kernel Headers

Piraeus Datastore uses [DRBD®](https://linbit.com/DRBD/) to mirror your volume data to one or more nodes. This document
aims to describe the way to install the necessary files for common Linux distributions.

To check if your nodes already have the necessary files installed, try running:

```
$ test -d /lib/modules/$(uname -r)/build/ && echo found headers
```

If the command prints `found headers`, your nodes are good to go.

## Installing on Ubuntu

Installation of Linux Kernel headers on Ubuntu can be done through the `apt` package manager:

```
$ sudo apt-get update
$ sudo apt-get install -y linux-headers-$(uname -r)
```

In addition, you can install the `linux-headers-virtual` package. This causes `apt upgrade` to install the headers
matching any newly installed kernel versions.

```
$ sudo apt-get update
$ sudo apt-get install -y linux-headers-virtual
```

## Installing on Debian

Installation of Linux Kernel headers on Debian can be done through the `apt` package manager:

```
$ sudo apt-get update
$ sudo apt-get install -y linux-headers-$(uname -r)
```

In addition, you can install an additional package, that causes `apt` to also install Kernel headers on upgrade:

```
$ sudo apt-get update
$ sudo apt-get install -y linux-headers-$(dpkg --print-architecture)
```

## Installing on RedHat Enterprise Linux

Installing on RedHat Enterprise Linux or compatible distributions, such as AlmaLinux or Rocky Linux can be done through
the `dnf` package manager:

```
$ sudo dnf install -y kernel-devel-$(uname -r)
```

In addition, you can install an additional package, that causes `yum` to also install Kernel headers on upgrade:

```
$ sudo dnf install -y kernel-devel
```
