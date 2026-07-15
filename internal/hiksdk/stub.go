//go:build !hiksdk

// This file makes the package compile in the default (pure-Go, no-cgo) build,
// where the SDK backend is excluded. Nothing imports hiksdk unless built with
// `-tags hiksdk`, so the package is intentionally empty here.
package hiksdk
