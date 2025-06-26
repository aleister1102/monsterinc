package reporter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/monsterinc/httpx"
)

// buildOutputPath constructs the output file path
func (r *HtmlReporter) buildOutputPath(baseOutputPath string, partNum, totalParts int) string {
	var filename string
	if totalParts == 1 {
		filename = fmt.Sprintf("%s.html", baseOutputPath)
	} else {
		filename = fmt.Sprintf("%s-part%d.html", baseOutputPath, partNum)
	}
	if filepath.IsAbs(baseOutputPath) || strings.Contains(baseOutputPath, string(filepath.Separator)) {
		if totalParts == 1 {
			if !strings.HasSuffix(baseOutputPath, ".html") {
				return baseOutputPath + ".html"
			}
			return baseOutputPath
		} else {
			basePath := strings.TrimSuffix(baseOutputPath, ".html")
			return fmt.Sprintf("%s-part%d.html", basePath, partNum)
		}
	}
	return filepath.Join(r.cfg.OutputDir, filename)
}

// prepareReportData sets up page data structure
func (r *HtmlReporter) prepareReportData(probeResults []*httpx.ProbeResult, secretFindings []models.SecretFinding, partInfo string) (*models.ReportPageData, error) {
	pageData := &models.ReportPageData{}

	r.setBasicReportInfo(pageData, partInfo)
	pageData.SecretFindings = secretFindings
	r.processProbeResults(probeResults, pageData)

	if jsonData, err := r.serializeTableData(pageData.ProbeResults); err == nil {
		pageData.ProbeResultsJSON = template.JS(jsonData)
	} else {
		r.logger.Error().Err(err).Msg("Failed to serialize probe results to JSON for template")
		pageData.ProbeResultsJSON = template.JS("[]")
	}

	return pageData, nil
}

// setBasicReportInfo sets basic information for the report
func (r *HtmlReporter) setBasicReportInfo(pageData *models.ReportPageData, partInfo string) {
	pageData.ReportTitle = r.buildPageTitle(partInfo)
	pageData.GeneratedAt = time.Now().Format("2006-01-02 15:04:05")
	pageData.Config = &models.ReporterConfigForTemplate{
		ItemsPerPage: r.getItemsPerPage(),
	}
	pageData.ItemsPerPage = r.getItemsPerPage()
	pageData.EnableDataTables = r.cfg.EnableDataTables
	pageData.ReportPartInfo = partInfo
	pageData.FaviconBase64 = r.favicon
}

// processProbeResults processes probe results and populates collections
func (r *HtmlReporter) processProbeResults(probeResults []*httpx.ProbeResult, pageData *models.ReportPageData) {
	hostnames := make(map[string]bool)
	statusCodes := make(map[int]bool)
	contentTypes := make(map[string]bool)
	technologies := make(map[string]bool)
	urlStatuses := make(map[string]bool)

	secretsMap := make(map[string][]models.SecretFinding)
	for _, sf := range pageData.SecretFindings {
		secretsMap[sf.SourceURL] = append(secretsMap[sf.SourceURL], sf)
	}

	displayResults := make([]models.ProbeResultDisplay, len(probeResults))
	for i, pr := range probeResults {
		if pr == nil {
			continue
		}
		displayResult := models.ToProbeResultDisplay(*pr)

		// Try to match secrets by multiple URL variations
		var secrets []models.SecretFinding
		if foundSecrets, ok := secretsMap[pr.InputURL]; ok {
			secrets = foundSecrets
		} else if foundSecrets, ok := secretsMap[pr.FinalURL]; ok {
			secrets = foundSecrets
		}
		displayResult.SecretFindings = secrets

		displayResults[i] = displayResult
		r.collectFilterData(*pr, hostnames, statusCodes, contentTypes, technologies, urlStatuses)
	}

	pageData.ProbeResults = displayResults
	r.sortAndAssignFilterData(pageData, hostnames, statusCodes, contentTypes, technologies, urlStatuses)
}

// TestProcessProbeResults is a test-only exported function to allow testing
// the unexported processProbeResults function.
func (r *HtmlReporter) TestProcessProbeResults(probeResults []*httpx.ProbeResult, pageData *models.ReportPageData) {
	r.processProbeResults(probeResults, pageData)
}

