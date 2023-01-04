package cluster

import "embed"

//go:embed satellite-common controller csi satellite patches
var Resources embed.FS
