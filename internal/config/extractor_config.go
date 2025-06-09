package config

// ExtractorConfig defines configuration for path extraction
type ExtractorConfig struct {
	Allowlist     []string `json:"allowlist,omitempty" yaml:"allowlist,omitempty"`
	CustomRegexes []string `json:"custom_regexes,omitempty" yaml:"custom_regexes,omitempty"`
	Denylist      []string `json:"denylist,omitempty" yaml:"denylist,omitempty"`
}

// NewDefaultExtractorConfig creates default extractor configuration
func NewDefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{
		CustomRegexes: []string{},
		Allowlist:     []string{},
		Denylist:      []string{},
	}
}
