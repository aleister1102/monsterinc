package reporter

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"monsterinc/internal/config"
	"monsterinc/internal/models"

	"github.com/rs/zerolog"
)

//go:embed templates/report.html.tmpl
var defaultTemplate embed.FS

// Asset embedding - only custom assets now
//
//go:embed assets/css/styles.css
var customCSS embed.FS

//go:embed assets/js/report.js
var reportJS embed.FS

// HtmlReporter is responsible for generating HTML reports from probe results.
// It uses Go's html/template package.
type HtmlReporter struct {
	config       *config.ReporterConfig // Configuration for the reporter
	logger       zerolog.Logger         // For logging reporter activities
	template     *template.Template     // Parsed HTML template
	templatePath string                 // Path to the HTML template file (optional, if not using embed)
}

// NewHtmlReporter creates a new HtmlReporter.
func NewHtmlReporter(cfg *config.ReporterConfig, appLogger zerolog.Logger) (*HtmlReporter, error) {
	if cfg == nil {
		cfg = &config.ReporterConfig{} // Use default config if nil
	}

	hr := &HtmlReporter{
		config: cfg,
		logger: appLogger, // Assume appLogger is always provided and initialized
	}

	// Load and parse the template
	templateName := "report.html.tmpl"
	var err error
	if hr.config.TemplatePath != "" {
		hr.logger.Info().Str("path", hr.config.TemplatePath).Msg("Loading template from custom path")
		hr.template, err = template.New(filepath.Base(hr.config.TemplatePath)).Funcs(templateFunctions).ParseFiles(hr.config.TemplatePath)
	} else {
		hr.logger.Info().Str("template", templateName).Msg("Loading embedded template")
		hr.template, err = template.New(templateName).Funcs(templateFunctions).ParseFS(defaultTemplate, "templates/"+templateName)
	}

	if err != nil {
		// Use the reporter's logger once it's potentially initialized (or a temp one if init fails early)
		// For simplicity, if template loading fails, this log might use the passed appLogger or a default if hr.logger isn't set yet.
		hr.logger.Error().Err(err).Msg("Failed to parse HTML template")
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	return hr, nil
}

// prepareReportData populates the ReportPageData struct based on probe results and reporter config.
// It now accepts a slice of pointers to ProbeResult.
func (r *HtmlReporter) prepareReportData(probeResults []*models.ProbeResult, urlDiffs map[string]models.URLDiffResult) (models.ReportPageData, error) {
	pageData := models.GetDefaultReportPageData() // Get a base structure
	pageData.ReportTitle = r.config.ReportTitle
	if pageData.ReportTitle == "" {
		pageData.ReportTitle = "MonsterInc Scan Report"
	}
	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 MST")
	// pageData.TotalResults will be len(pageData.ProbeResults) after processing
	pageData.ItemsPerPage = r.config.DefaultItemsPerPage
	if pageData.ItemsPerPage <= 0 {
		pageData.ItemsPerPage = 100 // Default fallback
	}
	pageData.EnableDataTables = r.config.EnableDataTables
	pageData.DiffSummaryData = make(map[string]models.DiffSummaryEntry)

	displayResults := make([]models.ProbeResultDisplay, 0)
	statusCodes := make(map[int]struct{})
	contentTypes := make(map[string]struct{})
	techs := make(map[string]struct{})
	rootTargetsEncountered := make(map[string]struct{}) // For UniqueRootTargets

	totalProcessedForSummary := 0
	// Iterate through urlDiffs to populate displayResults and DiffSummaryData
	for rootTgt, diffResult := range urlDiffs {
		rootTargetsEncountered[rootTgt] = struct{}{}
		currentRootNew := 0
		currentRootOld := 0
		currentRootExisting := 0
		// currentRootChanged := 0 // For future use

		for _, diffedURL := range diffResult.Results { // Corrected: Iterate over Results (slice)
			pr := diffedURL.ProbeResult // This is the full ProbeResult

			// Ensure RootTargetURL is consistent if not already set by orchestrator/differ
			if pr.RootTargetURL == "" {
				pr.RootTargetURL = rootTgt
			}

			displayPr := models.ToProbeResultDisplay(pr) // Use the existing helper
			displayPr.URLStatus = string(pr.URLStatus)   // Set DiffStatus from ProbeResult.URLStatus
			displayResults = append(displayResults, displayPr)
			totalProcessedForSummary++

			switch models.URLStatus(pr.URLStatus) { // Corrected: Cast pr.URLStatus to models.URLStatus
			case models.StatusNew:
				currentRootNew++
				pageData.SuccessResults++ // Assuming new is a success for this counter
			case models.StatusOld:
				currentRootOld++
				// Old URLs are not typically counted in active success/failure for the *current* scan
			case models.StatusExisting:
				currentRootExisting++
				pageData.SuccessResults++ // Assuming existing is a success
			// case models.StatusChanged:
			// currentRootChanged++
			// pageData.SuccessResults++ // Or handle as per business logic
			default:
				// For URLs from current scan that might have failed probing (e.g. no status code)
				// This part needs clarification: probeResults input vs diffResult data
				// For now, only New/Existing contribute to SuccessResults. FailedResults could be other conditions.
				if displayPr.StatusCode == 0 || displayPr.StatusCode >= 400 {
					pageData.FailedResults++
				} else {
					// if it's not new/existing but has a success code and not caught by other statuses
					// this logic path might need review depending on how pr.URLStatus is set for failed probes
				}
			}

			// Collect filter data from displayPr
			if displayPr.StatusCode > 0 {
				statusCodes[displayPr.StatusCode] = struct{}{}
			}
			if displayPr.ContentType != "" {
				contentTypes[strings.ToLower(strings.Split(displayPr.ContentType, ";")[0])] = struct{}{}
			}
			for _, techName := range displayPr.Technologies {
				if techName != "" {
					techs[techName] = struct{}{}
				}
			}
		} // End iterating diffedURL in a rootTgt

		pageData.DiffSummaryData[rootTgt] = models.DiffSummaryEntry{
			NewCount:      currentRootNew,
			OldCount:      currentRootOld,
			ExistingCount: currentRootExisting,
			// ChangedCount:  currentRootChanged,
		}
	} // End iterating urlDiffs

	// If probeResults parameter is not empty, it represents the *current* scan's raw probes.
	// This might be redundant if urlDiffs already contains all new/existing from current scan.
	// However, if some probes failed very early and didn't make it to diffing, they might be here.
	// For now, the primary source of truth for the report list is diffResult.DiffedURLs.
	// The stats like Success/Failed might need to reconcile if probeResults has items not in any diffResult.

	pageData.ProbeResults = displayResults
	pageData.TotalResults = len(displayResults) // Total items in the report table

	// If SuccessResults + FailedResults don't sum to TotalResults, it means some logic for counting is off
	// or not all displayResults are categorized. This is a sanity check point.
	// For now, we assume StatusNew and StatusExisting contribute to SuccessResults.
	// StatusOld items are in displayResults but might not count towards current scan's success/failure.

	// Populate unique filters
	for code := range statusCodes {
		pageData.UniqueStatusCodes = append(pageData.UniqueStatusCodes, code)
	}
	sort.Ints(pageData.UniqueStatusCodes)

	for ct := range contentTypes {
		pageData.UniqueContentTypes = append(pageData.UniqueContentTypes, ct)
	}
	sort.Strings(pageData.UniqueContentTypes)

	for tech := range techs {
		pageData.UniqueTechnologies = append(pageData.UniqueTechnologies, tech)
	}
	sort.Strings(pageData.UniqueTechnologies)

	for target := range rootTargetsEncountered { // Use the map built from urlDiffs keys
		pageData.UniqueRootTargets = append(pageData.UniqueRootTargets, target)
	}
	sort.Strings(pageData.UniqueRootTargets)

	// Serialize ProbeResults to JSON for JS
	resultsJSON, err := json.Marshal(pageData.ProbeResults)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal probe results to JSON for report page data")
		return pageData, fmt.Errorf("failed to marshal probe results to JSON: %w", err)
	}
	pageData.ProbeResultsJSON = template.JS(resultsJSON)
	// r.logger.Debug().Str("json", string(resultsJSON)).Msg("ProbeResultsJSON for template")

	return pageData, nil
}

