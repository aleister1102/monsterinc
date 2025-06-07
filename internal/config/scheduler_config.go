package config

// SchedulerConfig defines configuration for scheduler
type SchedulerConfig struct {
	CycleMinutes  int    `json:"cycle_minutes,omitempty" yaml:"cycle_minutes,omitempty" validate:"min=1"` // in minutes
	RetryAttempts int    `json:"retry_attempts,omitempty" yaml:"retry_attempts,omitempty" validate:"min=0"`
	SQLiteDBPath  string `json:"sqlite_db_path,omitempty" yaml:"sqlite_db_path,omitempty" validate:"required"`
}

// NewDefaultSchedulerConfig creates default scheduler configuration
func NewDefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		CycleMinutes:  DefaultSchedulerScanIntervalMinutes,
		RetryAttempts: DefaultSchedulerRetryAttempts,
		SQLiteDBPath:  DefaultSchedulerSQLiteDBPath,
	}
}
