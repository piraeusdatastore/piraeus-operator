# Distributions

Some Kubernetes distributions require changes to the default configuration to work.

## Openshift

We provide an example override file for Openshift. You can find it [here](../charts/piraeus/values-openshift.yaml).

Depending on your host OS, you might also need to create your own DRBD injector image. Read more
[here](./host-setup.md#injector-image-for-compiling-without-headers-on-host).

## microk8s

MicroK8s doesn't use the traditional `/var/lib/kubelet` directory for kubelet data. Instead, it uses
`/var/snap/microk8s/common/var/lib/kubelet`. To get the CSI driver to work on microk8s, use the following override
during installation:

```
--set csi.kubeletPath=/var/snap/microk8s/common/var/lib/kubelet
```

## k0s

MicroK8s doesn't use the traditional `/var/lib/kubelet` directory for kubelet data. Instead, it uses
`/var/lib/k0s/kubelet`. To get the CSI driver to work on k0s, use the following override
during installation:

```
--set csi.kubeletPath=/var/lib/k0s/kubelet
```
