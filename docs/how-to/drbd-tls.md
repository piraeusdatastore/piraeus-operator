# How to Use Transport Layer Security for DRBD Replication

This guide shows you how to use Transport Layer Security (TLS) for DRBD® replication.

TLS is a protocol designed to provide security, including privacy, integrity and authentication for network
communications. Starting with DRBD 9.2.6, replication traffic in DRBD can be encrypted using TLS, using the Linux
kernel [TLS Upper Layer Protocol](https://www.kernel.org/doc/html/latest/networking/tls.html#kernel-tls).

To complete this guide, you should be familiar with:

* editing `LinstorCluster` and `LinstorSatelliteConfiguration` resources.
* configuring [TLS certificates for internal traffic](./internal-tls.md).

## Prerequisites

* Install Piraeus Operator >= 2.3.0. The following command shows the deployed version, in this case v2.3.0:
  ```
  $ kubectl get pods -l app.kubernetes.io/component=piraeus-operator -ojsonpath='{.items[*].spec.containers[?(@.name=="manager")].image}{"\n"}'
  quay.io/piraeusdatastore/piraeus-operator:v2.3.0
  ```
* Use a host operating system with kernel TLS offload enabled. TLS offload was added in Linux 4.19. The following
  distributions are known to have TLS offload enabled:
  * RHEL >= 8.2
  * Ubuntu >= 22.04
  * Debian >= 12
* Have DRBD 9.2.6 or newer loaded. The following script shows the currently loaded DRBD version on all nodes:
  ```
  for SATELLITE in $(kubectl get pods -l app.kubernetes.io/component=linstor-satellite -ojsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'); do
    echo "$SATELLITE: $(kubectl exec $SATELLITE -- head -1 /proc/drbd)"
  done
  ```

## Configure TLS Between LINSTOR Controller and LINSTOR Satellite

This step is covered in [a dedicated guide](./internal-tls.md). Follow the steps in the guide, but set the
`tlsHandshakeDaemon` field in the `LinstorSatelliteConfiguration` resource:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: internal-tls
spec:
  internalTLS:
    tlsHandshakeDaemon: true
    # ...
```

This change will cause the Piraeus Operator to deploy an additional container named `ktls-utils` on the Satellite Pod.

```
$ kubectl logs -l app.kubernetes.io/component=linstor-satellite -c ktls-utils
tlshd[1]: Built from ktls-utils 0.10 on Oct  4 2023 07:26:06
tlshd[1]: Built from ktls-utils 0.10 on Oct  4 2023 07:26:06
tlshd[1]: Built from ktls-utils 0.10 on Oct  4 2023 07:26:06
```

## Configure TLS for DRBD

To instruct LINSTOR® to configure TLS for DRBD, the `DrbdOptions/Net/tls` property needs to be set to `yes`. This
can be done directly on the `LinstorCluster` resource, so it automatically applies to all resources, or as part of
the parameters in a StorageClass.

To apply the property cluster-wide, add the following property entry to your `LinstorCluster` resource:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  properties:
    - name: DrbdOptions/Net/tls
      value: "yes"
```

To apply the property only for certain StorageClasses, add the following parameter to the selected StorageClasses:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: example
provisioner: linstor.csi.linbit.com
parameters:
  property.linstor.csi.linbit.com/DrbdOptions/Net/tls: "yes"
  # ...
```

## Switch Existing Resources to TLS

DRBD does not support online reconfiguration to use DRBD. To switch existing resources to use TLS, for example by
setting the property on the `LinstorCluster` resource, you need to perform the following steps on all nodes:

First, you need to temporarily stop all replication and suspend all DRBD volumes using `drbdadm suspend-io all`.
The command is executed once on each LINSTOR Satellite Pod.

```
$ kubectl exec node1.example.com -- drbdadm suspend-io all
$ kubectl exec node2.example.com -- drbdadm suspend-io all
$ kubectl exec node3.example.com -- drbdadm suspend-io all
```

Next, you will need to disconnect all DRBD connections on all nodes.

```
$ kubectl exec node1.example.com -- drbdadm disconnect --force all
$ kubectl exec node2.example.com -- drbdadm disconnect --force all
$ kubectl exec node3.example.com -- drbdadm disconnect --force all
```

Now, we can safely reconnect DRBD connection paths, which configures the TLS connection parameters.

```
$ kubectl exec node1.example.com -- drbdadm adjust all
$ kubectl exec node2.example.com -- drbdadm adjust all
$ kubectl exec node3.example.com -- drbdadm adjust all
```

## Confirm DRBD Uses TLS

To confirm that DRBD is using TLS for replication, check the following resources.

First, confirm that the `ktls-utils` container performed the expected handshakes:

```
$ kubectl logs -l app.kubernetes.io/component=linstor-satellite -c ktls-utils
...
Handshake with node2.example.com (10.127.183.183) was successful
Handshake with node2.example.com (10.127.183.183) was successful
...
Handshake with node3.example.com (10.125.97.42) was successful
Handshake with node3.example.com (10.125.97.42) was successful
...
```

> [!NOTE]
> The following messages are expected when running `ktls-utils` in a container and can be safely ignored:
> ```
> File /etc/tlshd.d/tls.key: expected mode 600
> add_key: Bad message
> ```

Next, check the statistics on TLS sessions controlled by the kernel on each node. You should see an equal, nonzero
number of `TlsRxSw` and `TlsRxSw`.

```
$ kubectl exec node1.example.com -- cat /proc/net/tls_stat
TlsCurrTxSw                     	4
TlsCurrRxSw                     	4
TlsCurrTxDevice                 	0
TlsCurrRxDevice                 	0
TlsTxSw                         	4
TlsRxSw                         	4
TlsTxDevice                     	0
TlsRxDevice                     	0
TlsDecryptError                 	0
TlsRxDeviceResync               	0
TlsDecryptRetry                 	0
TlsRxNoPadViolation             	0
```

> [!NOTE]
> If your network card supports TLS offloading, you might see `TlsTxDevice` and `TlsRxDevice` being nonzero instead.
