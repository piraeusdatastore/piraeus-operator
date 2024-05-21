# `LinstorCluster`

This resource controls the state of the LINSTOR® cluster and integration with Kubernetes.
In particular, it controls:

* LINSTOR Controller
* LINSTOR CSI Driver
* `LinstorSatellite`, configured through `LinstorSatelliteConfiguration` resources.

## `.spec`

Configures the desired state of the cluster.

### `.spec.nodeSelector`

Selects on which nodes Piraeus Datastore should be deployed. Nodes that are
excluded by the selector will not be able to run any workload using a Piraeus volume.

If empty (the default), Piraeus will be deployed on all nodes in the cluster.

When this is used together with `.spec.nodeAffinity`, both need to match in order for
a node to run Piraeus.

#### Example

This example restricts Piraeus Datastore to nodes matching `example.com/storage: "yes"`:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  nodeSelector:
    example.com/storage: "yes"
```

### `.spec.nodeAffinity`

Selects on which nodes Piraeus Datastore should be deployed. Nodes that are
excluded by the affinity will not be able to run any workload using a Piraeus volume.

If empty (the default), Piraeus will be deployed on all nodes in the cluster.

When this is used together with `.spec.nodeSelector`, both need to match in order for
a node to run Piraeus.

#### Example

This example restricts Piraeus Datastore to nodes in zones `a` and `b`, but not on `control-plane` nodes:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  nodeAffinity:
    nodeSelectorTerms:
      - matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values:
              - a
              - b
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
```


### `.spec.repository`

Sets the default image registry to use for all Piraeus images. The full image name is
created by appending an image identifier and tag.

If empty (the default), Piraeus will use `quay.io/piraeusdatastore`.

The current list of default images is available [here](../../config/manager/0_piraeus_datastore_images.yaml).

#### Example

This example pulls all Piraeus images from `registry.example.com/piraeus-mirror`
rather than `quay.io/piraeusdatastore`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  repository: registry.example.com/piraeus-mirror
```

### `.spec.properties`

Sets the given properties on the LINSTOR Controller level, applying them to the whole Cluster.

#### Example

This example sets the port range used for DRBD® volumes to `10000-20000`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  properties:
    - name: TcpPortAutoRange
      value: "10000-20000"
```

### `.spec.linstorPassphraseSecret`

