# Creating Replicated Volumes

Learn about creating replicated volumes, making your data accessible on any cluster node.

In this tutorial you will learn how to create a replicated volume, and verify that the data stays accessible when
moving Pods from one node to another.

## Prerequisites

* A Kubernetes Cluster with at least two nodes.
* An installed and configured Piraeus Datastore. Learn how to get started in our [introduction tutorial](./get-started.md)

## Creating the Volume

First, we will create a new [`StorageClass`](https://kubernetes.io/docs/concepts/storage/storage-classes/) for our
replicated volumes. We will be using the `pool1` storage pool from the [Get Started tutorial](./get-started.md), but
this time also set the `placementCount` to 2, telling LINSTOR® to store the volume data on two nodes.

```
$ kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: piraeus-storage-replicated
provisioner: linstor.csi.linbit.com
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
parameters:
  linstor.csi.linbit.com/storagePool: pool1
  linstor.csi.linbit.com/placementCount: "2"
EOF
```

Next, we will again create a [`PersistentVolumeClaim`](https://kubernetes.io/docs/concepts/storage/persistent-volumes/),
requesting a 1G replicated volume from our newly created `StorageClass`.

```
$ kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: replicated-volume
spec:
  storageClassName: piraeus-storage-replicated
  resources:
    requests:
      storage: 1G
  accessModes:
    - ReadWriteOnce
EOF
```

For our workload, we will create a Pod which will use the replicated volume to log its name, the current date, and the
node it is running on.

```
$ kubectl apply -f - <<EOF
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
              name: replicated-volume
      volumes:
        - name: replicated-volume
          persistentVolumeClaim:
            claimName: replicated-volume
EOF
```

After a short wait, the Pod is `Running`, our `PersistentVolumeClaim` is now `Bound`, and we can see that LINSTOR
placed the volume on two nodes:

```
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-84dd47f4cb-trh4l
$ kubectl get persistentvolumeclaim
NAME                STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS                 AGE
replicated-volume   Bound    pvc-dbe422ac-c5ae-4786-a624-74d2be8a262d   976563Ki   RWO            piraeus-storage-replicated   1m
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor resource list-volumes
+-------------------------------------------------------------------------------------------------------------------------------------------+
| Node           | Resource                                 | StoragePool | VolNr | MinorNr | DeviceName    | Allocated | InUse  |    State |
|===========================================================================================================================================|
| n1.example.com | pvc-dbe422ac-c5ae-4786-a624-74d2be8a262d | pool1       |     0 |    1000 | /dev/drbd1000 | 16.91 MiB | InUse  | UpToDate |
| n2.example.com | pvc-dbe422ac-c5ae-4786-a624-74d2be8a262d | pool1       |     0 |    1000 | /dev/drbd1000 |   876 KiB | Unused | UpToDate |
+-------------------------------------------------------------------------------------------------------------------------------------------+
```

**NOTE:** If your cluster has three or more nodes, you will actually see a third volume, marked as `TieBreaker`. This is
intentional and improves the behaviour should one of the cluster nodes become unavailable.

Now, we can check that our Pod actually logged the expected information by reading `/volume/hello` in the Pod:

```
$ kubectl exec deploy/volume-logger -- cat /volume/hello
Hello from volume-logger-84dd47f4cb-trh4l, running on n1.example.com, started at Fri Feb  3 08:53:47 UTC 2023
```

## Testing Replication

Now, we will verify that when we move the Pod to another node, we still have access to the same data.

To test this, we will disable scheduling on the node the Pod is currently running on. This forces Kubernetes to move the
Pod to another node once we trigger a restart. In our examples, the Hello message tells us that the Pod was started on
`n1.example.com`, so this is the node we disable. Replace the name with your own node name.

```
$ kubectl cordon n1.example.com
node/n1.example.com cordoned
```

Now, we can trigger a new rollout of the deployment. Since we disabled scheduling `n1.example.com`, another node
will have to take over our Pod.

```
$ kubectl rollout restart deploy/volume-logger
deployment.apps/volume-logger restarted
$ kubectl wait pod --for=condition=Ready -l app.kubernetes.io/name=volume-logger
pod/volume-logger-5db9dd7b87-lps2f condition met
$ kubectl get pods -owide
NAME                             READY   STATUS    RESTARTS   AGE   IP            NODE             NOMINATED NODE   READINESS GATES
volume-logger-5db9dd7b87-lps2f   1/1     Running   0          26s   10.125.97.9   n2.example.com   <none>           <none>
```

As expected, the Pod is now running on a different node, in this case on `n2.example.com`.

Now, we can verify that the message from the original pod is still present:

```
$ kubectl exec deploy/volume-logger -- cat /volume/hello
Hello from volume-logger-84dd47f4cb-trh4l, running on n1.example.com, started at Fri Feb  3 08:53:47 UTC 2023
Hello from volume-logger-5db9dd7b87-lps2f, running on n2.example.com, started at Fri Feb  3 08:55:42 UTC 2023
```

As expected, we still see the message from `n1.example.com`, as well as the message from the new Pod on `n2.example.com`.

We can also see that LINSTOR now shows the volume as `InUse` on the new node:

```
$ kubectl -n piraeus-datastore exec deploy/linstor-controller -- linstor resource list-volumes
+-------------------------------------------------------------------------------------------------------------------------------------------+
| Node           | Resource                                 | StoragePool | VolNr | MinorNr | DeviceName    | Allocated | InUse  |    State |
|===========================================================================================================================================|
| n1.example.com | pvc-dbe422ac-c5ae-4786-a624-74d2be8a262d | pool1       |     0 |    1000 | /dev/drbd1000 | 16.91 MiB | Unused | UpToDate |
| n2.example.com | pvc-dbe422ac-c5ae-4786-a624-74d2be8a262d | pool1       |     0 |    1000 | /dev/drbd1000 |   952 KiB | InUse  | UpToDate |
+-------------------------------------------------------------------------------------------------------------------------------------------+
```

You have now successfully created a replicated volume and verified that the data is accessible from multiple nodes.

## Resetting the Disabled Node

Now that we have verified replication works, we can reset the disabled node:

```
$ kubectl uncordon n1.example.com
node/n1.example.com uncordoned
```
