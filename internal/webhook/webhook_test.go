package webhook_test

import (
	"context"
	"testing"

	linstorcsi "github.com/piraeusdatastore/linstor-csi/pkg/linstor"
	"github.com/stretchr/testify/assert"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/piraeusdatastore/piraeus-operator/v2/internal/webhook"
)

func TestStorageClass(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		storageclass storagev1.StorageClass
		expectError  bool
	}{
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "wrong-provisioner-no-error"},
				Provisioner: "unknown.example.com",
				Parameters: map[string]string{
					"foo": "bar",
				},
			},
		},
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "empty-parameters"},
				Provisioner: linstorcsi.DriverName,
				Parameters:  map[string]string{},
			},
		},
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "all-parameters-no-prefix"},
				Provisioner: linstorcsi.DriverName,
				Parameters: map[string]string{
					"AllowRemoteVolumeAccess": "false",
					"Autoplace":               "1",
					"ClientList":              "client-a client-b client c",
					"DisklessOnRemaining":     "true",
					"DisklessStoragePool":     "custom-diskless",
					"DoNotPlaceWithRegex":     "^bad-neighbor.*",
					"Encryption":              "true",
					"FSOpts":                  "-b 4096",
					"LayerList":               "drbd luks storage",
					"MountOpts":               "noatime",
					"NodeList":                "node-a node-b",
					"PlacementCount":          "1",
					"PlacementPolicy":         "Manual",
					"ReplicasOnDifferent":     "topology.kubernetes.io/zone",
					"ReplicasOnSame":          "topology.kubernetes.io/region",
					"SizeKiB":                 "1",
					"StoragePool":             "my-storage-pool",
					"PostMountXfsOpts":        "fsync",
					"ResourceGroup":           "my-group",
					"UsePVCName":              "true",
					"OverProvision":           "1.0",
				},
			},
		},
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "prefixed-unknown-parameter-is-fine"},
				Provisioner: linstorcsi.DriverName,
				Parameters: map[string]string{
					"example.com/foo": "bar",
				},
			},
		},
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "unknown-parameter-without-prefix-is-not-fine"},
				Provisioner: linstorcsi.DriverName,
				Parameters: map[string]string{
					"foo": "bar",
				},
			},
			expectError: true,
		},
		{
			storageclass: storagev1.StorageClass{
				ObjectMeta:  metav1.ObjectMeta{Name: "unparsable-parameter-is-not-fine"},
				Provisioner: linstorcsi.DriverName,
				Parameters: map[string]string{
					"LayerList": "drbd some-new-layer storage",
				},
			},
			expectError: true,
		},
	}
	var wsc webhook.StorageClass

	for i := range testcases {
		testcase := &testcases[i]
		t.Run(testcase.storageclass.Name, func(t *testing.T) {
			t.Parallel()
			_, err := wsc.ValidateCreate(context.Background(), &testcase.storageclass)
			if testcase.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
