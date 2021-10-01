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
of all LINSTOR resources before upgrading the cluster. In a future release, this step will be managed by the operator.

## Creating a backup of LINSTOR internal resources

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
