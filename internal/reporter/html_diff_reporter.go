package reporter

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/urlhandler"

	"io/fs"

	"encoding/base64"

	"github.com/rs/zerolog"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

//go:embed assets/img/favicon.ico
var faviconICODiff []byte

const (
	DefaultDiffReportDir       = "reports/diff"
	DefaultDiffReportAssetsDir = "reports/diff/assets"
)

// FileHistoryStore defines an interface for accessing file history records.
// This avoids a direct dependency on the concrete ParquetFileHistoryStore and facilitates testing.
type FileHistoryStore interface {
	GetAllRecordsWithDiff() ([]*models.FileHistoryRecord, error)
	GetAllLatestDiffResultsForURLs(urls []string) (map[string]*models.ContentDiffResult, error)
	// GetLatestRecordsWithDiffForHost(host string) ([]*models.FileHistoryRecord, error) //  Potentially more granular
}

// HtmlDiffReporter is responsible for generating HTML reports for content differences.
type HtmlDiffReporter struct {
	// cfg          *config.ReporterConfig // No longer needed directly for OutputDir
	logger       zerolog.Logger
	historyStore FileHistoryStore // Added FileHistoryStore
	template     *template.Template
}

// NewHtmlDiffReporter creates a new instance of HtmlDiffReporter.
// The ReporterConfig is no longer used for OutputDir by this reporter.
func NewHtmlDiffReporter(_ *config.ReporterConfig, logger zerolog.Logger, historyStore FileHistoryStore) (*HtmlDiffReporter, error) {
	// ReporterConfig (cfg) is passed but OutputDir from it is NOT used for diff reports.
	// Diff reports are always in DefaultDiffReportDir.

	if historyStore == nil {
		logger.Warn().Msg("HistoryStore is nil in NewHtmlDiffReporter. Aggregated reports will not be available.")
	}

	// Log current working directory for debugging
	if wd, err := os.Getwd(); err == nil {
		logger.Info().Str("working_directory", wd).Str("target_dir", DefaultDiffReportDir).Msg("Creating diff report directory.")
	}

	// Ensure the dedicated diff report output directory exists
	if err := os.MkdirAll(DefaultDiffReportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create diff report output directory %s: %w", DefaultDiffReportDir, err)
	}

	// Ensure the dedicated assets directory for diff reports exists
	if err := os.MkdirAll(DefaultDiffReportAssetsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create diff report assets directory %s: %w", DefaultDiffReportAssetsDir, err)
	}

	// Copy assets to the dedicated diff assets directory
	if err := copyEmbedDir(assetsFS, "assets", DefaultDiffReportAssetsDir); err != nil {
		logger.Warn().Err(err).Msg("Failed to copy assets for HTML diff reporter to " + DefaultDiffReportAssetsDir)
		// Continue without assets if copying fails, or handle more strictly
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"prettyJson": func(b []byte) template.HTML {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, b, "", "  "); err != nil {
				return template.HTML("Error pretty printing JSON")
			}
			return template.HTML(prettyJSON.String())
		},
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
		"operationToString": func(op models.DiffOperation) string {
			switch op {
			case models.DiffDelete:
				return "Delete"
			case models.DiffInsert:
				return "Insert"
			case models.DiffEqual:
				return "Equal"
			default:
				return "Unknown"
			}
		},
		"replaceNewlinesWithBR": func(s string) template.HTML {
			return template.HTML(strings.ReplaceAll(s, "\n", "<br>"))
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
	}).ParseFS(templatesFS, "templates/diff_report.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML diff template: %w", err)
	}
	logger.Info().Str("defined_templates", tmpl.DefinedTemplates()).Msg("HTML diff template parsed successfully")

	return &HtmlDiffReporter{
		// cfg:          cfg, // Not storing anymore
		logger:       logger,
		historyStore: historyStore, // Store injected historyStore
		template:     tmpl,
	}, nil
}

// copyEmbedDir copies a directory from an embed.FS to the filesystem.
func copyEmbedDir(efs embed.FS, srcDir, destDir string) error {
	return fs.WalkDir(efs, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate the destination path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}
		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			// Create directory if it doesn't exist
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
		} else {
			// Read file content from embed.FS
			data, err := efs.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read embedded file %s: %w", path, err)
			}
			// Write file to destination
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", destPath, err)
			}
		}
		return nil
	})
}

