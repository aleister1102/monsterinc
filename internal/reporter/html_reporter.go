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
	"monsterinc/internal/logger" // Corrected or placeholder logger import
	"monsterinc/internal/models"
)

//go:embed templates/report.html.tmpl
var reportTemplateFS embed.FS

//go:embed assets/css/styles.css
var stylesCSS embed.FS

//go:embed assets/js/report.js
var reportJS embed.FS

// HtmlReporter generates HTML reports from probe results.
// HtmlReporter generates HTML reports from probe results.
type HtmlReporter struct {
	config   *config.ReporterConfig
	logger   logger.Logger
	template *template.Template
}

// NewHtmlReporter creates a new HtmlReporter.
func NewHtmlReporter(cfg *config.ReporterConfig, lg logger.Logger) (*HtmlReporter, error) {
	tmpl, err := template.New("report.html.tmpl").Funcs(template.FuncMap{
		"joinTechnologies": func(techs []string) string {
			return strings.Join(techs, ", ")
		},
		"truncate": func(s string, length int) string {
			if len(s) > length {
				return s[:length-3] + "..."
			}
			return s
		},
		"lower": strings.ToLower,
		"replace": func(input, from, to string) string {
			return strings.ReplaceAll(input, from, to)
		},
	}).ParseFS(reportTemplateFS, "templates/report.html.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}

	return &HtmlReporter{
		config:   cfg,
		logger:   lg,
		template: tmpl,
	}, nil
}

// GenerateReport generates an HTML report from the given probe results.
// It saves the report to the specified outputPath.
func (r *HtmlReporter) GenerateReport(probeResults []models.ProbeResult, outputPath string) error {
	r.logger.Infof("Generating HTML report for %d results to %s", len(probeResults), outputPath)

	if len(probeResults) == 0 {
		r.logger.Info("No probe results to generate report for. Skipping.")
		return nil // FR2: Do not generate report if probeResults is empty
	}

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// It's possible the directory already exists, which is not an error for MkdirAll.
		// However, if it fails for other reasons (e.g. permissions), then it's an error.
		r.logger.Errorf("Failed to create output directory %s: %v", dir, err)
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		r.logger.Errorf("Failed to create output file %s: %v", outputPath, err)
		return fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}
	defer outputFile.Close()

	displayResults := make([]models.ProbeResultDisplay, 0, len(probeResults))
	rootTargetSet := make(map[string]struct{})

	for _, pr := range probeResults {
		// Basic transformation, can be expanded
		techNames := make([]string, len(pr.Technologies))
		for i, tech := range pr.Technologies {
			techNames[i] = tech.Name
		}

		displayResult := models.ProbeResultDisplay{
			InputURL:      pr.InputURL,
			FinalURL:      pr.FinalURL,
			StatusCode:    pr.StatusCode,
			ContentLength: pr.ContentLength,
			ContentType:   pr.ContentType,
			Title:         pr.Title,
			WebServer:     pr.WebServer,
			Technologies:  techNames,
			IPs:           pr.IPs, // Assuming IPs is already []string
			RootTargetURL: pr.RootTargetURL,
		}
		displayResults = append(displayResults, displayResult)

		if pr.RootTargetURL != "" {
			rootTargetSet[pr.RootTargetURL] = struct{}{}
		}
	}

	rootTargets := make([]string, 0, len(rootTargetSet))
	for target := range rootTargetSet {
		rootTargets = append(rootTargets, target)
	}
	sort.Strings(rootTargets)

	// Define headers for the table (Task 2.2)
	headers := []string{
		"Input URL", "Final URL", "Status Code", "Content Length",
		"Content Type", "Title", "Web Server", "Technologies", "IPs",
	}

	data := models.ReportPageData{
		Title:        fmt.Sprintf("MonsterInc Scan Report - %s", time.Now().Format("2006-01-02")), // More specific title
		Timestamp:    time.Now().Format(time.RFC1123),
		ProbeResults: displayResults, // Still pass this for contexts where JS might not run, or for simplicity if <noscript> required
		Headers:      headers,
		RootTargets:  rootTargets,
		ItemsPerPage: r.config.ItemsPerPage,
		TotalResults: len(displayResults),
		// ProbeResultsJSON will be set below
	}

	probeResultsJSONBytes, err := json.Marshal(displayResults)
	if err != nil {
		return fmt.Errorf("failed to marshal probe results to JSON: %w", err)
	}
	data.ProbeResultsJSON = template.JS(probeResultsJSONBytes)

	if r.config.EmbedAssets {
		// bcssBytes, err := bootstrapCSS.ReadFile("assets/css/bootstrap.min.css")
		// if err != nil {
		// 	return fmt.Errorf("failed to read bootstrap.min.css: %w", err)
		// }
		// data.BootstrapCSS = template.CSS(bcssBytes)

		scssBytes, err := stylesCSS.ReadFile("assets/css/styles.css")
		if err != nil {
			return fmt.Errorf("failed to read styles.css: %w", err)
		}
		data.StaticCSS = template.CSS(scssBytes)

		// jqBytes, err := jqueryJS.ReadFile("assets/js/jquery.min.js")
		// if err != nil {
		// 	return fmt.Errorf("failed to read jquery.min.js: %w", err)
		// }
		// data.JqueryJS = template.JS(jqBytes)

		// bjsBytes, err := bootstrapJS.ReadFile("assets/js/bootstrap.bundle.min.js")
		// if err != nil {
		// 	return fmt.Errorf("failed to read bootstrap.bundle.min.js: %w", err)
		// }
		// data.BootstrapJS = template.JS(bjsBytes)

		// dtBytes, err := dataTablesJS.ReadFile("assets/js/jquery.dataTables.min.js") // Commented out
		// if err != nil { // Commented out
		// 	// If DataTables is also CDN, this block would be removed/commented too // Commented out
		// 	return fmt.Errorf("failed to read jquery.dataTables.min.js: %w", err) // Commented out
		// } // Commented out
		// data.DataTablesJS = template.JS(dtBytes) // Commented out

		rjsBytes, err := reportJS.ReadFile("assets/js/report.js")
		if err != nil {
			return fmt.Errorf("failed to read report.js: %w", err)
		}
		data.StaticJS = template.JS(rjsBytes)
	} // Else, the template should be structured to link to these assets relatively e.g. "assets/css/styles.css"

	var buf bytes.Buffer
	if err := r.template.Execute(&buf, data); err != nil {
		r.logger.Errorf("Failed to execute template: %v", err)
		return fmt.Errorf("failed to execute template: %w", err)
	}

	_, err = outputFile.Write(buf.Bytes())
	if err != nil {
		r.logger.Errorf("Failed to write report to file: %v", err)
		return fmt.Errorf("failed to write report to file: %w", err)
	}

	r.logger.Infof("Successfully generated HTML report: %s", outputPath)
	return nil
}
