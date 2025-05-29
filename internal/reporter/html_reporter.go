package reporter

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/notifier"

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

//go:embed assets/img/favicon.ico
var faviconICO []byte

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
	moduleLogger := appLogger.With().Str("module", "HtmlReporter").Logger()

	if cfg.OutputDir == "" {
		cfg.OutputDir = config.DefaultReporterOutputDir // Use constant from config package
		moduleLogger.Info().Str("default_dir", cfg.OutputDir).Msg("OutputDir not specified in config, using default.")
	}

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		moduleLogger.Error().Err(err).Str("path", cfg.OutputDir).Msg("Failed to create report output directory.")
		return nil, fmt.Errorf("failed to create report output directory '%s': %w", cfg.OutputDir, err)
	}

	tmpl := template.New("report").Funcs(templateFunctions) // Add custom functions
	var parseErr error                                      // Use a distinct variable name for parsing errors

	if cfg.TemplatePath != "" {
		moduleLogger.Info().Str("template_path", cfg.TemplatePath).Msg("Loading custom report template from file.")
		// Attempt to parse the custom file. ParseFiles creates a new template associated with the receiver (tmpl)
		// and adds all parsed templates to it. If successful, the returned *template.Template is the same as tmpl.
		_, parseErr = tmpl.ParseFiles(cfg.TemplatePath) // Assign to _, as tmpl is already the correct *template.Template instance
		if parseErr != nil {
			moduleLogger.Error().Err(parseErr).Str("path", cfg.TemplatePath).Msg("Failed to parse custom report template from file.")
			return nil, fmt.Errorf("failed to parse custom report template '%s': %w", cfg.TemplatePath, parseErr)
		}
	} else {
		moduleLogger.Info().Msg("Loading embedded default report template.")
		defaultTemplateContent, errRead := defaultTemplate.ReadFile("templates/report.html.tmpl")
		if errRead != nil {
			moduleLogger.Error().Err(errRead).Msg("Failed to read embedded default report template.")
			return nil, fmt.Errorf("failed to load embedded default report template: %w", errRead)
		}

		cleanedTemplateContent := strings.ReplaceAll(string(defaultTemplateContent), "\r\n", "\n")

		// Parse the cleaned content into the existing tmpl instance.
		// The returned *template.Template is tmpl itself if successful.
		_, parseErr = tmpl.Parse(cleanedTemplateContent) // Assign to _, as tmpl is already the correct *template.Template instance
		if parseErr != nil {
			moduleLogger.Error().Err(parseErr).Msg("Failed to parse cleaned embedded report template.")
			return nil, fmt.Errorf("failed to parse cleaned embedded report template: %w", parseErr)
		}
	}

	// Log the name of the parsed template for debugging
	moduleLogger.Debug().Str("parsed_template_name", tmpl.Name()).Str("definition", tmpl.DefinedTemplates()).Msg("Template parsed/loaded")

	moduleLogger.Info().Msg("HtmlReporter initialized successfully.")
	return &HtmlReporter{
		config:       cfg,
		logger:       moduleLogger,
		template:     tmpl,
		templatePath: cfg.TemplatePath,
	}, nil
}