// GenerateDiffReport generates a single HTML report page containing diffs for multiple URLs.
// It now fetches only the latest diff for currently monitored URLs.
func (r *HtmlDiffReporter) GenerateDiffReport(monitoredURLs []string) (string, error) {
	r.logger.Info().Strs("monitored_urls", monitoredURLs).Int("monitored_count", len(monitoredURLs)).Msg("Generating aggregated HTML diff report for monitored URLs.")

	if r.historyStore == nil {
		r.logger.Error().Msg("HistoryStore is not available in HtmlDiffReporter. Cannot generate aggregated diff report.")
		return "", errors.New("historyStore is not configured for HtmlDiffReporter")
	}

	latestDiffsMap, err := r.historyStore.GetAllLatestDiffResultsForURLs(monitoredURLs)
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to get latest diff results for monitored URLs from history store.")
		return "", fmt.Errorf("failed to get latest diff results: %w", err)
	}

	r.logger.Info().Int("diff_results_retrieved", len(latestDiffsMap)).Int("monitored_urls_requested", len(monitoredURLs)).Msg("Retrieved latest diff results from history store.")

	// Process the diff results into displayable format
	var diffResultsDisplay []models.DiffResultDisplay
	for url, diffResult := range latestDiffsMap {
		if diffResult == nil {
			continue
		}

		if diffResult.IsIdentical {
			continue
		}

		// Calculate summary
		summary := createDiffSummary(diffResult.Diffs)

		display := models.DiffResultDisplay{
			URL:            url,
			Timestamp:      time.UnixMilli(diffResult.Timestamp), // Convert back to time.Time for display
			ContentType:    diffResult.ContentType,
			OldHash:        diffResult.OldHash,
			NewHash:        diffResult.NewHash,
			Summary:        summary,
			DiffHTML:       r.generateDiffHTML(diffResult.Diffs),
			Diffs:          diffResult.Diffs, // Keep raw diffs if needed by template or other logic
			IsIdentical:    diffResult.IsIdentical,
			ErrorMessage:   diffResult.ErrorMessage,
			ExtractedPaths: diffResult.ExtractedPaths,
		}
		diffResultsDisplay = append(diffResultsDisplay, display)
	}

	if len(diffResultsDisplay) == 0 {
		r.logger.Info().Msg("No relevant (non-identical) diffs found for monitored URLs. Aggregated report will not be generated.")
		return "", nil // No report to generate if no diffs
	}

	// Sort by URL for consistent report output
	sort.Slice(diffResultsDisplay, func(i, j int) bool {
		return diffResultsDisplay[i].URL < diffResultsDisplay[j].URL
	})

	// Define a fixed name for the aggregated report, always in DefaultDiffReportDir
	aggregatedReportFilename := "aggregated_diff_report.html"
	outputFilePath := filepath.Join(DefaultDiffReportDir, aggregatedReportFilename) // Use DefaultDiffReportDir

	// Ensure the directory exists (it should have been created by NewHtmlDiffReporter, but double check)
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil {
		r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to create directory for diff report.")
		return "", fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(outputFilePath), err)
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to create diff report file.")
		return "", fmt.Errorf("failed to create file %s: %w", outputFilePath, err)
	}
	defer file.Close()

	pageData := models.DiffReportPageData{
		ReportTitle: "MonsterInc Aggregated Content Diff Report",
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		DiffResults: diffResultsDisplay,
		TotalDiffs:  len(diffResultsDisplay),
		ReportType:  "aggregated",
		// ItemsPerPage and EnableDataTables can be set from config if needed by template for aggregated view
	}

	// Set favicon base64 data
	if len(faviconICODiff) > 0 {
		pageData.FaviconBase64 = base64.StdEncoding.EncodeToString(faviconICODiff)
	}

	// Execute the main template
	if err := r.template.ExecuteTemplate(file, "diff_report.html.tmpl", pageData); err != nil {
		r.logger.Error().Err(err).Msg("Failed to execute template for aggregated diff report.")
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	r.logger.Info().Str("path", outputFilePath).Int("diff_count", len(diffResultsDisplay)).Msg("Successfully generated aggregated HTML diff report.")
	return outputFilePath, nil
}

