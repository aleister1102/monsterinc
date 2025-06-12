package reporter

import "embed"

//go:embed assets/*
var assetsFS embed.FS

//go:embed templates/*
var templatesFS embed.FS

//go:embed assets/img/favicon.ico
var faviconICODiff []byte
