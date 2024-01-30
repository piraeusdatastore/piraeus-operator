# General notes

During the upgrade process, provisioning of volumes and attach/detach operations might not work. Existing
volumes and volumes already in use by a pod will continue to work without interruption.

To upgrade, apply the resource of the latest release. Use the same method that was used to create the initial deployment
(`kubectl` vs `helm`). There is no need to change existing `LinstorCluster`, `LinstorSatelliteConfiguration` or
`LinstorNodeConnection` resources.

To upgrade to the latest release using `kubectl`, run the following commands:

```
$ kubectl apply --server-side -k "https://github.com/piraeusdatastore/piraeus-operator//config/default?ref=v2.4.0"
$ kubectl wait pod --for=condition=Ready -n piraeus-datastore --all
```

# Upgrades from v2.3 to v2.4

Generally, no special steps required.

LINSTOR Satellites are now managed via DaemonSet resources. Any patch targeting a `satellite` Pod resources is
automatically converted to the equivalent DaemonSet resource patch. In the Pod list, you will see these Pods using
a new `linstor-satellite` prefix.

# Upgrades from v2.2 to v2.3

Removed the `NetworkPolicy` resource from default deployment. It can be reapplied as a
[separate step](./docs/how-to/network-policy.md).

# Upgrades from v2.1 to v2.2

Removed the dependency on cert-manager for the initial deployment. To clean up an existing `Certificate` resource,
run the following commands:

```
$ kubectl delete certificate -n piraeus-datastore piraeus-operator-serving-cert
```

# Upgrades from v2.0 to v2.1

No special steps required.

# Upgrades from v1 to v2

Please follow the specialized [upgrade guides](./docs/how-to/upgrade/index.md).

# Upgrade from v1.9 to v1.10

If you want to share DRBD configuration directories with the host, please update the CRDs before upgrading using helm:

```
$ kubectl replace -f ./charts/piraeus/crds
```

# Upgrade from v1.8 to v1.9

If you want to protect metrics endpoints, take a look on [guide on enabling rbac-proxy sidecars](doc/security.md).

