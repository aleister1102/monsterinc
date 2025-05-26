package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/logger"
	"monsterinc/internal/models"
	"monsterinc/internal/notifier"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"monsterinc/internal/scheduler"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Flags
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	urlListFileAlias := flag.String("u", "", "Alias for -urlfile")
	globalConfigFile := flag.String("globalconfig", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("gc", "", "Alias for -globalconfig")
	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	flag.Parse()

	// Check required --mode
	if *modeFlag == "" {
		log.Fatalln("[FATAL] --mode argument is required (onetime or automated)")
	}

	// urlfile alias logic
	if *urlListFile == "" && *urlListFileAlias != "" {
		*urlListFile = *urlListFileAlias
	}

	// globalconfig alias logic
	if *globalConfigFile == "" && *globalConfigFileAlias != "" {
		*globalConfigFile = *globalConfigFileAlias
	}

	// Load Global Configuration
	log.Println("[INFO] Main: Attempting to load global configuration...")
	gCfg, err := config.LoadGlobalConfig(*globalConfigFile)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not load global config: %v", err)
	}
	if gCfg == nil {
		log.Fatalf("[FATAL] Main: Loaded configuration is nil, though no error was reported. This should not happen.")
	}
	log.Println("[INFO] Main: Global configuration loaded successfully.")

	// Initialize zerolog logger
	zLogger, err := logger.New(gCfg.LogConfig)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not initialize logger: %v", err)
	}
	zLogger.Info().Msg("Logger initialized successfully.")

	// Override mode if --mode flag is set (takes precedence over config file)
	if *modeFlag != "" {
		gCfg.Mode = *modeFlag
		zLogger.Info().Str("mode", gCfg.Mode).Msg("Mode overridden by command line flag.")
	}

	// Ensure the reporter output directory exists before validation (if validator checks for existence)
	if gCfg.ReporterConfig.OutputDir != "" {
		if err := os.MkdirAll(gCfg.ReporterConfig.OutputDir, 0755); err != nil {
			zLogger.Fatal().Err(err).Str("directory", gCfg.ReporterConfig.OutputDir).Msg("Could not create report output directory before validation")
		}
	}

	// Validate the loaded configuration
	if err := config.ValidateConfig(gCfg); err != nil {
		zLogger.Fatal().Err(err).Msg("Configuration validation failed")
	}
	zLogger.Info().Msg("Configuration validated successfully.")

	discordNotifier, err := notifier.NewDiscordNotifier(gCfg.NotificationConfig, zLogger, &http.Client{Timeout: 20 * time.Second})
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier")
	}
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure all paths cancel the context

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		zLogger.Info().Str("signal", sig.String()).Msg("Received interrupt signal, initiating graceful shutdown...")
		// Send interruption notification if helper is available, regardless of mode
		if notificationHelper != nil {
			interruptionSummary := models.GetDefaultScanSummaryData()
			interruptionSummary.ScanID = fmt.Sprintf("%s_mode_interrupted_by_signal", gCfg.Mode)
			interruptionSummary.Status = string(models.ScanStatusFailed) // Or a new status like ScanStatusInterrupted
			interruptionSummary.ErrorMessages = []string{fmt.Sprintf("Application (%s mode) interrupted by signal: %s", gCfg.Mode, sig.String())}
			// Use a new context for this notification as the main one might be cancelling
			notificationCtx, notificationCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer notificationCancel()
			notificationHelper.SendScanCompletionNotification(notificationCtx, interruptionSummary)
			zLogger.Info().Msg("Interruption notification sent.")
		}
		cancel() // Cancel the main context to signal all operations to stop
	}()

	if gCfg.Mode == "automated" {
		zLogger.Info().Msg("Running in automated mode...")
		schedulerInstance, err := scheduler.NewScheduler(gCfg, *urlListFile, zLogger, notificationHelper)
		if err != nil {
			errorMessages := []string{fmt.Sprintf("Failed to initialize scheduler: %v", err)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", errorMessages) // Use original context for this
			zLogger.Fatal().Err(err).Msg("Failed to initialize scheduler")
		}

		if err := schedulerInstance.Start(ctx); err != nil { // Pass context to Start
			if ctx.Err() == context.Canceled {
				zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
			} else {
				errorMessages := []string{fmt.Sprintf("Scheduler error: %v", err)}
				notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerRuntime", errorMessages)
				zLogger.Error().Err(err).Msg("Scheduler error") // Changed to Error to avoid immediate exit if interruption notification is pending
			}
		}
		zLogger.Info().Msg("Automated mode completed or interrupted.")
	} else {
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(ctx, gCfg, *urlListFile, zLogger, notificationHelper) // Pass context
	}

	// Check if the context was cancelled (e.g. by signal)
	if ctx.Err() == context.Canceled {
		zLogger.Info().Msg("Application shutting down due to context cancellation.")
		// Potentially wait for a moment for async operations like notifications to complete if needed
		// time.Sleep(1 * time.Second) // Example: wait for notification to go out
	} else {
		zLogger.Info().Msg("Application finished.")
	}
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFile string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper) {
	// --- Initialize ParquetReader & Writer (needed for orchestrator) --- //
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	// --- Initialize ScanOrchestrator --- //
	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter)

	// --- Determine Seed URLs --- //
	var seedURLs []string
	if urlListFile != "" {
		appLogger.Info().Str("file", urlListFile).Msg("Reading seed URLs")
		file, errFile := os.Open(urlListFile)
		if errFile != nil {
			errorMessages := []string{fmt.Sprintf("Could not open URL list file '%s': %v", urlListFile, errFile)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanURLFileOpen", errorMessages) // Use a non-cancelled context for critical notifications
			appLogger.Error().Err(errFile).Str("file", urlListFile).Msg("Could not open URL list file")
			// Instead of Fatal, let the function return or handle the error to allow cleanup/interruption notification
			return // Or set an error and proceed to cleanup/notification logic at the end
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
			appLogger.Error().Err(scanner.Err()).Str("file", urlListFile).Msg("Error reading URL list file")
			return
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
			noSeedsMsg := "No seed URLs provided. Please specify via -urlfile or in input_config.input_urls in the config file."
			errorMessages := []string{noSeedsMsg}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanNoSeeds", errorMessages)
			appLogger.Error().Msg(noSeedsMsg) // Changed from Fatal
			return
		}
	}

	appLogger.Info().Int("count", len(seedURLs)).Msg("Starting onetime scan with seed URLs.")

	scanIdentifier := "onetime_scan"
	if urlListFile != "" {
		scanIdentifier = filepath.Base(urlListFile)
	} else if len(seedURLs) > 0 {
		scanIdentifier = seedURLs[0]
		if len(seedURLs) > 1 {
			scanIdentifier += fmt.Sprintf("_and_%d_more", len(seedURLs)-1)
		}
	}
	notificationHelper.SendScanStartNotification(ctx, scanIdentifier, seedURLs, len(seedURLs))

	scanSessionID := time.Now().Format("20060102-150405")

	appLogger.Info().Msg("Executing scan workflow via orchestrator...")
	startTime := time.Now()
	// ExecuteScanWorkflow should ideally accept and respect the context
	probeResults, urlDiffResults, workflowErr := scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
	scanDuration := time.Since(startTime)

	// Check if context was cancelled during workflow execution
	if ctx.Err() == context.Canceled {
		appLogger.Info().Msg("Onetime scan workflow interrupted.")
		// The main signal handler in main() should have sent an interruption notification.
		// We might not need to send another one here unless we want more specific details.
		return // Exit early as the scan was incomplete
	}

	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanID = scanIdentifier
	summaryData.Targets = seedURLs // This should be string URLs, ensure seedURLs is appropriate here or transform
	summaryData.TotalTargets = len(seedURLs)
	summaryData.ScanDuration = scanDuration

	if probeResults != nil {
		summaryData.ProbeStats.DiscoverableItems = len(probeResults)
		for _, pr := range probeResults {
			if pr.Error == "" && (pr.StatusCode < 400 || (pr.StatusCode >= 300 && pr.StatusCode < 400)) {
				summaryData.ProbeStats.SuccessfulProbes++
			} else {
				summaryData.ProbeStats.FailedProbes++
			}
		}
	}

	// Populate DiffStats from urlDiffResults
	if urlDiffResults != nil {
		for _, diffResult := range urlDiffResults { // urlDiffResults is a map[string]models.URLDiffResult
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
			// Note: diffResult.Changed is not currently populated by the differ, so not adding it here yet.
		}
	}

	if workflowErr != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData) // Use non-cancelled context
		appLogger.Error().Err(workflowErr).Msg("Scan workflow execution failed")             // Changed from Fatal
		return
	}
	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Error().Err(err).Msg("Failed to initialize HTML reporter") // Changed from Fatal
		return
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode)
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	if err := htmlReporter.GenerateReport(probeResultsPtr, urlDiffResults, reportPath); err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to generate HTML report: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Error().Err(err).Msg("Failed to generate HTML report") // Changed from Fatal
		return
	}
	appLogger.Info().Str("path", reportPath).Msg("HTML report generated successfully")
	summaryData.ReportPath = reportPath
	summaryData.Status = string(models.ScanStatusCompleted)

	notificationHelper.SendScanCompletionNotification(ctx, summaryData) // Use passed context

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}