// GenerateSingleDiffReport generates an HTML report for a single content diff.
// Reports are saved in DefaultDiffReportDir/diffs/
func (r *HtmlDiffReporter) GenerateSingleDiffReport(urlStr string, diffResult *models.ContentDiffResult, oldHash, newHash string, currentContent []byte) (string, error) {
	if diffResult == nil {
		r.logger.Warn().Str("url", urlStr).Msg("Received nil diffResult, cannot generate single diff report.")
		return "", errors.New("diffResult is nil")
	}
	r.logger.Info().Str("url", urlStr).Msg("Generating single HTML diff report.")

	sanitizedURL := urlhandler.SanitizeFilename(urlStr) // Call package function directly
	var reportFilename string
	// Use OldHash and NewHash from diffResult as they are now populated there
	if diffResult.OldHash != "" && diffResult.NewHash != "" {
		reportFilename = fmt.Sprintf("diff_%s_%s_vs_%s_%d.html", sanitizedURL, diffResult.OldHash[:min(8, len(diffResult.OldHash))], diffResult.NewHash[:min(8, len(diffResult.NewHash))], diffResult.Timestamp)
	} else {
		reportFilename = fmt.Sprintf("diff_%s_%d.html", sanitizedURL, diffResult.Timestamp)
	}

	// Single diffs go directly into DefaultDiffReportDir now
	outputFilePath := filepath.Join(DefaultDiffReportDir, reportFilename)

	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0755); err != nil { // Ensures DefaultDiffReportDir exists
		r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to create directory for single diff report.")
		return "", fmt.Errorf("failed to create output directory %s: %w", filepath.Dir(outputFilePath), err)
	}

	file, err := os.Create(outputFilePath)
	if err != nil {
		r.logger.Error().Err(err).Str("path", outputFilePath).Msg("Failed to create single diff report file.")
		return "", fmt.Errorf("failed to create file %s: %w", outputFilePath, err)
	}
	defer file.Close()

	displayDiff := models.DiffResultDisplay{
		URL:            urlStr,
		Timestamp:      time.UnixMilli(diffResult.Timestamp),
		ContentType:    diffResult.ContentType,
		OldHash:        diffResult.OldHash, // Populate from diffResult
		NewHash:        diffResult.NewHash, // Populate from diffResult
		Summary:        createDiffSummary(diffResult.Diffs),
		DiffHTML:       r.generateDiffHTML(diffResult.Diffs),
		Diffs:          diffResult.Diffs, // Keep raw diffs
		IsIdentical:    diffResult.IsIdentical,
		ErrorMessage:   diffResult.ErrorMessage,
		FullContent:    string(currentContent), // Add current content
		ExtractedPaths: diffResult.ExtractedPaths,
	}

	pageData := models.DiffReportPageData{
		ReportTitle: fmt.Sprintf("Content Diff Report: %s", urlStr),
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05 MST"),
		DiffResults: []models.DiffResultDisplay{displayDiff}, // Use DiffResults
		TotalDiffs:  1,
		ReportType:  "single",
	}

	// Set favicon base64 data
	if len(faviconICODiff) > 0 {
		pageData.FaviconBase64 = base64.StdEncoding.EncodeToString(faviconICODiff)
	}

	if err := r.template.ExecuteTemplate(file, "diff_report.html.tmpl", pageData); err != nil {
		r.logger.Error().Err(err).Msg("Failed to execute template for single diff report.")
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	r.logger.Info().Str("path", outputFilePath).Msg("Successfully generated single HTML diff report.")
	return outputFilePath, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SanitizeURLForFilename replaces characters in a URL that are not safe for filenames.
// ... existing code ...

// generateDiffHTML generates the HTML representation of the diffs.
func (r *HtmlDiffReporter) generateDiffHTML(diffs []models.ContentDiff) template.HTML {
	var htmlBuilder strings.Builder // Renamed to avoid conflict with html package
	for _, d := range diffs {
		// Escape HTML characters in diff text to prevent XSS and rendering issues
		escapedText := template.HTMLEscapeString(d.Text)

		// white-space: pre-wrap; in CSS will handle newlines correctly, so no need to replace with <br>
		// text := strings.ReplaceAll(d.Text, "\\n", "<br>") // Ensure newlines are rendered in HTML - REMOVED

		switch d.Operation {
		case models.DiffInsert:
			htmlBuilder.WriteString(fmt.Sprintf(`<ins style="background:#e6ffe6; text-decoration: none;">%s</ins>`, escapedText))
		case models.DiffDelete:
			htmlBuilder.WriteString(fmt.Sprintf(`<del style="background:#f8d7da; text-decoration: none;">%s</del>`, escapedText))
		case models.DiffEqual:
			htmlBuilder.WriteString(escapedText) // Directly use escaped text
		}
	}
	return template.HTML(htmlBuilder.String())
}

// createDiffSummary creates a textual summary of the diff.
func createDiffSummary(diffs []models.ContentDiff) string {
	insertions := 0
	deletions := 0
	for _, d := range diffs {
		if d.Operation == models.DiffInsert {
			insertions++
		} else if d.Operation == models.DiffDelete {
			deletions++
		}
	}
	if insertions == 0 && deletions == 0 {
		return "No textual changes detected."
	}
	return fmt.Sprintf("%d insertions (+), %d deletions (-).", insertions, deletions)
}

// templateFunctions provides helper functions accessible within the HTML template.
