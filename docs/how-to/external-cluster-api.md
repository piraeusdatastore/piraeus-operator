# Integrate with a Cluster API Management Cluster

This guide shows you how to integrate Piraeus in a cluster managed by Cluster API, where the Cluster API is running
in a different cluster from Piraeus.

To complete this guide, you should be familiar with:

* Cluster API concepts such as [Management Clusters](https://cluster-api.sigs.k8s.io/user/concepts#management-cluster).
* editing the Piraeus Operator Deployment, using either `kustomize` or `helm`.
* managing Service Accounts and RBAC resources.

## Preconditions

This guide assumes that the Management Cluster and the Workload Cluster are distinct clusters. If they are the same
cluster, no additional configuration is necessary to integrate with Cluster API and you can skip this guide.

## Create a ServiceAccount on the Management Cluster

Piraeus Operator needs to be able to update annotations on the Machine resource to
[hook into the Machine deletion process](../explanation/deletion-policies.md#satellite-evacuation-when-using-clusterapi).

Create a Service Account and matching RBAC resources in the Management Cluster using the following kubectl commands.
The commands assume that the Cluster API resources are deployed in the `workload-cluster-ns` namespace, and
`management.kubeconfig` is the name of the kubeconfig file used to access the Management Cluster.

```
$ export KUBECONFIG=management.kubeconfig
$ export KUBENS=workload-cluster-ns
$ kubectl --namespace=$KUBENS create serviceaccount piraeus-operator
serviceaccount/piraeus-operator created
$ kubectl --namespace=$KUBENS role piraeus-operator --verb=get,update --resource=machines.cluster.x-k8s.io
role.rbac.authorization.k8s.io/piraeus-operator created
$ kubectl --namespace=$KUBENS create rolebinding piraeus-operator --role=piraeus-operator --serviceaccount=$KUBENS:piraeus-operator
rolebinding.rbac.authorization.k8s.io/piraeus-operator created
```

## Create a Kubeconfig File for the Service Account

The following commands create a `piraeus-clusterapi.kubeconfig` file.

```
$ export KUBECONFIG=management.kubeconfig
$ export KUBENS=workload-cluster-ns
$ TOKEN="$(kubectl --namespace=$KUBENS create token piraeus-operator)"
$ kubectl config view --flatten --minify -ojson | jq --arg TOKEN "$TOKEN" '.users[0].user = {"token": $TOKEN}' > piraeus-clusterapi.kubeconfig
```

This file contains the access credentials that Piraeus Operator can use to get and set the necessary annotations
on the Machine resources in the Management Cluster.

## Configure the Piraeus Operator to Use the Kubeconfig File

Now, update the Piraeus Operator Deployment to use the `piraeus-clusterapi.kubeconfig` created in the previous step.

=== "Kustomize"

    Update `kustomization.yaml` to create a secret containing the `piraeus-clusterapi.kubeconfig` and apply a patch to
    let the Piraeus Operator use the updated configuration. Apply using `kubectl apply -k .`.

    ```yaml
    resources:
    - https://github.com/piraeusdatastore/piraeus-operator/releases/latest/download/manifest.yaml
    secretGenerator:
    - name: piraeus-clusterapi-kubeconfig
      namespace: piraeus-datastore
      files:
      - kubeconfig=piraeus-clusterapi.kubeconfig
      options:
        disableSuffixHash: true
    patches:
    - patch: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: piraeus-operator-controller-manager
          namespace: piraeus-datastore
        spec:
          template:
            spec:
              containers:
              - name: manager
                env:
                - name: CLUSTER_API_KUBECONFIG
                  value: /etc/clusterapi/kubeconfig
                volumeMounts:
                - name: piraeus-clusterapi-kubeconfig
                  mountPath: /etc/clusterapi
                  readOnly: true
              volumes:
              - name: piraeus-clusterapi-kubeconfig
                secret:
                  secretName: piraeus-clusterapi-kubeconfig
    ```

=== "Helm"

    Create or update `values.yaml`, replacing the content of the secret in `extraManifests` with the content of
    `piraeus-clusterapi.kubeconfig`. Update the Piraeus Operator deployment using
    `helm upgrade --namespace piraeus-datastore piraeus-operator ... --values values.yaml`.

    ```yaml
    operator:
      options:
        clusterApiKubeconfig: /etc/clusterapi/kubeconfig
      extraVolumeMounts:
      - name: piraeus-clusterapi-kubeconfig
        mountPath: /etc/clusterapi
        readOnly: true

    extraVolumes:
    - name: piraeus-clusterapi-kubeconfig
      secret:
        secretName: piraeus-clusterapi-kubeconfig

    extraManifests:
    - apiVersion: v1
      kind: Secret
      metadata:
        name: piraeus-clusterapi-kubeconfig
      stringData:
       kubeconfig: |
         REPLACE THIS
    ```

## Confirm a Working Cluster API Integration

To confirm that the integration works, check for the presence of the following annotations on the Machine resources:

```
$ export KUBECONFIG=management.kubeconfig
$ export KUBENS=workload-cluster-ns
$ kubectl get machines --namespace=$KUBENS -ocustom-columns='NAME:metadata.name,HOOK:metadata.annotations.pre-drain\.delete\.hook\.machine\.cluster\.x-k8s\.io/linstor-prepare-for-drain'
NAME                           HOOK
machine-without-integration    <none>
machine-with-integration
```

If the `HOOK` field shows `<none>`, the integration is not active. If the output is empty, the integration is
enabled.
