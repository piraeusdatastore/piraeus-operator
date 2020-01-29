# Piraeus Operator

This is the initial public alpha for the Piraeus Operator. Currently, it is
suitable for testing and development.

## Contributing

This Operator is currently under heavy development: documentation and examples will quickly
go out of date.

If you'd like to contribute, please visit https://gitlab.com/linbit/piraeus-operator
and look through the issues to see if there something you'd like to work on. If
you'd like to contribute something not in an existing issue, please open a new
issue beforehand.

If you'd like to report an issue, please use the issues interface in this
project's gitlab page.

## Building and Development

This project is managed via the operator-skd (version 0.9.0). Please refer to
the [documentation for the sdk](https://github.com/operator-framework/operator-sdk/tree/v0.9.x)

## Deployment

### Operator

The operator must be deployed within the cluster in order for it it to have access
to the controller endpoint, which is a kubernetes service. See the operator-sdk
guide.

Worker nodes will only run on kubelets labeled with `linstor.linbit.com/piraeus-node=true`

### Etcd

An etcd cluster must be running and reachable to use this operator. By default,
the controller will try to connect to `etcd-piraeus` on port `2379`

A simple in-memory etcd cluster can be set up using helm:
```
kubectl create -f examples/etcd-env-vars.yaml
helm repo add bitnami https://charts.bitnami.com/bitnami
helm install bitnami/etcd --name=etcd-piraeus --set statefulset.replicaCount=3 -f examples/etcd-values.yaml
```

If you encounter difficulties with the above steps, you may need to set RBAC
rules for the tiller component of helm:
```
kubectl create serviceaccount --namespace kube-system tiller
kubectl create clusterrolebinding tiller-cluster-rule --clusterrole=cluster-admin --serviceaccount=kube-system:tiller
kubectl patch deploy --namespace kube-system tiller-deploy -p '{"spec":{"template":{"spec":{"serviceAccount":"tiller"}}}}'
```

## License

Apache 2.0
