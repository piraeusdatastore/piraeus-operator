package controller

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"

	lapi "github.com/LINBIT/golinstor/client"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilexec "k8s.io/client-go/util/exec"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/fakelinstor"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/linstorhelper"
)

type fakeExecutor struct {
	fn    func(command []string) (string, string, error)
	calls int
}

func (e *fakeExecutor) Exec(_ context.Context, _, _, _ string, command []string) (string, string, error) {
	e.calls++
	return e.fn(command)
}

func constExecutor(stdout string, err error) func([]string) (string, string, error) {
	return func([]string) (string, string, error) {
		return stdout, "", err
	}
}

// backendAndDeviceExecutor answers the backend probe with backendOut/backendErr and the source-device probe like
// "stat" would: it echoes the requested devices that are in presentDevices and, if any are absent, also returns a
// non-zero exit error.
func backendAndDeviceExecutor(backendOut string, backendErr error, presentDevices []string) func([]string) (string, string, error) {
	return func(command []string) (string, string, error) {
		// The source-device probe is executed as `stat -L -c %n <dev>...`.
		if len(command) >= 4 && command[0] == "stat" {
			var present []string
			missing := false
			for _, dev := range command[4:] {
				if slices.Contains(presentDevices, dev) {
					present = append(present, dev)
				} else {
					missing = true
				}
			}

			var err error
			if missing {
				err = utilexec.CodeExitError{Err: errors.New("exit status 1"), Code: 1}
			}

			return strings.Join(present, "\n"), "", err
		}
		return backendOut, "", backendErr
	}
}

func TestStoragePoolBackendExists(t *testing.T) {
	t.Parallel()

	lvm := &piraeusiov1.LinstorStoragePool{Name: "pool1", LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1"}}
	thin := &piraeusiov1.LinstorStoragePool{Name: "pool1", LvmThinPool: &piraeusiov1.LinstorStoragePoolLvmThin{VolumeGroup: "vg1", ThinPool: "thin1"}}
	zfs := &piraeusiov1.LinstorStoragePool{Name: "pool1", ZfsPool: &piraeusiov1.LinstorStoragePoolZfs{ZPool: "tank"}}
	file := &piraeusiov1.LinstorStoragePool{Name: "pool1", FilePool: &piraeusiov1.LinstorStoragePoolFile{}}

	boom := errors.New("boom")

	tests := []struct {
		name       string
		pool       *piraeusiov1.LinstorStoragePool
		stdout     string
		execErr    error
		wantExists bool
		wantErr    bool
		wantCalls  int
	}{
		{name: "file pool not probed", pool: file, wantExists: true, wantCalls: 0},
		{name: "lvm exists", pool: lvm, stdout: "  vg0\n  vg1\n", wantExists: true, wantCalls: 1},
		{name: "lvm missing", pool: lvm, stdout: "  vg0\n", wantExists: false, wantCalls: 1},
		{name: "lvm probe error", pool: lvm, execErr: boom, wantExists: false, wantErr: true, wantCalls: 1},
		{name: "lvm-thin exists", pool: thin, stdout: "  vg1/thin1\n  vg1/other\n", wantExists: true, wantCalls: 1},
		{name: "lvm-thin missing", pool: thin, stdout: "  vg1/other\n", wantExists: false, wantCalls: 1},
		{name: "zfs exists", pool: zfs, stdout: "tank\ntank/sub\n", wantExists: true, wantCalls: 1},
		{name: "zfs missing", pool: zfs, stdout: "other\nother/sub\n", wantExists: false, wantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := &fakeExecutor{fn: constExecutor(tt.stdout, tt.execErr)}
			exists, err := storagePoolBackendExists(context.Background(), exec, "ns", "pod", tt.pool)

			if exists != tt.wantExists {
				t.Errorf("exists = %v, want %v", exists, tt.wantExists)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if exec.calls != tt.wantCalls {
				t.Errorf("exec calls = %d, want %d", exec.calls, tt.wantCalls)
			}
		})
	}
}

func TestSourceDeviceProbeCommand(t *testing.T) {
	t.Parallel()

	got := sourceDeviceProbeCommand([]string{"/dev/sdb", "/dev/sdc"})
	if !slices.Equal(got, []string{"stat", "-L", "-c", "%n", "/dev/sdb", "/dev/sdc"}) {
		t.Errorf("unexpected command: %v", got)
	}
}

func TestMissingSourceDevices(t *testing.T) {
	t.Parallel()

	// stat exits non-zero when some operands are absent; that is not an inconclusive probe.
	statMissing := utilexec.CodeExitError{Err: errors.New("exit status 1"), Code: 1}
	// A non-exit error means the probe could not run at all.
	transportErr := errors.New("boom")

	tests := []struct {
		name        string
		devices     []string
		stdout      string
		execErr     error
		wantMissing []string
		wantErr     bool
		wantCalls   int
	}{
		{name: "none requested", devices: nil, wantCalls: 0},
		{name: "all present", devices: []string{"/dev/sdb", "/dev/sdc"}, stdout: "/dev/sdb\n/dev/sdc\n", wantCalls: 1},
		{name: "one missing", devices: []string{"/dev/sdb", "/dev/sdc"}, stdout: "/dev/sdb\n", execErr: statMissing, wantMissing: []string{"/dev/sdc"}, wantCalls: 1},
		{name: "all missing", devices: []string{"/dev/sdb"}, execErr: statMissing, wantMissing: []string{"/dev/sdb"}, wantCalls: 1},
		{name: "probe error", devices: []string{"/dev/sdb"}, execErr: transportErr, wantErr: true, wantCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec := &fakeExecutor{fn: constExecutor(tt.stdout, tt.execErr)}
			missing, err := missingSourceDevices(context.Background(), exec, "ns", "pod", tt.devices)

			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(missing, tt.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tt.wantMissing)
			}
			if exec.calls != tt.wantCalls {
				t.Errorf("exec calls = %d, want %d", exec.calls, tt.wantCalls)
			}
		})
	}
}

