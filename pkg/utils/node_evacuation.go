package utils

import lclient "github.com/LINBIT/golinstor/client"

func IsUpToDateResource(res lclient.ResourceWithVolumes) bool {
	for _, volume := range res.Volumes {
		if volume.State.DiskState != "UpToDate" {
			return false
		}
	}
	return true
}

func IsDisklessResource(res lclient.ResourceWithVolumes) bool {
	// Skip Diskless pools
	for _, volume := range res.Volumes {
		if volume.State.DiskState != "Diskless" && volume.State.DiskState != "TieBreaker" {
			return false
		}
	}
	return true
}

func IsTieBreakerResource(res lclient.ResourceWithVolumes) bool {
	// Skip Diskless pools
	for _, volume := range res.Volumes {
		if volume.State.DiskState == "TieBreaker" {
			return true
		}
	}
	return false
}
