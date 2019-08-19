# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Removed
- `ControllerEndpoint` from the PiraeusNodeSet spec

### Changed
- Networking is now handled by a kubernetes service with the same name
  as the ControllerSet. The NodeSet must have the same name as the ControllerSet
  for networking to work properly.
- Controller Nodes now do not use HostNetwork
- Opt-in node labels for controllers and nodes are now
`linstor.linbit.com/piraeus-controller=true` and
`linstor.linbit.com/piraeus-node=true` respectively

## [v0.0.1] - 2019-07-19
### Added
- initial public version with docs
