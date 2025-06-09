package config

// LogConfig defines configuration for logging
type LogConfig struct {
	LogFile       string `json:"log_file,omitempty" yaml:"log_file,omitempty" validate:"omitempty,filepath"`
	LogFormat     string `json:"log_format,omitempty" yaml:"log_format,omitempty" validate:"omitempty,logformat"`
	LogLevel      string `json:"log_level,omitempty" yaml:"log_level,omitempty" validate:"omitempty,loglevel"`
	MaxLogBackups int    `json:"max_log_backups,omitempty" yaml:"max_log_backups,omitempty"`
	MaxLogSizeMB  int    `json:"max_log_size_mb,omitempty" yaml:"max_log_size_mb,omitempty"`
}

// NewDefaultLogConfig creates default log configuration
func NewDefaultLogConfig() LogConfig {
	return LogConfig{
		LogFile:       DefaultLogFile,
		LogFormat:     DefaultLogFormat,
		LogLevel:      DefaultLogLevel,
		MaxLogBackups: DefaultMaxLogBackups,
		MaxLogSizeMB:  DefaultMaxLogSizeMB,
	}
}