Configures the [LINSTOR passphrase](https://linbit.com/drbd-user-guide/linstor-guide-1_0-en/#s-linstor-encrypted-volumes),
used by LINSTOR when creating encrypted volumes and storing access credentials
for backups.

The referenced secret must exist in the same namespace as the operator
(by default `piraeus-datastore`), and have a `MASTER_PASSPHRASE` entry.

#### Example

This example configures a passphrase `example-passphrase`. Please **choose a different passphrase** for your deployment.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-passphrase
  namespace: piraeus-datastore
data:
  # CHANGE THIS TO USE YOUR OWN PASSPHRASE!
  # Created by: echo -n "example-passphrase" | base64
  MASTER_PASSPHRASE: ZXhhbXBsZS1wYXNzcGhyYXNl
---
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  linstorPassphraseSecret: linstor-passphrase
```

### `.spec.patches`

The given patches will be applied to all resources controlled by the operator. The patches are
forwarded to `kustomize` internally, and take the [same format](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/).

The unpatched resources are available in the [subdirectories of the `pkg/resources` directory](../../pkg/resources).

#### Warning

No checks are run on the result of user-supplied patches: the resources are applied
as-is. Patching some fundamental aspect, such as removing a specific volume from a
container may lead to a degraded cluster.

#### Example

This example sets a CPU limit of `10m` on the CSI Node init container and changes the
LINSTOR Controller service to run in `SingleStack` mode.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  patches:
    - target:
        kind: Daemonset
        name: csi-node
      patch: |-
        - op: add
          path: /spec/template/spec/initContainers/0/resources
          value:
            limits:
               cpu: 10m
    - target:
        kind: Service
        name: linstor-controller
      patch: |-
        apiVersion: v1
        kind: service
        metadata:
          name: linstor-controller
        spec:
          ipFamilyPolicy: SingleStack
```

### `.spec.externalController`

Configures the Operator to use an external controller instead of deploying one in the Cluster.

#### Example

This examples instructs the Operator to use the external LINSTOR Controller reachable at `http://linstor.example.com:3370`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  externalController:
    url: http://linstor.example.com:3370
```

### `.spec.controller`

Controls the LINSTOR Controller Deployment:
* Setting `enabled: false` disables the controller deployment entirely. See also [`.spec.externalController`](#specexternalcontroller).
* Setting a `podTemplate:` allows for simple modification of the LINSTOR Controller Deployment.

#### Example

This example configures a resource request of `memory: 1Gi` for the LINSTOR Controller Deployment:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  controller:
    enabled: true
    podTemplate:
      spec:
        containers:
          - name: linstor-controller
            resources:
              requests:
                memory: 1Gi
```

### `.spec.csiController`

Controls the CSI Controller Deployment:
* Setting `enabled: false` disables the deployment entirely.
* Setting a `podTemplate:` allows for simple modification of the CSI Controller Deployment.

#### Example

This example configures a resource request of `cpu: 10m` for the CSI Controller Deployment:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  csiController:
    enabled: true
    podTemplate:
      spec:
        containers:
          - name: linstor-csi
            resources:
              requests:
                memory: 1Gi
```

### `.spec.csiNode`

Controls the CSI Node DaemonSet:
* Setting `enabled: false` disables the deployment entirely.
* Setting a `podTemplate:` allows for simple modification of the CSI Node DaemonSet.

#### Example

This example configures a resource request of `cpu: 10m` for the CSI Node DaemonSet:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  csiNode:
    enabled: true
    podTemplate:
      spec:
        containers:
          - name: linstor-csi
            resources:
              requests:
                memory: 1Gi
```

### `.spec.highAvailabilityController`

Controls the High Availability Controller DaemonSet:
* Setting `enabled: false` disables the deployment entirely.
* Setting a `podTemplate:` allows for simple modification of the CSI Node Deployment.

#### Example

This example configures a resource request of `cpu: 10m` for the CSI Node Deployment:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  highAvailabilityController:
    enabled: true
    podTemplate:
      spec:
        containers:
          - name: ha-controller
            resources:
              requests:
                memory: 1Gi
```

### `.spec.internalTLS`

Configures a TLS secret used by the LINSTOR Controller to:
* Validate the certificate of the LINSTOR Satellites, that is the Satellites must have certificates signed by `ca.crt`.
* Provide a client certificate for authentication with LINSTOR Satellites, that is `tls.key` and `tls.crt` must be accepted by the Satellites.

To configure TLS communication between Satellite and Controller,
[`LinstorSatelliteConfiguration.spec.internalTLS`](linstorsatelliteconfiguration.md#specinternaltls) must be set accordingly.

Setting a `secretName` is optional, it will default to `linstor-controller-internal-tls`.

Optional, a reference to a [cert-manager `Issuer`](https://cert-manager.io/docs/concepts/issuer/) can be provided
to let the operator create the required secret.

If the referenced secret does not contain a `ca.crt` certificate of a certificate authority, a `caReference` pointing
to a secondary `Secret` or `ConfigMap` resource can be configured.

#### Example

This example creates a manually provisioned TLS secret and references it in the
LinstorCluster configuration.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-linstor-controller-tls
  namespace: piraeus-datastore
data:
  ca.crt: LS0tLS1CRUdJT...
  tls.crt: LS0tLS1CRUdJT...
  tls.key: LS0tLS1CRUdJT...
---
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  internalTLS:
    secretName: my-linstor-controller-tls
```

#### Example

This example sets up automatic creation of the LINSTOR Controller TLS secret using a
cert-manager issuer named `piraeus-root`. The certificates will be validated against the certificate in the `trust-root`
ConfigMap.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  internalTLS:
    certManager:
      kind: Issuer
      name: piraeus-root
    caReference:
      name: trust-root
      kind: ConfigMap
      key: root.crt
```

### `.spec.apiTLS`

Configures the TLS secrets used to secure the LINSTOR API. There are four different secrets to configure:

* `apiSecretName`: sets the name of the secret used by the LINSTOR Controller to enable HTTPS. Defaults to
  `linstor-api-tls`. All clients of the API must have certificates signed by the `ca.crt` of this secret.
* `clientSecretName`: sets the name of the secret used by the Operator to connect to the LINSTOR API. Defaults to
  `linstor-client-tls`. Must be trusted by `ca.crt` in the API Secret. Also used by the LINSTOR Controller to configure
  the included LINSTOR CLI.
* `csiControllerSecretName` sets the name of the secret used by the CSI Controller. Defaults to
  `linstor-csi-controller-tls`. Must be trusted by `ca.crt` in the API Secret.
* `csiNodeSecretName` sets the name of the secret used by the CSI Controller. Defaults to `linstor-csi-node-tls`.
  Must be trusted by `ca.crt` in the API Secret.

Optional, a reference to a [cert-manager `Issuer`](https://cert-manager.io/docs/concepts/issuer/) can be provided
to let the operator create the required secrets.

If the referenced secret does not contain a `ca.crt` certificate of a certificate authority, a `caReference` pointing
to a secondary `Secret` or `ConfigMap` resource can be configured.

#### Example

This example creates a manually provisioned TLS secret and references it in the
LinstorCluster configuration. It uses the same secret for all clients of the LINSTOR API.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: my-linstor-api-tls
  namespace: piraeus-datastore
data:
  ca.crt: LS0tLS1CRUdJT...
  tls.crt: LS0tLS1CRUdJT...
  tls.key: LS0tLS1CRUdJT...
---
apiVersion: v1
kind: Secret
metadata:
  name: my-linstor-client-tls
  namespace: piraeus-datastore
data:
  ca.crt: LS0tLS1CRUdJT...
  tls.crt: LS0tLS1CRUdJT...
  tls.key: LS0tLS1CRUdJT...
---
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  apiTLS:
    apiSecretName: my-linstor-api-tls
    clientSecretName: my-linstor-client-tls
    csiControllerSecretName: my-linstor-client-tls
    csiNodeSecretName: my-linstor-client-tls
```

#### Example

This example sets up automatic creation of the LINSTOR API and LINSTOR Client TLS secret using a
cert-manager issuer named `piraeus-root`. The certificates will be validated against the certificate in the `trust-root`
Secret.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  apiTLS:
    certManager:
      kind: Issuer
      name: piraeus-root
    caReference:
      name: trust-root
      kind: Secret
      key: root.crt
```

## `.status`

Reports the actual state of the cluster.

## `.status.conditions`

The Operator reports the current state of the Cluster through a set of conditions. Conditions are identified by their
`type`.

| `type`       | Explanation                                                                      |
|--------------|----------------------------------------------------------------------------------|
| `Applied`    | All Kubernetes resources controlled by the Operator are applied and up to date.  |
| `Available`  | The LINSTOR Controller is deployed and reponding to requests.                    |
| `Configured` | The LINSTOR Controller is configured with the properties from `.spec.properties` |
