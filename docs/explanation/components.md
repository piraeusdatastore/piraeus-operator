# Understanding Piraeus Datastore Components

Piraeus Datastore consists of several different components. Each component runs as a separate Deployment, DaemonSet or
plain Pod.

All Pods are labelled according to their component. You can check the Pods running in your own cluster by
running the following `kubectl` command:

```
$ kubectl get pods '-ocustom-columns=NAME:.metadata.name,COMPONENT:.metadata.labels.app\.kubernetes\.io/component'
NAME                                                   COMPONENT
ha-controller-vd82w                                    ha-controller
linstor-controller-6c8f8dc47-cm8hr                     linstor-controller
linstor-csi-controller-59b9968b86-ftl76                linstor-csi-controller
linstor-csi-node-hcmk9                                 linstor-csi-node
linstor-satellite-k8s-10.test-66687                    linstor-satellite
piraeus-operator-controller-manager-6dcfcb4568-6jntp   piraeus-operator
piraeus-operator-gencert-59449cb449-nzg6z              piraeus-operator-gencert
```

# `piraeus-operator`

The Piraeus Operator creates and maintains the other components, except for the `piraeus-operator-gencert` component.

Along with deploying the needed Kubernetes resources, it maintains the LINSTOR® Cluster state by:

* Registering satellites
* Creating storage pools
* Maintaining node labels

# `piraeus-operator-gencert`

The "generate certificate" Pod creates and maintains the TLS key and certificate used by the Piraeus Operator.
The TLS secret, named `webhook-server-cert`, is needed to start the Piraeus Operator Pod. In addition, this Pod
keeps the `ValidatingWebhookConfiguration` for the Piraeus Operator up-to-date.

The Piraeus Operator needs a TLS certificate to serve the validation endpoint for the custom resources it maintains.

Historically, TLS certificates where created by `cert-manager`. This was removed to reduce the number of dependencies
for deploying Piraeus Datastore.

# `linstor-controller`

The LINSTOR Controller is responsible for resource placement, resource configuration, and orchestration of any
operational processes that require a view of the whole cluster. It maintains a database of all the configuration
information for the whole cluster, stored as Kubernetes objects.

The LINSTOR Controller connects to the LINSTOR Satellites and sends them instructions for achieving the desired cluster
state.

It provides an external API used by LINSTOR CSI and Piraeus Operator to change the cluster state.

# `linstor-satellite`

The LINSTOR Satellite service runs on each node. It acts as the local configuration agent for LINSTOR managed storage.
It is stateless and receives all the information it needs from the LINSTOR Controller.

Satellites are started as DaemonSets, managed by the Piraeus Operator. We deploy one DaemonSet per node, this enables
having per-node configuration and customization of the Satellite Pods.

Satellites interact with the host operating system directly and are deployed as privileged containers. Integration
with the host operating system also leads to two noteworthy interactions with [Linux namespaces]:

* Any DRBD® device will inherit the network namespace of the Satellite Pods. Unless the Satellites are using
  host networking, DRBD will not be able to replicate data without a running Satellite Pod. See the
  [host networking guide] for more information.
* The Satellite process is spawned in a separate UTS namespace: this allows us to keep control of the hostname reported
  to DRBD tools, even when the Pod is using a generated name from the DaemonSet. Thus, DRBD connections will always use
  the Kubernetes node name.

  The use of separate UTS namespace should be completely transparent to users: running `kubectl exec ...` on a satellite
  Pod will drop you into this namespace, enabling you to run `drbdadm` commands as expected.

[Linux namespaces]: https://man7.org/linux/man-pages/man7/namespaces.7.html
[host networking guide]: ../how-to/drbd-host-networking.md

# `linstor-csi-controller`

The LINSTOR CSI Controller Pod creates, modifies and deletes volumes and snapshots. It translates the state of Kubernetes
resources (`StorageClass`, `PersistentVolumeClaims`, `VolumeSnapshots`) into their equivalent in LINSTOR.

# `linstor-csi-node`

The LINSTOR CSI Node Pods execute mount and unmount operations. Mount and Unmount are initiated by kubelet before
starting a Pod with a Piraeus volume.

They are deployed as a DaemonSet on every node in the cluster by default. There needs to be a LINSTOR Satellite running
on the same node as a CSI Node Pod.

# `ha-controller`

The Piraeus High Availability Controller will speed up the fail-over process for stateful workloads using Piraeus for
storage.

It is deployed on every node in the cluster and listens for DRBD® events to detect storage failures on other nodes. It
evicts Pods when it detects that the storage on their node is inaccessible.
