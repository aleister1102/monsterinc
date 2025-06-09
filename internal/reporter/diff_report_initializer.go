package reporter

import (
	"embed"
	"fmt"
	"html/template"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/rs/zerolog"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed assets/*
var assetsFS embed.FS

//go:embed assets/img/favicon.ico
var faviconICODiff []byte

// FileHistoryStore defines an interface for accessing file history records.
// This avoids a direct dependency on the concrete ParquetFileHistoryStore and facilitates testing.
type FileHistoryStore interface {
	GetAllRecordsWithDiff() ([]*models.FileHistoryRecord, error)
	GetAllLatestDiffResultsForURLs(urls []string) (map[string]*models.ContentDiffResult, error)
	// GetLatestRecordsWithDiffForHost(host string) ([]*models.FileHistoryRecord, error) //  Potentially more granular
}

// HtmlDiffReporter creates HTML reports for content differences (refactored version)
type HtmlDiffReporter struct {
	logger       zerolog.Logger
	historyStore FileHistoryStore
	template     *template.Template
	assetManager *AssetManager
	directoryMgr *DirectoryManager
	diffUtils    *DiffUtils
	config       *config.MonitorConfig
}

// NewHtmlDiffReporter creates a new instance of NewHtmlDiffReporter
func NewHtmlDiffReporter(logger zerolog.Logger, historyStore FileHistoryStore, monitorConfig *config.MonitorConfig) (*HtmlDiffReporter, error) {
	if historyStore == nil {
		logger.Warn().Msg("HistoryStore is nil in NewHtmlDiffReporter. Aggregated reports will not be available.")
	}

	if monitorConfig == nil {
		logger.Warn().Msg("MonitorConfig is nil, using default values for diff reporter")
		monitorConfig = &config.MonitorConfig{}
	}

	reporter := &HtmlDiffReporter{
		logger:       logger,
		historyStore: historyStore,
		assetManager: NewAssetManager(logger),
		directoryMgr: NewDirectoryManager(logger),
		diffUtils:    NewDiffUtils(),
		config:       monitorConfig,
	}

	if err := reporter.initializeDirectories(); err != nil {
		return nil, err
	}

	if err := reporter.initializeTemplate(); err != nil {
		return nil, err
	}

	if err := reporter.copyAssets(); err != nil {
		logger.Warn().Err(err).Msg("Failed to copy assets for HTML diff reporter")
	}

	return reporter, nil
}

// initializeDirectories initializes required directories
func (r *HtmlDiffReporter) initializeDirectories() error {
	r.directoryMgr.LogWorkingDirectory(DefaultDiffReportDir)
	return r.directoryMgr.EnsureDiffReportDirectories()
}

// initializeTemplate initializes template with functions
func (r *HtmlDiffReporter) initializeTemplate() error {
	tmpl, err := template.New("").Funcs(GetDiffTemplateFunctions()).ParseFS(templateFS, "templates/diff_report.html.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse HTML diff template: %w", err)
	}

	r.logger.Info().Str("defined_templates", tmpl.DefinedTemplates()).Msg("HTML diff template parsed successfully")
	r.template = tmpl
	return nil
}

// copyAssets copies embedded assets to assets directory
func (r *HtmlDiffReporter) copyAssets() error {
	return r.assetManager.CopyEmbedDir(assetsFS, "assets", DefaultDiffReportAssetsDir)
}
