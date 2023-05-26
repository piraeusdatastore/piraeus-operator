# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- New DRBD loader detection for:
  - Debian 12 (Bookworm)
  - Rocky Linux 8 & 9
- Report `seLinuxMount` capability for the CSI Driver, speeding up volume mounts with SELinux
  relabelling enabled.

## [v2.4.0] - 2024-01-30

### Added

- Validating Webhook for Piraeus StorageClasses.

### Breaking

- Use DaemonSet to manage Satellite resources instead of bare Pods. This enables better integration with
  common administrative tasks such as node draining. This change should be transparent for users, any patches
  applied on the satellite Pods are internally converted to work on the new DaemonSet instead.

### Changed

- Change default monitoring address for DRBD Reactor to support systems with IPv6 completely disabled.
- Updated images:
  * LINSTOR 1.26.1
  * LINSTOR CSI 1.4.0
  * DRBD 9.2.7
  * High Availability Controller 1.2.0
  * Latest CSI sidecars

## [v2.3.0] - 2023-12-05

### Added

- Add a new option `.spec.internalTLS.tlsHandshakeDaemon` to enable deployment of `tlshd` alongside the LINSTOR
  Satellite.
- Shortcuts to configure specific components. Components can be disabled by setting `enabled: false`, and the deployed
  workload can be influenced using the `podTemplate` value. Available components:
  - `LinstorCluster.spec.controller`
  - `LinstorCluster.spec.csiController`
  - `LinstorCluster.spec.csiNode`
  - `LinstorCluster.spec.highAvailabilityController`
- Shortcut to modify the pod of a satellite by adding a `LinstorSatelliteConfiguration.spec.podTemplate`, which is
  a shortcut for creating a `.spec.patches` patch.

### Changed

- Fixed service resources relying on default protocol version.
- Moved NetworkPolicy for DRBD out of default deployed resources.
- Updated images:
  * LINSTOR 1.25.1
  * LINSTOR CSI 1.3.0
  * DRBD Reactor 1.4.0
  * Latest CSI sidecars
- Add a default toleration for the HA Controller taints to the operator.

## [v2.2.0] - 2023-08-31

### Added

- A new `LinstorNodeConnection` resource, used to configure the LINSTOR Node Connection feature in a Kubernetes way.
- Allow image configuration to be customized by adding additional items to the config map. Items using a "greater" key
  take precedence when referencing the same images.
- Add image configuration for CSI sidecars.
- Check kernel module parameters for DRBD on load.
- Automatically set SELinux labels when loading kernel modules.
- Allow more complex node selection by adding `LinstorCluster.spec.nodeAffinity`.

### Changed

- Upgrade to operator-sdk 1.29
- Upgrade to kubebuilder v4 layout
- Updated images:
  * LINSTOR 1.24.2
  * LINSTOR CSI 1.2.3
  * DRBD 9.2.5

### Removed

- Disable operator metrics by default. This removes a dependency on an external container.
- Dependency on cert-manager for initial deployment.

### Fixed

- A crash caused by insufficient permissions on the LINSTOR Controller.
- Satellite will now restart if the Pods terminated for unexpected reasons.

## [v2.1.1] - 2023-05-24

### Added

- LINSTOR Controller deployment now runs DB migrations as a separate init container, creating
  a backup of current DB state if needed.
- Apply global rate limit to LINSTOR API, defaulting to 100 qps.

### Changed

- Store LINSTOR Satellite logs on the host.
- Updated images:
  * LINSTOR 1.23.0
  * LINSTOR CSI 1.1.0
  * DRBD Reactor 1.2.0
  * HA Controller 1.1.4
  * external CSI images upgraded to latest versions

### Fixed

- Fixed a bug where `LinstorSatellite` resources would be not be cleaned up when the satellite is already gone.
- Fixed a bug where the LINSTOR Controller would never report readiness when TLS is enabled.
- Fixed order in which patches are applied. Always apply user patches last.

## [v2.1.0] - 2023-04-24

### Added

- Ability to skip deploying the LINSTOR Controller by setting `LinstorCluster.spec.externalController`.
- Automatically reconcile changed image configuration.

### Changed

