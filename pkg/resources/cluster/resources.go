package cluster

import "embed"

//go:embed satellite-common controller csi satellite patches client-cert
var Resources embed.FS
