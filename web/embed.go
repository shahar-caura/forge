package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distEmbed embed.FS

// DistFS is the embedded SPA build output, rooted at dist/.
var DistFS, _ = fs.Sub(distEmbed, "dist")
