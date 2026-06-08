package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"

	utilexec "k8s.io/client-go/util/exec"

	piraeusiov1 "github.com/piraeusdatastore/piraeus-operator/v2/api/v1"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/podexec"
	"github.com/piraeusdatastore/piraeus-operator/v2/pkg/vars"
)

// storagePoolBackendExists probes the satellite node to determine whether the backend storage object (LVM volume
// group, LVM thin pool, ZFS dataset) referenced by the storage pool already exists. It returns (true, nil) for
// storage pools without a separate backend to probe (file based pools). A non-nil error means the probe was
// inconclusive (for example the satellite was not reachable).
func storagePoolBackendExists(ctx context.Context, exec podexec.Executor, namespace, podName string, pool *piraeusiov1.LinstorStoragePool) (bool, error) {
	command := pool.BackendProbeCommand()
	if command == nil {
		// No separate backend to probe (e.g. file based pools): treat the backend as present.
		return true, nil
	}

	stdout, stderr, err := exec.Exec(ctx, namespace, podName, vars.SatelliteContainerName, command)
	if err != nil {
		return false, fmt.Errorf("storage backend probe %v failed (stderr: %q): %w", command, strings.TrimSpace(stderr), err)
	}

	// The probe lists all existing backends of the pool's kind, one identifier per line. The backend exists if
	// the pool's backend identifier is among them.
	want := pool.PoolName()
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == want {
			return true, nil
		}
	}

	return false, nil
}

// sourceDeviceProbeCommand returns a command that prints, one per line, which of the given device paths exist on
// the node. "stat" prints each operand it can resolve (following symlinks with -L) and skips the rest. The paths
// are passed as plain arguments, so no shell interpolation is involved.
func sourceDeviceProbeCommand(devices []string) []string {
	return append([]string{"stat", "-L", "-c", "%n"}, devices...)
}

// missingSourceDevices probes the node and returns the subset of the given device paths that do not exist.
//
// A non-nil error means the probe was inconclusive. "stat" exits non-zero when some operands do not exist, which
// is expected here and not inconclusive: the existing devices are still listed on standard output.
func missingSourceDevices(ctx context.Context, exec podexec.Executor, namespace, podName string, devices []string) ([]string, error) {
	if len(devices) == 0 {
		return nil, nil
	}

	stdout, stderr, err := exec.Exec(ctx, namespace, podName, vars.SatelliteContainerName, sourceDeviceProbeCommand(devices))
	if err != nil {
		var exitErr utilexec.CodeExitError
		if !errors.As(err, &exitErr) {
			return nil, fmt.Errorf("source device probe failed (stderr: %q): %w", strings.TrimSpace(stderr), err)
		}
	}

	present := make(map[string]struct{})
	for _, line := range strings.Split(stdout, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			present[line] = struct{}{}
		}
	}

	var missing []string
	for _, dev := range devices {
		if _, ok := present[dev]; !ok {
			missing = append(missing, dev)
		}
	}

	return missing, nil
}
