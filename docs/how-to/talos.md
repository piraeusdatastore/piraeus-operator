# How to Load DRBD on Talos Linux

This guide shows you how to set the DRBD® Module Loader when using [Talos Linux].

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.
* using the `talosctl` command line to to access Talos Linux nodes.
* using the `kubectl` command line tool to access the Kubernetes cluster.

## Configure Talos system extension for DRBD

By default, the DRBD Module Loader will try to find the necessary header files to build DRBD from source on the host system. In [Talos Linux] these header files are not included in the host system. Instead, the Kernel modules is packed into a system extension.

Ensure Talos has the correct `drbd` [system extension](https://github.com/siderolabs/extensions) loaded for the running Kernel.

This is done by building a install image from the [talos factory](https://factory.talos.dev) with drbd included

You will also need to update the machine config to set the following kernel module parameters:

```yaml
machine:
  kernel:
    modules:
      - name: drbd
        parameters:
          - usermode_helper=disabled
      - name: drbd_transport_tcp
```

Validate `drbd` module is loaded:
```shell
$ talosctl -n <NODE_IP> read /proc/modules
drbd_transport_tcp 28672 - - Live 0xffffffffc046c000 (O)
drbd 643072 - - Live 0xffffffffc03b9000 (O)
```

Validate `drbd` module parameter `usermode_helper` is set to `disabled`:
```shell
$ talosctl -n <NODE_IP> read /sys/module/drbd/parameters/usermode_helper
disabled
```

## Configure the DRBD Module Loader

To change the DRBD Module Loader, so that it uses the modules provided by system extension, apply the following `LinstorSatelliteConfiguration`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: talos-loader-override
spec:
  podTemplate:
    spec:
      initContainers:
        - name: drbd-shutdown-guard
          $patch: delete
        - name: drbd-module-loader
          $patch: delete
      volumes:
        - name: run-systemd-system
          $patch: delete
        - name: run-drbd-shutdown-guard
          $patch: delete
        - name: systemd-bus-socket
          $patch: delete
        - name: lib-modules
          $patch: delete
        - name: usr-src
          $patch: delete
        - name: etc-lvm-backup
          hostPath:
            path: /var/etc/lvm/backup
            type: DirectoryOrCreate
        - name: etc-lvm-archive
          hostPath:
            path: /var/etc/lvm/archive
            type: DirectoryOrCreate
```

Explanation:

- `/etc/lvm/*` is read-only in Talos and therefore can't be used. Let's use `/var/etc/lvm/*` instead.
- Talos does not ship with Systemd, so everything Systemd related needs to be removed
- `/usr/lib/modules` and `/usr/src` are not needed as the Kernel module is already compiled and needs just to be used.

[Talos Linux]: https://talos.dev
