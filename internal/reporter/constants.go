package reporter

const (
	// Directory constants
	DefaultDiffReportDir       = "reports/diff"
	DefaultDiffReportAssetsDir = "reports/diff/assets"
	DefaultReportTemplateName  = "report.html.tmpl"

	// Embedded asset paths
	EmbeddedCSSPath = "assets/css/styles.css"
	EmbeddedJSPath  = "assets/js/report.js"

	// Report generation defaults
	DefaultItemsPerPage    = 25
	DefaultReportTitle     = "MonsterInc Scan Report"
	DefaultDiffReportTitle = "MonsterInc Aggregated Content Diff Report"

	// File permissions
	DirPermissions  = 0755
	FilePermissions = 0644

	// String manipulation
	HashLength = 8

	// Report generation limits
	DefaultMaxResultsPerFile = 1000
)
