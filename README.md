[![Release](https://img.shields.io/github/v/release/piraeusdatastore/piraeus-operator)](https://github.com/piraeusdatastore/piraeus-operator/releases)
![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.19%2B-success?logo=kubernetes&logoColor=lightgrey)
[![Build Status](https://github.com/piraeusdatastore/piraeus-operator/actions/workflows/build.yml/badge.svg)](https://quay.io/repository/piraeusdatastore/piraeus-operator?tag=latest&tab=tags)

# Piraeus Operator

The Piraeus Operator manages
[LINSTOR](https://github.com/LINBIT/linstor-server) clusters in Kubernetes.

All components of the LINSTOR software stack can be managed by the operator:
* DRBD
* LINSTOR
* LINSTOR CSI driver
* LINSTOR High-Availability Controller

## Legacy Operator

The previous version of Piraeus Operator is still available [here](https://github.com/piraeusdatastore/piraeus-operator/tree/master).

If you are **currently using Piraeus Operator v1**, please continue to use it. It will be maintained, receiving updates
to fix issues or new software versions until a stable upgrade path to v2 is available.

## Usage

To deploy Piraeus Operator v2 from this repository, simply run:

```
$ kubectl apply --server-side -k "https://github.com/piraeusdatastore/piraeus-operator//config/default?ref=v2"
# Verify the operator is running:
$ kubectl wait pod --for=condition=Ready -n piraeus-datastore -l app.kubernetes.io/component=piraeus-operator
pod/piraeus-operator-controller-manager-dd898f48c-bhbtv condition met
```

Now you can create a basic storage cluster by applying the `LinstorCluster` resource.

```
$ kubectl apply -f - <<EOF
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec: {}
EOF
```

## Documentation

### [Tutorials](./docs/tutorial)

Tutorials help you get started with Piraeus Operator.

### [How-To Guides](./docs/how-to)

How-To Guides show you how to configure a specific aspect or achieve a specific task with Piraeus Operator.

### [API Reference](./docs/reference)

The API Reference for the Piraeus Operator. Contains documentation of the LINSTOR related resources that the user can
modify or observe.

### [Understanding Piraeus Datastore](./docs/explanation)

These documents explain how Piraeus Datastore works, and why it works the way it does.

## Missing features

These are features that are currently present in Operator v1, and not yet available in this version of Operator v2:

* Backup of LINSTOR database before upgrades

## Upgrading

Please see the dedicated [UPGRADE document](./UPGRADE.md)

## Contributing

If you'd like to contribute, please visit https://github.com/piraeusdatastore/piraeus-operator
and look through the issues to see if there is something you'd like to work on. If
you'd like to contribute something not in an existing issue, please open a new
issue beforehand.

If you'd like to report an issue, please use the issues interface in this
project's github page.

## Building and Development

This project is built using the operator-sdk. Please refer to
the [documentation for the sdk](https://github.com/operator-framework/operator-sdk).

## License

Apache 2.0
