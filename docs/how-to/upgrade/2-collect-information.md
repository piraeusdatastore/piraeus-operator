# Collect Information before Performing the Upgrade

Before performing the upgrade and removing the old deployment, collect information on the state of your cluster.
This ensures the upgraded deployment will have a compatible configuration.

This is the second step when migrating Piraeus Operator from version 1 (v1) to version 2 (v2).
[Click here to get back to the overview](./index.md).

## Prerequisites

* [Install `kubectl`](https://kubernetes.io/docs/tasks/tools/)
* [Install `helm`](https://docs.helm.sh/docs/intro/install/)
* [Install `jq`](https://jqlang.github.io/jq/download/)

## Run the Data Collection Script

To collect the necessary information for the migration, run the script provided at
`docs/how-to/upgrade/collect-operator-v1-information.sh`. First, clone this repository:

```
$ git clone -b v2 https://github.com/piraeusdatastore/piraeus-operator.git
$ cd piraeus-operator
```

The script will ask you if you want to keep modifications made with the deployed Piraeus Operator.
For each modification, the script will show you the proposed change to the deployed resources and ask for
confirmation.

In addition, it will also inform you about values that cannot automatically be migrated, along with recommended
actions should you need to keep the modification.

```
$ docs/how-to/upgrade/collect-operator-v1-information.sh
Using kubectl context: kubernetes-admin@kubernetes
Found LinstorControllers: ["piraeus-cs"]
Found LinstorSatelliteSets: ["piraeus-ns"]
Found LinstorCSIDrivers: ["piraeus"]
Found additional environment variables passed to LINSTOR Controller
--- Default resource
+++ Updated resource
@@ -143,6 +143,12 @@ kind: Deployment
       - args:
         - startController
         env:
+        - name: FOO
+          value: bar
+        - name: EXAMPLE
+          valueFrom:
+            fieldRef:
+              fieldPath: status.podIP
         - name: JAVA_OPTS
           value: -Djdk.tls.acknowledgeCloseNotify=true
         - name: K8S_AWAIT_ELECTION_ENABLED
Apply the patch (Y/n)? y
...
---------------------------------------------------------------------------------------------
| After upgrading, apply the following resources. They have been saved to v2-resources.yaml |
---------------------------------------------------------------------------------------------

apiVersion: piraeus.io/v1
kind: LinstorCluster
metadata:
  name: linstorcluster
spec:
  linstorPassphraseSecret: piraeus-passphrase
  patches: []
...
```

A copy of the identified resources is saved at `v2-resources.yaml`.

## Collect Helm-Generated Secrets

Helm automatically generates a passphrase for LINSTOR. Once LINSTOR is initialized with the passphrase, the LINSTOR
Controller will not be able fully start without supplying the passphrase.

To migrate the generated passphrase to Piraeus Operator v2, create a backup of the secret:

```
$ kubectl get secrets piraeus-op-passphrase -ogo-template='{{.data.MASTER_PASSPHRASE}}{{"\n"}}'
ZVRxanl1cVBscXV1ZTk1cmEwVTk5b0ptd0F4OGFmWlVPUVA0NWNnUw==
```

Save the generated passphrase for later use.
