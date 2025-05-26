package main

import (
	"context"
	"errors"
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
	"monsterinc/internal/urlhandler"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
		criticalErrSummary := models.GetDefaultScanSummaryData()
		criticalErrSummary.Component = "DiscordNotifierInitialization"
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize DiscordNotifier: %v", err)}
		// Cannot use notificationHelper here as it's not initialized yet.
		// Log fatally as this is a critical setup step.
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
		if notificationHelper != nil {
			interruptionSummary := models.GetDefaultScanSummaryData()
			interruptionSummary.ScanSessionID = fmt.Sprintf("%s_mode_interrupted_by_signal", gCfg.Mode)
			interruptionSummary.TargetSource = "Signal Interrupt"
			interruptionSummary.Status = string(models.ScanStatusInterrupted) // Use new status
			interruptionSummary.ErrorMessages = []string{fmt.Sprintf("Application (%s mode) interrupted by signal: %s", gCfg.Mode, sig.String())}
			notificationCtx, notificationCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer notificationCancel()
			notificationHelper.SendScanCompletionNotification(notificationCtx, interruptionSummary)
			zLogger.Info().Msg("Interruption notification sent.")
		}
		cancel()
	}()

	if gCfg.Mode == "automated" {
		zLogger.Info().Msg("Running in automated mode...")
		schedulerInstance, err := scheduler.NewScheduler(gCfg, *urlListFile, zLogger, notificationHelper)
		if err != nil {
			criticalErrSummary := models.GetDefaultScanSummaryData()
			criticalErrSummary.Component = "SchedulerInitialization"
			criticalErrSummary.TargetSource = *urlListFile // Best guess for target source
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", err)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", criticalErrSummary)
			zLogger.Fatal().Err(err).Msg("Failed to initialize scheduler")
		}

		if err := schedulerInstance.Start(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
			} else {
				criticalErrSummary := models.GetDefaultScanSummaryData()
				criticalErrSummary.Component = "SchedulerRuntime"
				criticalErrSummary.TargetSource = *urlListFile // Best guess
				criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Scheduler error: %v", err)}
				notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerRuntime", criticalErrSummary)
				zLogger.Error().Err(err).Msg("Scheduler error")
			}
		}
		zLogger.Info().Msg("Automated mode completed or interrupted.")
	} else {
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(ctx, gCfg, *urlListFile, zLogger, notificationHelper)
	}

	if ctx.Err() == context.Canceled {
		zLogger.Info().Msg("Application shutting down due to context cancellation.")
	} else {
		zLogger.Info().Msg("Application finished.")
	}
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFile string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper) {
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter)

	var seedURLs []string
	var onetimeTargetSource string
	if urlListFile != "" {
		appLogger.Info().Str("file", urlListFile).Msg("Attempting to read seed URLs from file.")
		loadedURLs, errFile := urlhandler.ReadURLsFromFile(urlListFile, appLogger)
		if errFile != nil {
			appLogger.Error().Err(errFile).Str("file", urlListFile).Msg("Failed to load URLs from file. See previous logs for details.")
			criticalErrSummary := models.GetDefaultScanSummaryData()
			criticalErrSummary.TargetSource = urlListFile
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to load URLs from file '%s': %v. Check application logs.", urlListFile, errFile)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanURLFileLoad", criticalErrSummary)
		}
		seedURLs = loadedURLs
		onetimeTargetSource = filepath.Base(urlListFile)
		if len(seedURLs) == 0 {
			appLogger.Warn().Str("file", urlListFile).Msg("No valid URLs found in file, or file processing failed. Will attempt to use seeds from config if available.")
		}
	}

	if len(seedURLs) == 0 {
		if len(gCfg.InputConfig.InputURLs) > 0 {
			appLogger.Info().Int("count", len(gCfg.InputConfig.InputURLs)).Msg("Using seed URLs from global input_config.input_urls")
			seedURLs = gCfg.InputConfig.InputURLs
			onetimeTargetSource = "config_input_urls"
		} else {
			noSeedsMsg := "No seed URLs provided or loaded. Please specify via -urlfile or in input_config.input_urls in the config file."
			criticalErrSummary := models.GetDefaultScanSummaryData()
			if onetimeTargetSource == "" {
				criticalErrSummary.TargetSource = "Unknown/NotProvided"
			} else {
				criticalErrSummary.TargetSource = onetimeTargetSource
			}
			criticalErrSummary.ErrorMessages = []string{noSeedsMsg}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanNoSeeds", criticalErrSummary)
			appLogger.Error().Msg(noSeedsMsg)
			return
		}
	}

	appLogger.Info().Int("count", len(seedURLs)).Str("source", onetimeTargetSource).Msg("Starting onetime scan with seed URLs.")

	startSummary := models.GetDefaultScanSummaryData()
	startSummary.TargetSource = onetimeTargetSource
	startSummary.Targets = seedURLs
	startSummary.TotalTargets = len(seedURLs)
	notificationHelper.SendScanStartNotification(ctx, startSummary)

	scanSessionID := time.Now().Format("20060102-150405")
	startSummary.ScanSessionID = scanSessionID // Also add session ID to start summary if needed by formatter, though not strictly for this task

	appLogger.Info().Msg("Executing scan workflow via orchestrator...")
	startTime := time.Now()
	probeResults, urlDiffResults, workflowErr := scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
	scanDuration := time.Since(startTime)

	if ctx.Err() == context.Canceled {
		appLogger.Info().Msg("Onetime scan workflow interrupted.")
		return
	}

	summaryData := models.GetDefaultScanSummaryData()
	summaryData.ScanSessionID = scanSessionID
	summaryData.TargetSource = onetimeTargetSource
	summaryData.Targets = seedURLs
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

	if urlDiffResults != nil {
		for _, diffResult := range urlDiffResults {
			summaryData.DiffStats.New += diffResult.New
			summaryData.DiffStats.Old += diffResult.Old
			summaryData.DiffStats.Existing += diffResult.Existing
		}
	}

	if workflowErr != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Error().Err(workflowErr).Msg("Scan workflow execution failed")
		return
	}
	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Error().Err(err).Msg("Failed to initialize HTML reporter")
		return
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode)
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	if err := htmlReporter.GenerateReport(probeResultsPtr, urlDiffResults, reportPath); err != nil {
		summaryData.Status = string(models.ScanStatusFailed) // Or models.ScanStatusPartialComplete
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to generate HTML report: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData)
		appLogger.Error().Err(err).Msg("Failed to generate HTML report")
		return
	}
	appLogger.Info().Str("path", reportPath).Msg("HTML report generated successfully")
	summaryData.ReportPath = reportPath
	summaryData.Status = string(models.ScanStatusCompleted)

	notificationHelper.SendScanCompletionNotification(ctx, summaryData)

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}