// prepareReportData populates the ReportPageData struct based on probe results and reporter config.
// It now accepts a slice of pointers to ProbeResult.
func (r *HtmlReporter) prepareReportData(probeResults []*models.ProbeResult, urlDiffs map[string]models.URLDiffResult, secretFindings []models.SecretFinding) (models.ReportPageData, error) {
	r.logger.Debug().Msg("Preparing report page data.")
	pageData := models.GetDefaultReportPageData()
	pageData.ReportTitle = r.config.ReportTitle
	if pageData.ReportTitle == "" {
		pageData.ReportTitle = "MonsterInc Scan Report" // Default if empty
	}
	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 MST")
	pageData.ItemsPerPage = r.config.ItemsPerPage // Corrected: was DefaultItemsPerPage
	if pageData.ItemsPerPage <= 0 {
		pageData.ItemsPerPage = 25 // Default fallback for items per page
		r.logger.Debug().Int("default_items", pageData.ItemsPerPage).Msg("Using default items per page.")
	}
	pageData.EnableDataTables = r.config.EnableDataTables
	pageData.DiffSummaryData = make(map[string]models.DiffSummaryEntry)

	displayResults := make([]models.ProbeResultDisplay, 0)
	statusCodes := make(map[int]struct{})
	contentTypes := make(map[string]struct{})
	techs := make(map[string]struct{})
	rootTargetsEncountered := make(map[string]struct{})

	for rootTgt, diffResult := range urlDiffs {
		rootTargetsEncountered[rootTgt] = struct{}{} // Track unique root targets
		var currentRootNew, currentRootOld, currentRootExisting int

		for _, diffedURL := range diffResult.Results {
			pr := diffedURL.ProbeResult
			if pr.RootTargetURL == "" {
				pr.RootTargetURL = rootTgt
				r.logger.Debug().Str("url", pr.InputURL).Str("assigned_root_target", rootTgt).Msg("Assigned root target to probe result in diff.")
			}

			displayPr := models.ToProbeResultDisplay(pr)
			displayPr.URLStatus = string(pr.URLStatus)
			displayResults = append(displayResults, displayPr)

			switch models.URLStatus(pr.URLStatus) {
			case models.StatusNew:
				currentRootNew++
				pageData.SuccessResults++
			case models.StatusOld:
				currentRootOld++
			case models.StatusExisting:
				currentRootExisting++
				pageData.SuccessResults++
			default:
				if displayPr.StatusCode == 0 || displayPr.StatusCode >= 400 {
					pageData.FailedResults++
				}
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
		}
		pageData.DiffSummaryData[rootTgt] = models.DiffSummaryEntry{
			NewCount:      currentRootNew,
			OldCount:      currentRootOld,
			ExistingCount: currentRootExisting,
		}
	}

	pageData.ProbeResults = displayResults
	pageData.TotalResults = len(displayResults)

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

	for target := range rootTargetsEncountered {
		pageData.UniqueRootTargets = append(pageData.UniqueRootTargets, target)
	}
	sort.Strings(pageData.UniqueRootTargets)

	resultsJSON, err := json.Marshal(pageData.ProbeResults)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal ProbeResults to JSON for template consumption.")
		return pageData, fmt.Errorf("failed to marshal probe results to JSON: %w", err)
	}
	pageData.ProbeResultsJSON = template.JS(resultsJSON)
	r.logger.Debug().Int("total_results_for_json", len(pageData.ProbeResults)).Msg("ProbeResults marshalled to JSON for template.")

	// Process secret findings
	pageData.SecretFindings = secretFindings
	pageData.SecretStats = notifier.CalculateSecretStats(secretFindings)

	// Marshal secret findings to JSON for JavaScript processing
	secretFindingsJSON, err := json.Marshal(secretFindings)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal SecretFindings to JSON for template consumption.")
		return pageData, fmt.Errorf("failed to marshal secret findings to JSON: %w", err)
	}
	pageData.SecretFindingsJSON = template.JS(secretFindingsJSON)
	r.logger.Debug().Int("total_secret_findings_for_json", len(secretFindings)).Msg("SecretFindings marshalled to JSON for template.")

	return pageData, nil
}

// GenerateReport generates an HTML report from the probe results and diff results.
// outputPath is the desired path for the generated HTML file.
// urlDiffs is a map where the key is RootTargetURL and value is its corresponding URLDiffResult.
// secretFindings contains all secret detection findings to be included in the report.
// It now accepts a slice of pointers to ProbeResult.
func (r *HtmlReporter) GenerateReport(probeResults []*models.ProbeResult, urlDiffs map[string]models.URLDiffResult, secretFindings []models.SecretFinding, outputPath string) error {
	if len(probeResults) == 0 && len(urlDiffs) == 0 && len(secretFindings) == 0 && !r.config.GenerateEmptyReport {
		r.logger.Info().Msg("No probe results, URL diffs, or secret findings to report, and GenerateEmptyReport is false. Skipping report generation.")
		return nil
	}
	r.logger.Info().Int("probe_result_count", len(probeResults)).Int("url_diff_count", len(urlDiffs)).Int("secret_findings_count", len(secretFindings)).Str("output_path", outputPath).Msg("Generating HTML report.")

	pageData, err := r.prepareReportData(probeResults, urlDiffs, secretFindings)
	if err != nil {
		r.logger.Error().Err(err).Msg("Error preparing report data during GenerateReport.")
		return err
	}

	assetErrors := r.embedCustomAssets(&pageData)
	if len(assetErrors) > 0 {
		for _, assetErr := range assetErrors {
			r.logger.Warn().Err(assetErr).Msg("Failed to embed a custom asset into pageData.")
		}
	}

	if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
		return err // Error already logged by executeAndWriteReport
	}

	r.logger.Info().Str("path", outputPath).Msg("HTML report generated successfully.")
	return nil
}

