# API Reference

This is the API Reference for Piraeus Operator. A user may make modifications to these resources to change the cluster
state (`LinstorCluster` or `LinstorSatelliteConfiguration`) or check the status of a resource (`LinstorSatellite`).

<div class="cards grid" markdown>

*   __LinstorCluster__

    ---

    This resource controls the state of the LINSTOR® cluster and integration with Kubernetes.

    [:octicons-arrow-right-24: Reference](./linstorcluster.md)

*   __LinstorSatelliteConfiguration__

    ---

    This resource controls the state of the LINSTOR Satellites, optionally applying it to only a subset of nodes.

    [:octicons-arrow-right-24: Reference](./linstorsatelliteconfiguration.md)

*   __LinstorNodeConnection__

    ---

    This resource controls the state of the LINSTOR® node connections.

    [:octicons-arrow-right-24: Reference](./linstornodeconnection.md)

*   __LinstorSatellite__

    ---

    This resource controls the state of a single LINSTOR Satellite. This resource is not intended to be changed directly,
    instead it is created by the Piraeus Operator by merging all matching `LinstorSatelliteConfiguration` resources.

    [:octicons-arrow-right-24: Reference](./linstorsatellite.md)

</div>
