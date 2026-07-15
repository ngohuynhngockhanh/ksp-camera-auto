// Package web bundles the static web UI into the binary via go:embed so the
// tool ships as a single self-contained executable.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS

// Static returns the UI file system rooted at the static directory.
func Static() (fs.FS, error) {
	return fs.Sub(files, "static")
}
