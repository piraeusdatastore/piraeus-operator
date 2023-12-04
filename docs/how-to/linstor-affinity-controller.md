# How to Deploy the LINSTOR Affinity Controller

This guide shows you how to deploy the [LINSTOR Affinity Controller] for Piraeus Datastore.

The LINSTOR Affinity Controller keeps the affinity of your volumes in sync between Kubernetes and LINSTOR.

When using a strict volume affinity setting, such as `allowRemoteVolumeAccess: false`, the Persistent Volume (PV)
resource created by Piraeus Datastore will have a fixed affinity. When the volume is moved to a different node, for
example because one of the existing replicas is being evacuated, the PV is not updated automatically.

The LINSTOR Affinity controller watches PVs and LINSTOR resource and keeps the affinity up-to-date.

To complete this guide, you should be familiar with:

* Deploying workloads in Kubernetes using [`helm`](https://helm.sh/)

## Add the Piraeus Datastore Chart Repository

Piraeus Datastore maintains a helm chart repository for commonly deployed components, including the LINSTOR Affinity
Controller. To add the repository to your helm configuration, run:

```
$ helm repo add piraeus-charts https://piraeus.io/helm-charts/
```

## Deploy the LINSTOR Affinity Controller

After adding the repository, deploy the LINSTOR Affinity Controller:

```
$ helm install linstor-affinity-controller piraeus-charts/linstor-affinity-controller
NAME: linstor-affinity-controller
LAST DEPLOYED: Mon Dec  4 09:14:07 2023
NAMESPACE: piraeus-datastore
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
LINSTOR Affinity Controller deployed.

Used LINSTOR URL: http://linstor-controller.piraeus-datastore.svc:3370
```

If you deploy the LINSTOR Affinity Controller to the same namespace as the Piraeus Operator, the deployment will
automatically determine the necessary parameters for connecting to LINSTOR.

In some cases, helm may not be able to determine the connection parameters. In this case, you need to manually provide
the following values:

```yaml
linstor:
  # The URL of the LINSTOR Controller API. This example contains the default value.
  endpoint: http://linstor-controller.piraeus-datastore.svc:3370
  # This is the default URL when using TLS for securing the API
  #endpoint: https://linstor-controller.piraeus-datastore.svc:3371
  # This is the name of the secret containing TLS key and certificates for connecting to the LINSTOR API with TLS.
  clientSecret: ""
options:
  # This is the namespace used by Piraeus Operator to sync Kubernetes Node labels with LINSTOR node properties.
  # For Piraeus Operator, this needs to be set to the following value:
  propertyNamespace: Aux/topology
```

[LINSTOR Affinity Controller]: https://github.com/piraeusdatastore/linstor-affinity-controller
