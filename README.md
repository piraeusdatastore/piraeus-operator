[![Release](https://img.shields.io/github/v/release/piraeusdatastore/piraeus-operator)](https://github.com/piraeusdatastore/piraeus-operator/releases)
![](https://github.com/piraeusdatastore/piraeus-operator/workflows/check%20and%20build%20piraeus-operator/badge.svg)

# Piraeus Operator

The Piraeus Operator manages
[LINSTOR](https://github.com/LINBIT/linstor-server) clusters in Kubernetes.

All components of the LINSTOR software stack can be managed by the operator and
associated Helm chart:
* DRBD
* etcd cluster for LINSTOR
* LINSTOR
* LINSTOR CSI driver
* LINSTOR High Availability Controller
* Stork scheduler with LINSTOR integration

## Deployment with Helm v3 Chart

The operator can be deployed with Helm v3 chart in /charts as follows:

- Prepare the hosts for DRBD deployment. This depends on your host OS. You can find more information on the available
  options in the [host setup guide](./doc/host-setup.md).

- If you are deploying with images from private repositories, create
  a kubernetes secret to allow obtaining the images. This example will create
  a secret named `drbdiocred`:

    ```
    kubectl create secret docker-registry drbdiocred --docker-server=<SERVER> --docker-username=<YOUR_LOGIN> --docker-email=<YOUR_EMAIL> --docker-password=<YOUR_PASSWORD>
    ```

    The name of this secret must match the one specified in the Helm values, by
    passing `--set drbdRepoCred=drbdiocred` to helm.

- Configure storage for the LINSTOR etcd instance.
  There are various options for configuring the etcd instance for LINSTOR:
  * Use an existing storage provisioner with a default `StorageClass`.
  * [Use `hostPath` volumes](#linstor-etcd-hostpath-persistence).
  * Disable persistence for basic testing. This can be done by adding `--set
    etcd.persistentVolume.enabled=false` to the `helm install` command below.

- Configure a basic storage setup for LINSTOR:
  * Create storage pools from available devices. Recommended for simple set ups. [Guide](doc/storage.md#preparing-physical-devices)
  * Create storage pools from existing LVM setup. [Guide](doc/storage.md#configuring-storage-pool-creation)

  Read [the storage guide](doc/storage.md) and configure as needed.

- Read the [guide on securing the deployment](doc/security.md) and configure as needed.

- Read up on [optional components](doc/optional-components.md) and configure as needed.

- Finally create a Helm deployment named `piraeus-op` that will set up
  everything.

    ```
    helm install piraeus-op ./charts/piraeus
    ```

  You can pick from a number of example settings:

  * default values (Kubernetes v1.17+) [`values.yaml`](./charts/piraeus/values.yaml)
  * images optimized for CN [`values.cn.yaml`](./charts/piraeus/values.cn.yaml).
  * override for Openshift [`values-openshift.yaml`](./charts/piraeus/values-openshift.yaml)

  A full list of all available options to pass to helm can be found [here](./doc/helm-values.adoc).

### LINSTOR etcd `hostPath` persistence

In general, we recommend using pre-provisioned persistent volumes (PB) when using Etcd as the LINSTOR database.
You can use the included `pv-hostpath` Helm chart to quickly create such PVs.

```
helm install piraeus-etcd-pv ./charts/pv-hostpath
```

These PVs are of type `hostPath`, i.e. a directory on the host is shared with the container. By default, a volume is
created on every control-plane node (those labeled with `node-role.kubernetes.io/control-plane`). You can manually
specify on which nodes PVs should be created by using `--set nodes={<nodename1>,<nodename2>}`.

The chart defaults to using the `/var/lib/linstor-etcd` directory on the host. You can override this by using
`--set path=/new/path`.

#### `hostPath` volumes and SELinux

Clusters with SELinux enabled hosts (for example: OpenShift clusters) need to relabel the created directory. This
can be done automatically by passing `--set selinux=true` to the above `helm install` command.

#### Override the default choice of chown job image

The `pv-hostpath` chart will create a job for each created PV. The jobs are meant to ensure the host volumes are set up
with the correct permission, so that etcd can run as a non-privileged container. To override the default choice of
`quay.io/centos/centos:8`, use `--set chownerImage=<my-image>`.

### Using an existing database

LINSTOR can connect to an existing PostgreSQL, MariaDB or etcd database. For
instance, for a PostgresSQL instance with the following configuration:

```
POSTGRES_DB: postgresdb
POSTGRES_USER: postgresadmin
POSTGRES_PASSWORD: admin123
```

The Helm chart can be configured to use this database instead of deploying an
etcd cluster by adding the following to the Helm install command:

```
--set etcd.enabled=false --set "operator.controller.dbConnectionURL=jdbc:postgresql://postgres/postgresdb?user=postgresadmin&password=admin123"
```

### Pod resources

You can configure [resource requests and limits] for all deployed containers. Take a look at
this [example chart configuration.](./examples/resource-requirements.yaml)

[resource requests and limits]: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

### Running multiple replicas

Running multiple replicas of pods is recommended for high availability and fast error recovery.
The following components can be started with multiple replicas:

* Operator: Set [`operator.replicas`] to the desired number of operator pods.
* CSI: Set [`csi.controllerReplicas`] to the desired number of CSI Controller pods.
* Linstor Controller: Set [`operator.controller.replicas`] to the desired number of LINSTOR controller pods.
* Etcd: Set [`etcd.replicas`] to the desired number of Etcd pods.
* Stork: Set [`stork.replicas`] to the desired number of both Stork plugin and Stork scheduler pods.

[`operator.replicas`]: ./doc/helm-values.adoc#operatorreplicas
[`csi.controllerReplicas`]: ./doc/helm-values.adoc#csicontrollerreplicas
[`operator.controller.replicas`]: ./doc/helm-values.adoc#operatorcontrollerreplicas
[`etcd.replicas`]: ./doc/helm-values.adoc#etcdreplicas
[`stork.replicas`]: ./doc/helm-values.adoc#storkreplicas

### Influence pod scheduling

You can influence the assignement of various components to specific nodes. See the [scheduling guide.](./doc/scheduling.md)

### Terminating Helm deployment

To protect the storage infrastructure of the cluster from accidentally deleting vital components, it is necessary
to perform some manual steps before deleting a Helm deployment.

1. Delete all volume claims managed by piraeus components.
   You can use the following command to get a list of volume claims managed by Piraeus.
   After checking that non of the listed volumes still hold needed data, you can delete them using the generated
   `kubectl delete` command.

   ```
   $ kubectl get pvc --all-namespaces -o=jsonpath='{range .items[?(@.metadata.annotations.volume\.beta\.kubernetes\.io/storage-provisioner=="linstor.csi.linbit.com")]}kubectl delete pvc --namespace {.metadata.namespace} {.metadata.name}{"\n"}{end}'
   kubectl delete pvc --namespace default data-mysql-0
   kubectl delete pvc --namespace default data-mysql-1
   kubectl delete pvc --namespace default data-mysql-2
   ```

   **WARNING** These volumes, once deleted, cannot be recovered.

2. Delete the LINSTOR controller and satellite resources.

   Deployment of LINSTOR satellite and controller is controlled by the `linstorsatelliteset` and `linstorcontroller`
   resources. You can delete the resources associated with your deployment using `kubectl`

   ```
   kubectl delete linstorsatelliteset <helm-deploy-name>-ns
   kubectl delete linstorcontroller <helm-deploy-name>-cs
   ```

   After a short wait, the controller and satellite pods should terminate. If they continue to run, you can
   check the above resources for errors (they are only removed after all associated pods terminate)

3. Delete the Helm deployment.

   If you removed all PVCs and all LINSTOR pods have terminated, you can uninstall the helm deployment

   ```
   helm uninstall piraeus-op
   ```

   However due to the Helm's current policy, the Custom Resource Definitions named
   `linstorcontroller` and `linstorsatelliteset` will __not__ be deleted by the
   command.

   More information regarding Helm's current position on CRD's can be found
   [here](https://helm.sh/docs/topics/chart_best_practices/custom_resource_definitions/#method-1-let-helm-do-it-for-you).

## Deployment without using Helm v3 chart

### Configuration

The operator must be deployed within the cluster in order for it to have access
to the controller endpoint, which is a kubernetes service.

### Kubernetes Secret for Repo Access

If you are deploying with images from a private repository, create a kubernetes
secret to allow obtaining the images.  Create a secret named `drbdiocred` like
this:

```
kubectl create secret docker-registry drbdiocred --docker-server=<SERVER> --docker-username=<YOUR LOGIN> --docker-email=<YOUR EMAIL> --docker-password=<YOUR PASSWORD>
```

### Deploy Operator

First you need to create the resource definitions

```
kubectl create -f charts/piraeus/crds/piraeus.linbit.com_linstorcsidrivers_crd.yaml
kubectl create -f charts/piraeus/crds/piraeus.linbit.com_linstorsatellitesets_crd.yaml
kubectl create -f charts/piraeus/crds/piraeus.linbit.com_linstorcontrollers_crd.yaml
```

Then, take a look at the files in [`deploy/piraeus`](./deploy/piraeus) and make changes as
you see fit. For example, you can edit the storage pool configuration by editing
[`operator-satelliteset.yaml`](deploy/piraeus/templates/operator-satelliteset.yaml)
like shown in [the storage guide](./doc/storage.md#configuring-storage-pool-creation).

Now you can finally deploy the LINSTOR cluster with:
```
kubectl create -Rf deploy/piraeus/
```

## Upgrading

Please see the dedicated [UPGRADE document](./UPGRADE.md)

## Contributing

If you'd like to contribute, please visit https://github.com/piraeusdatastore/piraeus-operator
and look through the issues to see if there is something you'd like to work on. If
you'd like to contribute something not in an existing issue, please open a new
issue beforehand.

If you'd like to report an issue, please use the issues interface in this
project's github page.

## Building and Development

This project is built using the operator-sdk (version 0.19.4). Please refer to
the [documentation for the sdk](https://github.com/operator-framework/operator-sdk/tree/v0.19.x).

## License

Apache 2.0
