package cluster

import "embed"

//go:embed satellite-common controller csi satellite patches client-cert ha-controller
var Resources embed.FS
