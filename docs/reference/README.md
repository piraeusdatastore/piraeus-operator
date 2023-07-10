# API Reference

This is the API Reference for Piraeus Operator. A user may make modifications to these resources to change the cluster
state (`LinstorCluster` or `LinstorSatelliteConfiguration`) or check the status of a resource (`LinstorSatellite`).

### [`LinstorCluster`](./linstorcluster.md)

This resource controls the state of the LINSTOR® cluster and integration with Kubernetes.

### [`LinstorSatelliteConfiguration`](./linstorsatelliteconfiguration.md)

This resource controls the state of the LINSTOR Satellites, optionally applying it to only a subset of nodes.

### [`LinstorSatellite`](./linstorsatellite.md)

This resource controls the state of a single LINSTOR Satellite. This resource is not intended to be changed directly,
instead it is created by the Piraeus Operator by merging all matching `LinstorSatelliteConfiguration` resources.

### [`LinstorNodeConnection`](./linstornodeconnection.md)

This resource controls the state of the LINSTOR® node connections.
