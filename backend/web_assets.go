//go:build embed && !split

package main

import "embed"

//go:embed frontend/dist
var buildFS embed.FS

//go:embed frontend/dist/index.html
var indexPage []byte