- Fix an issue where the CSI node driver would use the CSI socket not through the expected path in the container.
- Updated images:
  * LINSTOR 1.22.0
  * LINSTOR CSI 1.0.1
  * DRBD 9.2.3
  * DRBD Reactor 1.1.0
  * HA Controller 1.1.3
  * external CSI images upgraded to latest versions

## [v2.0.1] - 2023-03-08

### Added

- `drbd-shutdown-guard` init-container, installing a systemd unit that runs on shutdown.
  It's purpose is to run `drbdsetup secondary --force` during shutdown, so that systemd can unmount all volumes, even with `suspend-io`.

### Changed

- Updated LINSTOR CSI to 1.0.0, mounting the `/run/mount` directory from the host to enable the `_netdev` mount option.

### Fixed

- HA Controller deployment requires additional rules to run on OpenShift.

## [v2.0.0] - 2023-02-23

### Breaking

- Removed existing CRD `LinstorController`, `LinstorSatelliteSet` and `LinstorCSIDriver`.
- Helm chart deprecated in favor of new `kustomize` deployment.
- Helm chart changed to only deploy the Operator. The LinstorCluster resource to create the storage cluster needs to be created separately.

### Added

- New CRDs to control storage cluster: `LinstorCluster` and `LinstorSatelliteConfiguration`.
- [Tutorials](./docs/tutorial) on how to get started.
- Automatic selection of loader images based on operating system of node.
- Customization of single nodes or groups of nodes.
- Possibility to run DRBD replication using the container network.
- Support for file system backed storage pools
- Default deployment for HA Controller. Since we switch to defaulting to `suspend-io` for lost quorum, we should include a way for Pods to get unstuck.

## [v1.10.0] - 2022-10-18

### Added

- Can set the variable `mountDrbdResourceDirectoriesFromHost` in the Helm chart to create hostPath Volumes for DRBD and LINSTOR configuration directories for the satellite set.

### Changed

- Change default bind address for satellite monitoring to use IPv6 anylocal `[::]`. This will still to work on IPv4
  only systems with IPv6 disabled via sysctl.
- Default images:
  * LINSTOR 1.20.0
  * DRBD 9.1.11
  * DRBD Reactor 0.9.0
  * external CSI images upgraded to latest versions

### Fixed

- Comparing IP addresses for registered components uses golang's built-in net.IP type.
- Restoring satellites after eviction only happens if the node is ready.

## [v1.9.1] - 2022-07-27

### Fixed

- ServiceMonitor resources can't be patched, instead we recreate them.

## [v1.9.0] - 2022-07-20

### Added

- Support for custom labels and annotations with added options to `values.yaml`.
- Instructions for deploying the affinity controller.

### Changed

- Satellite operator now reports basic satellite status even if controller is not reachable
- Query single satellite devices to receive errors when the satellite is offline instead of assuming
  devices are already configured.
