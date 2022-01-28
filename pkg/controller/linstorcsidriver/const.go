package linstorcsidriver

const (
	DefaultAttacherImage            = "k8s.gcr.io/sig-storage/csi-attacher:v3.4.0"
	DefaultLivenessProbeImage       = "k8s.gcr.io/sig-storage/livenessprobe:v2.5.0"
	DefaultNodeDriverRegistrarImage = "k8s.gcr.io/sig-storage/csi-node-driver-registrar:v2.4.0"
	DefaultProvisionerImage         = "k8s.gcr.io/sig-storage/csi-provisioner:v3.1.0"
	DefaultSnapshotterImage         = "k8s.gcr.io/sig-storage/csi-snapshotter:v5.0.1"
	DefaultResizerImage             = "k8s.gcr.io/sig-storage/csi-resizer:v1.4.0"
)
