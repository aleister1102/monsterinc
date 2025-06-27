package config

const (
	// Reporter Defaults
	DefaultReporterOutputDir    = "reports/scan"
	DefaultReporterItemsPerPage = 25
	DefaultReporterEmbedAssets  = true

	// Crawler Defaults
	DefaultCrawlerRequestTimeoutSecs    = 20
	DefaultCrawlerMaxConcurrentRequests = 10
	DefaultCrawlerMaxDepth              = 5

	// Storage Defaults
	DefaultStorageParquetBasePath  = "database"
	DefaultStorageCompressionCodec = "zstd"

	// Log Defaults
	DefaultLogLevel      = "info"
	DefaultLogFormat     = "console"
	DefaultLogFile       = ""
	DefaultMaxLogSizeMB  = 100
	DefaultMaxLogBackups = 3

	// Diff Defaults
	DefaultDiffPreviousScanLookbackDays = 7

	// Monitor Defaults - using fast path file extensions
	DefaultMonitorJSFileExtensions   = ".js,.jsx,.ts,.tsx"
	DefaultMonitorHTMLFileExtensions = ".html,.htm"

	// Normalizer Defaults
	DefaultNormalizerDefaultScheme = "http" // Example for future use

	// Scheduler Defaults
	DefaultSchedulerScanIntervalMinutes = 10080 // 7 days
	DefaultSchedulerRetryAttempts       = 2
	DefaultSchedulerSQLiteDBPath        = "database/scheduler/scheduler_history.db"
)
