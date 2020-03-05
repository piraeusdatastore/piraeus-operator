# Piraeus Operator

This is the initial public alpha for the Piraeus Operator. Currently, it is
suitable for testing and development.

## Contributing

This Operator is currently under heavy development: documentation and examples
will change frequently. Always use the latest release.

If you'd like to contribute, please visit https://gitlab.com/linbit/piraeus-operator
and look through the issues to see if there something you'd like to work on. If
you'd like to contribute something not in an existing issue, please open a new
issue beforehand.

If you'd like to report an issue, please use the issues interface in this
project's gitlab page.

## Building and Development

This project is built using the operator-sdk (version 0.13.0). Please refer to
the [documentation for the sdk](https://github.com/operator-framework/operator-sdk/tree/v0.13.x).

## Deployment with Helm v3 Chart

The operator can be deployed with Helm v3 chart in /charts as follows:

- Label the worker nodes with `linstor.linbit.com/piraeus-node=true` by
  running:

    ```
    kubectl label node <NODE_NAME> linstor.linbit.com/piraeus-node=true
    ```

- If you are deploying with images from private repositories, create
  a kubernetes secret to allow obtaining the images.  This example will create
  a secret named `drbdiocred`:

    ```
    kubectl create secret docker-registry drbdiocred --docker-server=<SERVER> --docker-username=<YOUR_LOGIN> --docker-email=<YOUR_EMAIL> --docker-password=<YOUR_PASSWORD>
    ```

    The name of this secret must match the one specified in the Helm values, by
    default `drbdiocred`.

* Configure storage for the LINSTOR etcd instance.
  There are various options for configuring the etcd instance for LINSTOR:
  * Use an existing storage provisioner with a default `StorageClass`.
  * [Use `hostPath` volumes](#linstor-etcd-hostpath-persistence).
  * Disable persistence for basic testing. This can be done by adding `--set
    etcd.persistence.enabled=false` to the `helm install` command below.

- Configure the LVM VG and LV names by setting the following values in a local
  values file or by using `--set` with the `helm install` command below.

    ```
    operator.nodeSet.spec.lvmPoolVgName=drbdpool # <- Volume Group name of the thick storage pool
    operator.nodeSet.spec.lvmThinPoolVgName=drbdpool # <- Volume Group name of the thin storage pool
    operator.nodeSet.spec.lvmThinPoolLvName=thinpool # <- Logical Volume name of the thin pool
    ```

- Finally create a Helm deployment named `piraeus-op` that will set up
  everything.

    ```
    helm dependency update ./charts/piraeus
    helm install piraeus-op ./charts/piraeus
    ```

### LINSTOR etcd `hostPath` persistence

You can use the included Helm templates to create `hostPath` persistent volumes.
Create as many PVs as needed to satisfy your configured etcd `replicaCount`
(default 3).

Create the `hostPath` persistent volumes, substituting cluster node names
accordingly in the `nodes=` option:

```
helm install linstor-etcd ./charts/pv-hostpath --set "nodes={<NODE0>,<NODE1>,<NODE2>}"
```

Persistence for etcd is enabled by default. The
`etcd.volumePermissions.enabled` key in the Helm values is also set so that the
`hostPath` volumes have appropriate permissions.

### Terminating Helm deployment

The Piraeus deployment can be terminated with:

```
helm delete piraeus-op
```

However due to the Helm's current policy, the Custom Resource Definitions named
`linstorcontrollerset` and `linstornodeset` will __not__ be deleted by the
command.  This will also cause the instances of those CRD's named
`piraeus-op-ns` and `piraeus-op-cs` to remain running.

To terminate those instances after the `helm delete` command, run

```
kubectl patch linstorcontrollerset piraeus-op-cs -p '{"metadata":{"finalizers":[]}}' --type=merge
kubectl patch linstornodeset piraeus-op-ns -p '{"metadata":{"finalizers":[]}}' --type=merge
```

After that, all the instances created by the Helm deployment will be terminated.

More information regarding Helm's current position on CRD's can be found
[here](https://helm.sh/docs/topics/chart_best_practices/custom_resource_definitions/#method-1-let-helm-do-it-for-you).

## Deployment without using Helm v3 chart

### Configuration

The operator must be deployed within the cluster in order for it it to have access
to the controller endpoint, which is a kubernetes service. See the operator-sdk
guide on the Operator Framework for more information.

Worker nodes will only run on kubelets labeled with `linstor.linbit.com/piraeus-node=true`

```
kubectl label no my-worker-node linstor.linbit.com/piraeus-node=true
```

### Etcd

An etcd cluster must be running and reachable to use this operator. By default,
the controller will try to connect to `piraeus-etcd` on port `2379`

A simple in-memory etcd cluster can be set up using helm:

```
kubectl create -f examples/etcd-env-vars.yaml
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install piraeus-etcd bitnami/etcd --set statefulset.replicaCount=3 -f examples/etcd-values.yaml
```

If you are using Helm 2 and encountering difficulties with the above steps, you
may need to set RBAC rules for the tiller component of helm:

```
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
```

### Kubernetes Secret for Repo Access

If you are deploying with images from a private repository, create a kubernetes
secret to allow obtaining the images.  Create a secret named `drbdiocred` like
this:

```
kubectl create secret docker-registry drbdiocred --docker-server=<SERVER> --docker-username=<YOUR LOGIN> --docker-email=<YOUR EMAIL> --docker-password=<YOUR PASSWORD>
```

### Deploy Operator

Inspect the basic deployment example (examples/piraeus-operator-part-1.yaml), it may be deployed by:

```
kubectl create -f examples/piraeus-operator-part-1.yaml
```

Lastly, edit the storage nodes' LVM VG and LV names in examples/piraeus-operator-part-2.yaml.  Now you can finally deploy the LINSTOR cluster with:

```
kubectl create -f examples/piraeus-operator-part-2.yaml
```

## License

Apache 2.0
