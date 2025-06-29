package config

// ReporterConfig defines configuration for generating reports
type ReporterConfig struct {
	DefaultItemsPerPage          int    `json:"default_items_per_page,omitempty" yaml:"default_items_per_page,omitempty"`
	EmbedAssets                  bool   `json:"embed_assets" yaml:"embed_assets"`
	EnableDataTables             bool   `json:"enable_data_tables" yaml:"enable_data_tables"`
	ItemsPerPage                 int    `json:"items_per_page,omitempty" yaml:"items_per_page,omitempty" validate:"omitempty,min=1"`
	MaxProbeResultsPerReportFile int    `mapstructure:"max_probe_results_per_report_file" json:"max_probe_results_per_report_file,omitempty" yaml:"max_probe_results_per_report_file,omitempty"`
	OutputDir                    string `json:"output_dir,omitempty" yaml:"output_dir,omitempty" validate:"omitempty,dirpath"`
	ReportTitle                  string `json:"report_title,omitempty" yaml:"report_title,omitempty"`
}

// NewDefaultReporterConfig creates default reporter configuration
func NewDefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		EmbedAssets:                  DefaultReporterEmbedAssets,
		EnableDataTables:             true,
		ItemsPerPage:                 DefaultReporterItemsPerPage,
		MaxProbeResultsPerReportFile: 1000, // Default to 1000 results per file
		OutputDir:                    DefaultReporterOutputDir,
		ReportTitle:                  "MonsterInc Scan Report",
	}
}
