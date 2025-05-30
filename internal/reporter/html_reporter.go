package reporter

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
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

//go:embed assets/css/styles.css
var defaultCSSEmbed embed.FS

//go:embed assets/js/report.js
var defaultJSEmbed embed.FS

//go:embed assets/img/favicon.ico
var faviconICO []byte

const (
	defaultReportTemplateName = "report.html.tmpl"
	// These are paths for the embed.FS to read from
	embeddedCSSPath = "assets/css/styles.css"
	embeddedJSPath  = "assets/js/report.js"
)

// HtmlReporter is responsible for generating HTML reports from probe results.
// It uses Go's html/template package.
type HtmlReporter struct {
	cfg          *config.ReporterConfig // Configuration for the reporter
	logger       zerolog.Logger         // For logging reporter activities
	template     *template.Template     // Parsed HTML template
	templatePath string                 // Path to the HTML template file (optional, if not using embed)
	favicon      string                 // Base64 encoded favicon
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

	// Define the function map for the template explicitly HERE
	funcMap := template.FuncMap{
		"json": func(v interface{}) (template.JS, error) {
			a, errMarshal := json.Marshal(v)
			if errMarshal != nil {
				return "", errMarshal
			}
			return template.JS(a), nil
		},
		"ToLower": strings.ToLower,
		"joinStrings": func(s []string, sep string) string {
			return strings.Join(s, sep)
		},
	}

	// Initialize tmpl with funcMap BEFORE parsing files or FS
	tmpl := template.New(defaultReportTemplateName).Funcs(funcMap)
	var parseErr error // Use a distinct variable name for parsing errors

	if cfg.TemplatePath != "" {
		moduleLogger.Info().Str("template_path", cfg.TemplatePath).Msg("Loading custom report template from file.")
		// Parse from files into the existing tmpl instance
		tmpl = template.New(filepath.Base(cfg.TemplatePath)).Funcs(funcMap)
		_, parseErr = tmpl.ParseFiles(cfg.TemplatePath)
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
		cfg:          cfg,
		logger:       moduleLogger,
		template:     tmpl,
		templatePath: cfg.TemplatePath,
		favicon:      base64.StdEncoding.EncodeToString(faviconICO),
	}, nil
}