// GenerateReport generates an HTML report from the probe results and diff results.
// outputPath is the desired path for the generated HTML file.
// urlDiffs is a map where the key is RootTargetURL and value is its corresponding URLDiffResult.
// It now accepts a slice of pointers to ProbeResult.
func (r *HtmlReporter) GenerateReport(probeResults []*models.ProbeResult, urlDiffs map[string]models.URLDiffResult, outputPath string) error {
	if len(probeResults) == 0 && !r.config.GenerateEmptyReport {
		r.logger.Info().Msg("No probe results to report, and GenerateEmptyReport is false. Skipping report generation.")
		return nil // FR2: Do not generate report if no results and config says so.
	}
	r.logger.Info().Msgf("Generating HTML report for %d probe results to %s", len(probeResults), outputPath)

	pageData, err := r.prepareReportData(probeResults, urlDiffs)
	if err != nil {
		r.logger.Error().Err(err).Msg("Error preparing report data")
		return err // Return the error from prepareReportData
	}

	// Embed custom assets before executing the template
	assetErrors := r.embedCustomAssets(&pageData)
	if len(assetErrors) > 0 {
		for _, assetErr := range assetErrors {
			r.logger.Warn().Err(assetErr).Msg("Failed to embed asset")
		}
		// Decide if asset embedding errors should be fatal or just warnings
	}

	// Execute template and write to file
	if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
		// Error is already logged by executeAndWriteReport
		return err
	}

	r.logger.Info().Msgf("HTML report successfully generated: %s", outputPath)
	return nil
}

