# How to Configure TLS Between LINSTOR Controller and LINSTOR Satellite

This guide shows you how to set up TLS between LINSTOR® Controller and LINSTOR Satellite.

To complete this guide, you should be familiar with:

* editing `LinstorCluster` and `LinstorSatelliteConfiguration` resources.
* using the LINSTOR client to verify success.
* using either [cert-manager](https://cert-manager.io) or `openssl` to create TLS certificates.

## Provision Keys and Certificates Using cert-manager

This method requires a working [cert-manager](https://cert-manager.io) deployment in your cluster. For an alternative
way to provision keys and certificates, see the [`openssl` section](#provision-keys-and-certificates-using-openssl)
below.

LINSTOR Controller and Satellite only need to trust each other, so it is good practice to have a CA only for those
components. Apply the following YAML to create a new Issuer:

```yaml
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ca-bootstrapper
  namespace: piraeus-datastore
spec:
  selfSigned: { }
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: linstor-internal-ca
  namespace: piraeus-datastore
spec:
  commonName: linstor-internal-ca
  secretName: linstor-internal-ca
  duration: 87600h # 10 years
  isCA: true
  usages:
    - signing
    - key encipherment
    - cert sign
  issuerRef:
    name: ca-bootstrapper
    kind: Issuer
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: linstor-internal-ca
  namespace: piraeus-datastore
spec:
  ca:
    secretName: linstor-internal-ca
```

Then, configure this new issuer to let the Operator provision the needed certificates:

```yaml
---
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  internalTLS:
    certManager:
      name: linstor-internal-ca
      kind: Issuer
---
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: internal-tls
spec:
  internalTLS:
    certManager:
      name: linstor-internal-ca
      kind: Issuer
```

This completes the necessary steps for secure TLS connections between LINSTOR Controller and Satellite using
cert-manager. Skip to the [last section](#verifying-tls-configuration) to verify TLS is working.

## Provision Keys and Certificates Using `openssl`

If you completed the [cert-manager section](#provision-keys-and-certificates-using-cert-manager) above, skip directly
to the [verifying the configuration](#verifying-tls-configuration) below.

This method requires the `openssl` program on the command line. For an alternative way to provision keys and
certificates, see the [cert-manager section](#provision-keys-and-certificates-using-cert-manager) above.

First, create a new Certificate Authority using a new key and a self-signed certificate, valid for 10 years:

```bash
openssl req -new -newkey rsa:4096 -days 3650 -nodes -x509 -keyout ca.key -out ca.crt -subj "/CN=linstor-internal-ca"
```

Then, create two new keys, one for the LINSTOR Controller, one for all Satellites:

```bash
openssl genrsa -out controller.key 4096
openssl genrsa -out satellite.key 4096
```

Next, we will create a certificate for each key, valid for 10 years, signed by the Certificate Authority:

```bash
openssl req -new -sha256 -key controller.key -subj "/CN=linstor-controller" -out controller.csr
openssl req -new -sha256 -key satellite.key -subj "/CN=linstor-satellite" -out satellite.csr
openssl x509 -req -in controller.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out controller.crt -days 3650 -sha256
openssl x509 -req -in satellite.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out satellite.crt -days 3650 -sha256
```

Now, we create Kubernetes secrets from the created keys and certificates:

```bash
kubectl create secret generic linstor-controller-internal-tls -n piraeus-datastore --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=controller.crt --from-file=tls.key=controller.key
kubectl create secret generic linstor-satellite-internal-tls -n piraeus-datastore --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=satellite.crt --from-file=tls.key=satellite.key
```

Finally, configure the Operator resources to reference the newly created secrets:

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  internalTLS:
    secretName: linstor-controller-internal-tls
---
apiVersion: piraeus.io/v1
kind: LinstorSatelliteConfiguration
metadata:
  name: internal-tls
spec:
  internalTLS:
    secretName: linstor-satellite-internal-tls
```

## Verifying TLS Configuration

You can verify the secure TLS connection between LINSTOR Controller and Satellite by checking the output of the
`linstor node list` command. If TLS is enabled, you will see `(SSL)` next to the active satellite address:

```text
$ kubectl exec -n piraeus-datastore deploy/linstor-controller -- linstor node list
+---------------------------------------------------------------------+
| Node               | NodeType  | Addresses                 | State  |
|=====================================================================|
| node01.example.com | SATELLITE | 10.116.72.142:3367 (SSL)  | Online |
| node02.example.com | SATELLITE | 10.127.183.140:3367 (SSL) | Online |
| node03.example.com | SATELLITE | 10.125.97.50:3367 (SSL)   | Online |
+---------------------------------------------------------------------+
```

If you see `(PLAIN)` instead, this indicates that the TLS configuration was not applied successfully. Check the
status of the `LinstorCluster` and `LinstorSatellite` resource.

If you see `(SSL)`, but the node remains offline, this usually indicates that a certificate is not trusted by the other
party. Verify that the Controller's `tls.crt` is trusted by the Satellite's `ca.crt` and vice versa. The following
script provides a quick way to verify that one TLS certificate is trusted by another:

```bash
function k8s_secret_trusted_by() {
	kubectl get secret -n piraeus-datastore -ogo-template='{{ index .data "tls.crt" | base64decode }}' "$1" > $1.tls.crt
	kubectl get secret -n piraeus-datastore -ogo-template='{{ index .data "ca.crt" | base64decode }}' "$2" > $2.ca.crt
	openssl verify -CAfile $2.ca.crt $1.tls.crt
}
k8s_secret_trusted_by satellite-tls controller-tls
# Expected output:
# satellite-tls.tls.crt: OK
```

All available options are documented in the reference for
[`LinstorCluster`](../reference/linstorcluster.md#specinternaltls) and
[`LinstorSatelliteConfiguration`](../reference/linstorsatelliteconfiguration.md#specinternaltls).
