The following document describes how to convert

# Upgrade to v1.0

Upgrades from v0.* versions to v1.0 are best-effort only. The following guide assumes
an upgrade from v0.5.0.

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

While the API version remains at `v1alpha1`, this project does not maintain
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
