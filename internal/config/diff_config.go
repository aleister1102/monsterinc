package config

// DiffConfig defines configuration for diffing
type DiffConfig struct {
	PreviousScanLookbackDays int `json:"previous_scan_lookback_days,omitempty" yaml:"previous_scan_lookback_days,omitempty"`
}

// NewDefaultDiffConfig creates default diff configuration
func NewDefaultDiffConfig() DiffConfig {
	return DiffConfig{
		PreviousScanLookbackDays: DefaultDiffPreviousScanLookbackDays,
	}
}
