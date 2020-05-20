# Security

This document describes the different options for enabling various security features available when
using this operator. The following guides assume the operator is installed [using Helm](../README.md#deployment-with-helm-v3-chart)

## Secure communication with an existing etcd instance

Secure communication to an `etcd` instance can be enabled by providing a CA certificate to the operator in form of a
kubernetes secret. The secret has to contain the key `ca.pem` with the PEM encoded CA certificate as value.

The secret can then be passed to the controller by passing the following argument to `helm install`
```
--set operator.controllerSet.dbCertSecret=<secret name>
```

### Authentication with `etcd` using certificates

If you want to use TLS certificates to authenticate with an `etcd` database, you need to set the following option on
helm install:
```
--set operator.controllerSet.dbUseClientCert=true
```

If this option is active, the secret specified in the above section must contain two additional keys:
* `client.cert` PEM formatted certificate presented to `etcd` for authentication
* `client.key` private key **in PKCS8 format**, matching the above client certificate.
  Keys can be converted into PKCS8 format using `openssl`:
  ```
  openssl pkcs8 -topk8 -nocrypt -in client-key.pem -out client-key.pkcs8
  ```

## Configuring secure communication between LINSTOR components

The default communication between LINSTOR components is not secured by TLS. If this is needed for your setup,
follow these steps:

* Create private keys in the java keystore format, one for the controller, one for all nodes:
  ```
  keytool -keyalg rsa -keysize 2048 -genkey -keystore node-keys.jks -storepass linstor -alias node -dname "CN=XX, OU=node, O=Example, L=XX, ST=XX, C=X"
  keytool -keyalg rsa -keysize 2048 -genkey -keystore control-keys.jks -storepass linstor -alias control -dname "CN=XX, OU=control, O=Example, L=XX, ST=XX, C=XX"
  ```
* Create a trust store with the public keys that each component needs to trust:
  * Controller needs to trust the nodes
  * Nodes need to trust the controller
  ```
  keytool -importkeystore -srcstorepass linstor -deststorepass linstor -srckeystore control-keys.jks -destkeystore node-trust.jks
  keytool -importkeystore -srcstorepass linstor -deststorepass linstor -srckeystore node-keys.jks -destkeystore control-trust.jks
  ```
* Create kubernetes secrets that can be passed to the controller and node pods
  ```
  kubectl create secret generic control-secret --from-file=keystore.jks=control-keys.jks --from-file=certificates.jks=control-trust.jks
  kubectl create secret generic node-secret --from-file=keystore.jks=node-keys.jks --from-file=certificates.jks=node-trust.jks
  ```
* Pass the names of the created secrets to `helm install`
  ```
  --set operator.nodeSet.sslSecret=node-secret --set operator.controllerSet.sslSecret=control-secret
  ```

:warning: It is currently **NOT** possible to change the keystore password. LINSTOR expects the passwords to be
`linstor`. This is a current limitation of LINSTOR.

## Configuring secure communications for the LINSTOR API

Various components need to talk to the LINSTOR controller via its REST interface. This interface can be
secured via HTTPS, which automatically includes authentication. For HTTPS+authentication to work, each component
needs access to:

* A private key
* A certificate based on the key
* A trusted certificate, used to verify that other components are trustworthy

The next sections will guide you through creating all required components.

#### Creating the private keys

Private keys can be created using java's keytool

```
keytool -keyalg rsa -keysize 2048 -genkey -keystore controller.pkcs12 -storetype pkcs12 -storepass linstor -ext san=dns:piraeus-op-cs.default.svc -dname "CN=XX, OU=controller, O=Example, L=XX, ST=XX, C=X" -validity 5000
keytool -keyalg rsa -keysize 2048 -genkey -keystore client.pkcs12 -storetype pkcs12 -storepass linstor -dname "CN=XX, OU=client, O=Example, L=XX, ST=XX, C=XX" -validity 5000
```

The clients need private keys and certificate in a different format, so we need to convert it
```
openssl pkcs12 -in client.pkcs12 -passin pass:linstor -out client.cert -clcerts -nokeys
openssl pkcs12 -in client.pkcs12 -passin pass:linstor -out client.key -nocerts -nodes
```

**NOTE**: The alias specified for the controller key (i.e. `-ext san=dns:piraeus-op-cs.default.svc`) has to exactly match the
service name created by the operator. When using `helm`, this is always of the form `<release-name>-cs.<release-namespace>.svc`.

:warning: It is currently NOT possible to change the keystore password. LINSTOR expects the passwords to be linstor. This is a current limitation of LINSTOR

#### Create the trusted certificates

For the controller to trust the clients, we can use the following command to create a truststore, importing the client certificate

```
keytool -importkeystore -srcstorepass linstor -srckeystore client.pkcs12 -deststorepass linstor -deststoretype pkcs12 -destkeystore controller-trust.pkcs12
```

For the client, we have to convert the controller certificate into a different format

```
openssl pkcs12 -in controller.pkcs12 -passin pass:linstor -out ca.pem -clcerts -nokeys
```

#### Create Kubernetes secrets

Now you can create secrets for the controller and for clients:

```
kubectl create secret generic http-controller --from-file=keystore.jks=controller.pkcs12 --from-file=truststore.jks=controller-trust.pkcs12
kubectl create secret generic http-client --from-file=ca.pem=ca.pem --from-file=client.cert=client.cert --from-file=client.key=client.key
```

The names of the secrets can be passed to `helm install` to configure all clients to use https.

```
--set linstorHttpsControllerSecret=http-controller  --set linstorHttpsClientSecret=http-client
```

## Automatically set the passphrase for encrypted volumes

Linstor can be used to create encrypted volumes using LUKS. The passphrase used when creating these volumes can
be set via a secret:

```
kubectl create secret generic linstor-pass --from-literal=MASTER_PASSPHRASE=<password>
```

On install, add the following arguments to the helm command:

```
--set operator.controllerSet.luksSecret=linstor-pass
```
