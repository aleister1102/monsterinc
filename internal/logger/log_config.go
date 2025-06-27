package logger

// Default log settings
const (
	DefaultLogFile       = "monsterinc.log"
	DefaultLogFormat     = "console"
	DefaultLogLevel      = "info"
	DefaultMaxLogBackups = 3
	DefaultMaxLogSizeMB  = 100
)

// FileLogConfig defines configuration for logging from a config file
type FileLogConfig struct {
	LogFile       string `json:"log_file,omitempty" yaml:"log_file,omitempty" validate:"omitempty,filepath"`
	LogFormat     string `json:"log_format,omitempty" yaml:"log_format,omitempty" validate:"omitempty,logformat"`
	LogLevel      string `json:"log_level,omitempty" yaml:"log_level,omitempty" validate:"omitempty,loglevel"`
	MaxLogBackups int    `json:"max_log_backups,omitempty" yaml:"max_log_backups,omitempty"`
	MaxLogSizeMB  int    `json:"max_log_size_mb,omitempty" yaml:"max_log_size_mb,omitempty"`
}

// NewDefaultFileLogConfig creates default log configuration
func NewDefaultFileLogConfig() FileLogConfig {
	return FileLogConfig{
		LogFile:       DefaultLogFile,
		LogFormat:     DefaultLogFormat,
		LogLevel:      DefaultLogLevel,
		MaxLogBackups: DefaultMaxLogBackups,
		MaxLogSizeMB:  DefaultMaxLogSizeMB,
	}
}
