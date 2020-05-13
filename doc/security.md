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
