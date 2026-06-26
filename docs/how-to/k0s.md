# How to Configure Piraeus Datastore on k0s

This guide shows you how to configure Piraeus Datastore when using [k0s](https://k0sproject.io).

To complete this guide, you should be familiar with:

* editing `LinstorCluster` resources.

## Configure the CSI Driver

Because k0s store their state in a separate directory (`/var/lib/k0s`) the LINSTOR CSI Driver needs to be updated to use a new path for mounting volumes.

To change the LINSTOR CSI Driver, so that it uses the k0s state paths, apply the following `LinstorCluster`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  csiNode:
    enabled: true
    podTemplate:
      spec:
        containers:
          - name: linstor-csi
            volumeMounts:
            - mountPath: /var/lib/k0s/kubelet
              name: publish-dir
              mountPropagation: Bidirectional
          - name: csi-node-driver-registrar
            env:
              - name: KUBELET_REGISTRATION_PATH
                value: /var/lib/k0s/kubelet/plugins/linstor.csi.linbit.com/csi.sock
        volumes:
         - name: publish-dir
           hostPath:
             path: /var/lib/k0s/kubelet
         - name: registration-dir
           hostPath:
             path: /var/lib/k0s/kubelet/plugins_registry
         - name: plugin-dir
           hostPath:
              path: /var/lib/k0s/kubelet/plugins/linstor.csi.linbit.com
```
