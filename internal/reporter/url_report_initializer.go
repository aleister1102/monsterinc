package reporter

import (
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

//go:embed templates/report_client_side.html.tmpl
var defaultTemplate embed.FS

//go:embed assets/img/favicon.ico
var faviconICO []byte

const (
	defaultHtmlReportTemplateName = "report_client_side.html.tmpl"
	embeddedCSSPath               = "assets/css/report_client_side.css"
	embeddedJSPath                = "assets/js/report_client_side.js"
)

// HtmlReporter generates HTML reports from scan results
type HtmlReporter struct {
	cfg          *config.ReporterConfig
	logger       zerolog.Logger
	template     *template.Template
	favicon      string
	assetManager *AssetManager
	directoryMgr *DirectoryManager
}

// NewHtmlReporter creates a new HtmlReporter instance
func NewHtmlReporter(cfg *config.ReporterConfig, appLogger zerolog.Logger) (*HtmlReporter, error) {
	reporter := &HtmlReporter{
		cfg:          cfg,
		logger:       appLogger.With().Str("component", "HtmlReporter").Logger(),
		assetManager: NewAssetManager(appLogger),
		directoryMgr: NewDirectoryManager(appLogger),
	}

	if err := reporter.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize reporter: %w", err)
	}

	if err := reporter.initializeOutputDirectory(); err != nil {
		return nil, err
	}

	// Copy assets to output directory if not embedding
	if !cfg.EmbedAssets {
		if err := reporter.copyAssets(); err != nil {
			reporter.logger.Warn().Err(err).Msg("Failed to copy assets for HTML reporter")
		}
	}

	return reporter, nil
}

// initializeOutputDirectory ensures output directory exists
func (r *HtmlReporter) initializeOutputDirectory() error {
	if r.cfg.OutputDir == "" {
		r.cfg.OutputDir = config.DefaultReporterOutputDir
	}

	return r.directoryMgr.EnsureOutputDirectories(r.cfg.OutputDir)
}

// setupTemplate initializes the HTML template with function map
func (r *HtmlReporter) setupTemplate() error {
	return r.loadEmbeddedTemplate(template.New("report"))
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

// getItemsPerPage returns configured items per page
func (r *HtmlReporter) getItemsPerPage() int {
	if r.cfg.ItemsPerPage > 0 {
		return r.cfg.ItemsPerPage
	}
	return DefaultItemsPerPage
}

// copyAssets copies embedded assets to output directory
func (r *HtmlReporter) copyAssets() error {
	assetsDir := r.cfg.OutputDir + "/assets"
	return r.assetManager.CopyEmbedDir(assetsFS, "assets", assetsDir)
}

func (r *HtmlReporter) initialize() error {
	r.initializeFavicon()
	return r.setupTemplate()
}
