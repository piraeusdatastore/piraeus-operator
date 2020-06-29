# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.5.0...HEAD
[v0.5.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.4.1...v0.5.0
[v0.4.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.4.0...v0.4.1
[v0.4.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.2...v0.3.0
[v0.2.2]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.1.4...v0.2.0
[v0.1.4]: https://github.com/piraeusdatastore/piraeus-operator/compare/v0.0.1...v0.1.4
[v0.0.1]: https://github.com/piraeusdatastore/piraeus-operator/tree/v0.0.1
