# Upgrading Piraeus Operator from Version 1 to Version 2

The following document guides you through the upgrade process for Piraeus Operator from version 1 ("v1")
to version 2 ("v2").

Piraeus Operator v2 offers improved convenience and customization. This however made it necessary to make significant
changes to the way Piraeus Operator manages Piraeus Datastore. As such, upgrading from v1 to v2 is a procedure
requiring manual oversight.

Upgrading Piraeus Operator is done in four steps:

* [Step 1]: (Optional) Migrate the LINSTOR database to use the `k8s` backend.
* [Step 2]: Collect information about the current deployment.
* [Step 3]: Remove the Piraeus Operator v1 deployment, keeping existing volumes untouched.
* [Step 4]: Deploy Piraeus Operator v2 using the information gathered in step 2.

[Step 1]: ./1-migrate-database.md
[Step 2]: ./2-collect-information.md
[Step 3]: ./3-remove-operator-v1.md
[Step 4]: ./4-install-operator-v2.md

## Prerequisites

This guide assumes:

* You used Helm to create the original deployment and are familiar with upgrading Helm deployments.
* Your Piraeus Datastore deployment is up-to-date with the latest v1 release. Check the releases [here](https://github.com/piraeusdatastore/piraeus-operator/releases?q=v1&expanded=true).
* You have the following command line tools available:
  - [`kubectl`](https://kubernetes.io/docs/tasks/tools/)
  - [`helm`](https://docs.helm.sh/docs/intro/install/)
  - [`jq`](https://jqlang.github.io/jq/download/)
* You are familiar with the `linstor` command line utility, specifically to verify the cluster state.

## Key Changes Between Operator V1 and V2

### Administrative Changes

* The resources `LinstorController`, `LinstorSatelliteSet` and `LinstorCSIDriver` have been replaced by
  [`LinstorCluster`](../../reference/linstorcluster.md) and
  [`LinstorSatelliteConfgiuration`](../../reference/linstorsatelliteconfiguration.md).
* The default deployment runs the LINSTOR Satellite in the [container network](../drbd-host-networking.md).
  The migration script will propose changing to the host network.

### Operational Changes

* In Operator v1, all labels on the Kubernetes node resource were replicated on the satellite, making them usable in the
  `replicasOnSame` and `replicasOnDifferent` parameters on the storage class. In Operator v2, only the following
  labels are automatically synchronized.
  * `kubernetes.io/hostname`
  * `topology.kubernetes.io/region`
  * `topology.kubernetes.io/zone`
  Use [`LinstorSatelliteConfiguration.spec.properties`](../../reference/linstorsatelliteconfiguration.md#specproperties)
  to synchronize additional labels.
* The following settings are applied by Operator v2 cluster-wide:
  * DrbdOptions/Net/rr-conflict: retry-connect
  * DrbdOptions/Resource/on-suspended-primary-outdated: force-secondary
  * DrbdOptions/Resource/on-no-data-accessible: suspend-io
  * DrbdOptions/auto-quorum: suspend-io
* Operator v2 also includes a [High-Availability Controller](https://github.com/piraeusdatastore/piraeus-ha-controller)
  deployment to prevent stuck nodes caused by suspended DRBD devices.
