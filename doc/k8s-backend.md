# Using LINSTOR's Kubernetes Backend

Starting with release 1.16.0, [LINSTOR](https://github.com/linbit/linstor-server) can use the Kubernetes API directly
to store the cluster state. The LINSTOR controller manages all required API definitions, including on upgrade.

**NOTE:** It is currently **NOT** possible to migrate from an existing cluster with Etcd backend to the Kubernetes
backend.

When using the new backend you can disable deploying ETCD. This also means that no existing PersistentVolumes are
necessary. Create an override file named `k8s-backend.yaml`:

```yaml
etcd:
  enabled: false
operator:
  controller:
    dbConnectionURL: k8s
```

Then apply it using:

```
$ helm install piraeus-op ./charts/piraeus --values k8s-backend.yaml
```

However, since this feature is quite new, there might be situations in which LINSTOR fails to upgrade resources,
leaving the LINSTOR cluster in an unusable state. For this reason, we strongly recommend doing a manual backup
of all LINSTOR resources before upgrading the cluster.

Beginning with operator version v1.8.0, the operator will create a snapshot of all LINSTOR internal resources
before changing the controller image. The backup is available as a secret named `linstor-backup-<hash>`. To copy
the backup to your local machine, you can use the following command:

```
kubectl get secret linstor-backup-<hash> -o 'go-template={{index .data ".binaryData.backup.tar.gz" | base64decode}}' > linstor-backup.tar.gz
```

In case the backup is too large to fit into a config map, the backup is instead written to the operator container.
You will see an error message on the `LinstorController` resource, instructing you how to continue.

First you will want to copy the backup to a safe location outside the pod and create an empty secret to indicate
that the backup is safe and the operator can proceed:

```
kubectl cp <piraeus-operator-pod>:/run/linstor-backups/linstor-backup-<some-hash>.tar.gz <destination-path>
kubectl create secret linstor-backup-<same-hash>
```

We will remove the `IHaveBackedUpAllMyLinstorResources=true` switch in a release after 1.8.0. Upgrades from before
1.8.0 will skip the above backup step, so you should always do a manual backup (described below) in this case.

## Manually creating a backup of LINSTOR internal resources

1. Stop the current controller:
   ```
   $ kubectl patch linstorcontroller piraeus-op-cs "{"spec":{"replicas": 0}}"
   $ kubectl rollout status --watch deployment/piraeus-op-cs-controller
   ```
2. The following command will create a file `crds.yaml`, which stores the current state of all LINSTOR Custom Resource
   Definitions:
   ```
   $ kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs kubectl get crds -oyaml > crds.yaml
   ```
3. In addition to the definitions, the actual resources must be be backed up as well:
   ```
   $ kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs -i{} sh -c "kubectl get {} -oyaml > {}.yaml"
   ```
4. Run the chart upgrade using `--set IHaveBackedUpAllMyLinstorResources=true` to acknowledge you have executed the
   above steps.

## Restore a backup after a failed upgrade

1. Unpack the backup
   ```
   $ tar xvf linstor-backup.tar.gz
   crds.yaml
   ....
   ```
2. Stop the current controller:
   ```
   $ kubectl patch linstorcontroller piraeus-op-cs "{"spec":{"replicas": 0}}"
   $ kubectl rollout status --watch deployment/piraeus-op-cs-controller
   ```
3. Delete existing resources
   ```
   $ kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs --no-run-if-empty kubectl delete crds
   ```
4. Apply the old LINSTOR CRDs
   ```
   $ kubectl apply -f crds.yaml
   ```
5. Apply the old LINSTOR resource state
   ```
   $ kubectl apply -f *.internal.linstor.linbit.com.yaml
   ```
6. Re-apply the helm chart using the old LINSTOR version
   ```
   $ helm upgrade piraeus-op charts/piraeus --set operator.controller.controllerImage=... --set operator.satelliteSet.satelliteImage=...
   ```

## Delete the database

If you are using the `k8s` database backend, you may want to remove the associated resources after removing Piraeus.
Otherwise, you may run LINSTOR with outdated information if you decide to install it again. To create a local backup
and then delete the database resources, run:

```
kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs kubectl get crds -oyaml > crds.yaml
kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs -i{} sh -c "kubectl get {} -oyaml > {}.yaml"
# Check the the resources are properly backed up now!
# Then delete the LINSTOR database:
kubectl get crds | grep -o ".*.internal.linstor.linbit.com" | xargs --no-run-if-empty kubectl delete crds
```
