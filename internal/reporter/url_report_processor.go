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
)

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
	hostnamesEncountered := make(map[string]struct{})
	urlStatuses := make(map[string]struct{})

	for _, pr := range probeResults {
		displayPr := models.ToProbeResultDisplay(*pr)
		r.ensureRootTargetURL(pr, &displayPr)

		displayResults = append(displayResults, displayPr)
		r.updateCountsAndCollections(*pr, pageData, statusCodes, contentTypes, techs, urlStatuses)

		// Only add hostname to filter if the probe result has meaningful data
		// (successful response or at least some useful information)
		if r.shouldIncludeHostnameInFilter(pr) {
			hostname := r.extractHostnameFromURL(displayPr.InputURL)
			if hostname != "" {
				hostnamesEncountered[hostname] = struct{}{}
			}
		}
	}

	r.finalizePageData(pageData, displayResults, statusCodes, contentTypes, techs, hostnamesEncountered, urlStatuses)
}

// shouldIncludeHostnameInFilter determines if a hostname should be included in the filter dropdown
// Only include hostnames that have meaningful data (successful responses, interesting status codes, etc.)
func (r *HtmlReporter) shouldIncludeHostnameInFilter(pr *models.ProbeResult) bool {
	// Include if:
	// 1. Successful response (2xx, 3xx)
	// 2. Client error that might be interesting (4xx)
	// 3. Has title, technologies, or other useful metadata
	// 4. No major errors in probing

	if pr.Error != "" && pr.StatusCode == 0 {
		// Pure error with no response - skip
		return false
	}

	if pr.StatusCode >= 200 && pr.StatusCode < 500 {
		// Any response from 200-499 is potentially interesting
		return true
	}

	if pr.Title != "" || len(pr.Technologies) > 0 || pr.ContentType != "" {
		// Has useful metadata even if status code is not ideal
		return true
	}

	if pr.StatusCode == 500 || pr.StatusCode == 502 || pr.StatusCode == 503 {
		// Server errors might be interesting for security analysis
		return true
	}

	// Skip other cases (timeouts, DNS errors, etc.)
	return false
}

// extractHostnameFromURL extracts hostname from URL for grouping
func (r *HtmlReporter) extractHostnameFromURL(urlStr string) string {
	if hostname, err := urlhandler.ExtractHostname(urlStr); err == nil {
		return hostname
	}
	return ""
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
func (r *HtmlReporter) updateCountsAndCollections(pr models.ProbeResult, pageData *models.ReportPageData, statusCodes map[int]struct{}, contentTypes map[string]struct{}, techs map[string]struct{}, urlStatuses map[string]struct{}) {
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
	if pr.URLStatus != "" {
		urlStatuses[pr.URLStatus] = struct{}{}
	}
}

// finalizePageData sets final collections and data on page data
func (r *HtmlReporter) finalizePageData(pageData *models.ReportPageData, displayResults []models.ProbeResultDisplay, statusCodes map[int]struct{}, contentTypes map[string]struct{}, techs map[string]struct{}, hostnamesEncountered map[string]struct{}, urlStatuses map[string]struct{}) {
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
	for hn := range hostnamesEncountered {
		pageData.UniqueHostnames = append(pageData.UniqueHostnames, hn)
	}
	for us := range urlStatuses {
		pageData.UniqueURLStatuses = append(pageData.UniqueURLStatuses, us)
	}

	// Sort slices for consistent display
	sort.Ints(pageData.UniqueStatusCodes)
	sort.Strings(pageData.UniqueContentTypes)
	sort.Strings(pageData.UniqueTechnologies)
	sort.Strings(pageData.UniqueHostnames)
	sort.Strings(pageData.UniqueURLStatuses)

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