We've also disabled the `haController` component in our chart. The replacement is available from
[artifacthub.io](https://artifacthub.io/packages/helm/piraeus-charts/piraeus-ha-controller), containing much needed
improvements in robustness and fail-over speed. If you want to still use the old version, set
`haController.enabled=true`.

# Upgrade from v1.7 to v1.8

If you need to set the number of worker threads, or you need to set the log level of LINSTOR components, please update
the CRDs before upgrading using helm:

```
$ kubectl replace -f ./charts/piraeus/crds
```

In case if you have SSL enabled installation, you need to regenerate your certificates in PEM format.
Read the [guide on securing the deployment](doc/security.md) and repeat the described steps.

# Upgrade from v1.6 to v1.7

Node labels are now automatically applied to LINSTOR satellites as "Auxiliary Properties". That means you can reuse your
existing Kubernetes Topology information (for example `topology.kubernetes.io/zone` labels) when scheduling volumes
using the `replicasOnSame` and `replicasOnDifferent` settings. However, this also means that the Operator will delete
any existing auxiliary properties that were already applied. To apply auxiliary properties to satellites, you _have to_
apply a label to the Kubernetes node object.

# Upgrade from v1.5 to v1.6

The `csi-snapshotter` subchart was removed from this repository. Users who relied on it for their snapshot support
should switch to the seperate charts provided by the Piraeus team on [artifacthub.io](https://artifacthub.io/packages/search?repo=piraeus-charts)
The new charts also include notes on how to upgrade to newer CRD versions.

To support the new `csi.kubeletPath` option an update to the LinstorCSIDriver CRD is required:

```
$ kubectl replace -f ./charts/piraeus/crds
```

# Upgrade from v1.4 to v1.5

To make use of the new `monitoringImage` value in the LinstorSatelliteSet CRD, you need to replace the existing CRDs
before running the upgrade. If the CRDs are not upgraded, the operator will not deploy the monitoring container
alongside the satellites.

```
$ kubectl replace -f ./charts/piraeus/crds
```

Then run the upgrade:

```
helm upgrade piraeus-op ./charts/piraeus -f <overrides>
```

# Upgrade from v1.3 to v1.4

No special steps required, unless using the newly introduced `additionalEnv` and `additionalProperties` overrides.

If you plan to use the new overrides, replace the CustomResourceDefinitions before running the upgrade

```
$ kubectl replace -f ./charts/piraeus/crds
```

Then run the upgrade:

```
helm upgrade piraeus-op ./charts/piraeus -f <overrides>
```

# Upgrade from v1.2 to v1.3

No special steps required. After checking out the new repo, run a simple helm upgrade:

```
helm upgrade piraeus-op ./charts/piraeus -f <overrides>
```

# Upgrade from v1.1 to v1.2

Piraeus v1.2 is supported on Kubernetes 1.17+. If you are using an older Kubernetes distribution, you may need
to change the default settings (for example [the CSI provisioner](https://kubernetes-csi.github.io/docs/external-provisioner.html))

To start the upgrade process, ensure you have a backup of the LINSTOR Controller database. If you are using
the etcd deployment included in Piraeus, you can create a backup using:

```
kubectl exec piraeus-op-etcd-0 -- etcdctl snapshot save /tmp/save.db
kubectl cp piraeus-op-etcd-0:/tmp/save.db save.db
```

Now you can start the upgrade process. Simply run `helm upgrade piraeus-op ./charts/piraeus`. If you installed Piraeus
with customization, pass the same options you used for `helm install` to `helm upgrade`. This will cause the operator
pod to be re-created and shortly after all other Piraeus pods.

There is a known issue when updating the CSI components: the pods will not be updated to the newest image and the
`errors` section of the LinstorCSIDrivers resource shows an error updating the DaemonSet. In this case, manually
delete `deployment/piraeus-op-csi-controller` and `daemonset/piraeus-op-csi-node`. They will be re-created by the
operator.

After a short wait, all pods should be running and ready. Check that no errors are listed in the status section of
LinstorControllers, LinstorSatelliteSets and LinstorCSIDrivers.

# Upgrade from v1.0 to v1.1

* The LINSTOR controller image given in `operator.controller.controllerImage` has to have
  its entrypoint set to [`k8s-await-election v0.2.0`](https://github.com/LINBIT/k8s-await-election/)
  or newer. All images starting with `piraeus-server:v1.8.0` meet this requirement.

  Older images will not work, as the `Service` will not automatically pick up on the active pod.

  To upgrade, first update the deployed LINSTOR image to a compatible version, then upgrade the
  operator.

# Upgrade to v1.0

Upgrades from v0.* versions to v1.0 are best-effort only. The following guide assumes
an upgrade from v0.5.0.

* CRDs have been updated and set to version `v1`. You need to replace any existing CRDs with these new ones:
  ```
  kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorcontrollers_crd.yaml
  kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorcsidrivers_crd.yaml
  kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorsatellitesets_crd.yaml
  ```

* Renamed `LinstorNodeSet` to `LinstorSatelliteSet`. This brings the operator in line with other LINSTOR resources.
  Existing `LinstorNodeSet` resources will automatically be migrated to `LinstorSatelliteSet`. The old resources will
  not be deleted. You can verify that migration was successful by running the following command:
  ```
  $ kubectl get linstornodesets.piraeus.linbit.com -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.ResourceMigrated}{"\t"}{.status.DependantsMigrated}{"\n"}{end}'
  piraeus-op-ns true    true
  ```
  If both values are `true`, migration was successful. The old resource can be removed after migration.

* Renamed `LinstorControllerSet` to `LinstorController`. The old name implied the existence of multiple (separate)
  controllers. Existing `LinstorControllerSet` resources will automatically be migrated to `LinstorController`. The old
  resources will not be deleted. You can verify that migration was successful by running the following command:
  ```
  $ kubectl get linstorcontrollersets.piraeus.linbit.com -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.ResourceMigrated}{"\t"}{.status.DependantsMigrated}{"\n"}{end}'
  piraeus-op-cs true    true
  ```
  If both values are `true`, migration was successful. The old resource can be removed after migration.

* Along with the CRDs, Helm settings changed too:
  * `operator.controllerSet` to `operator.controller`
  * `operator.nodeSet` to `operator.satelliteSet`
  If old settings are used, Helm will return an error.

* Node scheduling no longer relies on `linstor.linbit.com/piraeus-node` labels. Instead, all CRDs support
  setting pod [affinity] and [tolerations].
  In detail:
  * `linstorcsidrivers` gained 4 new resource keys, with no change in default behaviour:
    * `nodeAffinity` affinity passed to the csi nodes
    * `nodeTolerations` tolerations passed to the csi nodes
    * `controllerAffinity` affinity passed to the csi controller
    * `controllerTolerations` tolerations passed to the csi controller
  * `linstorcontrollerset` gained 2 new resource keys, with no change in default behaviour:
    * `affinity` affinity passed to the linstor controller pod
    * `tolerations` tolerations passed to the linstor controller pod
  * `linstornodeset` gained 2 new resource keys, **with change in default behaviour**:
    * `affinity` affinity passed to the linstor controller pod
    * `tolerations` tolerations passed to the linstor controller pod

    Previously, linstor satellite pods required nodes marked with `linstor.linbit.com/piraeus-node=true`. The new
    default value does not place this restriction on nodes. To restore the old behaviour, set the following affinity:
    ```yaml
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
          - matchExpressions:
            - key: linstor.linbit.com/piraeus-node
              operator: In
              values:
              - "true"
    ```

[affinity]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
[tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/

* Other changed helm settings:
  `drbdKernelModuleInjectionMode`: renamed to `kernelModuleInjectionMode`
  `kernelModImage`: renamed to `kernelModuleInjectionImage`
  If old settings are used, Helm will return an error.

# Upgrade between v0.* versions

While the API version was `v1alpha1`, this project did not maintain
stability of or provide conversion of the Custom Resource Definitions.

If you are using the Helm deployment, you may find that upgrades fail with
errors similar to the following:

```
UPGRADE FAILED: cannot patch "piraeus-op-cs" with kind LinstorController: LinstorController.piraeus.linbit.com "piraeus-op-cs" is invalid: spec.etcdURL: Required value
```

The simplest solution in this case is to manually replace the CRD:

```
kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorcsidrivers_crd.yaml
kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorsatellitesets_crd.yaml
kubectl replace -f charts/piraeus/crds/piraeus.linbit.com_linstorcontrollers_crd.yaml
```

Then continue with the Helm upgrade. Values that are lost during the
replacement will be set again by Helm.
