package controller

import (
	"context"
	"fmt"
	"strings"

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
