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
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

//go:embed templates/report.html.tmpl
var defaultTemplate embed.FS

//go:embed assets/img/favicon.ico
var faviconICO []byte

const (
	defaultHtmlReportTemplateName = "report.html.tmpl"
	embeddedCSSPath               = "assets/css/styles.css"
	embeddedJSPath                = "assets/js/report.js"
)

// HtmlReporter uses composition of utility modules for better maintainability
type HtmlReporter struct {
	cfg          *config.ReporterConfig
	logger       zerolog.Logger
	template     *template.Template
	templatePath string
	favicon      string
	assetManager *AssetManager
	directoryMgr *DirectoryManager
}

// NewHtmlReporter creates a new HtmlReporter using utility modules
func NewHtmlReporter(cfg *config.ReporterConfig, appLogger zerolog.Logger) (*HtmlReporter, error) {
	moduleLogger := appLogger.With().Str("module", "HtmlReporter").Logger()

	reporter := &HtmlReporter{
		cfg:          cfg,
		logger:       moduleLogger,
		templatePath: cfg.TemplatePath,
		assetManager: NewAssetManager(moduleLogger),
		directoryMgr: NewDirectoryManager(moduleLogger),
	}

	if err := reporter.initializeOutputDirectory(); err != nil {
		return nil, err
	}

	if err := reporter.setupTemplate(); err != nil {
		return nil, err
	}

	reporter.initializeFavicon()

	moduleLogger.Info().Msg("HtmlReporter initialized successfully.")
	return reporter, nil
}

// initializeOutputDirectory ensures output directory exists
func (r *HtmlReporter) initializeOutputDirectory() error {
	if r.cfg.OutputDir == "" {
		r.cfg.OutputDir = config.DefaultReporterOutputDir
		r.logger.Info().Str("default_dir", r.cfg.OutputDir).Msg("OutputDir not specified, using default.")
	}

	return r.directoryMgr.EnsureOutputDirectories(r.cfg.OutputDir)
}

// setupTemplate initializes the HTML template with function map
func (r *HtmlReporter) setupTemplate() error {
	funcMap := r.createTemplateFunctionMap()
	tmpl := template.New(defaultHtmlReportTemplateName).Funcs(funcMap)

	if r.cfg.TemplatePath != "" {
		return r.loadCustomTemplate(tmpl)
	}

	return r.loadEmbeddedTemplate(tmpl)
}

// createTemplateFunctionMap creates the function map for HTML templates
func (r *HtmlReporter) createTemplateFunctionMap() template.FuncMap {
	funcMap := GetCommonTemplateFunctions()

	// Add HTML reporter specific functions
	funcMap["json"] = func(v interface{}) (template.JS, error) {
		a, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(a), nil
	}

	return funcMap
}

// loadCustomTemplate loads template from file path
func (r *HtmlReporter) loadCustomTemplate(tmpl *template.Template) error {
	r.logger.Info().Str("template_path", r.cfg.TemplatePath).Msg("Loading custom report template from file.")

	customTmpl := template.New(filepath.Base(r.cfg.TemplatePath)).Funcs(r.createTemplateFunctionMap())
	_, err := customTmpl.ParseFiles(r.cfg.TemplatePath)
	if err != nil {
		r.logger.Error().Err(err).Str("path", r.cfg.TemplatePath).Msg("Failed to parse custom report template.")
		return fmt.Errorf("failed to parse custom report template '%s': %w", r.cfg.TemplatePath, err)
	}

	r.template = customTmpl
	return nil
}

// loadEmbeddedTemplate loads the default embedded template
func (r *HtmlReporter) loadEmbeddedTemplate(tmpl *template.Template) error {
	r.logger.Info().Msg("Loading embedded default report template.")

	templateContent, err := defaultTemplate.ReadFile("templates/report.html.tmpl")
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to read embedded default report template.")
		return fmt.Errorf("failed to load embedded default report template: %w", err)
	}

	cleanedContent := strings.ReplaceAll(string(templateContent), "\r\n", "\n")
	_, err = tmpl.Parse(cleanedContent)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to parse cleaned embedded report template.")
		return fmt.Errorf("failed to parse cleaned embedded report template: %w", err)
	}

	r.template = tmpl
	r.logger.Debug().Str("parsed_template_name", tmpl.Name()).Msg("Template loaded successfully")
	return nil
}

// initializeFavicon sets up the base64 encoded favicon
func (r *HtmlReporter) initializeFavicon() {
	if len(faviconICO) > 0 {
		r.favicon = base64.StdEncoding.EncodeToString(faviconICO)
	} else {
		r.logger.Warn().Msg("Failed to load favicon, using empty string")
		r.favicon = ""
	}
}

