# `LinstorNodeConnection`

This resource controls the state of the LINSTOR® node connections.

Node connections control the DRBD options set on a particular path, such as which protocol and network interface to use.

## `.spec`

Configures the desired state of the node connections.

### `.spec.selector`

Selects which connections the resource should apply to. By default, a `LinstorNodeConnection` resource applies to every
possible connection in the cluster.

A resource applies to a connection if one of the provided selectors match. A selector itself can contain multiple
expressions. If multiple expressions are specified in a selector, the connection must match all of them.

Every expression requires a label name (`key`) on which it operates and an operator `op`. Depending on the operator,
you can also specify specific values for the node label.

The following operators are available:

* `Exists`, the label specified in `key` is present on both nodes of the connection, with any value. This is the
  default.
* `DoesNotExist`, the label specified in `key` is not present on the nodes in the connections.
* `In`, the label specified in `key` matches any of the provided `values` for the nodes in the connection.
* `NotIn` the label specified in `key` does not match any of the provided `values` for the nodes in the connection.
* `Same` the label specified in `key` has the same value for the nodes in the connection.
* `NotSame` the label specified in `key` has different values for the nodes in the connection.

#### Example

This example restricts the resource to connections between nodes matching `example.com/storage: "yes"`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorNodeConnection
metadata:
  name: selector
spec:
  selector:
    - matchLabels:
        - key: example.com/storage
          op: In
          values:
            - yes
```

#### Example

This example restricts the resource to connections between nodes in the same region, but different zones.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorNodeConnection
metadata:
  name: selector
spec:
  selector:
    - matchLabels:
        - key: topology.kubernetes.io/region
          op: Same
        - key: topology.kubernetes.io/zone
          op: NotSame
```

### `.spec.paths`

Paths configure one or more network connections between nodes. If a path is configured, LINSTOR will use the given
network interface to configure DRBD replication. The network interfaces have to be registered with LINSTOR first, using
`linstor node interface create ...`.

#### Example

This example configures all nodes to use the "data-nic" network interface instead of the default interface.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorNodeConnection
metadata:
  name: network-paths
spec:
  paths:
    - name: path1
      interface: data-nic
```

### `.spec.properties`

Sets the given properties on the LINSTOR Node Connection level.

#### Example

This example sets the DRBD® protocol to `C`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorNodeConnection
metadata:
  name: drbd-options
spec:
  properties:
    - name: DrbdOptions/Net/protocol
      value: C
```

## `.status`

Reports the actual state of the connections.

### `.status.conditions`

The Operator reports the current state of the LINSTOR Node Connection through a set of conditions. Conditions are
identified by their `type`.

| `type`       | Explanation                                                            |
|--------------|------------------------------------------------------------------------|
| `Configured` | The LINSTOR Node Connection is applied to all matching pairs of nodes. |