func TestReconcileStoragePools(t *testing.T) {
	t.Parallel()

	lvmWithSource := piraeusiov1.LinstorStoragePool{
		Name:    "pool1",
		LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1"},
		Source:  &piraeusiov1.LinstorStoragePoolSource{HostDevices: []string{"/dev/sdb"}},
	}
	lvmNoSource := piraeusiov1.LinstorStoragePool{
		Name:    "pool1",
		LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1"},
	}

	tests := []struct {
		name                string
		pool                piraeusiov1.LinstorStoragePool
		exec                func([]string) (string, string, error)
		wantErr             bool
		wantRegistered      bool
		wantDevicePoolCalls int
	}{
		{
			name:                "backend exists with source registers directly",
			pool:                lvmWithSource,
			exec:                constExecutor("  vg1\n", nil),
			wantRegistered:      true,
			wantDevicePoolCalls: 0,
		},
		{
			name:                "backend missing with source creates device pool",
			pool:                lvmWithSource,
			exec:                backendAndDeviceExecutor("  vg0\n", nil, []string{"/dev/sdb"}),
			wantRegistered:      true,
			wantDevicePoolCalls: 1,
		},
		{
			name:                "backend missing with missing source device is not created",
			pool:                lvmWithSource,
			exec:                backendAndDeviceExecutor("  vg0\n", nil, nil),
			wantErr:             true,
			wantRegistered:      false,
			wantDevicePoolCalls: 0,
		},
		{
			name: "source device probe failure is reported without creating device pool",
			pool: lvmWithSource,
			exec: func(command []string) (string, string, error) {
				if len(command) >= 4 && command[0] == "stat" {
					return "", "", errors.New("device probe failed")
				}
				return "  vg0\n", "", nil
			},
			wantErr:             true,
			wantRegistered:      false,
			wantDevicePoolCalls: 0,
		},
		{
			name:                "backend missing without source is not registered",
			pool:                lvmNoSource,
			exec:                constExecutor("  vg0\n", nil),
			wantErr:             true,
			wantRegistered:      false,
			wantDevicePoolCalls: 0,
		},
		{
			name:                "probe failure is reported without registering",
			pool:                lvmNoSource,
			exec:                constExecutor("", errors.New("exec failed")),
			wantErr:             true,
			wantRegistered:      false,
			wantDevicePoolCalls: 0,
		},
		{
			name:                "probe failure with source is reported without creating device pool",
			pool:                lvmWithSource,
			exec:                constExecutor("", errors.New("exec failed")),
			wantErr:             true,
			wantRegistered:      false,
			wantDevicePoolCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := fakelinstor.New()
			defer fake.Server.Close()

			u, err := url.Parse(fake.Server.URL)
			if err != nil {
				t.Fatalf("failed to parse fake server URL: %v", err)
			}

			lapiClient, err := lapi.NewClient(lapi.BaseURL(u))
			if err != nil {
				t.Fatalf("failed to create LINSTOR client: %v", err)
			}

			r := &LinstorSatelliteReconciler{
				Namespace:   "ns",
				log:         logr.Discard(),
				PodExecutor: &fakeExecutor{fn: tt.exec},
			}

			node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}}
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns"}}
			lsatellite := &piraeusiov1.LinstorSatellite{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Spec:       piraeusiov1.LinstorSatelliteSpec{StoragePools: []piraeusiov1.LinstorStoragePool{tt.pool}},
			}

			err = r.reconcileStoragePools(context.Background(), &linstorhelper.Client{Client: lapiClient}, lsatellite, node, pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("reconcileStoragePools() err = %v, wantErr %v", err, tt.wantErr)
			}

			registered := slices.ContainsFunc(fake.StoragePools(), func(sp lapi.StoragePool) bool {
				return sp.StoragePoolName == tt.pool.Name && sp.NodeName == lsatellite.Name
			})
			if registered != tt.wantRegistered {
				t.Errorf("storage pool registered = %v, want %v", registered, tt.wantRegistered)
			}

			if got := fake.DevicePoolCreations(); got != tt.wantDevicePoolCalls {
				t.Errorf("device pool creations = %d, want %d", got, tt.wantDevicePoolCalls)
			}
		})
	}
}

