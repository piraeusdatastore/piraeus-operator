package cluster

import "embed"

//go:embed satellite-common controller csi satellite
var Resources embed.FS
