package models

import "html/template"

// ProbeResultDisplay is a subset of ProbeResult tailored for display in the HTML report.
// It should align with the fields available in models.ProbeResult and what the report needs.
type ProbeResultDisplay struct {
	InputURL      string // Changed from OriginalURL
	FinalURL      string
	StatusCode    int
	ContentLength int64
	ContentType   string
	Title         string
	WebServer     string   // Changed from ServerHeader
	Technologies  []string // This will be a list of tech names
	IPs           []string // Changed from IPAddress (string) to []string
	RootTargetURL string   // Make sure this is populated for JS filtering
	// Add other fields as needed for display
}

// ReportPageData holds all the data needed to render the HTML report template.
type ReportPageData struct {
	Title            string
	Timestamp        string
	ProbeResults     []ProbeResultDisplay // This will be used by Go template if JS is disabled or for initial render (though JS now handles it)
	Headers          []string             // Table headers
	RootTargets      []string             // Unique root targets for navigation
	ItemsPerPage     int                  // For pagination
	TotalResults     int
	CurrentFilter    string       // For search persistence
	StaticCSS        template.CSS // For custom styles.css
	StaticJS         template.JS  // For custom report.js
	DataTablesJS     template.JS  // For DataTables.js if kept local
	CustomFontCSS    template.CSS // If a custom font is embedded
	ProbeResultsJSON template.JS  // JSON string of ProbeResults for JS initialization
}

// TableColumn represents a column in the HTML report table
type TableColumn struct {
	Name       string // Display name of the column
	Identifier string // Identifier for sorting/filtering, matches ProbeResultDisplay field name (lowercase)
	Sortable   bool   // Whether the column is sortable
}
