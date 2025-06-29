package reporter

import (
	"html/template"
	"time"

	"github.com/aleister1102/monsterinc/internal/differ"
	"github.com/aleister1102/monsterinc/internal/httpxrunner"
)

// ProbeResultDisplay is a struct tailored for displaying probe results in the HTML report.
// It might omit or reformat fields from the main ProbeResult struct.
type ProbeResultDisplay struct {
	InputURL        string
	FinalURL        string
	Method          string
	StatusCode      int
	ContentLength   int64
	ContentType     string
	Title           string
	WebServer       string
	Technologies    []string // Kept as a slice for easier template handling, join in template if needed
	IPs             []string
	CNAMEs          []string
	ASN             int
	ASNOrg          string
	TLSVersion      string
	TLSCipher       string
	TLSCertIssuer   string
	TLSCertExpiry   string // Formatted string for display
	Duration        float64
	Headers         map[string]string
	Body            string // Or a snippet, or path to stored body
	Error           string
	Timestamp       string // Formatted string for display
	IsSuccess       bool   // Helper for template logic
	HasTechnologies bool   // Helper
	HasTLS          bool   // Helper
	HasASN          bool   // Helper for template to check if ASN info is present
	HasCNAMEs       bool   // Helper
	HasIPs          bool   // Helper
	RootTargetURL   string // Added for multi-target navigation
	URLStatus       string `json:"diff_status,omitempty"` // Changed from URLStatus
}

// ReportPageData holds all the data needed to render the HTML report page.
type ReportPageData struct {
	ReportTitle    string
	GeneratedAt    string // Formatted timestamp
	ProbeResults   []ProbeResultDisplay
	TotalResults   int
	SuccessResults int
	FailedResults  int
	Config         *ReporterConfigForTemplate // To pass some config like ItemsPerPage
	// Additional metadata can be added here
	UniqueStatusCodes  []int
	UniqueContentTypes []string
	UniqueTechnologies []string
	UniqueRootTargets  []string                        // Deprecated: keeping for backward compatibility
	UniqueHostnames    []string                        // New: for hostname-based grouping
	UniqueURLStatuses  []string                        // For diff status filtering
	CustomCSS          template.CSS                    // For embedded styles.css
	ReportJs           template.JS                     // Embedded custom report.js
	URLDiffs           map[string]differ.URLDiffResult `json:"url_diffs,omitempty"` // Added to hold raw diff results
	Theme              string                          // e.g., "dark" or "light"
	FilterPlaceholders map[string]string               // e.g. "Search Title..."
	TableHeaders       []string                        // For dynamic table generation if needed
	ItemsPerPage       int                             // From config
	EnableDataTables   bool                            // From config, determines if CDN links for DataTables are included
	ShowTimelineView   bool                            // Future feature?
	ErrorMessage       string                          // If report generation has a top-level error
	FaviconBase64      string                          // Base64 encoded favicon
	ProbeResultsJSON   template.JS                     `json:"-"` // JSON string of ProbeResults for JavaScript processing

	// Diffing summary data, map key is RootTargetURL
	DiffSummaryData map[string]DiffSummaryEntry `json:"diff_summary_data"`

	// Report Part Information (for multi-part reports)
	ReportPartInfo string `json:"report_part_info,omitempty"`
}

// ReporterConfigForTemplate is a subset of reporter configurations relevant for the template.
type ReporterConfigForTemplate struct {
	ItemsPerPage int
}

// Helper function to transform ProbeResult to ProbeResultDisplay
// This function should be in a package that can import both models and httpxrunner if ProbeResult is from there.
// For now, assuming ProbeResult is models.ProbeResult.
func ToProbeResultDisplay(pr httpxrunner.ProbeResult) ProbeResultDisplay {
	// Determine if the probe was successful (e.g., status code 2xx and no major error)
	isSuccess := pr.Error == "" && (pr.StatusCode >= 200 && pr.StatusCode < 400) // Consider 3xx as success for reachability

	var technologies []string
	for _, t := range pr.Technologies {
		technologies = append(technologies, t.Name)
	}

	return ProbeResultDisplay{
		InputURL:        pr.InputURL,
		FinalURL:        pr.FinalURL,
		Method:          pr.Method,
		StatusCode:      pr.StatusCode,
		ContentLength:   pr.ContentLength,
		ContentType:     pr.ContentType,
		Title:           pr.Title,
		WebServer:       pr.WebServer,
		Technologies:    technologies,
		IPs:             pr.IPs,
		CNAMEs:          pr.CNAMEs,
		ASN:             pr.ASN,
		ASNOrg:          pr.ASNOrg,
		Duration:        pr.Duration,
		Headers:         pr.Headers,
		Body:            pr.Body, // Consider snippet or link
		Error:           pr.Error,
		Timestamp:       pr.Timestamp.Format("2006-01-02 15:04:05 MST"),
		IsSuccess:       isSuccess,
		HasTechnologies: len(technologies) > 0,
		HasASN:          pr.ASN != 0,
		HasCNAMEs:       len(pr.CNAMEs) > 0,
		HasIPs:          len(pr.IPs) > 0,
		RootTargetURL:   pr.RootTargetURL, // Use the correct RootTargetURL from ProbeResult
		URLStatus:       pr.URLStatus,     // Assign URLStatus
	}
}

// Add more helper functions or structs as needed for the report.
// For example, a struct to hold filter options populated from the data.

func GetDefaultReportPageData() ReportPageData {
	return ReportPageData{
		ReportTitle: "MonsterInc Scan Report",
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		Theme:       "light", // Default theme
		FilterPlaceholders: map[string]string{
			"globalSearch":   "Search all fields...",
			"titleSearch":    "Filter by Title...",
			"techSearch":     "Filter by Technology...",
			"finalUrlSearch": "Filter by Final URL...",
		},
		TableHeaders: []string{ // Default headers, can be customized
			"Input URL", "Final URL", "Status", "Title", "Technologies", "Web Server", "Content Type", "Length", "IPs",
		},
		ItemsPerPage:     10,   // Default, should come from config
		EnableDataTables: true, // Default, should come from config
	}
}

// DiffSummaryEntry holds counts for a specific root target's diff results
type DiffSummaryEntry struct {
	NewCount      int `json:"new_count"`
	OldCount      int `json:"old_count"`
	ExistingCount int `json:"existing_count"`
	ChangedCount  int `json:"changed_count"` // Keep for future use
}

// SetCustomCSS sets the custom CSS for the report page
func (rpd *ReportPageData) SetCustomCSS(css template.CSS) {
	rpd.CustomCSS = css
}

// SetReportJs sets the report JavaScript for the report page
func (rpd *ReportPageData) SetReportJs(js template.JS) {
	rpd.ReportJs = js
}
