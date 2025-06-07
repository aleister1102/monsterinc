package config

// ReporterConfig defines configuration for generating reports
type ReporterConfig struct {
	DefaultItemsPerPage          int    `json:"default_items_per_page,omitempty" yaml:"default_items_per_page,omitempty"`
	EmbedAssets                  bool   `json:"embed_assets" yaml:"embed_assets"`
	EnableDataTables             bool   `json:"enable_data_tables" yaml:"enable_data_tables"`
	GenerateEmptyReport          bool   `json:"generate_empty_report" yaml:"generate_empty_report"`
	ItemsPerPage                 int    `json:"items_per_page,omitempty" yaml:"items_per_page,omitempty" validate:"omitempty,min=1"`
	MaxProbeResultsPerReportFile int    `mapstructure:"max_probe_results_per_report_file" json:"max_probe_results_per_report_file,omitempty" yaml:"max_probe_results_per_report_file,omitempty"`
	OutputDir                    string `json:"output_dir,omitempty" yaml:"output_dir,omitempty" validate:"omitempty,dirpath"`
	ReportTitle                  string `json:"report_title,omitempty" yaml:"report_title,omitempty"`
	TemplatePath                 string `json:"template_path,omitempty" yaml:"template_path,omitempty"`
}

// NewDefaultReporterConfig creates default reporter configuration
func NewDefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		EmbedAssets:                  DefaultReporterEmbedAssets,
		EnableDataTables:             true,
		GenerateEmptyReport:          false,
		ItemsPerPage:                 DefaultReporterItemsPerPage,
		MaxProbeResultsPerReportFile: 1000, // Default to 1000 results per file
		OutputDir:                    DefaultReporterOutputDir,
		ReportTitle:                  "MonsterInc Scan Report",
		TemplatePath:                 "",
	}
}

// DiffReporterConfig defines configuration for diff reporting
type DiffReporterConfig struct {
	MaxDiffFileSizeMB int `json:"max_diff_file_size_mb,omitempty" yaml:"max_diff_file_size_mb,omitempty"`
}

// NewDefaultDiffReporterConfig creates default diff reporter configuration
func NewDefaultDiffReporterConfig() DiffReporterConfig {
	return DiffReporterConfig{
		MaxDiffFileSizeMB: 10,
	}
}