// executeAndWriteReport executes the HTML template with the given data and writes the output to a file.
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	var buf bytes.Buffer
	if err := r.template.Execute(&buf, pageData); err != nil {
		r.logger.Error().Err(err).Msg("Error executing HTML template")
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		r.logger.Error().Err(err).Str("path", filepath.Dir(outputPath)).Msg("Error creating directory for report")
		return fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(outputPath), err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		r.logger.Error().Err(err).Str("path", outputPath).Msg("Error writing HTML report to file")
		return fmt.Errorf("failed to write HTML report to %s: %w", outputPath, err)
	}
	return nil
}

// embedCustomAssets only embeds styles.css and report.js now
func (r *HtmlReporter) embedCustomAssets(pageData *models.ReportPageData) []error {
	var errs []error

	cCSS, err := customCSS.ReadFile("assets/css/styles.css")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to read embedded styles.css: %w", err))
	} else {
		pageData.CustomCSS = template.CSS(cCSS)
	}

	rpJS, err := reportJS.ReadFile("assets/js/report.js")
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to read embedded report.js: %w", err))
	} else {
		pageData.ReportJs = template.JS(rpJS)
	}

	// DataTables are now CDN, so no embedding logic here for them by default.
	// If a config option to embed them was re-introduced, it would go here.

	return errs
}

// templateFunctions provides helper functions accessible within the HTML template.
var templateFunctions = template.FuncMap{
	"jsonMarshal": func(v interface{}) template.JS {
		a, err := json.Marshal(v)
		if err != nil {
			// This function is called from within a template, direct logging might be tricky
			// Consider returning an error string or using a global logger if absolutely necessary
			// For now, print to stderr as a fallback.
			fmt.Fprintf(os.Stderr, "[ERROR] Template: jsonMarshal error: %v\n", err)
			return ""
		}
		return template.JS(a)
	},
	"ToLower": strings.ToLower,
	"joinStrings": func(s []string, sep string) string {
		return strings.Join(s, sep)
	},
	"formatTime": func(t time.Time, layout string) string {
		if t.IsZero() {
			return "N/A"
		}
		return t.Format(layout)
	},
	"safeHTML": func(s string) template.HTML {
		return template.HTML(s)
	},
	"inc": func(i int) int {
		return i + 1
	},
}
