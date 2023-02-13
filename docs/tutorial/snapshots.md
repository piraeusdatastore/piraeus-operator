# Creating and Restoring From Snapshots

Learn how to create snapshots of your data, and how you can restore the data in a snapshot.

Snapshots create a copy of the volume content at a particular point in time. This copy remains untouched when you make
modifications to the volume content. This, for example, enables you to create backups of your data before performing
modifications or deletion on your data.

Since a backup is useless, unless you have a way to restore it, this tutorial will teach you both how to create a
snapshot, and how to restore in the case of accidental deletion of your data.

## Prerequisites

* An installed and configured Piraeus Datastore. Learn how to get started in our [introduction tutorial](./get-started.md)
* A storage pool supporting snapshots. LINSTOR supports snapshots for `LVM_THIN`, `FILE_THIN`, `ZFS` and `ZFS_THIN` pools.
  If you followed [the introduction tutorial](./get-started.md), you are using the supported `FILE_THIN` pool.
* A cluster with [`snapshot-controller`](https://github.com/kubernetes-csi/external-snapshotter/) deployed. To check if
  it is already deployed, try running:
  ```
  $ kubectl api-resources --api-group=snapshot.storage.k8s.io -oname
  volumesnapshotclasses.snapshot.storage.k8s.io
  volumesnapshotcontents.snapshot.storage.k8s.io
  volumesnapshots.snapshot.storage.k8s.io
  ```
  If your output looks like above, you are good to go.
  If your output is empty, you should deploy a snapshot controller. You can quickly deploy it by using:
  ```
  kubectl apply -k https://github.com/kubernetes-csi/external-snapshotter//client/config/crd
  kubectl apply -k https://github.com/kubernetes-csi/external-snapshotter//deploy/kubernetes/snapshot-controller
  ```

## Creating an Example Workload

We will be using the same workload as in the [replicated volumes tutorial](./replicated-volumes.md). This workload
will save the Pods name, the node it is running on, and a timestamp to our volume. By logging this information to our
volume, we can easily keep track of our data.

First, we create our `StorageClass`, `PersistentVolumeClaim` and `Deployment`:

```
$ kubectl apply -f - <<EOF
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-storage
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  linstor.csi.linbit.com/storagePool: pool1
---
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
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: volume-logger
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: volume-logger
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app.kubernetes.io/name: volume-logger
    spec:
      terminationGracePeriodSeconds: 0
      containers:
        - name: volume-logger
          image: busybox
          args:
            - sh
            - -c
            - |
              echo "Hello from \$HOSTNAME, running on \$NODENAME, started at \$(date)" >> /volume/hello
              # We use this to keep the Pod running
              tail -f /dev/null
          env:
            - name: NODENAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          volumeMounts:
            - mountPath: /volume
              name: data-volume
      volumes:
        - name: data-volume
          persistentVolumeClaim:
            claimName: data-volume
EOF
```

Then, we wait for the Pod to start, and verify that the expected information was logged to our volume:

```
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-cbcd897b7-jrmks condition met
$ kubectl exec deploy/volume-logger -- cat /volume/hello
Hello from volume-logger-cbcd897b7-jrmks, running on n3.example.com, started at Mon Feb 13 15:32:46 UTC 2023
```

## Creating a Snapshot

Creating a snapshot requires the creation of a [`SnapshotClass`](https://kubernetes.io/docs/concepts/storage/volume-snapshot-classes/) first.
The `SnapshotClass` specifies our `linstor.csi.linbit.com` provisioner, and sets the clean-up policy for our snapshots
to `Delete`, meaning deleting the Kubernetes resources will also delete the snapshots in LINSTOR®.

```
$ kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: piraeus-snapshots
driver: linstor.csi.linbit.com
deletionPolicy: Delete
EOF
```

Next, we will request the creation of a snapshot using a [`VolumeSnapshot`](https://kubernetes.io/docs/concepts/storage/volume-snapshots/#volumesnapshots) resource.
The `VolumeSnapshot` resource references the `PersistentVolumeClaim` resource we created initially, as well as our
newly created `VolumeSnapshotClass`.

```
$ kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: data-volume-snapshot-1
spec:
  volumeSnapshotClassName: piraeus-snapshots
  source:
    persistentVolumeClaimName: data-volume
EOF
```

Now, we need to wait for the snapshot to be created. We can then also verify its creation in LINSTOR:

```
$ kubectl wait volumesnapshot --for=jsonpath='{.status.readyToUse}'=true data-volume-snapshot-1
volumesnapshot.snapshot.storage.k8s.io/data-volume-snapshot-1 condition met
$ kubectl get volumesnapshot data-volume-snapshot-1
NAME                     READYTOUSE   SOURCEPVC     SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS       SNAPSHOTCONTENT                                    CREATIONTIME   AGE
data-volume-snapshot-1   true         data-volume                           1Gi           piraeus-snapshots   snapcontent-a8757c1d-cd37-42d2-9557-a24b1222d118   15s            15s
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor snapshot list
+---------------------------------------------------------------------------------------------------------------------------------------------------------+
| ResourceName                             | SnapshotName                                  | NodeNames      | Volumes  | CreatedOn           | State      |
|=========================================================================================================================================================|
| pvc-9c04b307-d22d-454f-8f24-ed5837fe4426 | snapshot-a8757c1d-cd37-42d2-9557-a24b1222d118 | n3.example.com | 0: 1 GiB | 2023-02-13 15:36:18 | Successful |
+---------------------------------------------------------------------------------------------------------------------------------------------------------+
```

## Modifying the Data

Now we want to simulate a situation where we accidentally delete some important data. The important data in our example
workload is the log of Pod name, Node name and timestamp. We will manually delete the file on the volume to simulate
accidental removal of an important file on a persistent volume:

```
$ kubectl exec deploy/volume-logger -- rm /volume/hello
$ kubectl exec deploy/volume-logger -- cat /volume/hello
cat: can't open '/volume/hello': No such file or directory
command terminated with exit code 1
```

## Restoring From a Snapshot

This is the exact situation where snapshots can come in handy. Since we created a snapshot before we made removed the
file, we can create a new volume to recover our data. We will be replacing the existing `data-volume` with a new version
based on the snapshot.

First, we will stop the Deployment by scaling it down to zero Pods, so that it does not interfere with our next steps:

```
$ kubectl scale deploy/volume-logger --replicas=0
deployment.apps "volume-logger" deleted
$ kubectl rollout status deploy/volume-logger
deployment "volume-logger" successfully rolled out
```

Next, we will also remove the `PersistentVolumeClaim`. We still have the snapshot which contains the data we want to
restore, so we can safely remove the volume.

```
$ kubectl delete pvc/data-volume
persistentvolumeclaim "data-volume" deleted
```

Now, we will create a new `PersistentVolumeClaim`, referencing our snapshot. This will create a volume, using the data
from the snapshot.

```
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-volume
spec:
  storageClassName: piraeus-storage
  resources:
    requests:
      storage: 1Gi
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: data-volume-snapshot-1
  accessModes:
    - ReadWriteOnce
EOF
```

Since we named this new volume `data-volume`, we can just scale up our Deployment again, and the new Pod will start
using the restored volume:

```
$ kubectl scale deploy/volume-logger --replicas=1
deployment.apps/volume-logger scaled
```

After the Pod started, we can once again verify the content of our volume:

```
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-cbcd897b7-5qjbz condition met
$ kubectl exec deploy/volume-logger -- cat /volume/hello
Hello from volume-logger-cbcd897b7-jrmks, running on n3.example.com, started at Mon Feb 13 15:32:46 UTC 2023
Hello from volume-logger-cbcd897b7-gr6hh, running on n3.example.com, started at Mon Feb 13 15:42:17 UTC 2023
```

You have now successfully created a snapshot, and used it to back up and restore a volume after accidental deletion.
