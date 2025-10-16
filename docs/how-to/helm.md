# Use Helm to deploy Piraeus Datastore

This guide shows you how to deploy Piraeus Datastore using [Helm].

To complete this guide, you should be familiar with:

* deploying Helm Charts from [OCI registries].
* overriding values in Helm Charts.

[Helm]: https://helm.sh
[OCI registries]: https://helm.sh/docs/topics/registries/

## Deploying the Piraeus Operator

The first step when deploying Piraeus Datastore is creating the Piraeus Operator deployment. Running the following
command will install or upgrade Piraeus Datastore in the `piraeus-datastore` namespace including the necessary CRDs:

```
helm upgrade --install --create-namespace --namespace piraeus-datastore piraeus-operator oci://ghcr.io/piraeusdatastore/piraeus-operator/piraeus --set installCRDs=true --wait
```

There are ways to customize the deployment of the Operator. To show the available values, run the following command:

```
helm show values oci://ghcr.io/piraeusdatastore/piraeus-operator/piraeus
```

## Deploying Piraeus Datastore

After deploying the Piraeus Operator, you can create the Piraeus Datastore deployment:

```
helm upgrade --install --namespace piraeus-datastore linstor-cluster oci://ghcr.io/piraeusdatastore/helm-charts/linstor-cluster
```

This chart allows the creation of:

* the central [LinstorCluster] resource.
* one or more [LinstorSatelliteConfiguration] resources to customize the Satellites.
* one or more [LinstorNodeConnection] resource to customize DRBD® replication paths and settings.
* TLS Certificates for securing Controller and Satellite connections.
* Resources to monitor the LINSTOR® cluster with Prometheus.
* StorageClasses, VolumeSnapshotClasses and VolumeGroupSnapshotClasses.

To show the available values, run the following command:

```
helm show values oci://ghcr.io/piraeusdatastore/helm-charts/linstor-cluster
```

[LinstorCluster]: ../reference/linstorcluster.md
[LinstorSatelliteConfiguration]: ../reference/linstorsatelliteconfiguration.md
[LinstorNodeConnection]: ../reference/linstornodeconnection.md