// executeAndWriteReport executes the HTML template with the given data and writes the output to a file.
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	var buf bytes.Buffer

	r.logger.Debug().Str("executing_template_name", r.template.Name()).Str("definition", r.template.DefinedTemplates()).Msg("Before executing template")

	if err := r.template.Execute(&buf, pageData); err != nil {
		r.logger.Error().Err(err).Msg("Error executing HTML template into buffer.")
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		r.logger.Error().Err(err).Str("path", filepath.Dir(outputPath)).Msg("Error creating directory for HTML report file.")
		return fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(outputPath), err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		r.logger.Error().Err(err).Str("path", outputPath).Msg("Error writing HTML report to file.")
		return fmt.Errorf("failed to write HTML report to %s: %w", outputPath, err)
	}
	r.logger.Debug().Str("path", outputPath).Int("bytes_written", buf.Len()).Msg("Successfully wrote HTML report to file.")
	return nil
}

// embedCustomAssets only embeds styles.css and report.js now
func (r *HtmlReporter) embedCustomAssets(pageData *models.ReportPageData) []error {
	var errs []error
	r.logger.Debug().Msg("Attempting to embed custom assets (CSS, JS).")

	cCSS, err := customCSS.ReadFile("assets/css/styles.css")
	if err != nil {
		newErr := fmt.Errorf("failed to read embedded styles.css: %w", err)
		r.logger.Warn().Err(newErr).Msg("Could not read embedded styles.css")
		errs = append(errs, newErr)
	} else {
		pageData.CustomCSS = template.CSS(cCSS)
		r.logger.Debug().Int("css_size", len(cCSS)).Msg("Embedded styles.css successfully.")
	}

	rpJS, err := reportJS.ReadFile("assets/js/report.js")
	if err != nil {
		newErr := fmt.Errorf("failed to read embedded report.js: %w", err)
		r.logger.Warn().Err(newErr).Msg("Could not read embedded report.js")
		errs = append(errs, newErr)
	} else {
		pageData.ReportJs = template.JS(rpJS)
		r.logger.Debug().Int("js_size", len(rpJS)).Msg("Embedded report.js successfully.")
	}

	// Favicon logic
	if len(faviconICO) > 0 {
		pageData.FaviconBase64 = base64.StdEncoding.EncodeToString(faviconICO)
		r.logger.Debug().Int("favicon_size_original", len(faviconICO)).Int("favicon_base64_len", len(pageData.FaviconBase64)).Msg("Embedded favicon.ico successfully as base64.")
	} else {
		r.logger.Warn().Msg("Embedded favicon.ico data is empty.")
	}

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
	"slice": func(s string, start int, end ...int) string {
		if len(s) == 0 {
			return s
		}
		if start < 0 {
			start = len(s) + start
		}
		if start < 0 {
			start = 0
		}
		if start >= len(s) {
			return ""
		}

		if len(end) > 0 {
			endIdx := end[0]
			if endIdx < 0 {
				endIdx = len(s) + endIdx
			}
			if endIdx > len(s) {
				endIdx = len(s)
			}
			if endIdx <= start {
				return ""
			}
			return s[start:endIdx]
		}
		return s[start:]
	},
	"eq": func(a, b interface{}) bool {
		return a == b
	},
	"gt": func(a, b interface{}) bool {
		switch av := a.(type) {
		case int:
			if bv, ok := b.(int); ok {
				return av > bv
			}
		case string:
			if bv, ok := b.(string); ok {
				return len(av) > len(bv)
			}
		}
		return false
	},
}
