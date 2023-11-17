# Remove the Piraeus Operator v1 Deployment

Migrating to the new Piraeus Operator deployment requires temporarily removing the existing deployment.

After this step no new volumes can be created and no existing volume can be attached or detached, until the
new deployment is rolled out.

This is the third step when migrating Piraeus Operator from version 1 (v1) to version 2 (v2).
[Click here to get back to the overview](./index.md).

## Prerequisites

* [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [Install `helm`](https://docs.helm.sh/docs/intro/install/)

## Scale Down the Operator Deployment

To prevent modification of the existing cluster, scale down the existing Piraeus Operator deployment.

```
$ helm upgrade piraeus-op ./charts/piraeus --set operator.replicas=0
$ kubectl rollout status -w deploy/piraeus-op-operator
deployment "piraeus-op-operator" successfully rolled out
```

## Remove the Finalizers from the Piraeus Resources

The Operator sets Finalizers on the resource it controls. This prevents the deletion of these resources when the
Operator is not running. Remove the Finalizers by applying a patch:

```
$ kubectl patch linstorsatellitesets piraeus-op-ns --type merge --patch '{"metadata": {"finalizers": []}}'
linstorsatelliteset.piraeus.linbit.com/piraeus-op-ns patched
$ kubectl patch linstorcontrollers piraeus-op-cs --type merge --patch '{"metadata": {"finalizers": []}}'
linstorcontroller.piraeus.linbit.com/piraeus-op-cs patched
```

## Remove the Piraeus Resources

Having removed the Finalizers, you can delete the Piraeus Resources. This will stop the LINSTOR Cluster, and no
new volumes can be created, and existing volumes will not attach or detach. Volumes already attached to a Pod will
continue to replicate.

```
$ kubectl delete linstorcsidrivers/piraeus-op
$ kubectl delete linstorsatellitesets/piraeus-op-ns
$ kubectl delete linstorcontrollers/piraeus-op-cs
```

## Remove the Piraeus Deployment

As a last step, you can completely remove the helm deployment, deleting additional resources such as service accounts
and RBAC resources. In addition, also clean up the Custom Resource Definitions:

```
$ helm uninstall piraeus-op
$ kubectl delete crds linstorcontrollers.piraeus.linbit.com linstorsatellitesets.piraeus.linbit.com linstorcontrollers.piraeus.linbit.com
```

## Optional: Remove additional Piraeus Components

If you have deployed additional components from Piraeus, such as the HA Controller or LINSTOR Affinity Controller, you
will need to remove them, too. After completing the migration, you can install them again, if needed.

```
$ helm uninstall piraeus-ha-controller
$ helm uninstall linstor-affinity-controller
```
