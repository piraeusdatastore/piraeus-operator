package linstorcsidriver

const (
	DefaultAttacherImage            = "k8s.gcr.io/sig-storage/csi-attacher:v3.0.2"
	DefaultLivenessProbeImage       = "k8s.gcr.io/sig-storage/livenessprobe:v2.1.0"
	DefaultNodeDriverRegistrarImage = "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.0.1"
	DefaultProvisionerImage         = "k8s.gcr.io/sig-storage/csi-provisioner:v2.0.4"
	DefaultSnapshotterImage         = "k8s.gcr.io/sig-storage/csi-snapshotter:v3.0.2"
	DefaultResizerImage             = "k8s.gcr.io/sig-storage/csi-resizer:v1.0.1"
)
