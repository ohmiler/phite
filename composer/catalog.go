package composercatalog

import (
	"bytes"
	_ "embed"
)

//go:embed manifest.json
var embeddedManifest []byte

// Manifest returns an isolated copy of the Composer Manifest embedded in Phite.
func Manifest() []byte {
	return bytes.Clone(embeddedManifest)
}
