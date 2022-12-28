# `LinstorSatelliteConfiguration`

This resource controls the state of one or more LINSTOR® satellites.

## `.spec`

Configures the desired state of satellites.

### `.spec.nodeSelector`

Selects which nodes the LinstorSatelliteConfiguration should apply to. If empty, the configuration applies to all nodes.

#### Example

This example sets the `AutoplaceTarget` property to `no` on all nodes labelled `piraeus.io/autoplace: "no"`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: disabled-nodes
spec:
  nodeSelector:
    piraeus.io/autplace: "no"
  properties:
    - name: AutoplaceTarget
      value: "no"
```

### `.spec.properties`

Sets the given properties on the LINSTOR Satellite level.

The property value can either be set directly using `value`, or inherited from the Kubernetes Node's metadata using
`valueFrom`. Metadata fields are specified using the same syntax as the [Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api)
for Pods.

In addition, setting `optional` to true means the property is only applied if the value is not empty. This is useful
in case the property value should be inherited from the node's metadata

#### Example

This examples sets three Properties on every satellite:
* `PrefNic` (the preferred network interface) is always set to `default-ipv6`
* `Aux/example-property` (an auxiliary property, unused by LINSTOR itself) takes the value from the `piraeus.io/example`
  label of the Kubernetes Node. If a node has no `piraeus.io/example` label, the property value will be `""`.
* `AutoplaceTarget` (if set to `no`, will exclude the node from LINSTOR's Autoplacer) takes the value from the
  `piraeus.io/autoplace` annotation of the Kubernetes Node. If a node has no `piraeus.io/autoplace` annotation, the
  property will not be set.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: ipv6-nodes
spec:
  properties:
    - name: PrefNic
      value: "default-ipv6"
    - name: Aux/example-property
      valueFrom:
        nodeFieldRef: metadata.labels['piraeus.io/example']
    - name: AutoplaceTarget
      valueFrom:
        nodeFieldRef: metadata.annotations['piraeus.io/autoplace']
      optional: yes
```

### `.spec.storagePools`

Configures LINSTOR Storage Pools.

Every Storage Pool needs at least a `name`, and a type. Types are specified by setting a (potentially empty) value on
the matching key. Available types are:

* `lvmPool`: Configures a [LVM Volume Group](https://linbit.com/drbd-user-guide/drbd-guide-9_0-en/#s-lvm-primer) as storage
  pool. Defaults to using the storage pool name as the VG name. Can be overridden by setting `volumeGroup`.
* `lvmThinPool`: Configures a [LVM Thin Pool](https://man7.org/linux/man-pages/man7/lvmthin.7.html) as storage pool.
  Defaults to using the storage pool name as name for the thin pool volume and the storage pool name prefixed by
  `linstor_` as the VG name. Can be overridden by setting `thinPool` and `volumeGroup`.
* `filePool`: Configures a file system based storage pool. Configures a host directory as location for the volume files.
  Defaults to using the `/var/lib/linstor-pools/<storage pool name>` directory.
* `fileThinPool`: Configures a file system based storage pool. Behaves the same as `filePool`, except the files will
  be thinly allocated on file systems that support sparse files.

Optionally, you can configure LINSTOR to automatically create the backing pools. `source.hostDevices` takes a list
of raw block devices, which LINSTOR will prepare as the chosen backing pool.

#### Example

This example configures three LINSTOR Storage Pools on all satellites:
* A LVM Pool named `vg1`. It will use the VG `vg1`, which needs to exist on the nodes already.
* A LVM Thin Pool named `vg1-thin`. It will use the thin pool `vg1/thin`, which also needs to exist on the nodes.
* A LVM Pool named `vg2-from-raw-devices`. It will use the VG `vg2`, which will be created on demand from the raw
  devices `/dev/sdb` and `/dev/sdc` if it does not exist already.
* A File System Pool named `fs1`. It will use the `/var/lib/linstor-pools/fs1` directory on the host, creating the
  directory if necessary.
* A File System Pool namef `fs2`, using sparse files. It will use the custom `/mnt/data` directory on the host.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: storage-satellites
spec:
  storagePools:
    - name: vg1
      lvmPool: {}
    - name: vg1-thin
      lvmThinPool:
        volumeGroup: vg1
        thinPool: thin
    - name: vg2-from-raw-devices
      lvmPool:
        volumeGroup: vg2
      source:
        hostDevices:
          - /dev/sdb
          - /dev/sdc
    - name: fs1
      filePool: {}
    - name: fs2
      fileThinPool:
        directory: /mnt/data
```

### `.spec.internalTLS`

Configures a TLS secret used by the LINSTOR Satellites to:
* Validate the certificate of the LINSTOR Controller, that is the Controller must have certificates signed by `ca.crt`.
* Provide a server certificate for authentication by the LINSTOR Controller, that is `tls.key` and `tls.crt` must be accepted by the Controller.

To configure TLS communication between Satellite and Controller, [`LinstorCluster.spec.internalTLS`](linstorcluster.md#specinternaltls) must be set accordingly.

Setting a `secretName` is optional, it will default to `<node-name>-tls`, where `<node-name>` is replaced with the
name of the Kubernetes Node.

Optional, a reference to a [cert-manager `Issuer`](https://cert-manager.io/docs/concepts/issuer/) can be provided
to let the operator create the required secret.

#### Example

This example creates a manually provisioned TLS secret and references it in the LinstorSatelliteConfiguration, setting
it for all nodes.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-node-tls
  namespace: piraeus-datastore
data:
  ca.crt: LS0tLS1CRUdJT...
  tls.crt: LS0tLS1CRUdJT...
  tls.key: LS0tLS1CRUdJT...
---
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: satellite-tls
spec:
  internalTLS:
    secretName: my-node-tls
```

#### Example

This example sets up automatic creation of the LINSTOR Satellite TLS secrets using a
cert-manager issuer named `piraeus-root`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: satellite-tls
spec:
  internalTLS:
    certManager:
      kind: Issuer
      name: piraeus-root
```

### `.spec.patches`

The given patches will be applied to all resources controlled by the operator. The patches are
forwarded to `kustomize` internally, and take the [same format](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/).

The unpatched resources are available in the [subdirectories of the `pkg/resources/satellite` directory](../../pkg/resources/satellite).

#### Warning

No checks are run on the result of user-supplied patches: the resources are applied
as-is. Patching some fundamental aspect, such as removing a specific volume from a
container may lead to a degraded cluster.

#### Example

This example configures the LINSTOR Satellite to use the "TRACE" log level, creating very verbose output.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: all-satellites
spec:
  patches:
    - target:
        kind: ConfigMap
        name: satellite-config
      patch: |-
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: satellite-config
        data:
          linstor_satellite.toml: |
            [logging]
              linstor_level = "TRACE"
```

## `.status`

Reports the actual state of the cluster.

## `.status.conditions`

The Operator reports the current state of the Satellite Configuration through a set of conditions. Conditions are
identified by their `type`.

| `type`       | Explanation                                                              |
|--------------|--------------------------------------------------------------------------|
| `Applied`    | The given configuration was applied to all `LinstorSatellite` resources. |
