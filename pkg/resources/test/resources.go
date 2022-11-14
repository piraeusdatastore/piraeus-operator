package test

import "embed"

var (
	//go:embed empty
	EmptyResources embed.FS

	//go:embed basic
	BasicResources embed.FS
)
