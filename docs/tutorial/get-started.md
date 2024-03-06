# Get Started

Learn about the ways to get started with Piraeus Datastore by deploying Piraeus Operator and provisioning your first volume.

## Prerequisites

* [Install the Linux kernel headers on the hosts](./install-kernel-headers.md).
* [Install `kubectl` version `>= 1.22`](https://kubernetes.io/docs/tasks/tools/)

## 1: Install Piraeus Operator

In this tutorial we will be using `kubectl` with the built-in `kustomize` feature to deploy Piraeus Operator.
All resources needed to run Piraeus Operator are included in a single Kustomization.

Install Piraeus Operator by running:

```bash
$ kubectl apply --server-side -k "https://github.com/piraeusdatastore/piraeus-operator//config/default?ref=v2.4.1"
namespace/piraeus-datastore configured
...
```

The Piraeus Operator will be installed in a new namespace `piraeus-datastore`. After a short wait the operator will be
ready. The following command waits until the Operator is ready:

```
$ kubectl wait pod --for=condition=Ready -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
pod/piraeus-operator-controller-manager-dd898f48c-bhbtv condition met
```

## 2: Deploy Piraeus Datastore

Now, we will deploy Piraeus Datastore using a new resource managed by Piraeus Operator. We create a `LinstorCluster`,
which creates all the necessary resources (Deployments, Pods, and so on...) for our Datastore:

```
$ kubectl apply -f - <<EOF
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec: {}
EOF
```

Again, all workloads will be deployed to the `piraeus-datastore` namespace. After a short wait the Datastore will ready:

```
$ kubectl wait pod --for=condition=Ready -n piraeus-datastore -l app.kubernetes.io/name=piraeus-datastore
pod/linstor-controller-65cbbc74db-9vm9n condition met
pod/linstor-csi-controller-5ccb7d84cd-tvd9h condition met
pod/linstor-csi-node-2lkpd condition met
pod/linstor-csi-node-hbcvv condition met
pod/linstor-csi-node-hmrd7 condition met
pod/n1.example.com condition met
pod/n2.example.com condition met
pod/n3.example.com condition met
pod/piraeus-operator-controller-manager-dd898f48c-bhbtv condition met
```

We can now inspect the state of the deployed LINSTOR® Cluster using the `linstor` client:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor node list
+-------------------------------------------------------------------+
| Node           | NodeType  | Addresses                   | State  |
|===================================================================|
| n1.example.com | SATELLITE | 10.116.72.166:3366 (PLAIN)  | Online |
| n2.example.com | SATELLITE | 10.127.183.190:3366 (PLAIN) | Online |
| n3.example.com | SATELLITE | 10.125.97.33:3366 (PLAIN)   | Online |
+-------------------------------------------------------------------+
```

## 3: Configuring Storage

We have not yet configured any storage location for our volumes. This can be accomplished by creating a new
`LinstorSatelliteConfiguration` resource. We will create a storage pool of type `fileThinPool` on each node.
We chose `fileThinPool` as it does not require further configuration on the host.

```
$ kubectl apply -f - <<EOF
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: storage-pool
spec:
  storagePools:
    - name: pool1
      fileThinPool:
        directory: /var/lib/piraeus-datastore/pool1
EOF
```

This will cause some Pods to be recreated. While this occurs `linstor node list` will temporarily show offline nodes:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor node list
+--------------------------------------------------------------------+
| Node           | NodeType  | Addresses                   | State   |
|====================================================================|
| n1.example.com | SATELLITE | 10.116.72.166:3366 (PLAIN)  | OFFLINE |
| n2.example.com | SATELLITE | 10.127.183.190:3366 (PLAIN) | OFFLINE |
| n3.example.com | SATELLITE | 10.125.97.33:3366 (PLAIN)   | OFFLINE |
+--------------------------------------------------------------------+
```

Waiting a bit longer, the nodes will be `Online` again. Once the nodes are connected again, we can verify that
the storage pools were configured:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor storage-pool list
+---------------------------------------------------------------------------------------------------------------------------------------------------------+
| StoragePool          | Node           | Driver    | PoolName                         | FreeCapacity | TotalCapacity | CanSnapshots | State | SharedName |
|=========================================================================================================================================================|
| DfltDisklessStorPool | n1.example.com | DISKLESS  |                                  |              |               | False        | Ok    |            |
| DfltDisklessStorPool | n2.example.com | DISKLESS  |                                  |              |               | False        | Ok    |            |
| DfltDisklessStorPool | n3.example.com | DISKLESS  |                                  |              |               | False        | Ok    |            |
| pool1                | n1.example.com | FILE_THIN | /var/lib/piraeus-datastore/pool1 |    24.54 GiB |     49.30 GiB | True         | Ok    |            |
| pool1                | n2.example.com | FILE_THIN | /var/lib/piraeus-datastore/pool1 |    23.03 GiB |     49.30 GiB | True         | Ok    |            |
| pool1                | n3.example.com | FILE_THIN | /var/lib/piraeus-datastore/pool1 |    26.54 GiB |     49.30 GiB | True         | Ok    |            |
+---------------------------------------------------------------------------------------------------------------------------------------------------------+
```

## 4: Using Piraeus Datastore

We now have successfully deployed and configured Piraeus Datastore, and are ready to create our first
[`PersistentVolume`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) in Kubernetes.

First, we will set up a new [`StorageClass`](https://kubernetes.io/docs/concepts/storage/storage-classes/) for our
volumes. In the `StorageClass`, we specify the storage pool from above:

```
$ kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-storage
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  linstor.csi.linbit.com/storagePool: pool1
EOF
```

Next, we will create a [`PersistentVolumeClaim`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
requesting 1G of storage from our newly created `StorageClass`.

```
$ kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-volume
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  accessModes:
    - ReadWriteOnce
EOF
```

When we check the created PersistentVolumeClaim, we can see that it remains in `Pending` state.

```
$ kubectl get persistentvolumeclaim
NAME          STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS      AGE
data-volume   Pending                                      piraeus-storage   14s
```

We first need to create a "consumer", which in this case is just a `Pod`. For our consumer, we will create a Deployment
for a simple web server, serving files from our volume.

```
$ kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-server
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: web-server
  template:
    metadata:
      labels:
        app.kubernetes.io/name: web-server
    spec:
      containers:
        - name: web-server
          image: nginx
          volumeMounts:
            - mountPath: /usr/share/nginx/html
              name: data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: data-volume
EOF
```

After a short wait, the Pod is `Running`, and our `PersistentVolumeClaim` is now `Bound`:

```
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=web-server
pod/web-server-84867b5449-hgdzx condition met
$ kubectl get persistentvolumeclaim
NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS      AGE
data-volume   Bound    pvc-9e1149e7-33db-47a7-8fc6-172514422143   1Gi        RWO            piraeus-storage   1m
```

Checking the running container, we see that the volume is mounted where we expected it:

```
$ kubectl exec deploy/web-server -- df -h /usr/share/nginx/html
Filesystem      Size  Used Avail Use% Mounted on
/dev/drbd1000   973M   24K  906M   1% /usr/share/nginx/html
```

Taking a look with the `linstor` client, we can see that the volume is listed in LINSTOR and marked as `InUse` by the
Pod.

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor resource list-volumes
+-------------------------------------------------------------------------------------------------------------------------------------------+
| Node           | Resource                                 | StoragePool | VolNr | MinorNr | DeviceName    | Allocated | InUse  |    State |
|===========================================================================================================================================|
| n1.example.com | pvc-9e1149e7-33db-47a7-8fc6-172514422143 | pool1       |     0 |    1000 | /dev/drbd1000 | 16.91 MiB | InUse  | UpToDate |
+-------------------------------------------------------------------------------------------------------------------------------------------+
```

We have now successfully set up Piraeus Datastore and used it to provision a Persistent Volume in a Kubernetes cluster.
