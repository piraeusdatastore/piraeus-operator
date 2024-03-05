# How to Load DRBD on Flatcar Container Linux

This guide shows you how to set up the DRBD® Module Loader when using [Flatcar Container Linux](https://www.flatcar.org).

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.

## Configure the DRBD Module Loader

Flatcar Container Linux uses a read-only `/usr` file system. For building DRBD from source on Flatcar Container Linux,
the default bind mount for the not existing `/usr/src` directory needs to be disabled for the `drbd-module-loader` init container.

To change the configuration for the `drbd-module-loader` container, apply the following `LinstorSatelliteConfiguration`:

```yaml
---
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: no-usr-src-mount
spec:
  podTemplate:
    spec:
      volumes:
        - name: usr-src
          $patch: delete
      initContainers:
        - name: drbd-module-loader
          volumeMounts:
            - mountPath: /usr/src
              name: usr-src
              $patch: delete
```
