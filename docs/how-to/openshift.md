# How to Load DRBD on OpenShift

This guide shows you how to set the DRBD® Module Loader when using OpenShift.

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.
* using the `oc` command line tool to access the OpenShift cluster.

## Find the Driver Toolkit Image

The [Driver Toolkit](https://github.com/openshift/driver-toolkit) image contains the necessary files for building DRBD.
Every OpenShift release includes a version of the Driver Toolkit image matching the operating system version of the
release. You can use `oc` to print the correct image version:

```
$ oc adm release info --image-for=driver-toolkit
quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:1328c4e7944b6d8eda40a8f789471a1aec63abda75ac1199ce098b965ec16709
```

## Configure the DRBD Module Loader

By default, the DRBD Module Loader will try to find the necessary header files to build DRBD from source on the host
system. In OpenShift, these header files are not included in the host system. Instead, they are included in the Driver
Toolkit image.

To change the DRBD Module Loader, so that it uses the header files included in the Driver Toolkit image, apply the
following `LinstorSatelliteConfiguration`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: openshift-loader-override
spec:
  podTemplate:
    spec:
      volumes:
        - name: usr-src
          emptyDir: { }
          hostPath:
            $patch: delete
      initContainers:
        - name: kernel-header-copy
          image: quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:1328c4e7944b6d8eda40a8f789471a1aec63abda75ac1199ce098b965ec16709
          args:
            - cp
            - -avt
            - /container/usr/src
            - /usr/src/kernels
          volumeMounts:
            - name: usr-src
              mountPath: /container/usr/src
          securityContext:
            privileged: true
        - name: drbd-module-loader
          securityContext:
            privileged: true
```

**NOTE**: Replace the `image` of the `kernel-header-copy` container with the image returned by `oc adm release info`.

After the automatic restart of the LINSTOR® Satellite Pods, DRBD will be built from source, using the correct header
files.

## OpenShift Update Considerations

OpenShift updates also update the host operating system. Since every node will be rebooted during the update, the DRBD
Module loader needs to rebuild DRBD once after the restart for the new host operating system.

To ensure that the DRBD rebuild is successful, you need to update the image in the `openshift-loader-override`
configuration before starting the upgrade process. You can extract the Driver Toolkit image for the new OpenShift
version before starting the upgrade like this:

```
$ oc adm release info 4.12.2 --image-for=driver-toolkit
quay.io/openshift-release-dev/ocp-v4.0-art-dev@sha256:1328c4e7944b6d8eda40a8f789471a1aec63abda75ac1199ce098b965ec16709
```