func TestReconcileStoragePoolsSharedSpace(t *testing.T) {
	t.Parallel()

	fake := fakelinstor.New()
	defer fake.Server.Close()

	u, err := url.Parse(fake.Server.URL)
	if err != nil {
		t.Fatalf("failed to parse fake server URL: %v", err)
	}

	lapiClient, err := lapi.NewClient(lapi.BaseURL(u))
	if err != nil {
		t.Fatalf("failed to create LINSTOR client: %v", err)
	}

	r := &LinstorSatelliteReconciler{
		Namespace:   "ns",
		log:         logr.Discard(),
		PodExecutor: &fakeExecutor{fn: constExecutor("  vg1\n", nil)},
	}

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "ns"}}
	lsatellite := &piraeusiov1.LinstorSatellite{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec: piraeusiov1.LinstorSatelliteSpec{
			StoragePools: []piraeusiov1.LinstorStoragePool{{
				Name:    "pool1",
				LvmPool: &piraeusiov1.LinstorStoragePoolLvm{VolumeGroup: "vg1", SharedSpace: "shared", ExternalLocking: true},
			}},
		},
	}

	err = r.reconcileStoragePools(context.Background(), &linstorhelper.Client{Client: lapiClient}, lsatellite, node, pod)
	if err != nil {
		t.Fatalf("reconcileStoragePools() err = %v", err)
	}

	var registered *lapi.StoragePool
	for _, sp := range fake.StoragePools() {
		if sp.StoragePoolName == "pool1" && sp.NodeName == "node1" {
			registered = &sp
			break
		}
	}
	if registered == nil {
		t.Fatalf("storage pool not registered")
	}

	if registered.FreeSpaceMgrName != "shared" {
		t.Errorf("free space manager = %q, want %q", registered.FreeSpaceMgrName, "shared")
	}
	if !registered.ExternalLocking {
		t.Errorf("external locking not set on registered storage pool")
	}
}
