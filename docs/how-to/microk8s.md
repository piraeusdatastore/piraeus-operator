# How to Configure Piraeus Datastore on MicroK8s

This guide shows you how to configure Piraeus Datastore when using [MicroK8s](https://microk8s.io/).

To complete this guide, you should be familiar with:

* editing `LinstorCluster` resources.

## Configure the CSI Driver

MicroK8s is distributed as a [Snap](https://ubuntu.com/core/services/guide/snaps-intro). Because Snaps store their state
in a separate directory (`/var/snap`) the LINSTOR CSI Driver needs to be updated to use a new path for mounting volumes.

To change the LINSTOR CSI Driver, so that it uses the MicroK8s state paths, apply the following `LinstorCluster`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  patches:
    - target:
        name: linstor-csi-node
        kind: DaemonSet
      patch: |
        apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: linstor-csi-node
        spec:
          template:
            spec:
              containers:
              - name: linstor-csi
                volumeMounts:
                - mountPath: /var/lib/kubelet
                  name: publish-dir
                  $patch: delete
                - mountPath: /var/snap/microk8s/common/var/lib/kubelet
                  name: publish-dir
                  mountPropagation: Bidirectional
```
