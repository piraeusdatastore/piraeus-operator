package cluster

import "embed"

//go:embed satellite-common controller csi-controller csi-node satellite patches ha-controller
var Resources embed.FS
