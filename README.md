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


## Usage

To deploy Piraeus Operator v2, first make sure to deploy [cert-manager](https://cert-manager.io)

```
$ kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.10.0/cert-manager.yaml
```

Then, deploy the Operator from this repository:

```
$ git clone https://github.com/piraeusdatastore/piraeus-operator --branch=v2
$ cd piraeus-operator
$ kubectl create -k config/default
# Verify the operator is running:
$ kubectl get pods -n piraeus-datastore
NAME                                                 READY   STATUS    RESTARTS   AGE
piraeus-operator-piraeus-operator-748c57bb8d-65cvw   2/2     Running   0          55s
```

Now you can create a basic storage cluster by applying the [sample resources](./config/samples)

```
$ kubectl create -k config/samples
linstorclusters.piraeus.io/linstorcluster created
linstorsatelliteconfigurations.piraeus.io/all-satellites created
```

## Configuration

We are currently working on improving our documentation. Additional tutorials, how-to guides and explanation will be
added soon.

### [API Reference](./docs/reference)

The API Reference for the Piraeus Operator. Contains documentation of the LINSTOR related resources that the user can
modify or observe.

## Missing features

These are features that are currently present in Operator v1, and not yet available in this version of Operator v2:

* Automatic TLS set up between LINSTOR components
* Backup of LINSTOR database before upgrades
* Ensuring the CSI driver is reporting correct node labels by restarting the node pods.


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
