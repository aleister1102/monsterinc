package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/models"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"monsterinc/internal/scheduler"
	"monsterinc/internal/notifier"
	"os"
	"path/filepath"
	"strings"
	"time"
	"context"
	"net/http"

	"github.com/rs/zerolog"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Flags
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	urlListFileAlias := flag.String("u", "", "Alias for -urlfile")
	globalConfigFile := flag.String("globalconfig", "config.yaml", "Path to the global YAML/JSON configuration file.")
	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	flag.Parse()

	// Check required --mode
	if *modeFlag == "" {
		fmt.Println("[FATAL] --mode argument is required (onetime or automated)")
		os.Exit(1)
	}

	// urlfile alias logic
	if *urlListFile == "" && *urlListFileAlias != "" {
		*urlListFile = *urlListFileAlias
	}

	// Load Global Configuration
	log.Printf("[INFO] Main: Loading global configuration from %s", *globalConfigFile)
	gCfg, err := config.LoadGlobalConfig(*globalConfigFile)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not load global config from '%s': %v", *globalConfigFile, err)
	}

	// Override mode if --mode flag is set (takes precedence over config file)
	if *modeFlag != "" {
		gCfg.Mode = *modeFlag
	}

	// Ensure the reporter output directory exists before validation (if validator checks for existence)
	if gCfg.ReporterConfig.OutputDir != "" {
		if err := os.MkdirAll(gCfg.ReporterConfig.OutputDir, 0755); err != nil {
			log.Fatalf("[FATAL] Main: Could not create report output directory '%s' before validation: %v", gCfg.ReporterConfig.OutputDir, err)
		}
	}

	// Validate the loaded configuration
	if err := config.ValidateConfig(gCfg); err != nil {
		log.Fatalf("[FATAL] Main: Configuration validation failed: %v", err)
	}

	// --- Initialize Logger (Example - you might have a dedicated logger package) ---
	// Based on gCfg.LogConfig, set up your logger. For now, using standard log.
	// appLogger := log.Default() // Using standard logger for all components for now
	// Replace standard logger with zerolog
	zLogger := setupZeroLogger(gCfg.LogConfig)
	appLogger := zLogger // Keep appLogger for now for compatibility with scheduler, or refactor scheduler
	_ = appLogger        // temp use to avoid unused error

	// --- Initialize Notifier ---
	discordNotifier, err := notifier.NewDiscordNotifier(gCfg.NotificationConfig, zLogger, &http.Client{Timeout: 20 * time.Second})
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier")
	}
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	// Send critical error notification if initialization up to this point failed, for example.
	// For now, we assume config loading itself is a critical point. If it failed, we wouldn't reach here.
	// A more robust critical error handling can be added for other components.

	// Check mode and run appropriate logic
	if gCfg.Mode == "automated" {
		// Automated mode - initialize and run scheduler
		zLogger.Info().Msg("Running in automated mode...")

		// Pass notificationHelper to the scheduler
		schedulerInstance, err := scheduler.NewScheduler(gCfg, *urlListFile, zLogger, notificationHelper) // Pass notificationHelper
		if err != nil {
			// Send critical notification before fatal
			errorMessages := []string{fmt.Sprintf("Failed to initialize scheduler: %v", err)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", errorMessages)
			zLogger.Fatal().Err(err).Msg("Failed to initialize scheduler")
		}

		// Start scheduler (this will block until scheduler stops)
		if err := schedulerInstance.Start(); err != nil {
			// Send critical notification before fatal
			errorMessages := []string{fmt.Sprintf("Scheduler error: %v", err)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerRuntime", errorMessages)
			zLogger.Fatal().Err(err).Msg("Scheduler error")
		}

		// Scheduler has stopped
		zLogger.Info().Msg("Automated mode completed.")

	} else {
		// Onetime mode - run single scan
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(gCfg, *urlListFile, zLogger, notificationHelper) // Pass notificationHelper
	}
}

func runOnetimeScan(gCfg *config.GlobalConfig, urlListFile string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper) { // Changed logger type and added notificationHelper
	// --- Initialize ParquetReader & Writer (needed for orchestrator) --- //
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger) // Use zerolog
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger) // Use zerolog
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	// --- Initialize ScanOrchestrator --- //
	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter) // Use zerolog

	// --- Determine Seed URLs --- //
	var seedURLs []string
	if urlListFile != "" {
		appLogger.Info().Str("file", urlListFile).Msg("Reading seed URLs")
		file, errFile := os.Open(urlListFile)
		if errFile != nil {
			errorMessages := []string{fmt.Sprintf("Could not open URL list file '%s': %v", urlListFile, errFile)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanURLFileOpen", errorMessages)
			appLogger.Fatal().Err(errFile).Str("file", urlListFile).Msg("Could not open URL list file")
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			urlStr := strings.TrimSpace(scanner.Text())
			if urlStr != "" && (strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://")) {
				seedURLs = append(seedURLs, urlStr)
			}
		}
		file.Close()
		if scanner.Err() != nil {
			errorMessages := []string{fmt.Sprintf("Error reading URL list file '%s': %v", urlListFile, scanner.Err())}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanURLFileRead", errorMessages)
			appLogger.Fatal().Err(scanner.Err()).Str("file", urlListFile).Msg("Error reading URL list file")
		}
		if len(seedURLs) == 0 {
			appLogger.Warn().Str("file", urlListFile).Msg("No valid URLs found in file. Using seeds from config if available.")
		}
	}

	if len(seedURLs) == 0 { // If file was empty or not provided, try config
		if len(gCfg.InputConfig.InputURLs) > 0 {
			appLogger.Info().Int("count", len(gCfg.InputConfig.InputURLs)).Msg("Using seed URLs from global input_config.input_urls")
			seedURLs = gCfg.InputConfig.InputURLs
		} else {
			errorMessages := []string{"No seed URLs provided. Please specify via -urlfile or in input_config.input_urls in the config file."}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanNoSeeds", errorMessages)
			appLogger.Fatal().Msg("No seed URLs provided.")
		}
	}

	appLogger.Info().Int("count", len(seedURLs)).Msg("Starting onetime scan with seed URLs.")

	// --- Send Scan Start Notification ---
	// Use input file path or a generic ID for scanID in onetime mode
	scanIdentifier := "onetime_scan"
	if urlListFile != "" {
		scanIdentifier = filepath.Base(urlListFile)
	} else if len(seedURLs) > 0 {
	    scanIdentifier = seedURLs[0] // Use first URL as an identifier if no file
	    if len(seedURLs) > 1 {
	        scanIdentifier += fmt.Sprintf("_and_%d_more", len(seedURLs)-1)
	    }
	}
	notificationHelper.SendScanStartNotification(context.Background(), scanIdentifier, seedURLs, len(seedURLs))

	// --- Generate Scan Session ID --- //
	scanSessionID := time.Now().Format("20060102-150405")

	// --- Execute Scan Workflow via Orchestrator --- //
	appLogger.Info().Msg("Executing scan workflow via orchestrator...")
	startTime := time.Now() // For duration calculation
	probeResults, urlDiffResults, workflowErr := scanOrchestrator.ExecuteScanWorkflow(seedURLs, scanSessionID)
	scanDuration := time.Since(startTime)

	// --- Prepare Scan Summary for Notification ---
	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanID = scanIdentifier
	summaryData.Targets = seedURLs
	summaryData.TotalTargets = len(seedURLs)
	summaryData.ScanDuration = scanDuration
	// Populate ProbeStats and DiffStats (simplified for now, orchestrator should provide this)
	if probeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(probeResults) // Example, adjust as per actual data
		// This needs more accurate population based on actual success/failure from probeResults
		// For now, assume all discovered are successful for simplicity
		summaryData.ProbeStats.SuccessfulProbes = len(probeResults)
	}
	if urlDiffResults != nil {
		for _, diff := range urlDiffResults {
			summaryData.DiffStats.New += diff.New
			summaryData.DiffStats.Existing += diff.Existing
			summaryData.DiffStats.Old += diff.Old
		}
	}

	if workflowErr != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Fatal().Err(workflowErr).Msg("Scan workflow execution failed")
	}
	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	// Note: The orchestrator.ExecuteScanWorkflow now returns all probe results, including those that might not have been part of a diff.
	// The `updatedProbeResults` logic from the old main.go might need re-evaluation if specific filtering was intended before reporting.
	// For now, we directly use `probeResults` from the orchestrator for the report.

	// --- HTML Report Generation --- //
	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger) // Use zerolog
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		// Try to send notification even if report init fails
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Fatal().Err(err).Msg("Failed to initialize HTML reporter")
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode) // gCfg.Mode will be "onetime"
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	// Create a slice of pointers to models.ProbeResult for the reporter
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	if err := htmlReporter.GenerateReport(probeResultsPtr, urlDiffResults, reportPath); err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to generate HTML report: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Fatal().Err(err).Msg("Failed to generate HTML report")
	}
	appLogger.Info().Str("path", reportPath).Msg("HTML report generated successfully")
	summaryData.ReportPath = reportPath // Add report path for notification attachment
	summaryData.Status = string(models.ScanStatusCompleted)

	// --- Send Scan Completion Notification ---
	notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}

// setupZeroLogger initializes zerolog based on config
func setupZeroLogger(logCfg config.LogConfig) zerolog.Logger {
	var logger zerolog.Logger
	logLevel, err := zerolog.ParseLevel(logCfg.LogLevel)
	if err != nil {
		logLevel = zerolog.InfoLevel // Default to info if parsing fails
		fmt.Fprintf(os.Stderr, "Invalid log level '%s', defaulting to 'info'\n", logCfg.LogLevel)
	}

	if logCfg.LogFormat == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).Level(logLevel).With().Timestamp().Logger()
	} else if logCfg.LogFormat == "json" {
		// TODO: Add file logging support if logCfg.LogFile is set
		logger = zerolog.New(os.Stderr).Level(logLevel).With().Timestamp().Logger()
	} else { // Default to text or console-like if format is unknown
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).Level(logLevel).With().Timestamp().Logger()
		fmt.Fprintf(os.Stderr, "Unknown log format '%s', defaulting to 'console'\n", logCfg.LogFormat)
	}
	return logger
}
