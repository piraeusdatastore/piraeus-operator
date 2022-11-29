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

The following customizations are available in the `LinstorCluster` resource:

* Set a custom registry base. All Piraeus images will use that registry instead of `quay.io/piraeusdatastore`.
  ```yaml
  apiVersion: piraeus.io/v1
  kind: LinstorCluster
  metadata:
    name: linstorcluster
  spec:
    repository: registry.example.com/piraeus
  ```
* A node selector. Satellites will only be started on nodes matching the given labels.
  ```yaml
  apiVersion: piraeus.io/v1
  kind: LinstorCluster
  metadata:
    name: linstorcluster
  spec:
    nodeSelector:
      piraeus.io/satellite: "true"
  ```
* A list of properties to apply on the controller level. For example, to configure LINSTOR to allocate Ports for DRBD
  starting at 8000 (instead of the default 7000), you can apply the following change to the LinstorCluster resource
  ```yaml
  apiVersion: piraeus.io/v1
  kind: LinstorCluster
  metadata:
    name: linstorcluster
  spec:
    properties:
      - name: TcpPortAutoRange
        value: "8000"
  ```
* [Kustomize patches](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/) to apply to
  resources. When the operator applies resources, it will use `kustomize` to adapt the [base resources](./pkg/resources).

  For example, to change the number of replicas for the CSI Controller deployment, you can use the following patch:
  ```yaml
  apiVersion: piraeus.io/v1
  kind: LinstorCluster
  metadata:
    name: linstorcluster
  spec:
    patches:
      - target:
          kind: Deployment
          name: csi-controller
        patch: |-
          apiVersion: apps/v1
          kind: csi-controller
          metadata:
            name: csi-controller
          spec:
            replicas: 3
  ```

The `LinstorSatelliteConfiguration` resources allow customizing a set of satellites at once. The `spec.nodeSelector` is
used to determine which customization is applied on a node.

For example, if you have a set of storage nodes, all labelled with `example.com/storage-node=""`, and you want to:
* Configure the default network interface to be the IPv6 address of your Pods.
* Configure a storage pool on all nodes, setting up on the devices `/dev/vdb` using a LVM thinpool `vg1/thin1` and naming
  it `thin1` in LINSTOR.
```
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: storage-satellites
spec:
  nodeSelector:
    example.com/storage-node: ""
  properties:
    - name: PrefNic
      value: default-ipv6
  storagePools:
    - name: thin1
      lvmThin:
        volumeGroup: vg1
        thinPool: thin1
      source:
        hostDevices:
          - /dev/vdb
```

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
