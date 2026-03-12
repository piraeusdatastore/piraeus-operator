# How to Configure Piraeus Datastore on minikube

This guide shows you how to configure Piraeus Datastore when using [minikube](https://minikube.sigs.k8s.io/docs/).

To complete this guide, you should be familiar with:

* editing `LinstorSatelliteConfiguration` resources.

## Configure the DRBD Module Loader for minikube

minikube uses a stripped down Linux system to run its nodes. The system does not include the required sources to build
DRBD. In order for the DRBD Module Loader to work, the Linux sources need to be fetched on demand.

Configure the DRBD Module Loader to fetch the required Linux sources on demand by applying the following configuration:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: minikube
spec:
  nodeAffinity:
    nodeSelectorTerms:
    - matchExpressions:
      - key: minikube.k8s.io/name
        operator: Exists
  podTemplate:
    spec:
      volumes:
      - name: usr-src
        hostPath:
          $patch: delete
        emptyDir: {}
      initContainers:
      - name: drbd-module-loader
        command:
        - /bin/sh
        - -exc
        - |
          if [ ! -e /proc/drbd ]; then
            apt-get update
            apt-get install -y flex bison bc libssl-dev libelf-dev
            curl -fsSL https://www.kernel.org/pub/linux/kernel/v$(uname -r | cut -d. -f1).x/linux-$(uname -r).tar.gz | tar xzC /usr/src
            cd /usr/src/linux-$(uname -r)
            zcat /proc/config.gz > .config
            make olddefconfig
            make modules_prepare
            ln -snf /usr/lib/modules/$(uname -r)/Module.symvers .
            cd /
          fi
          exec env "LB_MAKEOPTS=KDIR=/usr/src/linux-$(uname -r)" LB_HOW=compile /entry.sh
        securityContext:
          privileged: true
          readOnlyRootFilesystem: false
        volumeMounts:
        - name: usr-src
          mountPath: /usr/src
          readOnly: false
```

Applying this configuration will let the `drbd-module-loader` container of the LINSTOR Satellite Pods complete.
