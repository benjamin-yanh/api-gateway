//go:build !embed || split

package main

import "embed"

// The standalone backend and split binaries do not serve frontend assets.
var buildFS embed.FS
var indexPage []byte