// GenerateReport generates HTML reports from probe results
func (r *HtmlReporter) GenerateReport(probeResults []*models.ProbeResult, baseOutputPath string) ([]string, error) {
	if len(probeResults) == 0 {
		r.logger.Warn().Msg("No probe results provided for report generation.")
		return []string{}, nil
	}

	if baseOutputPath == "" {
		baseOutputPath = "report"
	}

	maxResults := r.cfg.MaxProbeResultsPerReportFile
	if maxResults <= 0 {
		maxResults = DefaultMaxResultsPerFile
	}

	if len(probeResults) <= maxResults {
		return r.generateSingleReport(probeResults, baseOutputPath)
	}

	return r.generateChunkedReports(probeResults, baseOutputPath, maxResults)
}

// generateSingleReport creates a single HTML report file
func (r *HtmlReporter) generateSingleReport(probeResults []*models.ProbeResult, baseOutputPath string) ([]string, error) {
	pageData, err := r.prepareReportData(probeResults, "")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare report data: %w", err)
	}

	outputPath := r.buildOutputPath(baseOutputPath, 0, 1)
	if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
		return nil, fmt.Errorf("failed to write report: %w", err)
	}

	r.logger.Info().Str("path", outputPath).Int("results", len(probeResults)).Msg("Single HTML report generated")
	return []string{outputPath}, nil
}

// generateChunkedReports creates multiple HTML report files for large result sets
func (r *HtmlReporter) generateChunkedReports(probeResults []*models.ProbeResult, baseOutputPath string, maxResults int) ([]string, error) {
	totalChunks := (len(probeResults) + maxResults - 1) / maxResults
	outputPaths := make([]string, 0, totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * maxResults
		end := start + maxResults
		if end > len(probeResults) {
			end = len(probeResults)
		}

		chunk := probeResults[start:end]
		partInfo := fmt.Sprintf("Part %d of %d", i+1, totalChunks)

		pageData, err := r.prepareReportData(chunk, partInfo)
		if err != nil {
			return outputPaths, fmt.Errorf("failed to prepare data for chunk %d: %w", i+1, err)
		}

		outputPath := r.buildOutputPath(baseOutputPath, i+1, totalChunks)
		if err := r.executeAndWriteReport(pageData, outputPath); err != nil {
			return outputPaths, fmt.Errorf("failed to write chunk %d: %w", i+1, err)
		}

		outputPaths = append(outputPaths, outputPath)
		r.logger.Debug().Int("chunk", i+1).Str("path", outputPath).Msg("Chunked report generated")
	}

	r.logger.Info().Int("total_files", len(outputPaths)).Int("total_results", len(probeResults)).Msg("Multi-part HTML report generated")
	return outputPaths, nil
}

// buildOutputPath constructs the output file path
func (r *HtmlReporter) buildOutputPath(baseOutputPath string, partNum, totalParts int) string {
	var filename string
	if totalParts == 1 {
		filename = fmt.Sprintf("%s.html", baseOutputPath)
	} else {
		filename = fmt.Sprintf("%s-part%d.html", baseOutputPath, partNum)
	}
	// Check if baseOutputPath is already an absolute path or includes directory
	// If it contains path separators, treat it as a full path
	if filepath.IsAbs(baseOutputPath) || strings.Contains(baseOutputPath, string(filepath.Separator)) {
		// baseOutputPath already contains the full path, just add extension if needed
		if totalParts == 1 {
			if !strings.HasSuffix(baseOutputPath, ".html") {
				return baseOutputPath + ".html"
			}
			return baseOutputPath
		} else {
			// Remove .html extension if present, then add part number
			basePath := strings.TrimSuffix(baseOutputPath, ".html")
			return fmt.Sprintf("%s-part%d.html", basePath, partNum)
		}
	}

	// baseOutputPath is just a filename, join with OutputDir
	return filepath.Join(r.cfg.OutputDir, filename)
}

// prepareReportData sets up page data structure
func (r *HtmlReporter) prepareReportData(probeResults []*models.ProbeResult, partInfo string) (models.ReportPageData, error) {
	var pageData models.ReportPageData

	r.setBasicReportInfo(&pageData, partInfo)
	r.processProbeResults(probeResults, &pageData)
	r.assetManager.EmbedAssetsIntoPageData(&pageData, assetsFS, assetsFS, r.cfg.EmbedAssets)

	pageData.FaviconBase64 = r.favicon

	return pageData, nil
}

