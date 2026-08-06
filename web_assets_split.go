//go:build split

package main

import "embed"

// Split control-plane and relay binaries never serve frontend assets. Empty
// values keep the legacy all-in-one code buildable without copying web/dist.
var buildFS embed.FS
var indexPage []byte