- Disabled the legacy HA Controller deployment by default. It has been replaced a separate [chart](https://artifacthub.io/packages/helm/piraeus-charts/piraeus-ha-controller).
- Default images:
  * LINSTOR 1.19.1
  * LINSTOR CSI 0.20.0
  * DRBD 9.1.8
  * DRBD Reactor 0.8.0
  * external CSI images to latest versions

### Fixed

- `last-applied-configuration `annotation was never updated, so updating of some fields was not performed correctly.

## [v1.8.2] - 2022-05-24

### Added

- Option to disable creating monitoring resources (Services and ServiceMonitors)
- Add options `csi.controllerSidecars`, `csi.controllerExtraVolumes`, `csi.nodeSidecars`, `csi.nodeExtraVolumes`,
  `operator.controller.sidecars`, `operator.controller.extraVolumes`, `operator.satelliteSet.sidecars`,
  `operator.satelliteSet.extraVolumes` to allow specifying extra sidecar containers.
- Add options `operator.controller.httpBindAddress`, `operator.controller.httpsBindAddress`,
  `operator.satelliteSet.monitoringBindAddress` to allow specifying bind address.
- Add example values and doc reference to run piraeus-operator with rbac-proxy.

### Changed

- Default images:
  * LINSTOR 1.18.2
  * LINSTOR CSI 0.19.1
  * DRBD Reactor 0.7.0

### Removed
- Get rid of operator-sdk binary, use native controller-gen instead

## [v1.8.1] - 2022-05-12

### Changed

- Default images:
  * LINSTOR 1.18.1
  * LINSTOR CSI 0.19.0
  * DRBD 9.1.7
  * DRBD Reactor 0.6.1

### Added

- Upgrades involving k8s databases no long require manually confirming a backup exists using
  `--set IHaveBackedUpAllMyLinstorResources=true`.

## [v1.8.0] - 2022-03-15

### Added

- Allow setting the number of parallel requests created by the CSI sidecars. This limits the load on the LINSTOR
  backend, which could easily overload when creating many volumes at once.
- Unify certificates format for SSL enabled installation, no more java tooling required.
- Automatic certificates generation using Helm or Cert-manager
- HA Controller and CSI components now wait for the LINSTOR API to be initialized using InitContainers.

### Changed

- Create backups of LINSTOR resource if the "k8s" database backend is used _and_ an image change is detected. Backups
  are stored in Secret resources as a `tar.gz`. If the secret would get too big, the backup can be downloaded from
  the operator pod.
- Default images:
  * LINSTOR 1.18.0
  * LINSTOR CSI 0.18.0
  * DRBD 9.1.6
  * DRBD Reactor 0.5.3
  * LINSTOR HA Controller 0.3.0
  * CSI Attacher v3.4.0
  * CSI Node Driver Registrar v2.4.0
  * CSI Provisioner v3.1.0
  * CSI Snapshotter v5.0.1
  * CSI Resizer v1.4.0
  * Stork v2.8.2
- Stork updated to support Kubernetes v1.22+.
- Satellites no longer have a readiness probe defined. This caused issues in the satellites by repeatedly opening
  unexpected connections, especially when using SSL.
- Only query node devices if a storage pool needs to be created.
- Use cached storage pool response to avoid causing excessive load on LINSTOR satellites.
- Protect LINSTOR passphrase from accidental deletion by using a finalizer.

### Breaking

* If you have SSL configured, then the certificates must be regenerated in PEM format.
  Learn more in the [upgrade guide](./UPGRADE.md#upgrade-from-v10-to-head).

## [v1.7.1] - 2022-01-18

### Added

- Allow the external-provisioner and external-snapshotter access to secrets. This is required to support StorageClass
  and SnapshotClass [secrets](https://kubernetes-csi.github.io/docs/secrets-and-credentials-storage-class.html).
- Instruct external-provisioner to pass PVC name+namespace to the CSI driver, enabling optional support for PVC based
  names for LINSTOR volumes.
- Allow setting the log level of LINSTOR components via CRs. Other components are left using their default log level.
  The new default log level is INFO (was DEBUG previously, which was often too verbose).
- Override the kernel source directory used when compiling DRBD (defaults to /usr/src). See
  [`operator.satelliteSet.kernelModuleInjectionAdditionalSourceDirectory`](./doc/helm-values.adoc#operatorsatellitesetkernelmoduleinjectionadditionalsourcedirectory)
- etcd-chart: add option to set priorityClassName.

### Fixed

- Use correct secret name when setting up TLS for satellites
- Correctly configure ServiceMonitor resource if TLS is enabled for LINSTOR Controller.

## [v1.7.0] - 2021-12-14

### Added

- `pv-hostpath`: automatically determine on which nodes PVs should be created if no override is given.
- Automatically add labels on Kubernetes Nodes to LINSTOR satellites as Auxiliary Properties. This enables using
  Kubernetes labels for volume scheduling, for example using `replicasOnSame: topology.kubernetes.io/zone`.
- Support LINSTORs `k8s` backend by adding the necessary RBAC resources and [documentation](./doc/k8s-backend.md).
- Automatically create a LINSTOR passphrase when none is configured.
- Automatic eviction and deletion of offline satellites if the Kubernetes node object was also deleted.

### Changed

- Default images:
  * `quay.io/piraeusdatastore/piraeus-server:v1.17.0`
  * `quay.io/piraeusdatastore/piraeus-csi:v0.17.0`
  * `quay.io/piraeusdatastore/drbd9-bionic:v9.1.4`
  * `quay.io/piraeusdatastore/drbd-reactor:v0.4.4`
- Recreates or updates to the satellite pods are now applied at once, instead of waiting for a node to complete before
  moving to the next.
- Enable CSI topology by default, allowing better volume scheduling with `volumeBindingMode: WaitForFirstConsumer`.
- Disable STORK by default. Instead, we recommend using `volumeBindingMode: WaitForFirstConsumer` in storage classes.

## [v1.6.0] - 2021-09-02

### Added

- Allow CSI to work with distributions that use a kubelet working directory other than `/var/lib/kubelet`. See
  the [`csi.kubeletPath`](./doc/helm-values.adoc#csikubeletpath) option.
- Enable [Storage Capacity Tacking]. This enables Kubernetes to base Pod scheduling decisions on remaining storage
  capacity. The feature is in beta and enabled by default starting with Kubernetes 1.21.

[Storage Capacity Tacking]: https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/1472-storage-capacity-tracking

### Changed

- Disable Stork Health Monitoring by default. Stork cannot distinguish between control plane and data plane issues,
  which can lead to instances where Stork will migrate a volume that is still mounted on another node, making the
  volume effectively unusable.

- Updated operator to kubernetes v1.21 components.

- Default images:
  * `quay.io/piraeusdatastore/piraeus-server:v1.14.0`
  * `quay.io/piraeusdatastore/drbd9-bionic:v9.0.30`
  * `quay.io/piraeusdatastore/drbd-reactor:v0.4.3`
  * `quay.io/piraeusdatastore/piraeus-ha-controller:v0.2.0`
  * external CSI images

### Removed

- The cluster-wide snapshot controller is no longer deployed as a dependency of the piraeus-operator chart.
  Instead, separate charts are available on [artifacthub.io](https://artifacthub.io/packages/search?repo=piraeus-charts)
  that deploy the snapshot controller and extra validation for snapshot resources.

  The subchart was removed, as it unnecessarily tied updates of the snapshot controller to piraeus and vice versa. With
  the tightened validation starting with snapshot CRDs `v1`, moving the snapshot controller to a proper chart seems
  like a good solution.

## [v1.5.1] - 2021-06-21

### Changed

- Default images:
  * Piraeus Server v1.13.0
  * Piraeus CSI v0.13.1
  * CSI Provisioner v2.1.2

## [v1.5.0] - 2021-05-12

### Added

- All operator-managed workloads apply recommended labels. This requires the recreation of Deployments and DaemonSets
  on upgrade. This is automatically handled by the operator, however any customizations applied to the deployments
  not managed by the operator will be reverted in the process.
- Use [`drbd-reactor`](https://github.com/LINBIT/drbd-reactor/) to expose Prometheus endpoints on each satellite.
- Configure `ServiceMonitor` resources if they are supported by the cluster (i.e. prometheus operator is configured)

### Changed

- CSI Nodes no longer use `hostNetwork: true`. The pods already got the correct hostname via the downwardAPI and do not
  talk to DRBD's netlink interface directly.
- External: CSI snapshotter subchart now packages `v1` CRDs. Fixes deprecation warnings when installing
  the snapshot controller.
- Default images:
  * Piraeus Server v1.12.3
  * Piraeus CSI v0.13.0
  * DRBD v9.0.29

## [v1.4.0] - 2021-04-07

### Added

* Additional environment variables and Linstor properties can now be set in the `LinstorController` CRD.
* Set node name variable for Controller Pods, enabling [k8s-await-election] to correctly set up the endpoint for
  hairpin mode.

### Fixed

* Update the network address of controller pods if they diverged between Linstor and kubernetes. This can happen after
  a node restart, where a pod is recreated with the same name but different IP address.

[k8s-await-election]: https://github.com/LINBIT/k8s-await-election/commit/60748fcec722e4c32b8881c4c84957cc50543db2

## [v1.3.1] - 2021-01-14

### Added

* New guide on host preparation [here.](./doc/host-setup.md)

### Changed

* Default image updated:
  * `operator.satelliteSet.kernelModuleInjectionImage`: `quay.io/piraeusdatastore/drbd9-bionic:v9.0.27`
  * `operator.satelliteSet.satelliteImage`: `quay.io/piraeusdatastore/piraeus-server:v1.11.1`
  * `operator.controller.controllerImage`: `quay.io/piraeusdatastore/piraeus-server:v1.11.1`
  * `haController.image`: `quay.io/piraeusdatastore/piraeus-ha-controller:v0.1.3`
  * `pv-hostpath`: `chownerImage`: `quay.io/centos/centos:8`

## [v1.3.0] - 2020-12-21

### Added

* New component: `haController` will deploy the [Piraeus High Availability Controller].
  More information is available in the [optional components page](./doc/optional-components.md#high-availability-controller)
* Enable strict checking of DRBD parameter to disable usermode helper in container environments.
* Override the image used in "chown" jobs in the `pv-hostpath` chart by using `--set chownerImage=<my-image>`.

[Piraeus High Availability Controller]: https://github.com/piraeusdatastore/piraeus-ha-controller

### Changed

* Updated `operator-sdk` to v0.19.4
* Set CSI component timeout to 1 minute to reduce the number of retries in the CSI driver
* Default images updated:
  * `operator.controller.controllerImage`: `quay.io/piraeusdatastore/piraeus-server:v1.11.0`
  * `operator.satelliteSet.satelliteImage`: `quay.io/piraeusdatastore/piraeus-server:v1.11.0`
  * `operator.satelliteSet.kernelModuleInjectionImage`: `quay.io/piraeusdatastore/drbd9-bionic:v9.0.26`
  * `csi.pluginImage`: `quay.io/piraeusdatastore/piraeus-csi:v0.11.0`

### Fixed

* Fixed Helm warnings when setting "csi.controllerAffinity", "operator.controller.affinity" and
  "operator.satelliteSet.storagePools".

## [v1.2.0] - 2020-11-18

### Added

* `storagePools` can now also set up devices similar to `automaticStorageType`, but with more fine grained control.
  See the [updated storage guide](./doc/storage.md#preparing-physical-devices)
* New Helm options to disable creation of LinstorController and LinstorSatelliteSet resource
  `operator.controller.enabled` and `operator.satelliteSet.enabled`.
* New Helm option to override the generated controller endpoint: `controllerEndpoint`
* Allow overriding the default `securityContext` on a component basis:
  - `etcd.podsecuritycontext` sets the securityContext of etcd pods
  - `stork.podsecuritycontext` sets the securityContext of stork plugin and scheduler pods
  - `csi-snapshotter.podsecuritycontext` sets the securityContext of the CSI-Snapshotter pods
  - `operator.podsecuritycontext` sets the securityContext of the operator pods
* Example settings for openshift
* LINSTOR controller runs with additional GID 1000, to ensure write access to log directory

### Changed

* Fixed a bug in `pv-hostpath` where permissions on the created directory are not applied on all nodes.
* Volumes created by `pv-hostpath` are now group writable. This makes them easier to integrate with `fsGroup` settings.
* Default value for affinity on LINSTOR controller and CSI controller changed. The new default is to distribute the pods
  across all available nodes.
* Default value for tolerations for etcd pods changed. They are now able to run on master nodes.
* Updates to LinstorController, LinstorSatelliteSet and LinstorCSIDriver are now propagated across all created resources
* Updated default images:
  - csi sidecar containers updated (compatible with Kubernetes v1.17+)
  - LINSTOR 1.10.0
  - LINSTOR CSI 0.10.0

### Deprecation

* Using `automaticStorageType` is deprecated. Use the `storagePools` values instead.

## [v1.1.0] - 2020-10-13

### Breaking

* The LINSTOR controller image given in `operator.controller.controllerImage` has to have
  its entrypoint set to [`k8s-await-election v0.2.0`](https://github.com/LINBIT/k8s-await-election/)
  or newer. Learn more in the [upgrade guide](./UPGRADE.md#upgrade-from-v10-to-head).

### Added

* LINSTOR controller can be started with multiple replicas. See [`operator.controller.replicas`](./doc/helm-values.adoc#operatorcontrollerreplicas).
  NOTE: This requires support from the container. You need `piraeus-server:v1.8.0` or newer.
* The `pv-hostpath` helper chart automatically sets up permissions for non-root etcd containers.
* Disable securityContext enforcement by setting `global.setSecurityContext=false`.
* Add cluster roles to work with OpenShift's SCC system.
* Control volume placement and accessibility by using CSIs Topology feature. Controlled by setting
  [`csi.enableTopology`](./doc/helm-values.adoc#csienabletopology).
* All pods use a dedicated service account to allow for fine-grained permission control.
* The new [helm section `psp.*`](./doc/helm-values.adoc#psp) can automatically configure the ServiceAccount
  of all components to use the appropriate [PSP](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) roles.

### Changed

* Default values:
  * `operator.controller.controllerImage`: `quay.io/piraeusdatastore/piraeus-server:v1.9.0`
  * `operator.satelliteSet.satelliteImage`: `quay.io/piraeusdatastore/piraeus-server:v1.9.0`
  * `operator.satelliteSet.kernelModuleInjectionImage`: `quay.io/piraeusdatastore/drbd9-bionic:v9.0.25`
  * `stork.storkImage`: `docker.io/openstorage/stork:2.5.0`
* linstor-controller no longer starts in a privileged container.

### Removed

* legacy CRDs (LinstorControllerSet, LinstorNodeSet) have been removed.
* `v1alpha` CRD versions have been removed.
* default pull secret `drbdiocred` removed. To keep using it, use `--set drbdRepoCred=drbdiocred`.

## [v1.0.0] - 2020-08-06

### Added

* `v1` of all CRDs
* Central value for controller image pull policy of all pods. Use `--set global.imagePullPolicy=<value>` on
  helm deployment.
* `charts/piraeus/values.cn.yaml` a set of helm values for faster image download for CN users.
* Allow specifying [resource requirements] for all pods. In helm you can set:
  * `etcd.resources` for etcd containers
  * `stork.storkResources` for stork plugin resources
  * `stork.schedulerResources` for the kube-scheduler deployed for use with stork
  * `csi-snapshotter.resources` for the cluster snapshotter controller
  * `csi.resources` for all CSI related containers. for brevity, there is only one setting for ALL CSI containers. They
    are all stateless go process which use the same amount of resources.
  * `operator.resources` for operator containers
  * `operator.controller.resources` for LINSTOR controller containers
  * `operator.satelliteSet.resources` for LINSTOR satellite containers
  * `operator.satelliteSet.kernelModuleInjectionResources` for kernel module injector/builder containers
* Components deployed by the operator can now run with multiple replicas. Components
  elect a leader, that will take on the actual work as long as it is active. Should one
  pod go down, another replica will take over.
  Currently these components support multiple replicas:
  * `etcd` => set `etcd.replicas` to the desired count
  * `stork` => set `stork.replicas` to the desired count for stork scheduler and controller
  * `snapshot-controller` => set `csi-snapshotter.replicas` to the desired count for cluster-wide CSI snapshot controller
  * `csi-controller` => set `csi.controllerReplicas` to the desired count for the linstor CSI controller
  * `operator` => set `operator.replicas` to have multiple replicas of the operator running
* Reference docs for all helm settings. [Link](./doc/helm-values.adoc)
* `stork.schedulerTag` can override the automatically chosen tag for the `kube-scheduler` image.
  Previously, the tag always matched the kubernetes release.

[resource requirements]: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

### Changed

* Renamed `LinstorNodeSet` to `LinstorSatelliteSet`. This brings the operator in line with other LINSTOR resources.
  Existing `LinstorNodeSet` resources will automatically be migrated to `LinstorSatelliteSet`.
* Renamed `LinstorControllerSet` to `LinstorController`. The old name implied the existence of multiple (separate)
  controllers. Existing `LinstorControllerSet` resources will automatically be migrated to `LinstorController`.
* Helm values renamed to align with new CRD names:
  * `operator.controllerSet` to `operator.controller`
  * `operator.nodeSet` to `operator.satelliteSet`
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

[affinity]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity
[tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
* Controller is now a Deployment instead of StatefulSet.
* Renamed `kernelModImage` to `kernelModuleInjectionImage`
* Renamed `drbdKernelModuleInjectionMode` to `KernelModuleInjectionMode`

## [v0.5.0] - 2020-06-29

### Added
* Support volume resizing with newer CSI versions.
* A new Helm chart `csi-snapshotter` that deploys extra components needed for volume snapshots.
* Add new kmod injection mode `DepsOnly`. Will try load kmods for LINSTOR layers from the host. Deprecates `None`.

* Automatic deployment of [Stork](https://github.com/libopenstorage/stork) scheduler configured for LINSTOR.

### Removed

### Changed
* Replaced `bitnami/etcd` dependency with vendored custom version
  Some important keys for the `etcd` helm chart have changed:
  * `statefulset.replicaCount` -> `replicas`
  * `persistence.enabled` -> `persistentVolume.enabled`
  * `persistence.size` -> `persistentVolume.storage`
  * `àuth.rbac` was removed: use [tls certificates](./doc/security.md#authentication-with-etcd-using-certificates)
  * `auth.peer.useAutoTLS` was removed
  * `envVarsConfigMap` was removed
  * When using etcd with TLS enabled:
    * For peer communication, peers need valid certificates for `*.<release-name>-etcd` (was `.<release-name>>-etcd-headless.<namespace>.svc.cluster.local`)
    * For client communication, servers need valid certificates for `*.<release-name>-etcd`  (was `.<release-name>>-etcd.<namespace>.svc.cluster.local`)

## [v0.4.1] - 2020-06-10

### Added
* Automatic storage pool creation via `automaticStorageType` on `LinstorNodeSet`. If this option is set, LINSTOR
  will create a storage pool based on all available devices on a node.

### Changed
* Moved storage documentation to the [storage guide](./doc/storage.md)
* Helm: update default images

## [v0.4.0] - 2020-06-05

### Added

* Secured database connection for Linstor: When using the `etcd` connector, you can specify a secret containing a CA
  certificate to switch from HTTP to HTTPS communication.
* Secured connection between Linstor components: You can specify TLS keys to secure the communication between
  controller and satellite
* Secure storage with LUKS: You can specify the master passphrase used by Linstor when creating encrypted volumes
  when installing via Helm.
* Authentication with etcd using TLS client certificates.
* Secured connection between linstor-client and controller (HTTPS). More in the [security guide](./doc/security.md#configuring-secure-communications-for-the-linstor-api)
* Linstor controller endpoint can now be customized for all resources. If not specified, the old default values will be
  filled in.

### Removed

* NodeSet service (`piraeus-op-ns`) was replaced by the ControllerSet service (`piraeus-op-cs`) everywhere

### Changed
* CSI storage driver setup: move setup from helm to go operator. This is mostly an internal change.
  These changes may be of note if you used a non-default CSI configuration:
  * helm value `csi.image` was renamed to `csi.pluginImage`
  * CSI deployment can be controlled by a new resource `linstorcsidrivers.piraeus.linbit.com`
* PriorityClasses are not automatically created. When not specified, the priority class is:
  * "system-node-critical", if deployed in "kube-system" namespace
  * default PriorityClass in other namespaces
* RBAC rules for CSI: creation moved to deployment step (Helm/OLM). ServiceAccounts should be specified in CSI resource.
  If no ServiceAccounts are named, the implicitly created accounts from previous deployments will be used.
* Helm: update default images

## [v0.3.0] - 2020-05-08

### Changed

* Use single values for images in CRDs instead of specifying the version
  separately
* Helm: Use single values for images instead of specifying repo, name and
  version separately
* Helm: Replace fixed storage pool configuration with list
* Helm: Do not create any storage pools by default
* Helm: Replace `operator.nodeSet.spec` and `operator.controllerSet.spec` by
  just `operator.nodeSet` and `operator.controllerSet`.

## [v0.2.2] - 2020-04-24

### Changed

* Fix reporting of errors in LinstorControllerSet status

## [v0.2.1] - 2020-04-14

### Changed

* Helm: Update LINSTOR server dependencies to fix startup problems

## [v0.2.0] - 2020-04-10

### Added

* Helm: Allow an existing database to be used instead of always setting up
  a dedicated etcd instance

### Changed

* Rename `etcdURL` parameter of LinstorControllerSet to `dbConnectionURL` to
  reflect the fact that it can be used for any database type
* Upgrade to operator-sdk v0.16.0
* Helm: Create multiple volumes with a single `pv-hostchart` installation
* Helm: Update dependencies

## [v0.1.4] - 2020-03-05

### Added

* Helm: Add support for `hostPath` `PersistentVolume` persistence of etcd

### Removed

* Helm: Remove vendored etcd chart from repository

### Changed

* Rename CRDs from Piraeus* to Linstor*
* Make priority classes configurable
* Fix LINSTOR Controller/Satellite arguments
* Helm: Make etcd persistent by default
* Helm: Fix deployment of permissions objects into a non-default namespace
* Helm: Set default etcd size to 1Gi
* Helm: Update dependent image versions
* Docker: Change base image to Debian Buster

## [v0.1.3] - 2020-02-24

### Added

* Support for kernel module injection based on shipped modules - necessary for
  CoreOS support.

## [v0.1.2.1] - 2020-02-21

### Added

- /charts contains Helm v3 chart for this operator

### Changed

- CRDs contain additional Spec parameters that allow customizing image repo and
  tag/version of the image.
- Another Spec parameter 'drbdRepoCred' can specify the name of the k8s secret
  used to access the container images.
- LINSTOR Controller image now contains the LINSTOR client, away from the
  Satellite images as it was previously the case.  Hence, the readiness probe
  is changed to use `curl` instead of `linstor` client command.

## [v0.1.0] - 2020-01-27

### Added

- examples/operator-intra.yaml file to bundle all the rbac, crds, etc to run the
  operator
- EtcdURL field to controllersetcontroller spec. default: etcd-piraeus:2379
- Host networking on the LINSTOR Satellite pods with DNSClusterFirstWithHostNet
  DNS policy
- NodeSet service for the Satellite pods that also point to the Controller
  service for LINSTOR discovery

### Removed

- `ControllerEndpoint` and `DefaultController` from the PiraeusNodeSet spec

### Changed

- Controller persistence is now handled by etcd. There must be a reachable and
  operable etcd cluster for this operator to work.
- Networking is now handled by a kubernetes service with the same name
  as the ControllerSet. The NodeSet must have the same name as the ControllerSet
  for networking to work properly.
- Opt-in node label for nodes is now `linstor.linbit.com/piraeus-node=true`
- Remove requirement for `kube-system` namespace
- Naming convention for NodeSet and ControllerSet Pods
- Updated ports for LINSTOR access on NodeSet and ControllerSet pods
- Updated framework to work with Operator Framework 0.13.0
- API Versions on PriorityClass, DaemonSet, StatefulSet, and CRD kinds to reflect
  K8s 1.16.0 release

## [v0.0.1] - 2019-07-19

### Added

- Initial public version with docs

[v0.5.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.2...v0.3.0
[v0.2.2]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.1.4...v0.2.0
[v0.1.4]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.0.1...v0.1.4
[v0.0.1]: https://github.com/piraeusdatastore/piraeus-operator/tree/v0.0.1
[v1.0.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.5.0...v1.0.0
[v1.1.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.0.0...v1.1.0
[v1.2.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.1.0...v1.2.0
[v1.3.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.2.0...v1.3.0
[v1.3.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.3.0...v1.3.1
[v1.4.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.3.1...v1.4.0
[v1.5.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.4.0...v1.5.0
[v1.5.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.5.0...v1.5.1
[v1.6.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.5.1...v1.6.0
[v1.7.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.6.0...v1.7.0
[v1.7.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.7.0...v1.7.1
[v1.8.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.7.1...v1.8.0
[v1.8.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.8.0...v1.8.1
[v1.8.2]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.8.1...v1.8.2
[v1.9.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.8.2...v1.9.0
[v1.9.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.9.0...v1.9.1
[v1.10.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.9.1...v1.10.0
[v2.0.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v1.10.0...v2.0.0
[v2.0.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.0.0...v2.0.1
[v2.1.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.0.1...v2.1.0
[v2.1.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.1.0...v2.1.1
[v2.2.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.1.1...v2.2.0
[v2.3.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.2.0...v2.3.0
[v2.4.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.3.0...v2.4.0
[Unreleased]: https://github.com/piraeusdatastore/piraeus-operator/compare/v2.4.0...HEAD
