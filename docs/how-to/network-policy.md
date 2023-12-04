# How to Deploy a NetworkPolicy for Piraeus Datastore

[Network Policies] offer a way to restrict network access of specific pods. They can be used to block undesired
network connections to specific Pods.

Piraeus Datastore provides basic Network Policies that configure restricted access to the LINSTOR Satellites.
The provided policy restricts incoming network connections to LINSTOR Satellite to only the following sources:

* Other LINSTOR Satellite Pods to allow for DRBD replication
* The LINSTOR Controller Pod, to facilitate LINSTOR Cluster operations
* Metrics collection from DRBD Reactor.

To deploy the Network Policy, apply the following remote resource:

```
$ kubectl apply --server-side -k "https://github.com/piraeusdatastore/piraeus-operator//config/extras/network-policy?ref=v2"
networkpolicy.networking.k8s.io/satellite serverside-applied
```

[Network Policies]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
