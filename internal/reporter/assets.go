package reporter

import "embed"

//go:embed all:assets
var assetsFS embed.FS

//go:embed all:templates
var templatesFS embed.FS
