# `LinstorSatellite`

This resource controls the state of a LINSTOR® satellite.

**NOTE:** This resource is not intended to be changed directly, instead it is created by the Piraeus Operator
by merging all matching [`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md) resources.

## `.spec`

Holds the desired state the satellite.

### `.spec.repository`

Holds the default image registry to use for all Piraeus images. Inherited from
[`LinstorCluster`](./linstorcluster.md#specrepository).

If empty (the default), the operator will use `quay.io/piraeusdatastore`.

### `.spec.clusterRef`

Holds a reference to the [`LinstorCluster`](./linstorcluster.md) that controls this satellite.

### `.spec.storagePools`

Holds the storage pools to configure on the node. Inherited from matching
[`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md#specstoragepools) resources.

### `.spec.properties`

Holds the properties which should be set on the node level. Inherited from matching
[`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md#specproperties) resources.

### `.spec.internalTLS`

Configures a TLS secret used by the LINSTOR Satellite. Inherited from matching
[`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md#specproperties) resources.

### `.spec.patches`

Holds patches to apply to the Kubernetes resources. Inherited from matching
[`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md#specpatches) resources.

## `.status`

Reports the actual state of the satellite.

## `.status.conditions`

The Operator reports the current state of the LINSTOR Satellite through a set of conditions. Conditions are
identified by their `type`.

| `type`                | Explanation                                                                                          |
|-----------------------|------------------------------------------------------------------------------------------------------|
| `Applied`             | All Kubernetes resources were applied.                                                               |
| `Available`           | The LINSTOR Satellite is connected to the LINSTOR Controller                                         |
| `Configured`          | Storage Pools and Properties are configured on the Satellite                                         |
| `EvacuationCompleted` | Only available when the Satellite is being deleted: Indicates progress of the eviction of resources. |
