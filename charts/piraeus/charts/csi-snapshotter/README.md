# csi-snapshotter

Deploy the cluster-wide snapshot controller. This chart can be used if your Kubernetes distribution does not bundle
their own snapshot controller. Make sure if you even need this chart!

This chart basically automates the 2 steps described [here](https://github.com/kubernetes-csi/external-snapshotter#usage):

* It installs the Snapshot CRDs
* It installs the cluster wide snapshot controller

## Attribution

See [NOTICE.txt](./NOTICE.txt)
