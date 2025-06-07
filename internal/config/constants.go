package config

const (
	// Reporter Defaults
	DefaultReporterOutputDir    = "reports/scan"
	DefaultReporterItemsPerPage = 25
	DefaultReporterEmbedAssets  = true

	// Crawler Defaults
	DefaultCrawlerUserAgent             = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	DefaultCrawlerRequestTimeoutSecs    = 20
	DefaultCrawlerMaxConcurrentRequests = 10
	DefaultCrawlerMaxDepth              = 5
	DefaultCrawlerRespectRobotsTxt      = true

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
