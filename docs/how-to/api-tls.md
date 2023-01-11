# How to Configure TLS for the LINSTOR API

This guide shows you how to set up TLS for the LINSTOR® API. The API, served by the LINSTOR Controller, is used by
clients such as the CSI Driver and the Operator itself to control the LINSTOR Cluster.

To complete this guide, you should be familiar with:

* editing `LinstorCluster` resources.
* using either [cert-manager](https://cert-manager.io) or `openssl` to create TLS certificates.

## Provision Keys and Certificates Using cert-manager

This method requires a working [cert-manager](https://cert-manager.io) deployment in your cluster. For an alternative
way to provision keys and certificates, see the [`openssl` section](#provision-keys-and-certificates-using-openssl)
below.

When using TLS, the LINSTOR API uses client certificates for authentication. It is good practice to have a separate CA just for these certificates.

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
  name: linstor-api-ca
  namespace: piraeus-datastore
spec:
  commonName: linstor-api-ca
  secretName: linstor-api-ca
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
  name: linstor-api-ca
  namespace: piraeus-datastore
spec:
  ca:
    secretName: linstor-api-ca
```

Then, configure this new issuer to let the Operator provision the needed certificates:

```yaml
---
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  apiTLS:
    certManager:
      name: linstor-api-ca
      kind: Issuer
```

This completes the necessary steps for secure the LINSTOR API with TLS using cert-manager. Skip to the [last section](#verifying-tls-configuration) to verify TLS is working.

## Provision Keys and Certificates Using `openssl`

If you completed the [cert-manager section](#provision-keys-and-certificates-using-cert-manager) above, skip directly
to the [verifying the configuration](#verifying-tls-configuration) below.

This method requires the `openssl` program on the command line. For an alternative way to provision keys and
certificates, see the [cert-manager section](#provision-keys-and-certificates-using-cert-manager) above.

First, create a new Certificate Authority using a new key and a self-signed certificate, valid for 10 years:

```bash
openssl req -new -newkey rsa:4096 -days 3650 -nodes -x509 -keyout ca.key -out ca.crt -subj "/CN=linstor-api-ca"
```

Then, create two new keys, one for the LINSTOR API server, one for all LINSTOR API clients:

```bash
openssl genrsa -out api-server.key 4096
openssl genrsa -out api-client.key 4096
```

Next, we will create a certificate for the server. The clients might use different shortened service names, so
we need to specify multiple subject names:

```bash
cat /etc/ssl/openssl.cnf > api-csr.cnf
cat >> api-csr.cnf <<EOF
[ v3_req ]
subjectAltName = @alt_names
[ alt_names ]
DNS.0 = linstor-controller.piraeus-datastore.svc.cluster.local
DNS.1 = linstor-controller.piraeus-datastore.svc
DNS.2 = linstor-controller
EOF
openssl req -new -sha256 -key api-server.key -subj "/CN=linstor-controller" -config api-csr.cnf -extensions v3_req -out api-server.csr
openssl x509 -req -in api-server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -config api-csr.cnf -extensions v3_req -out api-server.crt -days 3650 -sha256
```

For the client certificate, simply setting one subject name is enough:

```bash
openssl req -new -sha256 -key api-client.key -subj "/CN=linstor-client" -out api-client.csr
openssl x509 -req -in api-client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out api-client.crt -days 3650 -sha256
```

Now, we create Kubernetes secrets from the created keys and certificates:

```bash
kubectl create secret generic linstor-api-tls -n piraeus-datastore --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=api-server.crt --from-file=tls.key=api-server.key
kubectl create secret generic linstor-client-tls -n piraeus-datastore --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=api-client.crt --from-file=tls.key=api-client.key
```

Finally, configure the Operator resources to reference the newly created secrets. We configure the same client secret for all components for simplicity.

```yaml
apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  apiTLS:
    apiSecretName: linstor-api-tls
    clientSecretName: linstor-client-tls
    csiControllerSecretName: linstor-client-tls
    csiNodeSecretName: linstor-client-tls
```

## Verifying TLS Configuration

You can verify that the secure API is running by manually connecting to the HTTPS endpoint using `curl`

```text
$ kubectl exec -n piraeus-datastore deploy/linstor-controller -- curl --key /etc/linstor/client/tls.key --cert /etc/linstor/client/tls.crt --cacert /etc/linstor/client/ca.crt https://linstor-controller.piraeus-datastore.svc:3371/v1/controller/version
{"version":"1.20.2","git_hash":"58a983a5c2f49eb8d22c89b277272e6c4299457a","build_time":"2022-12-14T14:21:28+00:00","rest_api_version":"1.16.0"}%
```

If the command is successful, the API is using HTTPS and clients are able to connect with their certificates.

If you see an error, make sure that the client certificates are trusted by the API secret, and vice versa.
The following script provides a quick way to verify that one TLS certificate is trusted by another:

```bash
function k8s_secret_trusted_by() {
	kubectl get secret -n piraeus-datastore -ogo-template='{{ index .data "tls.crt" | base64decode }}' "$1" > $1.tls.crt
	kubectl get secret -n piraeus-datastore -ogo-template='{{ index .data "ca.crt" | base64decode }}' "$2" > $2.ca.crt
	openssl verify -CAfile $2.ca.crt $1.tls.crt
}
k8s_secret_trusted_by linstor-client-tls linstor-api-tls
# Expected output:
# linstor-client-tls.tls.crt: OK
```

Another issue might be the API endpoint using a Certificate not using the expected service name.
A typical error message for this issue would be:

```text
curl: (60) SSL: no alternative certificate subject name matches target host name 'linstor-controller.piraeus-datastore.svc'
```

In this case, make sure you have specified the right subject names when provisioning the certificates.

All available options are documented in the reference for
[`LinstorCluster`](../reference/linstorcluster.md#specapitls).
