# How to Restore a LINSTOR Database Backup

This guide shows you how to restore a LINSTOR® Controller from a database backup. A backup is created automatically
on every database migration of the default `k8s` database.

> [!CAUTION]
> Restoring from a backup means all changes made to the Cluster state after the backup was taken are lost.
> This includes information about Persistent Volumes and Volume Snapshots that where created after the backup.

To complete this guide, you should be familiar with:

* using the `kubectl` command line tool to access the Kubernetes cluster.

## Find the Latest Backup

The backup is stored in Kubernetes Secrets. List all backups by using the following command:

```
$ kubectl get secrets --field-selector type=piraeus.io/linstor-backup --sort-by .metadata.creationTimestamp -ocustom-columns="NAME:.metadata.name,CREATED-AT:metadata.creationTimestamp,VERSION:.metadata.annotations.piraeus\.io/linstor-version"
NAME                                                     CREATED-AT             VERSION
linstor-backup-for-linstor-controller-db7fbfd95-zsfmm    2024-11-04T07:54:29Z   LINSTOR Controller 1.27.0
linstor-backup-for-linstor-controller-745d54bf99-544hf   2024-11-04T08:03:59Z   LINSTOR Controller 1.29.1
```

Select the backup you want to restore, making note of the name. For example, to restore the LINSTOR 1.27.0 version,
we set:

```
$ BACKUP_NAME=linstor-backup-for-linstor-controller-db7fbfd95-zsfmm
```

## Temporarily stop the LINSTOR Controller Deployment

To safely restore the database, ensure that the LINSTOR Controller is shut down by scaling the Piraeus Operator and
LINSTOR Controller deployment to 0 replicas:

```
$ kubectl scale deployment piraeus-operator-controller-manager --replicas 0
deployment.apps/piraeus-operator-controller-manager scaled
$ kubectl scale deployment linstor-controller --replicas 0
deployment.apps/linstor-controller scaled
```

## Create a New Backup of the Current Cluster State

Since the restore process is destructive, first create a backup of the current database:

```
$ mkdir backup
$ cd backup
$ kubectl api-resources --api-group=internal.linstor.linbit.com -oname | xargs --no-run-if-empty kubectl get crds -oyaml > crds.yaml
$ kubectl api-resources --api-group=internal.linstor.linbit.com -oname | xargs --no-run-if-empty -I {} sh -c 'kubectl get {} -oyaml > {}.yaml'
```

## Restore the Database

Copy and unpack the selected backup to a local directory by using the following commands:

```
$ mkdir restore
$ cd restore
# Replace $BACKUP_NAME with your selected backup name
$ kubectl get secrets --sort-by=.metadata.name -l piraeus.io/backup=$BACKUP_NAME -ogo-template='{{range .items}}{{index .data "backup.tar.gz" | base64decode}}{{end}}' > backup.tar.gz
$ tar -xvf backup.tar.gz
crds.yaml
[...]
```

Then, replace the current database with the database from the backup:

```
$ kubectl api-resources --api-group=internal.linstor.linbit.com -oname | xargs --no-run-if-empty kubectl delete crds
$ kubectl create -f .
```

## Restart the LINSTOR Controller Deployment

Now we can safely restart the Piraeus Operator and LINSTOR Controller Deployment.

```
$ kubectl scale deployment piraeus-operator-controller-manager --replicas 1
deployment.apps/piraeus-operator-controller-manager scaled
$ kubectl scale deployment linstor-controller --replicas 1
deployment.apps/linstor-controller scaled
$ kubectl rollout status deployment piraeus-operator-controller-manager
Waiting for deployment "piraeus-operator-controller-manager" rollout to finish: 0 of 1 updated replicas are available...
deployment "piraeus-operator-controller-manager" successfully rolled out
$ kubectl rollout status deployment linstor-controller
Waiting for deployment "linstor-controller" rollout to finish: 0 of 1 updated replicas are available...
deployment "linstor-controller" successfully rolled out
```