// setBasicReportInfo sets basic information for the report
func (r *HtmlReporter) setBasicReportInfo(pageData *models.ReportPageData, partInfo string) {
	if r.cfg.ReportTitle != "" {
		pageData.ReportTitle = r.cfg.ReportTitle
	} else {
		pageData.ReportTitle = DefaultReportTitle
	}

	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05")
	pageData.Config = &models.ReporterConfigForTemplate{
		ItemsPerPage: r.getItemsPerPage(),
	}
	pageData.ItemsPerPage = r.getItemsPerPage()
	pageData.EnableDataTables = r.cfg.EnableDataTables
	pageData.ReportPartInfo = partInfo
}

// processProbeResults processes probe results and populates collections
func (r *HtmlReporter) processProbeResults(probeResults []*models.ProbeResult, pageData *models.ReportPageData) {
	var displayResults []models.ProbeResultDisplay
	statusCodes := make(map[int]struct{})
	contentTypes := make(map[string]struct{})
	techs := make(map[string]struct{})
	rootTargetsEncountered := make(map[string]struct{})

	for _, pr := range probeResults {
		displayPr := models.ToProbeResultDisplay(*pr)
		r.ensureRootTargetURL(pr, &displayPr)

		displayResults = append(displayResults, displayPr)
		r.updateCountsAndCollections(*pr, pageData, statusCodes, contentTypes, techs)

		if displayPr.RootTargetURL != "" {
			rootTargetsEncountered[displayPr.RootTargetURL] = struct{}{}
		}
	}

	r.finalizePageData(pageData, displayResults, statusCodes, contentTypes, techs, rootTargetsEncountered)
}

// ensureRootTargetURL ensures RootTargetURL is properly set
func (r *HtmlReporter) ensureRootTargetURL(pr *models.ProbeResult, displayPr *models.ProbeResultDisplay) {
	if displayPr.RootTargetURL == "" {
		if pr.RootTargetURL != "" {
			displayPr.RootTargetURL = pr.RootTargetURL
		} else {
			displayPr.RootTargetURL = displayPr.InputURL
		}
	}
}

// updateCountsAndCollections updates various statistics and collections
func (r *HtmlReporter) updateCountsAndCollections(pr models.ProbeResult, pageData *models.ReportPageData, statusCodes map[int]struct{}, contentTypes map[string]struct{}, techs map[string]struct{}) {
	pageData.TotalResults++
	if pr.StatusCode >= 200 && pr.StatusCode < 300 {
		pageData.SuccessResults++
	} else {
		pageData.FailedResults++
	}

	if pr.StatusCode > 0 {
		statusCodes[pr.StatusCode] = struct{}{}
	}
	if pr.ContentType != "" {
		contentTypes[pr.ContentType] = struct{}{}
	}
	for _, tech := range pr.Technologies {
		if tech.Name != "" {
			techs[tech.Name] = struct{}{}
		}
	}
}

// finalizePageData sets final collections and data on page data
func (r *HtmlReporter) finalizePageData(pageData *models.ReportPageData, displayResults []models.ProbeResultDisplay, statusCodes map[int]struct{}, contentTypes map[string]struct{}, techs map[string]struct{}, rootTargetsEncountered map[string]struct{}) {
	pageData.ProbeResults = displayResults

	// Convert maps to slices
	for sc := range statusCodes {
		pageData.UniqueStatusCodes = append(pageData.UniqueStatusCodes, sc)
	}
	for ct := range contentTypes {
		pageData.UniqueContentTypes = append(pageData.UniqueContentTypes, ct)
	}
	for t := range techs {
		pageData.UniqueTechnologies = append(pageData.UniqueTechnologies, t)
	}
	for rt := range rootTargetsEncountered {
		pageData.UniqueRootTargets = append(pageData.UniqueRootTargets, rt)
	}

	// Convert ProbeResults to JSON for JavaScript
	if jsonData, err := json.Marshal(displayResults); err != nil {
		r.logger.Error().Err(err).Msg("Failed to marshal probe results to JSON")
		pageData.ProbeResultsJSON = template.JS("[]")
	} else {
		pageData.ProbeResultsJSON = template.JS(jsonData)
	}
}

// executeAndWriteReport executes template and writes to file
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	var htmlBuffer bytes.Buffer
	if err := r.template.Execute(&htmlBuffer, pageData); err != nil {
		r.logger.Error().Err(err).Str("output", outputPath).Msg("Failed to execute template")
		return fmt.Errorf("template execution failed: %w", err)
	}

	if err := os.WriteFile(outputPath, htmlBuffer.Bytes(), FilePermissions); err != nil {
		r.logger.Error().Err(err).Str("output", outputPath).Msg("Failed to write report file")
		return fmt.Errorf("failed to write report to %s: %w", outputPath, err)
	}

	return nil
}

// getItemsPerPage returns configured items per page
func (r *HtmlReporter) getItemsPerPage() int {
	if r.cfg.ItemsPerPage > 0 {
		return r.cfg.ItemsPerPage
	}
	return DefaultItemsPerPage
}
