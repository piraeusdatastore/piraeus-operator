# How to Troubleshoot Piraeus Datastore

This guide helps you diagnose common problems with Piraeus Datastore.

Piraeus Datastore reconciles in two tiers: the Operator turns a `LinstorCluster` into cluster-wide workloads and one
`LinstorSatellite` per node, and each Satellite registers its node and storage pools in LINSTOR. A volume then flows
from a `StorageClass` through the CSI driver into LINSTOR and onto those nodes. Knowing which tier is stuck tells you
how to troubleshoot.

This guide is structured as follows:

1. [**The Basic Health Check**](#basic-health-check) helps you to determine which tier is stuck and hence which of the
   troubleshooting sections to read.
2. [**The deployment itself is unhealthy**](#troubleshooting-the-operator-deployment) — the Operator, the LINSTOR®
   Controller, or the per-node Satellites are not coming up.
3. [**The deployment is healthy, but a volume will not provision or mount**](#troubleshooting-volume-creation).
4. [**Collecting Diagnostics for a Bug Report**](#collecting-diagnostics-for-a-bug-report).

All examples assume the default namespace `piraeus-datastore`. Adjust `-n piraeus-datastore` if you installed
elsewhere.

## Basic Health Check

Run these four checks before anything else. They usually point at the right section below.

```bash
# 1. Are all Operator, Controller, CSI, and Satellite Pods running?
kubectl get pods -n piraeus-datastore

# 2. Is the cluster reported as available and configured?
kubectl get linstorcluster

# 3. Is every expected Satellite connected and configured?
kubectl get linstorsatellite

# 4. What is the Operator itself logging?
kubectl logs -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator --tail=50
```

If you see any errors, or something is not `AVAILABLE`/`CONNECTED`/`CONFIGURED`, proceed with
[**troubleshooting the operator deployment**](#troubleshooting-the-operator-deployment).

If all looks good, but volume provisioning does not work, skip to
[**troubleshooting volume creation**](#troubleshooting-volume-creation).

## Troubleshooting the Operator Deployment

Work from the outside in: Operator Pod → admission webhook → `LinstorCluster` → LINSTOR Controller → Satellites.

### The Operator Pod is not Running or not Ready

```bash
kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
kubectl describe pod -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
```

-   **`ImagePullBackOff` / `ErrImagePull`** — the cluster cannot pull the Operator image. Check network access to the
    registry, and configure an image pull secret if your registry needs authentication. For air-gapped registries, see
    [Deploy Piraeus Datastore behind an HTTP Proxy](./http-proxy.md) and
    [Image Configuration](../explanation/image-configuration.md).
-   **`CrashLoopBackOff`** — read the logs (`kubectl logs …`). Messages containing `forbidden` mean the Operator's
    RBAC is incomplete, which usually indicates a partial or hand-edited install. Re-apply the full manifest or Helm
    chart rather than a subset.
-   **The CRDs are missing** — `kubectl get crds | grep piraeus.io` should list four CRDs. If they are absent, the
    install did not complete; re-apply it.

### Applying a resource is rejected by a webhook

```text
Error from server (InternalError): error when creating "cluster.yaml": Internal error occurred:
failed calling webhook "vlinstorcluster.kb.io": ... x509: certificate signed by unknown authority
```

The Operator serves validating webhooks on port `9443`, using a certificate provisioned either by the bundled
`gencert` Job or by cert-manager. If the Operator Pod is not ready, or the serving certificate is missing, every
`LinstorCluster`/`LinstorSatelliteConfiguration`/`StorageClass` apply fails at admission.

```bash
kubectl get validatingwebhookconfigurations | grep piraeus
kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
kubectl get jobs,secrets -n piraeus-datastore | grep -i cert
```

Confirm the Operator Pod is `Ready`. If you install with cert-manager, confirm the `Certificate`/`Secret` for the
webhook was issued. `connection refused` instead of an `x509` error means the webhook Service has no ready endpoints —
again, an unready Operator Pod.

### The cluster resources are unhealthy

The `LinstorCluster` and `LinstorSatellite` resources report their state through conditions. `kubectl get` shows the
summary columns; the full message (including any error) is in the object itself:

```bash
kubectl get linstorcluster -o jsonpath='{range .items[*].status.conditions[*]}{.type}={.status} ({.reason}): {.message}{"\n"}{end}'
kubectl get linstorsatellite <node-name> -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}): {.message}{"\n"}{end}'
```

The Operator reports three condition `type`s on both resources:

| `type`       | Meaning when `True`                                                       |
|--------------|--------------------------------------------------------------------------|
| `Applied`    | All Kubernetes resources the Operator manages are applied and up to date. |
| `Available`  | The LINSTOR Controller (or Satellite) is deployed and responding.        |
| `Configured` | The desired properties and storage pools are configured in LINSTOR.      |

A condition that is `False` or `Unknown` carries a `message`. When the reconcile hit an error, the message is prefixed
with `Error:` and names the exact operation that failed — read it first.

### The `LinstorCluster` never becomes `Applied`

```bash
kubectl get linstorcluster
kubectl get linstorcluster -o jsonpath='{.items[*].status.conditions[?(@.type=="Applied")].message}{"\n"}'
kubectl logs -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
```

`Applied=False` with an `Error:` message means server-side apply of a managed resource failed. The message names the
resource; the Operator log has the full error. Common causes are RBAC gaps (a `forbidden` on a specific resource kind)
or an invalid user `spec.patches` entry that produces a resource the API server rejects.

### The LINSTOR Controller is not `Available`

```bash
kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=linstor-controller
kubectl logs -n piraeus-datastore deploy/linstor-controller
```

The Controller uses the `k8s` database backend (LINSTOR state stored as CRDs) by default. If it crash-loops on
startup after previously working, the database may be inconsistent — see
[Restore a LINSTOR Database Backup](./restore-linstor-db.md). If you point at an external Controller instead, verify
its URL and host networking as described in
[Use an Existing LINSTOR Cluster](./external-controller.md).

### Satellites are missing, or their Pods will not schedule

If `kubectl get linstorsatellite` lists **fewer nodes than expected**, no `LinstorSatellite` was generated for those
nodes — the cluster's `spec.nodeSelector`/`spec.nodeAffinity` does not match them:

```bash
kubectl get nodes --show-labels
kubectl get linstorcluster -o jsonpath='{.items[*].spec.nodeSelector}{"\n"}{.items[*].spec.nodeAffinity}{"\n"}'
```

If a `LinstorSatellite` **exists but its Pod is `Pending`**:

```bash
kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=linstor-satellite -o wide
kubectl describe pod -n piraeus-datastore <satellite-pod>
```

-   Satellites run as **privileged** Pods (they load kernel modules and manage storage). Under a restricted
    [Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/), the namespace must
    allow the `privileged` profile, or the Pod is rejected. See [Piraeus Components](../explanation/components.md) for
    why this privilege is required.
-   A `Pending` Pod with a taint message needs a matching toleration in `spec.tolerations`.

### A Satellite Pod is crash-looping

The DRBD kernel module is loaded by the `drbd-module-loader` **init container**, so when it cannot load the module the
Pod never reaches its main container:

```bash
kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=linstor-satellite
kubectl describe pod -n piraeus-datastore <satellite-pod>
```

`Init:CrashLoopBackOff` or `Init:Error` points at the module loader. Read its log:

```bash
kubectl logs -n piraeus-datastore <satellite-pod> -c drbd-module-loader
```

Depending on the message, continue with the matching guide:

-   Missing build prerequisites → [Install Kernel Headers to build DRBD](./install-kernel-headers.md).
-   Wrong load method or unsupported distribution → [Configure the DRBD Module Loader](./drbd-loader.md).
-   `Required key not available` / signature rejected on a Secure Boot node →
    [Load DRBD with Secure Boot Enabled](./secure-boot.md).

### A Satellite Pod is Running but the node is `OFFLINE`

If `linstor node list` shows a node as `OFFLINE` even though its Satellite Pod is `Running` and `Ready`, the LINSTOR
Controller cannot establish or keep a connection to the Satellite. This is a Controller-to-Satellite connection
problem, not a DRBD problem. The two common causes are:

-   **Networking** — the Controller cannot reach the Satellite's port (`3366` without TLS, `3367` with TLS). Look for a
    restrictive [NetworkPolicy](./network-policy.md), a firewall, or a routing / host-network mismatch between the
    Controller and the node. Confirm the Satellite's Pod IP is reachable from the Controller Pod.
-   **TLS certificates** — when TLS between Controller and Satellite is configured, a missing, expired, or mismatched
    certificate (or a CA that the other side does not trust) breaks the connection. Verify the certificates as described
    in
    [Configure TLS Between LINSTOR Controller and LINSTOR Satellite](./internal-tls.md#verifying-tls-configuration).

## Troubleshooting Volume Creation

Once the deployment is healthy, a volume follows this path: a `PersistentVolumeClaim` references a `StorageClass` with
`provisioner: linstor.csi.linbit.com`; the CSI Controller asks LINSTOR to place the volume in a storage pool on enough
nodes; then the CSI Node plugin attaches and mounts it where the consuming Pod runs. A failure at any step leaves the
PVC `Pending` or the Pod stuck in `ContainerCreating`.

### The StorageClass is rejected on creation

```text
admission webhook "vstorageclass.kb.io" denied the request: ...
```

Piraeus validates every `StorageClass` whose `provisioner` is `linstor.csi.linbit.com` at admission time and rejects
invalid CSI `parameters`. Read the rejection message, fix the offending parameter key or value, and re-apply. Only the
LINSTOR CSI provisioner is validated; other provisioners pass through untouched.

### The PVC stays `Pending`

```bash
kubectl describe pvc <name>
```

The events at the bottom (recorded by the CSI provisioner) are the primary signal.

-   **`waiting for first consumer to be created before binding`** — this is normal for a `StorageClass` with
    `volumeBindingMode: WaitForFirstConsumer` (the default in the tutorials). The volume is only provisioned once a Pod
    that mounts the PVC is scheduled. Create the consumer Pod.
-   **Not enough replicas / no suitable node** — the `linstor.csi.linbit.com/placementCount` requested is larger than
    the number of nodes that have the target storage pool, or those nodes lack capacity. Lower `placementCount`, add
    nodes/pools, or free capacity.
-   **Unknown / empty storage pool** — the `linstor.csi.linbit.com/storagePool` parameter names a pool that is not
    registered on any node. Confirm the pool exists (next section).
-   **Another event referencing a LINSTOR Error Report** — this often indicates a misconfiguration or a bug in LINSTOR.
    You can check the details by running

    ```bash
    kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor error-report list
    kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor error-report show <error-report-id>
    ```

### A storage pool is missing or a Satellite is not `Configured`

```bash
kubectl get linstorsatellite
kubectl get linstorsatellite <node-name> -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}): {.message}{"\n"}{end}'
kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor storage-pool list
```

Since v2.10.8 the Operator only registers a storage pool once its backing device exists, so that a pool never lingers
in LINSTOR's `error` state. Instead, the Satellite reports `Configured=False` with one of these messages:

-   **`storage pool "<name>" not registered: backend "<vg/zpool>" (<KIND>) does not exist on node "<node>" yet`** — the
    backing LVM volume group or ZFS pool is not present, and no source devices are configured to create it. Either
    create the volume group / ZFS pool on the node yourself, or set `spec.storagePools[].source.hostDevices` in the
    `LinstorSatelliteConfiguration` so the Operator creates the backend from raw devices.
-   **`storage pool "<name>" not created: source device(s) [...] do not exist on node "<node>"`** — the device paths in
    `source.hostDevices` are wrong or the disks are absent on that node. Fix the paths (they must be under `/dev/`) or
    attach the disks.

See [LinstorSatelliteConfiguration → storagePools](../reference/linstorsatelliteconfiguration.md#specstoragepools) for
the full field reference.

### The node is `OFFLINE` in LINSTOR

```bash
kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor node list
```

If the node that should host the volume is `OFFLINE`, no volume can be placed there. This is a deployment-tier
problem — jump back to
[A Satellite Pod is Running but the node is `OFFLINE`](#a-satellite-pod-is-running-but-the-node-is-offline).

### The Pod is stuck in `ContainerCreating`

The PVC is `Bound`, but the Pod cannot attach or mount the volume:

```bash
kubectl describe pod <consumer-pod>
```

-   **`FailedAttachVolume` / `FailedMount`** — check that the CSI Node plugin is running on the node where the Pod is
    scheduled:

    ```bash
    kubectl get pods -n piraeus-datastore -l app.kubernetes.io/component=linstor-csi-node -o wide
    ```

    On MicroK8s, k0s, and similar distributions the kubelet plugin path differs; if the CSI Node plugin never
    registers, follow the matching [Kubernetes distribution guide](./README.md#kubernetes-distributions).

-   **The DRBD resource is unhealthy** — inspect the resource and any LINSTOR error reports:

    ```bash
    kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor resource list
    kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor error-report list
    ```

### Volume expansion does not take effect

Resizing a PVC only works when the `StorageClass` sets `allowVolumeExpansion: true`. After increasing
`spec.resources.requests.storage`, watch the PVC conditions; the CSI resizer performs the resize and the filesystem is
grown on next mount.

## Collecting Diagnostics for a Bug Report

When you open an issue, attach an SOS report — it bundles logs and cluster state from the Controller and all
Satellites. The report is written **inside the Controller Pod** (under `/var/log/linstor/`), so you have to copy it
out afterwards.

First, create the report. The command prints the path of the file it created — note it:

```bash
kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor sos-report create
```

Then copy that file to your machine. `kubectl cp` cannot address a Deployment, so resolve the Controller Pod name
first:

```bash
POD=$(kubectl -n piraeus-datastore get pod -l app.kubernetes.io/component=linstor-controller \
  -o jsonpath='{.items[0].metadata.name}')

# List the reports and copy the one you just created (replace <report>.tar.gz with the real name)
kubectl -n piraeus-datastore exec "$POD" -- ls -rt /var/log/linstor
kubectl -n piraeus-datastore cp "$POD:/var/log/linstor/<report>.tar.gz" ./<report>.tar.gz
```

!!! note

    `kubectl cp` requires `tar` in the container (the LINSTOR image ships it) and a leading `/` on absolute paths, as
    shown. If the path from the `create` output differs from `/var/log/linstor/`, use the path it printed.

Also include the Operator log and the relevant conditions:

```bash
kubectl logs -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator --tail=200
kubectl get linstorcluster,linstorsatellite -o yaml
```
