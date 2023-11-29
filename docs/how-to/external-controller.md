# How to Use an Existing LINSTOR Cluster

This guide shows you how to connect Piraeus with an existing LINSTOR® Cluster, managed outside Kubernetes.

To complete this guide, you should be familiar with:

* editing `LinstorCluster` resources.

## Configure the Piraeus Resources

To use an externally managed LINSTOR Cluster, specify the URL of the LINSTOR Controller in the `LinstorCluster`
resource. In the following example, the Controller is reachable at `http://linstor-controller.example.com:3370`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  externalController:
    url: http://linstor-controller.example.com:3370
```

After applying the `LinstorCluster`, Piraeus will make the following changes to the default deployment:

* The `linstor-controller` Deployment is removed
* The `linstor-csi-controller` and `linstor-csi-node` resources will reference the external controller.

## Configuring Host Networking for LINSTOR Satellites

Normally the Pod network is not reachable from outside the Kubernetes Cluster.
In this case the existing LINSTOR Controller won't be able to communicate with the Satellites in the Kubernetes cluster.
So you should configure your Satellites to use [host networking](./drbd-host-networking.md).

To use host networking, apply the following configuration resource:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: host-network
spec:
  podTemplate:
    spec:
      hostNetwork: true
```

## Verifying the Configuration

To verify that the external LINSTOR Controller is configured, check:

* The `Available` condition on the `LinstorCluster` resource reports the expected Controller URL:
  ```
  $ kubectl get LinstorCluster -ojsonpath='{.items[].status.conditions[?(@.type=="Available")].message}{"\n"}'
  Controller 1.20.3 (API: 1.16.0, Git: 8d19a891df018f6e3d40538d809904f024bfe361) reachable at 'http://linstor-controller.example.com:3370'
  ```
* The `linstor-csi-controller` Deployment uses the expected URL:
  ```
  $ kubectl get -n piraeus-datastore deployment linstor-csi-controller -ojsonpath='{.spec.template.spec.containers[?(@.name=="linstor-csi")].env[?(@.name=="LS_CONTROLLERS")].value}{"\n"}'
  http://linstor-controller.example.com:3370
  ```
* The `linstor-csi-node` Deployment uses the expected URL:
  ```
  $ kubectl get -n piraeus-datastore daemonset linstor-csi-node -ojsonpath='{.spec.template.spec.containers[?(@.name=="linstor-csi")].env[?(@.name=="LS_CONTROLLERS")].value}{"\n"}'
  http://linstor-controller.example.com:3370
  ```
* The Kubernetes nodes are registered as Satellite nodes on the LINSTOR Controller:
  ```
  $ kubectl get nodes -owide
  NAME               STATUS   ROLES           AGE   VERSION   INTERNAL-IP      EXTERNAL-IP   OS-IMAGE                    KERNEL-VERSION                 CONTAINER-RUNTIME
  k8s-1-26-10.test   Ready    control-plane   22m   v1.26.3   192.168.122.10   <none>        AlmaLinux 9.1 (Lime Lynx)   5.14.0-162.22.2.el9_1.x86_64   containerd://1.6.20
  ...
  $ linstor node list
  ╭─────────────────────────────────────────────────────────────────────╮
  ┊ Node             ┊ NodeType  ┊ Addresses                   ┊ State  ┊
  ╞═════════════════════════════════════════════════════════════════════╡
  ┊ k8s-1-26-10.test ┊ SATELLITE ┊ 192.168.122.10:3366 (PLAIN) ┊ Online ┊
  ...
  ```
