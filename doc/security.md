# Security

This document describes the different options for enabling various security features available when
using this operator. The following guides assume the operator is installed [using Helm](../README.md#deployment-with-helm-v3-chart)

## Secure communication with an existing etcd instance

Secure communication to an `etcd` instance can be enabled by providing a CA certificate to the operator in form of a
kubernetes secret. The secret has to contain the key `ca.crt` with the PEM encoded CA certificate as value.

The secret can then be passed to the controller by passing the following argument to `helm install`
```
--set operator.controller.dbCertSecret=<secret name>
```

### Authentication with `etcd` using certificates

If you want to use TLS certificates to authenticate with an `etcd` database, you need to set the following option on
helm install:
```
--set operator.controller.dbUseClientCert=true
```

If this option is active, the secret specified in the above section must contain two additional keys:
* `tls.crt` certificate presented to `etcd` for authentication
* `tls.key` private key, matching the above client certificate.

## Configuring secure communication between LINSTOR components

The default communication between LINSTOR components is not secured by TLS. If this is needed for your setup,
choose one of three methods:

**Generate certificates using cert-manager**

Requires [cert-manager](https://cert-manager.io/docs/)) installed in your cluster

* Pass the following option to `helm install`.
```
--set linstorSslMethod=cert-manager
```
All required certificates will be issued automatically using cert-manager custom resources.

**Generate certificates using Helm**

* Pass the following option to `helm install`.
```
--set linstorSslMethod=helm
```
All required certificates will be generated automatically using Helm during initial installation.

**Generate and import certificates manually**

* Create private key and self-signed certificate for your certificate authority:

  ```
  openssl req -new -newkey rsa:2048 -days 5000 -nodes -x509 -keyout ca.key -out ca.crt -subj "/CN=piraeus-system"
  ```

* Create private keys, one for the controller, one for all nodes:
  ```
  openssl genrsa -out control.key 2048
  openssl genrsa -out node.key 2048
  ```
* Create trusted certificates for controller and nodes:
  ```
  openssl req -new -sha256 -key control.key -subj "/CN=system:control" -out control.csr
  openssl req -new -sha256 -key node.key -subj "/CN=system:node" -out node.csr
  openssl x509 -req -in control.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out control.crt -days 5000 -sha256
  openssl x509 -req -in node.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out node.crt -days 5000 -sha256
  ```
* Create kubernetes secrets that can be passed to the controller and node pods:
  ```
  kubectl create secret generic control-secret --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=control.crt --from-file=tls.key=control.key
  kubectl create secret generic node-secret --type=kubernetes.io/tls --from-file=ca.crt=ca.crt --from-file=tls.crt=node.crt --from-file=tls.key=node.key
  ```
* Pass the names of the created secrets to `helm install`:
  ```
  --set operator.satelliteSet.sslSecret=node-secret --set operator.controller.sslSecret=control-secret
  ```

## Configuring secure communications for the LINSTOR API

Various components need to talk to the LINSTOR controller via its REST interface. This interface can be
secured via HTTPS, which automatically includes authentication. For HTTPS+authentication to work, each component
needs access to:

* A private key
* A certificate based on the key
* A trusted certificate, used to verify that other components are trustworthy

Choose one of three methods:

**Generate certificates using cert-manager**

Requires [cert-manager](https://cert-manager.io/docs/)) installed in your cluster

* Pass the following option to `helm install`.
```
--set linstorHttpsMethod=cert-manager
```
All required certificates will be issued automatically using cert-manager custom resources.

**Generate certificates using Helm**

* Pass the following option to `helm install`.
```
--set linstorHttpsMethod=helm
```
All required certificates will be issued automatically using Helm during initial installation.

**Generate and import certificates manually**

The next sections will guide you through creating all required components.

* Create private key and self-signed certificate for your client certificate authority:

  ```
  openssl req -new -newkey rsa:2048 -days 5000 -nodes -x509 -keyout client-ca.key -out client-ca.crt -subj "/CN=piraeus-client-ca"
  ```

* Create private keys, one for the controller, one for clients:
  ```
  openssl genrsa -out controller.key 2048
  openssl genrsa -out client.key 2048
  ```
* Create trusted certificates for controller and clients:
  ```
  openssl req -new -sha256 -key controller.key -subj "/CN=piraeus-controller" -out controller.csr
  openssl req -new -sha256 -key client.key -subj "/CN=piraeus-client" -out client.csr
  openssl x509 -req -in controller.csr -CA client-ca.crt -CAkey client-ca.key -CAcreateserial -out controller.crt -days 5000 -sha256 -extensions 'v3_req' -extfile <(printf '%s\n' '[v3_req]' extendedKeyUsage=serverAuth subjectAltName=DNS:piraeus-op-cs.default.svc)
  openssl x509 -req -in client.csr -CA client-ca.crt -CAkey client-ca.key -CAcreateserial -out client.crt -days 5000 -sha256
  ```
  **NOTE**: The alias specified for the controller key (i.e. `DNS:piraeus-op-cs.default.svc`) has to exactly match the
  service name created by the operator. When using `helm`, this is always of the form `<release-name>-cs.<release-namespace>.svc`.

* Now you can create secrets for the controller and for clients:
  ```
  kubectl create secret generic http-controller --type=kubernetes.io/tls --from-file=ca.crt=client-ca.crt --from-file=tls.crt=controller.crt --from-file=tls.key=controller.key
  kubectl create secret generic http-client --type=kubernetes.io/tls --from-file=ca.crt=client-ca.crt --from-file=tls.crt=client.crt --from-file=tls.key=client.key
  ```
* The names of the secrets can be passed to `helm install` to configure all clients to use https.

  ```
  --set linstorHttpsControllerSecret=http-controller  --set linstorHttpsClientSecret=http-client
  ```

## Automatically set the passphrase for LINSTOR

LINSTOR may need to store sensitive information in its database, for example for encrypted volumes using the LUKS layer,
or when storing credentials for backup locations. To protect this information, LINSTOR will encrypt it using a master
passphrase before storing. When using Piraeus, this master passphrase is automatically created by helm and stored in a
Kubernetes secret.

If you want to manually set the passphrase, use:

```
kubectl create secret generic linstor-pass --from-literal=MASTER_PASSPHRASE=<password>
```

On install, add the following arguments to the helm command:

```
--set operator.controller.luksSecret=linstor-pass
```
