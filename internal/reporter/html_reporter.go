package reporter

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"monsterinc/internal/config"
	"monsterinc/internal/models"
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
	logger       *log.Logger            // For logging reporter activities
	template     *template.Template     // Parsed HTML template
	templatePath string                 // Path to the HTML template file (optional, if not using embed)
}

// NewHtmlReporter creates a new HtmlReporter.
func NewHtmlReporter(cfg *config.ReporterConfig, appLogger *log.Logger) (*HtmlReporter, error) {
	if cfg == nil {
		cfg = &config.ReporterConfig{} // Use default config if nil
	}
	if appLogger == nil {
		appLogger = log.New(os.Stdout, "[Reporter] ", log.LstdFlags)
	}

	hr := &HtmlReporter{
		config: cfg,
		logger: appLogger,
	}

	// Load and parse the template
	templateName := "report.html.tmpl"
	var err error
	if hr.config.TemplatePath != "" {
		hr.logger.Printf("Loading template from custom path: %s", hr.config.TemplatePath)
		hr.template, err = template.New(filepath.Base(hr.config.TemplatePath)).Funcs(templateFunctions).ParseFiles(hr.config.TemplatePath)
	} else {
		hr.logger.Printf("Loading embedded template: %s", templateName)
		hr.template, err = template.New(templateName).Funcs(templateFunctions).ParseFS(defaultTemplate, "templates/"+templateName)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	return hr, nil
}

// prepareReportData populates the ReportPageData struct based on probe results and reporter config.
func (r *HtmlReporter) prepareReportData(probeResults []models.ProbeResult) (models.ReportPageData, error) {
	pageData := models.GetDefaultReportPageData() // Get a base structure
	pageData.ReportTitle = r.config.ReportTitle
	if pageData.ReportTitle == "" {
		pageData.ReportTitle = "MonsterInc Scan Report"
	}
	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 MST")
	pageData.TotalResults = len(probeResults)
	pageData.ItemsPerPage = r.config.DefaultItemsPerPage
	if pageData.ItemsPerPage <= 0 {
		pageData.ItemsPerPage = 10 // Default fallback
	}
	pageData.EnableDataTables = r.config.EnableDataTables // Pass DataTables config

	// Prepare display results and gather stats/filters
	displayResults := make([]models.ProbeResultDisplay, 0, len(probeResults))
	statusCodes := make(map[int]struct{})
	contentTypes := make(map[string]struct{})
	techs := make(map[string]struct{})
	rootTargets := make(map[string]struct{}) // For multi-target navigation

	for _, pr := range probeResults {
		displayPr := models.ToProbeResultDisplay(pr)
		displayResults = append(displayResults, displayPr)

		if displayPr.IsSuccess {
			pageData.SuccessResults++
		} else {
			pageData.FailedResults++
		}
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
		if displayPr.RootTargetURL != "" { // Collect root targets
			rootTargets[displayPr.RootTargetURL] = struct{}{}
		}
	}
	pageData.ProbeResults = displayResults

	// Serialize ProbeResults to JSON for JS
	resultsJSON, err := json.Marshal(displayResults)
	if err != nil {
		return pageData, fmt.Errorf("failed to marshal probe results to JSON: %w", err)
	}
	pageData.ProbeResultsJSON = template.JS(resultsJSON)

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

	for target := range rootTargets { // Populate unique root targets
		pageData.UniqueRootTargets = append(pageData.UniqueRootTargets, target)
	}
	sort.Strings(pageData.UniqueRootTargets)

	// Asset embedding will be handled in GenerateReport before template execution.

	return pageData, nil
}

// GenerateReport generates an HTML report from the given probe results.
// probeResults should be models.ProbeResult from the main application flow.
func (r *HtmlReporter) GenerateReport(probeResults []models.ProbeResult, outputPath string) error {
	if len(probeResults) == 0 && !r.config.GenerateEmptyReport {
		r.logger.Println("No probe results to report, and GenerateEmptyReport is false. Skipping report generation.")
		return nil // FR2: Do not generate report if no results and config says so.
	}
	r.logger.Printf("Generating HTML report for %d probe results to %s", len(probeResults), outputPath)

	pageData, err := r.prepareReportData(probeResults)
	if err != nil {
		r.logger.Printf("Error preparing report data: %v", err)
		return err // Return the error from prepareReportData
	}

	// Embed custom assets before executing the template
	assetErrors := r.embedCustomAssets(&pageData)
	if len(assetErrors) > 0 {
		for _, assetErr := range assetErrors {
			r.logger.Printf("[WARN] Failed to embed asset: %v", assetErr)
		}
		// Decide if asset embedding errors should be fatal or just warnings
	}

	// Execute template and write to file
	if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
		// Error is already logged by executeAndWriteReport
		return err
	}

	r.logger.Printf("HTML report successfully generated: %s", outputPath)
	return nil
}

// executeAndWriteReport executes the HTML template with the given data and writes the output to a file.
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	var buf bytes.Buffer
	if err := r.template.Execute(&buf, pageData); err != nil {
		r.logger.Printf("Error executing template: %v", err)
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		r.logger.Printf("Error creating directory for report: %v", err)
		return fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(outputPath), err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		r.logger.Printf("Error writing HTML report to file: %v", err)
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
	"joinStrings": func(s []string, sep string) string {
		return strings.Join(s, sep)
	},
	"toLower": strings.ToLower,
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

// TODO: Add unit tests for HtmlReporter in html_reporter_test.go
