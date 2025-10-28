package frontend

import (
	"embed"
)

//go:embed all:dist
var FilesFS embed.FS
