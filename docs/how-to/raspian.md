# How to Load DRBD on Raspberry Pi OS / Raspian

This guide shows you how to set the DRBD® Module Loader when using [Raspberry Pi OS / Raspian].

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.

## Configure the DRBD module loader for Raspberry Pi OS / Raspian

By default, the DRBD Module Loader will try to find the necessary header files to build DRBD from source on the host
system. In [Raspberry Pi OS / Raspian] these files reference specific tools that are not mounted into the build
container by default, leading to errors such as:

```
make[2]: *** No rule to make target '/usr/src/linux-headers-6.6.51+rpt-common-rpi/scripts/Makefile.extrawarn'.  Stop.
make[1]: *** [Makefile:236: compat.h] Error 2
make: *** [Makefile:131: module] Error 2
```

To resolve this issue, apply a LINSTOR Satellite configuration that passes the tool directory into the build container:

```yaml hl_lines="14 18"
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: raspi-kbuild
spec:
  nodeSelector:
    kubernetes.io/arch: arm64
  podTemplate:
    spec:
      volumes:
      - name: raspi-kbuild-tools
        hostPath:
          type: Directory
          path: /usr/lib/linux-kbuild-6.6.51+rpt/
      initContainers:
      - name: drbd-module-loader
        volumeMounts:
        - mountPath:  /usr/lib/linux-kbuild-6.6.51+rpt/
          name: raspi-kbuild-tools
          readOnly: true
```

The specific directory that needs to be passed may change over time. Update the configuration above with the directory
determined by running the following command on the host:

```
$ dirname $(readlink -f /usr/src/linux-headers-$(uname -r)/scripts)
/usr/lib/linux-kbuild-6.6.51+rpt/
```

Apply the update configuration file. The LINSTOR Satellite containers will restart and start building the DRBD module.

[Raspberry Pi OS / Raspian]: https://www.raspberrypi.com/software/