func (r *HtmlReporter) shouldIncludeHostnameInFilter(pr *httpx.ProbeResult) bool {
	if pr.Error != "" && pr.StatusCode == 0 {
		return false
	}
	if pr.StatusCode >= 200 && pr.StatusCode < 500 {
		return true
	}
	if pr.Title != "" || len(pr.Technologies) > 0 || pr.ContentType != "" {
		return true
	}
	if pr.StatusCode == 500 || pr.StatusCode == 502 || pr.StatusCode == 503 {
		return true
	}
	return false
}

func (r *HtmlReporter) extractHostnameFromURL(urlStr string) string {
	if hostname, err := urlhandler.ExtractHostname(urlStr); err == nil {
		return hostname
	}
	return ""
}

// executeAndWriteReport executes template and writes to file
func (r *HtmlReporter) executeAndWriteReport(pageData models.ReportPageData, outputPath string) error {
	// Embed assets into page data if configured
	if r.assetManager != nil && r.cfg.EmbedAssets {
		r.assetManager.EmbedAssetsIntoPageDataWithPaths(&pageData, assetsFS, assetsFS, embeddedCSSPath, embeddedJSPath, r.cfg.EmbedAssets)
	}

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

func (r *HtmlReporter) buildPageTitle(partInfo string) string {
	if r.cfg.ReportTitle != "" {
		return r.cfg.ReportTitle
	}
	return DefaultReportTitle
}

func (r *HtmlReporter) serializeTableData(probeResults []models.ProbeResultDisplay) (string, error) {
	bytes, err := json.Marshal(probeResults)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func (r *HtmlReporter) collectFilterData(pr httpx.ProbeResult, hostnames map[string]bool, statusCodes map[int]bool, contentTypes map[string]bool, technologies map[string]bool, urlStatuses map[string]bool) {
	if r.shouldIncludeHostnameInFilter(&pr) {
		hostname := r.extractHostnameFromURL(pr.InputURL)
		if hostname != "" {
			hostnames[hostname] = true
		}
	}

	if pr.StatusCode > 0 {
		statusCodes[pr.StatusCode] = true
	}
	if pr.ContentType != "" {
		contentTypes[pr.ContentType] = true
	}
	for _, tech := range pr.Technologies {
		if tech.Name != "" {
			technologies[tech.Name] = true
		}
	}
	if pr.URLStatus != "" {
		urlStatuses[pr.URLStatus] = true
	}
}

func (r *HtmlReporter) sortAndAssignFilterData(pageData *models.ReportPageData, hostnames map[string]bool, statusCodes map[int]bool, contentTypes map[string]bool, technologies map[string]bool, urlStatuses map[string]bool) {
	hostnamesSlice := make([]string, 0, len(hostnames))
	for hostname := range hostnames {
		hostnamesSlice = append(hostnamesSlice, hostname)
	}
	sort.Strings(hostnamesSlice)
	pageData.UniqueHostnames = hostnamesSlice

	statusCodesSlice := make([]int, 0, len(statusCodes))
	for sc := range statusCodes {
		statusCodesSlice = append(statusCodesSlice, sc)
	}
	sort.Ints(statusCodesSlice)
	pageData.UniqueStatusCodes = statusCodesSlice

	contentTypesSlice := make([]string, 0, len(contentTypes))
	for ct := range contentTypes {
		contentTypesSlice = append(contentTypesSlice, ct)
	}
	sort.Strings(contentTypesSlice)
	pageData.UniqueContentTypes = contentTypesSlice

	technologiesSlice := make([]string, 0, len(technologies))
	for tech := range technologies {
		technologiesSlice = append(technologiesSlice, tech)
	}
	sort.Strings(technologiesSlice)
	pageData.UniqueTechnologies = technologiesSlice

	urlStatusesSlice := make([]string, 0, len(urlStatuses))
	for us := range urlStatuses {
		urlStatusesSlice = append(urlStatusesSlice, us)
	}
	sort.Strings(urlStatusesSlice)
	pageData.UniqueURLStatuses = urlStatusesSlice
}
