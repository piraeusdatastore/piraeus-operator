package cluster

import "embed"

//go:embed satellite-common controller csi-controller csi-node satellite patches client-cert ha-controller
var Resources embed.FS
