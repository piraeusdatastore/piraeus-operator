# `LinstorSatelliteConfiguration`

This resource allows customizing a set of satellites at once. The `spec.nodeSelector` is
used to determine which customization is applied on a node.

For example, if you have a set of storage nodes, all labelled with `example.com/storage-node=""`, and you want to:
* Configure the default network interface to be the IPv6 address of your Pods.
* Configure secured control plane traffic through TLS, using a [cert-manager](https://cert-manager.io/) issuer named
  `piraeus-root`.
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
  internalTLS:
    certManager:
      kind: Issuer
      name: piraeus-root
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
