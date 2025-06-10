package reporter

import (
	"embed"
	"encoding/base64"
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
	funcMap := r.createTemplateFunctionMap()
	tmpl := template.New(defaultHtmlReportTemplateName).Funcs(funcMap)

	if r.cfg.TemplatePath != "" {
		return r.loadCustomTemplate()
	}

	return r.loadEmbeddedTemplate(tmpl)
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
