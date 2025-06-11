# How Piraeus Operator Selects Image Versions for Deployed Resources

Piraeus Datastore consists of many different components, most of which follow their own release cycle. For this reason,
Piraeus Operator comes with a default configuration that uses the latest versions of all components at the time of
release.

## The Default Image Configuration

The default deployment of Piraeus Datastore creates a special ConfigMap resource named `piraeus-operator-image-config`.
It contains two entries that represent YAML documents:

* [`0_piraeus_datastore_images.yaml`](https://github.com/piraeusdatastore/piraeus-operator/tree/v2/config/manager/0_piraeus_datastore_images.yaml),
  which contains the image configuration for all components developed under the Piraeus Datastore umbrella.
* [`0_sig_storage_images.yaml`](https://github.com/piraeusdatastore/piraeus-operator/tree/v2/config/manager/0_sig_storage_images.yaml),
  which contains the image configuration for all components developed under the [Kubernetes CSI](https://kubernetes-csi.github.io/) umbrella.

The YAML documents follow the following scheme:

```yaml
base: quay.io/piraeusdatastore
```

The "base" repository. All images configured in this YAML document will start with `quay.io/piraeusdatastore/`.

```yaml
components:
  linstor-controller:
    tag: v1.31.0
    image: piraeus-server
```

The `components` are the image names used in the workload resources. `image` is the image repository to use in the given
"base" registry. `tag` is the image tag. There is also the option to set a `digest`, which takes precedence over tags,
but is not used in the default configuration.

These images are referenced in the Piraeus resources. For example, the LINSTOR Controller deployment has the
[following container configuration](https://github.com/piraeusdatastore/piraeus-operator/tree/v2/pkg/resources/cluster/controller/controller-deployment.yaml),
which will be replaced given the above configuration with `quay.io/piraeusdatastore/piraeus-server:v1.31.0`:

```yaml
# ...
      containers:
        - name: linstor-controller
          # will be replaced with:
          # image: quay.io/piraeusdatastore/piraeus-server:v1.31.0
          image: linstor-controller
# ...
```

For cases where it is necessary to use different images based on the node the workload is running on, there is the
option to set an additional `match` configuration:

```yaml
components:
  drbd-module-loader:
    tag: v9.2.13
    image: drbd9-noble
    match:
      - osImage: Red Hat Enterprise Linux Server 7\.
        image: drbd9-centos7
      - osImage: Red Hat Enterprise Linux 8\.
        image: drbd9-almalinux8
# ...
```

When a `match` section is present, the Operator will first try to match the Kubernetes Node "OS Image" (as reported by
[`status.nodeInfo.osImage`](https://kubernetes.io/docs/reference/kubernetes-api/cluster-resources/node-v1/#NodeStatus))
against the given `osImage`. If a match is found, the matches' `image` is used instead of the component one. The
`osImage` value is a [regular expression](https://pkg.go.dev/regexp). This is currently only used for the
`drbd-module-loader`.

## External Cluster Automations

Apart from the matching of "OS Image" for the drbd-module-loader, there is a second automation built into the Operator:
When using an [external LINSTOR cluster](../how-to/external-controller.md), the Operator will match the LINSTOR version
as reported by the external cluster with matching tags.

For example, assuming the external LINSTOR Cluster is running 1.31.2, as reported here:

```
$ linstor controller version
linstor controller 1.31.2; GIT-hash: 6a9a4bb37f547ff73317585a3efa217523e01f43
```

The operator will automatically replace the default tags of the `linstor-satellite` component with the tag "v1.31.2".

## Options for Overriding the Image Configuration

In certain cases, it might be necessary to update or override parts of this configuration. The following methods are
available to change the default image configuration.

### Overriding the Image Configuration via LinstorCluster

The following configuration will replace the `base:` setting in the image configuration with `registry.example.com/piraeus`.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  repository: registry.example.com/piraeus
```

This example results in the following changes:

| Component          | Default Image                                   | New Image                                           |
|--------------------|-------------------------------------------------|-----------------------------------------------------|
| linstor-controller | quay.io/piraeusdatastore/piraeus-server:v1.31.0 | registry.example.com/piraeus/piraeus-server:v1.31.0 |
| csi-attacher       | registry.k8s.io/sig-storage/csi-attacher:v4.8.1 | registry.example.com/piraeus/csi-attacher:v4.8.1    |

### Overriding the Image Configuration by Adding New YAML Documents

The Operator will read all YAML documents in the Image Configuration ConfigMap. The files are read in ascending order,
so providing a `1_override.yaml` document ensures the Operator will read this document after the default configuration.
Since the Operator applies a "last configuration wins" algorithm for components, this document overrides the default
configuration.

The following example replaces the `linstor-controller` with version 1.31.1 and pins the `linstor-satellite` image
to a specific digest:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: piraeus-operator-image-config
data:
  0_piraeus_datastore_images.yaml: |
    # ...
  0_sig_storage_images.yaml: |
    # ...
  1_override.yaml: |
    ---
    base: quay.io/piraeusdatastore
    components:
      linstor-controller:
        image: piraeus-server
        tag: v1.31.1
      linstor-satellite:
        image: piraeus-server
        digest: sha256:8bc7fd45545744864633371161e6fcd6bcaea1e7a21fb053588f970b9f912b07
```

This example results in the following changes:

| Component          | Default Image                                   | New Image                                                                                                       |
|--------------------|-------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| linstor-controller | quay.io/piraeusdatastore/piraeus-server:v1.31.0 | quay.io/piraeusdatastore/piraeus-server:v1.31.1                                                                 |
| linstor-satellite  | quay.io/piraeusdatastore/piraeus-server:v1.31.0 | quay.io/piraeusdatastore/piraeus-server@sha256:8bc7fd45545744864633371161e6fcd6bcaea1e7a21fb053588f970b9f912b07 |

### Overriding the Image Configuration by Adding Environment Variables to the Operator Deployment

To support the recommendation for "offline enabled" Operators, the Operator can also be configured using environment
variables. These take precedence over all other configuration options. To override a specific image set the
`RELATED_IMAGE_<component>` or `RELATED_IMAGE_<component>_<image>` environment variable. The second form is useful
for overriding images from `match` configurations.

The following example replaces the `linstor-controller` with version and 1.31.1 and replaces the AlmaLinux 9 loader
image with a different image:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: piraeus-operator-controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: RELATED_IMAGE_linstor-controller
          value: quay.io/piraeusdatastore/piraeus-server:v1.31.1
        - name: RELATED_IMAGE_drbd-module-loader_drbd9-almalinux9
          value: registry.example.com/piraeus/drbd@sha256:16e5adec119cd0849e019fb40dbb2b256a3579a3a7e4e34b6ec35252edfeb231
```

This example results in the following changes:

| Component          | Default Image                                     | New Image                                                                                                 |
|--------------------|---------------------------------------------------|-----------------------------------------------------------------------------------------------------------|
| linstor-controller | quay.io/piraeusdatastore/piraeus-server:v1.31.0   | quay.io/piraeusdatastore/piraeus-server:v1.31.1                                                           |
| drbd-module-loader | quay.io/piraeusdatastore/drbd9-almalinux9:v9.2.13 | registry.example.com/piraeus/drbd@sha256:16e5adec119cd0849e019fb40dbb2b256a3579a3a7e4e34b6ec35252edfeb231 |
