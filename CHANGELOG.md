# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Removed

### Changed

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
