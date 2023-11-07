# Migrating the LINSTOR Controller Database to Use the `k8s` Backend

This document guides you through the process of migrating an existing LINSTOR® Cluster to use the `k8s` database
backend.

This is the optional first step when migrating Piraeus Operator from version 1 (v1) to version 2 (v2).
[Click here to get back to the overview](./index.md).

## Prerequisites

* [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [Install `helm`](https://docs.helm.sh/docs/intro/install/)

## When to Migrate the LINSTOR Controller Database to the `k8s` Backend

LINSTOR can use different database backends to store important state, such as the available volumes and where
they are deployed. The available backends are:

* An [H2 database](https://h2database.com/html/main.html) stored in a local file on the controller.
* A compatible database server, such as [MySQL](https://www.mysql.com/) or [PostgreSQL](https://www.postgresql.org/).
* An [etcd cluster](https://etcd.io/).
* An `k8s` backend, using Custom Resources to store the cluster state directly in the Kubernetes API.

Of these backends, only the `k8s` backend works without any additional components when running in the
Kubernetes cluster.

Because the Piraeus Operator v1 is older than the LINSTOR `k8s` backend, the default for a long time was
the use of a separate etcd cluster, deployed by default when using Piraeus Operator v1. Since then, etcd
proved to be difficult to maintain, often requiring manual intervention.

Because of this, **consider migrating away from separate etcd cluster to using the `k8s` backend**.

If you already use one of the other backends, no migration is necessary. You can check the currently used
backend of your LINSTOR cluster by running the following command:

```
$ kubectl exec deploy/piraeus-op-cs-controller -- cat /etc/linstor/linstor.toml
[db]
  connection_url = "etcd://piraeus-op-etcd:2379"
```

If the `connection_url` value starts with `etcd://` you are using the etcd backend and should consider
migrating.

## Migrating the LINSTOR Controller Database

During the migration, the LINSTOR Controller needs to be stopped. This means that provisioning and deletion
of volumes will not work for the duration of the migration. Existing volumes will continue working normally.

### 1. Stop the LINSTOR Controller

To prevent unwanted modifications to the database during the migration, you need to stop the LINSTOR Controller.
To stop the LINSTOR Controller, set the expected replicas for the controller to zero.

First, find the currently deployed Piraeus Datastore release. For this guide, it is 1.10.8:

```
$ helm list
NAME        NAMESPACE          REVISION  UPDATED                                 STATUS    CHART           APP VERSION
piraeus-op  piraeus-datastore  1         2023-11-07 10:30:44.184389038 +0100 CET deployed  piraeus-1.10.8  1.10.8
```

Then change the deployment to deploy no replicas for the LINSTOR Controller. Ensure to check out the chart version
currently deployed, so that no accidental upgrade happens:

```
$ git checkout v1.10.8
$ helm upgrade piraeus-op ./charts/piraeus --reuse-values --set operator.controller.replicas=0
$ kubectl rollout status deploy/piraeus-op-cs-controller -w
deployment "piraeus-op-cs-controller" successfully rolled out
```

### 2. Prepare a Pod for Running the Migration

To run the migration, you need access to the `linstor-database` tool, which comes preinstalled with the LINSTOR
Controller image. The following command displays the LINSTOR Controller image, the name of the ConfigMap storing the
LINSTOR Configuration, and the service account for LINSTOR Controller. Take note of them, as they will be used for the
migration Pod.

```
$ kubectl get deploy/piraeus-op-cs-controller --output=jsonpath='IMAGE={$.spec.template.spec.containers[?(@.name=="linstor-controller")].image}{"\n"}CONFIG_MAP={$.spec.template.spec.volumes[?(@.name=="linstor-conf")].configMap.name}{"\n"}SERVICE_ACCOUNT={$.spec.template.spec.serviceAccountName}{"\n"}'
IMAGE=quay.io/piraeusdatastore/piraeus-server:v1.24.2
CONFIG_MAP=piraeus-op-cs-controller-config
SERVICE_ACCOUNT=linstor-controller
```

Using the information from the LINSTOR Controller deployment, create a Pod used during the migration process. Replace
`$IMAGE`, `$CONFIG_MAP` and `$SERVICE_ACCOUNT` with the appropriate values.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: linstor-database-migration
spec:
  serviceAccountName: $SERVICE_ACCOUNT
  containers:
    - name: backup
      image: $IMAGE
      command:
        - /bin/bash
        - -c
        - "sleep infinity"
      volumeMounts:
        - name: linstor-conf
          mountPath: /etc/linstor
          readOnly: true
        - name: backup
          mountPath: /backup
        - name: logs
          mountPath: /logs
  volumes:
    - name: backup
      emptyDir: {}
    - name: logs
      emptyDir: {}
    - name: linstor-conf
      configMap:
        name: $CONFIG_MAP
```

### 3. Create a Database Backup

Using the newly created migration Pod, create a database backup using the `linstor-database` tool.

```
$ kubectl exec linstor-database-migration -- /usr/share/linstor-server/bin/linstor-database export-db -c /etc/linstor /backup/backup-before-migration.json
Loading configuration file "/etc/linstor/linstor.toml"
15:22:55.684 [main] INFO LINSTOR/linstor-db -- SYSTEM - ErrorReporter DB first time init.
15:22:55.685 [main] INFO LINSTOR/linstor-db -- SYSTEM - Log directory set to: './logs'
15:22:56.417 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.modularcrypto.FipsCryptoModule"
15:22:56.418 [main] INFO LINSTOR/linstor-db -- SYSTEM - Extension module "com.linbit.linstor.modularcrypto.FipsCryptoModule" is not installed
15:22:56.418 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.modularcrypto.JclCryptoModule"
15:22:56.426 [main] INFO LINSTOR/linstor-db -- SYSTEM - Dynamic load of extension module "com.linbit.linstor.modularcrypto.JclCryptoModule" was successful
15:22:56.427 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.spacetracking.ControllerSpaceTrackingModule"
15:22:56.429 [main] INFO LINSTOR/linstor-db -- SYSTEM - Dynamic load of extension module "com.linbit.linstor.spacetracking.ControllerSpaceTrackingModule" was successful
15:22:57.666 [main] INFO LINSTOR/linstor-db -- SYSTEM - Initializing the etcd database
15:22:57.667 [main] INFO LINSTOR/linstor-db -- SYSTEM - etcd connection URL is "etcd://piraeus-op-etcd:2379"
15:22:59.480 [main] INFO LINSTOR/linstor-db -- SYSTEM - Export finished
```

Then, copy the just created backup to permanent storage, so that it is not lost should the migration pod restart:

```
$ kubectl cp linstor-database-migration:/backup/backup-before-migration.json backup-before-migration.json
```

### 4. Update the LINSTOR Configuration to Point to the New Database Backend

After creating and storing the backup it is safe to change the LINSTOR configuration. This is done by running another
`helm upgrade`, changing the database configuration for the LINSTOR Controller. Once again, ensure that you apply the
chart version currently deployed, so that no accidental upgrade happens:

```
$ helm upgrade piraeus-op ./charts/piraeus --reuse-values --set operator.controller.dbConnectionURL=k8s
```

This will also change the configuration in the `linstor-database-migration` Pod, but it might need some time to
propagate. Eventually, you will see the `k8s` database connection:

```
$ kubectl exec linstor-database-migration -- cat /etc/linstor/linstor.toml
[db]
  connection_url = "k8s"
```

### 5. Import the Database from the Backup

After changing the configuration, you can again use the `linstor-database` tool to restore the database backup to the
new backend.

```
$ kubectl exec linstor-database-migration -- /usr/share/linstor-server/bin/linstor-database import-db -c /etc/linstor /backup/backup-before-migration.json
Loading configuration file "/etc/linstor/linstor.toml"
15:39:41.270 [main] INFO LINSTOR/linstor-db -- SYSTEM - ErrorReporter DB version 1 found.
15:39:41.272 [main] INFO LINSTOR/linstor-db -- SYSTEM - Log directory set to: './logs'
15:39:41.848 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.modularcrypto.FipsCryptoModule"
15:39:41.849 [main] INFO LINSTOR/linstor-db -- SYSTEM - Extension module "com.linbit.linstor.modularcrypto.FipsCryptoModule" is not installed
15:39:41.850 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.modularcrypto.JclCryptoModule"
15:39:41.858 [main] INFO LINSTOR/linstor-db -- SYSTEM - Dynamic load of extension module "com.linbit.linstor.modularcrypto.JclCryptoModule" was successful
15:39:41.859 [main] INFO LINSTOR/linstor-db -- SYSTEM - Attempting dynamic load of extension module "com.linbit.linstor.spacetracking.ControllerSpaceTrackingModule"
15:39:41.860 [main] INFO LINSTOR/linstor-db -- SYSTEM - Dynamic load of extension module "com.linbit.linstor.spacetracking.ControllerSpaceTrackingModule" was successful
15:39:43.307 [main] INFO LINSTOR/linstor-db -- SYSTEM - Initializing the k8s crd database connector
15:39:43.307 [main] INFO LINSTOR/linstor-db -- SYSTEM - Kubernetes-CRD connection URL is "k8s"
15:40:22.418 [main] INFO LINSTOR/linstor-db -- SYSTEM - Import finished
```

### 6. Start the LINSTOR Controller

Now that the new database backend is initialized, it is safe to start the LINSTOR Controller again. Simply change the
number of replicas for the LINSTOR Controller back to the original value before starting the migration. Once again,
ensure that you apply the chart version currently deployed, so that no accidental upgrade happens:

```
$ helm upgrade piraeus-op ./charts/piraeus --reuse-values --set operator.controller.replicas=1
deployment "piraeus-op-cs-controller" successfully rolled out
```

Once the LINSTOR Controller is up and running, verify the state of your cluster: check all nodes and resource are in
the expected state:

```
$ kubectl exec deploy/piraeus-op-cs-controller -- linstor node list
$ kubectl exec deploy/piraeus-op-cs-controller -- linstor storage-pool list
$ kubectl exec deploy/piraeus-op-cs-controller -- linstor resource list-volumes
$ kubectl exec deploy/piraeus-op-cs-controller -- linstor error-reports list
```

### 7. Clean up Migration Resources

After successful migration, you can clean up left over resources.

First, you can remove the `linstor-database-migration` Pod:

```
$ kubectl delete pod linstor-database-migration
```

Then, you can also remove the etcd deployment by upgrading the helm deployment, setting `etcd.enabled=false`:

```
$ helm upgrade piraeus-op ./charts/piraeus --reuse-values --set etcd.enabled=false
```
