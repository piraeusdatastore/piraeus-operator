# Influencing Kubernetes scheduling

This is meant as a short introduction to the various options that influence scheduling when using the Piraeus chart.
To influence pod placement, we use the Kubernetes concepts of [tolerations] and [affinity].

[tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
[affinity]: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity

## High Availability by distributing pods

If you are running multiple replicas to achieve High Availability, you want pods to be distributed across as many
nodes as possible. This ensures that failure of a single node does not interrupt a critical set of pods.

The way to achieve scheduling on different nodes is by using `podAntiAffinity`:

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchLabels:
            app: etcd
        topologyKey: "kubernetes.io/hostname"
```

This affinity setting ensures that only one pod with label `app: etcd` will be scheduled per node.

Piraeus will use such `affinity` settings by default for:
* `etcd` change by setting `operator.etcd`
* `operator` change by setting `operator.affinity`
* `Piraeus controller` change by setting `operator.controller.affinity`
* `CSI controller` change by setting `csi.controllerAffinity`

## Place pods on master nodes

To allow pods to be placed on master nodes, you need add [tolerations]:

```yaml
tolerations:
- key: node-role.kubernetes.io/master
  operator: "Exists"
  effect: "NoSchedule"
```

This toleration allows pods to be scheduled on master nodes.

Note that using tolerations this way only _allows_ scheduling on master nodes, but does not force it. The pod can still
end up on a worker node. To force scheduling on master nodes, use affinity settings to set the `nodeAffinity`.

By default, piraeus will set this toleration for the `etcd` pods. Tolerations can be set on:
* `etcd` by setting `etcd.tolerations`
* `operator` by setting `operator.tolerations`
* `Piraeus controller` by setting `operator.controller.tolerations`
* `Piraeus satellites` by setting `operator.satelliteSet.tolerations`
* `CSI controller` by setting `csi.controllerTolerations`
* `CSI nodes` by setting `csi.nodeTolerations`
