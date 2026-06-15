package restconfig

import "embed"

// Files contains the canonical YAML templates embedded into the rest binary.
// The CLI copies only these YAML files; this Go source remains part of the generator.
//
//go:embed *.yaml
var Files embed.FS
