package reporter

const (
	// Directory constants
	DefaultDiffReportDir       = "reports/diff"
	DefaultDiffReportAssetsDir = "reports/diff/assets"
	DefaultReportTemplateName  = "report_client_side.html.tmpl"

	// Embedded asset paths for scan reports
	EmbeddedCSSPath = "assets/css/report_client_side.css"
	EmbeddedJSPath  = "assets/js/report_client_side.js"

	// Embedded asset paths for diff reports
	EmbeddedDiffCSSPath = "assets/css/diff_report_client_side.css"
	EmbeddedDiffJSPath  = "assets/js/diff_report_client_side.js"

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
