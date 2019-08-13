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

## Building, Deployment and Usage

This project is managed via the operator-skd (version 0.9.0). Please refer to
the [documentation for the sdk](https://github.com/operator-framework/operator-sdk/tree/v0.9.x)

## Usage

The operator must be deployed within the cluster in order for it it to have access
to the controller endpoint, which is a kubernetes service. See the operator-sdk
guide.

Controllers nodes will only run on kubelets labeled with `linstor.linbit.com/linstor-node-type=controller`
and nodes will only run on kubelets labeled with `linstor.linbit.com/linstor-node-type=storage`

## License

Apache 2.0
