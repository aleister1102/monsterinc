package config

import "time"

// ProgressConfig contains configuration for progress display
type ProgressConfig struct {
	// DisplayInterval is how often to display progress updates (in seconds)
	DisplayInterval int `json:"display_interval,omitempty" yaml:"display_interval,omitempty" validate:"min=1,max=60"`

	// EnableProgress enables or disables progress display
	EnableProgress bool `json:"enable_progress,omitempty" yaml:"enable_progress,omitempty"`

	// ShowETAEstimation enables or disables ETA calculation and display
	ShowETAEstimation bool `json:"show_eta_estimation,omitempty" yaml:"show_eta_estimation,omitempty"`
}

// NewDefaultProgressConfig creates a new ProgressConfig with default values
func NewDefaultProgressConfig() ProgressConfig {
	return ProgressConfig{
		DisplayInterval:   3,    // 3 seconds default
		EnableProgress:    true, // enabled by default
		ShowETAEstimation: true, // show ETA by default
	}
}

// GetDisplayIntervalDuration returns the display interval as time.Duration
func (pc *ProgressConfig) GetDisplayIntervalDuration() time.Duration {
	return time.Duration(pc.DisplayInterval) * time.Second
}
