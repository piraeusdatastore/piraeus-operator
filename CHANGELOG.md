# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- examples/operator-intra.yaml file to bundle all the rbac, crds, etc to run the
  operator
- EtcdURL field to controllersetcontroller spec. default: etcd-piraeus:2379

### Removed
- `ControllerEndpoint` from the PiraeusNodeSet spec

### Changed
- Controller persistence is now handled by etcd. There must be a reachable and
  operable etcd cluster for this operator to work.
- Networking is now handled by a kubernetes service with the same name
  as the ControllerSet. The NodeSet must have the same name as the ControllerSet
  for networking to work properly.
- Controller Nodes now do not use HostNetwork
- Opt-in node label for nodes is now `linstor.linbit.com/piraeus-node=true`
- Remove requirement for `kube-system` namespace

## [v0.0.1] - 2019-07-19
### Added
- initial public version with docs