// prepareReportData populates the ReportPageData struct based on probe results and reporter config.
// It now accepts a slice of pointers to ProbeResult.
// urlDiffs map[string]models.URLDiffResult, // This parameter is no longer used directly for populating ProbeResults
func (r *HtmlReporter) prepareReportData(probeResults []*models.ProbeResult, secretFindings []models.SecretFinding, partInfo string) (models.ReportPageData, error) {
	r.logger.Debug().Msg("Preparing report page data.")
	pageData := models.GetDefaultReportPageData()
	pageData.ReportTitle = r.cfg.ReportTitle
	if pageData.ReportTitle == "" {
		pageData.ReportTitle = "MonsterInc Scan Report" // Default if empty
	}
	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05 MST")
	pageData.ItemsPerPage = r.cfg.ItemsPerPage
	if pageData.ItemsPerPage <= 0 {
		pageData.ItemsPerPage = 25
		r.logger.Debug().Int("default_items", pageData.ItemsPerPage).Msg("Using default items per page.")
	}
	pageData.EnableDataTables = r.cfg.EnableDataTables
	pageData.DiffSummaryData = make(map[string]models.DiffSummaryEntry) // Will be populated later
	pageData.FaviconBase64 = r.favicon
	pageData.ReportPartInfo = partInfo

	displayResults := make([]models.ProbeResultDisplay, 0, len(probeResults))
	statusCodes := make(map[int]struct{})
	contentTypes := make(map[string]struct{})
	techs := make(map[string]struct{})
	rootTargetsEncountered := make(map[string]struct{}) // To populate UniqueRootTargets

	for i, prPointer := range probeResults {
		if prPointer == nil {
			r.logger.Warn().Int("index", i).Msg("Encountered a nil ProbeResult pointer, skipping.")
			continue
		}
		pr := *prPointer // Dereference prPointer to get the actual ProbeResult struct

		// Explicitly log the pr.URLStatus field for the raw ProbeResult
		r.logger.Debug().Int("index", i).Str("inputURL", pr.InputURL).Str("urlStatus_from_pr", string(pr.URLStatus)).Msg("Checking raw ProbeResult.URLStatus before Display conversion")

		// Log a sample ProbeResult (e.g., the first one in the chunk)
		if i == 0 {
			r.logger.Debug().Interface("sample_probe_result_raw_in_chunk", pr).Int("chunk_part_info_for_raw", len(partInfo)).Msg("Sample raw ProbeResult before Display conversion")
		}

		displayPr := models.ToProbeResultDisplay(pr) // Use pr directly

		// Log a sample ProbeResultDisplay (e.g., the first one in the chunk)
		if i == 0 {
			r.logger.Debug().Interface("sample_probe_result_display_in_chunk", displayPr).Int("chunk_part_info_for_display", len(partInfo)).Msg("Sample ProbeResultDisplay after conversion")
		}

		// Ensure RootTargetURL is present
		if pr.RootTargetURL == "" {
			// Attempt to derive from InputURL if truly empty, though it should ideally be set
			u, err := url.Parse(pr.InputURL)
			if err == nil {
				pr.RootTargetURL = u.Scheme + "://" + u.Host
				r.logger.Warn().Str("input_url", pr.InputURL).Str("derived_root", pr.RootTargetURL).Msg("ProbeResult had empty RootTargetURL, derived from InputURL.")
			} else {
				pr.RootTargetURL = pr.InputURL // Fallback, less ideal
				r.logger.Warn().Str("input_url", pr.InputURL).Msg("ProbeResult had empty RootTargetURL and could not parse InputURL to derive it. Using InputURL as RootTargetURL.")
			}
			// Update displayPr as well if pr.RootTargetURL was modified
			displayPr.RootTargetURL = pr.RootTargetURL
		}
		rootTargetsEncountered[pr.RootTargetURL] = struct{}{}

		displayResults = append(displayResults, displayPr)

		// Update Success/Failed counts based on URLStatus or StatusCode
		// A result is 'successful' for reporting if it's New or Existing (implies it was probed successfully)
		// 'Old' results are not typically part of the current scan's success/failure metrics directly,
		// but their presence is noted in the diff summary.
		currentURLStatus := models.URLStatus(pr.URLStatus) // Cast to models.URLStatus for comparison
		if currentURLStatus == models.StatusNew || currentURLStatus == models.StatusExisting {
			pageData.SuccessResults++
		} else if currentURLStatus != models.StatusOld { // Not New, Existing, or Old
			// Consider it failed if not explicitly successful and not 'Old'.
			// This includes errors during probing (where IsSuccess would be false)
			// or other statuses that are not 'Old'.
			if (pr).Error != "" || (pr).StatusCode == 0 || (pr).StatusCode >= 400 {
				pageData.FailedResults++
			} else {
				// This case might cover successfully probed items that are not New/Existing/Old
				// (e.g. if a new status is introduced). For now, count as success if IsSuccess is true.
				// This might need refinement depending on how URLStatus is used for non-diff states.
				pageData.SuccessResults++
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

	pageData.ProbeResults = displayResults
	pageData.TotalResults = len(displayResults)

	// Log details about pageData.ProbeResults before marshalling
	r.logger.Debug().Int("display_results_count_for_part", len(pageData.ProbeResults)).Str("part_info", partInfo).Msg("Count of ProbeResultDisplay items before JSON marshalling for this part")
	if len(pageData.ProbeResults) > 0 {
		r.logger.Debug().Interface("first_display_result_for_json_in_part", pageData.ProbeResults[0]).Str("part_info", partInfo).Msg("First ProbeResultDisplay item to be marshalled for this part")
	}

	// Calculate DiffSummaryData based on the processed pageData.ProbeResults
	tempDiffSummary := make(map[string]struct {
		NewCount      int
		OldCount      int
		ExistingCount int
	})

	for _, dispPr := range pageData.ProbeResults {
		summaryEntry := tempDiffSummary[dispPr.RootTargetURL]
		switch models.URLStatus(dispPr.URLStatus) {
		case models.StatusNew:
			summaryEntry.NewCount++
		case models.StatusOld:
			summaryEntry.OldCount++
		case models.StatusExisting:
			summaryEntry.ExistingCount++
		}
		tempDiffSummary[dispPr.RootTargetURL] = summaryEntry
	}
	for rootTgt, counts := range tempDiffSummary {
		pageData.DiffSummaryData[rootTgt] = models.DiffSummaryEntry{
			NewCount:      counts.NewCount,
			OldCount:      counts.OldCount,
			ExistingCount: counts.ExistingCount,
		}
	}

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
// secretFindings contains all secret detection findings to be included in the report.
// It now accepts a slice of pointers to ProbeResult.
// urlDiffs map[string]models.URLDiffResult, // This parameter is no longer used
func (r *HtmlReporter) GenerateReport(probeResults []*models.ProbeResult, secretFindings []models.SecretFinding, baseOutputPath string) ([]string, error) {
	if !r.cfg.GenerateEmptyReport && len(probeResults) == 0 && len(secretFindings) == 0 {
		r.logger.Info().Msg("No results to report and generate_empty_report is false. Skipping report generation.")
		return []string{}, nil
	}

	maxResultsPerFile := r.cfg.MaxProbeResultsPerReportFile
	totalResults := len(probeResults)
	var reportFilePaths []string

	if totalResults == 0 { // Handle case with only secret findings or diffs, no probe results
		pageData, err := r.prepareReportData(nil, secretFindings, "1/1") // urlDiffs was already removed here
		if err != nil {
			return nil, fmt.Errorf("failed to prepare report data for empty probe results: %w", err)
		}
		if err := r.executeAndWriteReport(pageData, baseOutputPath); err != nil {
			return []string{baseOutputPath}, fmt.Errorf("failed to write single report (no probe results): %w", err) // return path for potential cleanup
		}
		return []string{baseOutputPath}, nil
	}

	numParts := 1
	if maxResultsPerFile > 0 {
		numParts = (totalResults + maxResultsPerFile - 1) / maxResultsPerFile
	}

	for i := 0; i < numParts; i++ {
		start := i * maxResultsPerFile
		end := start + maxResultsPerFile
		if end > totalResults {
			end = totalResults
		}
		chunkResults := probeResults[start:end]

		partInfo := fmt.Sprintf("%d/%d", i+1, numParts)
		outputPath := baseOutputPath
		if numParts > 1 {
			ext := filepath.Ext(baseOutputPath)
			baseName := strings.TrimSuffix(baseOutputPath, ext)
			outputPath = fmt.Sprintf("%s_part_%d%s", baseName, i+1, ext)
		}

		pageData, err := r.prepareReportData(chunkResults, secretFindings, partInfo) // urlDiffs was already removed here
		if err != nil {
			r.logger.Error().Err(err).Int("part", i+1).Msg("Failed to prepare report data for part")
			// Decide if we should continue with other parts or fail all
			// For now, we try to generate other parts, but return an error at the end
			continue // Skip this part, but collect its path for error reporting if needed
		}

		if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
			// Log error, add path to a list of failed parts, and potentially return a multi-error
			r.logger.Error().Err(err).Str("path", outputPath).Msg("Failed to execute/write report part")
			// Even if a part fails, add its path so caller can attempt cleanup if desired.
			// The error from this function will indicate overall failure.
			// We could accumulate errors and return a combined error.
			return reportFilePaths, fmt.Errorf("failed to write report part %s: %w", outputPath, err)
		}
		reportFilePaths = append(reportFilePaths, outputPath)
	}

	if len(reportFilePaths) == 0 && totalResults > 0 { // If all parts failed for some reason
		return nil, fmt.Errorf("all report parts failed to generate for %d results", totalResults)
	}

	return reportFilePaths, nil
}

// executeAndWriteReport executes the HTML template with the given data and writes the output to a file.
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	// Embed assets if configured, right before template execution for this specific file part.
	if r.cfg.EmbedAssets {
		// Embed CSS
		cssContent, cssErr := r.embedAssetContent(true) // true for isCSS
		if cssErr != nil {
			r.logger.Warn().Err(cssErr).Str("output_path", outputPath).Msg("Failed to embed CSS, report styling might be affected.")
		}
		pageData.CustomCSS = template.CSS(cssContent)

		// Embed JS
		jsContent, jsErr := r.embedAssetContent(false) // false for isCSS
		if jsErr != nil {
			r.logger.Warn().Err(jsErr).Str("output_path", outputPath).Msg("Failed to embed JS, report functionality might be affected.")
		}
		pageData.ReportJs = template.JS(jsContent)
	} else {
		// Logic for non-embedded assets (paths) would go here.
		pageData.CustomCSS = ""
		pageData.ReportJs = ""
		r.logger.Info().Str("output_path", outputPath).Msg("Asset embedding is disabled. Styling/JS might be missing unless template handles external files.")
	}

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

// embedAssetContent reads and concatenates asset files.
// It now only loads the default embedded asset (CSS or JS based on isCSS flag).
func (r *HtmlReporter) embedAssetContent(isCSS bool) (string, error) {
	var assetData []byte
	var err error
	var defaultAssetPath string
	var embeddedFS embed.FS
	assetTypeStr := "JS"

	if isCSS {
		assetTypeStr = "CSS"
		defaultAssetPath = embeddedCSSPath
		embeddedFS = defaultCSSEmbed
	} else {
		defaultAssetPath = embeddedJSPath
		embeddedFS = defaultJSEmbed
	}

	r.logger.Debug().Str("asset", defaultAssetPath).Msgf("Using default embedded %s asset.", assetTypeStr)
	assetData, err = embeddedFS.ReadFile(defaultAssetPath)
	if err != nil {
		r.logger.Error().Err(err).Str("asset", defaultAssetPath).Msgf("FATAL: Failed to read default embedded %s asset. This should not happen.", assetTypeStr)
		return "", fmt.Errorf("failed to read default embedded %s asset '%s': %w", assetTypeStr, defaultAssetPath, err)
	}

	return string(assetData), nil
}

// templateFunctions provides helper functions accessible within the HTML template.
var templateFunctions = template.FuncMap{
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
