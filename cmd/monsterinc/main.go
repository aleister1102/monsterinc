package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/logger"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/scanner"
	"github.com/aleister1102/monsterinc/internal/scheduler"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	flags := ParseFlags()

	gCfg, err := loadConfiguration(flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Main: %v\n", err)
		os.Exit(1)
	}

	zLogger, err := initializeLogger(gCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Main: %v\n", err)
		os.Exit(1)
	}

	discordHttpClient, err := common.NewHTTPClientFactory(zLogger).CreateDiscordClient(
		20 * time.Second,
	)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to create Discord HTTP client.")
	}
	discordNotifier, err := notifier.NewDiscordNotifier(zLogger, discordHttpClient)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier infra.")
	}
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	scanner := initializeScanner(gCfg, zLogger)

	ms, err := initializeMonitoringService(gCfg, flags.MonitorTargetsFile, zLogger, notificationHelper)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize monitoring service.")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandling(cancel, zLogger)

	var schedulerPtr *scheduler.Scheduler
	runApplicationLogic(ctx, gCfg, flags, zLogger, notificationHelper, scanner, ms, &schedulerPtr)

	var monitorWg sync.WaitGroup
	shutdownServices(ms, schedulerPtr, &monitorWg, zLogger, ctx)
}

// loadConfiguration loads the global configuration from the specified file,
// Refactored ✅
func loadConfiguration(flags AppFlags) (*config.GlobalConfig, error) {
	// Use a basic logger for config loading
	basicLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	gCfg, err := config.LoadGlobalConfig(flags.GlobalConfigFile, basicLogger)
	if err != nil {
		return nil, fmt.Errorf("could not load global config using path '%s': %w", flags.GlobalConfigFile, err)
	}
	if gCfg == nil {
		return nil, fmt.Errorf("loaded configuration is nil, though no error was reported. This should not happen")
	}
	fmt.Println("[INFO] Main: Global configuration loaded successfully.")

	if flags.Mode != "" {
		gCfg.Mode = flags.Mode
		fmt.Printf("[INFO] Main: Mode set to '%s' from command line flag.\n", gCfg.Mode)
	}

	if gCfg.ReporterConfig.OutputDir != "" {
		if err := os.MkdirAll(gCfg.ReporterConfig.OutputDir, 0755); err != nil {
			return gCfg, fmt.Errorf("could not create default report output directory '%s': %w", gCfg.ReporterConfig.OutputDir, err)
		}
	}

	if err := config.ValidateConfig(gCfg); err != nil {
		return gCfg, fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("[INFO] Main: Configuration validated successfully.\n")
	return gCfg, nil
}

// initializeLogger initializes the logger based on the global configuration.
// Refactored ✅
func initializeLogger(gCfg *config.GlobalConfig) (zerolog.Logger, error) {
	zLogger, err := logger.New(gCfg.LogConfig)
	if err != nil {
		return zerolog.Nop(), fmt.Errorf("could not initialize logger: %w", err)
	}
	zLogger.Info().Msg("Logger initialized successfully.")

	return zLogger, nil
}

// initializeScanner initializes the scanner with the provided global configuration and logger.
// Refactored ✅
func initializeScanner(gCfg *config.GlobalConfig, appLogger zerolog.Logger) *scanner.Scanner {
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	scanner := scanner.NewScanner(gCfg, appLogger, parquetReader, parquetWriter)
	return scanner
}

// initializeMonitoringService initializes the file monitoring service if enabled and a monitor targets file is provided.
// Refactored ✅
func initializeMonitoringService(
	gCfg *config.GlobalConfig,
	monitorTargetsFile string,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) (*monitor.MonitoringService, error) {
	if monitorTargetsFile == "" || !gCfg.MonitorConfig.Enabled {
		zLogger.Info().Msg("Monitoring service not initialized: no monitor targets file provided or monitoring is disabled in configuration.")
		return nil, nil // No monitoring service to initialize
	}

	// Initialize monitoring service
	monitorLogger := zLogger.With().Str("service", "FileMonitor").Logger()
	ms := monitor.NewMonitoringService(
		gCfg,
		monitorLogger,
		notificationHelper,
	)
	zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service initialized.")

	// Preload monitor targets if provided
	if monitorTargetsFile != "" {
		if err := preloadMonitoringTargets(gCfg, ms, monitorTargetsFile, zLogger); err != nil {
			return nil, fmt.Errorf("failed to preload monitoring targets: %w", err)
		}
		zLogger.Info().Str("file", monitorTargetsFile).Msg("Preloaded monitoring targets from file.")
	}

	return ms, nil
}

func preloadMonitoringTargets(
	gCfg *config.GlobalConfig,
	ms *monitor.MonitoringService,
	monitorTargetsFile string,
	zLogger zerolog.Logger,
) error {
	targetManager := urlhandler.NewTargetManager(zLogger)
	monitorTargets, _, err := targetManager.LoadAndSelectTargets(
		monitorTargetsFile,
		gCfg.MonitorConfig.InputURLs,
		gCfg.MonitorConfig.InputFile,
	)
	if err != nil {
		return fmt.Errorf("failed to load monitor targets from file '%s': %w", monitorTargetsFile, err)
	}

	monitorUrls := targetManager.GetTargetStrings(monitorTargets) // Convert to string slice for notification
	if len(monitorUrls) == 0 {
		zLogger.Warn().Msg("No valid monitor targets loaded from file.")
		return fmt.Errorf("no valid monitor targets loaded from file '%s'", monitorTargetsFile)
	}
	zLogger.Info().Int("count", len(monitorUrls)).Str("file", monitorTargetsFile).Msg("Loaded monitor targets from file.")

	ms.Preload(monitorUrls)

	return nil
}

func setupSignalHandling(
	cancel context.CancelFunc,
	zLogger zerolog.Logger,
) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		zLogger.Info().Str("signal", sig.String()).Msg("Received interrupt signal, initiating graceful shutdown...")
		cancel()
	}()
}

// runApplicationLogic orchestrates the main application logic based on the provided configuration and flags.
// Refactored ✅
func runApplicationLogic(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	flags AppFlags,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	scanner *scanner.Scanner,
	monitoringService *monitor.MonitoringService, schedulerPtr **scheduler.Scheduler,
) {
	scanTargetsFile := ""
	if flags.ScanTargetsFile != "" {
		scanTargetsFile = flags.ScanTargetsFile
		zLogger.Info().Str("file", scanTargetsFile).Msg("Using -st for main scan targets.")
	}

	monitorTargetsFile := ""
	if flags.MonitorTargetsFile != "" {
		monitorTargetsFile = flags.MonitorTargetsFile
		zLogger.Info().Str("file", monitorTargetsFile).Msg("Using -mt for monitor targets.")
	}

	if gCfg.Mode == "onetime" && scanTargetsFile != "" {
		runOnetimeScan(
			ctx,
			gCfg,
			scanTargetsFile,
			zLogger,
			notificationHelper,
			scanner,
		)
	} else if gCfg.Mode == "automated" {
		runAutomatedScan(
			ctx,
			gCfg,
			scanTargetsFile,
			scanner,
			monitorTargetsFile,
			monitoringService,
			zLogger,
			notificationHelper,
			schedulerPtr,
		)
	}
}

func runOnetimeScan(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	scanTargetsFile string,
	baseLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	scanner *scanner.Scanner,
) {
	// Load seed URLs using TargetManager
	targetManager := urlhandler.NewTargetManager(baseLogger)
	scanTargets, targetSource, err := targetManager.LoadAndSelectTargets(
		scanTargetsFile,
		gCfg.InputConfig.InputURLs,
		gCfg.InputConfig.InputFile,
	)

	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to load seed URLs for onetime scan.")

		criticalErrSummary := models.GetDefaultScanSummaryData()
		criticalErrSummary.ScanMode = gCfg.Mode
		criticalErrSummary.TargetSource = scanTargetsFile
		if criticalErrSummary.TargetSource == "" {
			criticalErrSummary.TargetSource = "config"
		}
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to load seed URLs: %v", err)}
		notificationHelper.SendCriticalErrorNotification(context.Background(), "SeedURLLoad", criticalErrSummary)
		return
	}

	if len(scanTargets) == 0 {
		baseLogger.Info().Msg("Onetime scan: No seed URLs loaded, scan will not run.")

		if targetSource != "" && targetSource != "no_input" {
			noTargetsSummary := models.GetDefaultScanSummaryData()
			noTargetsSummary.TargetSource = targetSource
			noTargetsSummary.Status = string(models.ScanStatusNoTargets)
			noTargetsSummary.ErrorMessages = []string{"No URLs provided or loaded."}
			notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, notifier.ScanServiceNotification, nil)
		} else {
			baseLogger.Info().Msg("No targets specified for onetime scan via CLI or config. 'NO_TARGETS' notification will be skipped.")
		}
		return
	}

	// Send start notification
	scanMode := "onetime"
	scanUrls := targetManager.GetTargetStrings(scanTargets) // Convert to string slice for notification
	baseLogger.Info().Int("count", len(scanUrls)).Str("source", targetSource).Msg("Starting onetime scan with seed URLs.")

	// Prepare scan summary data for start notification
	startSummary := models.GetDefaultScanSummaryData()
	startSummary.ScanSessionID = time.Now().Format("20060102-150405")
	startSummary.TargetSource = targetSource
	startSummary.ScanMode = scanMode
	startSummary.Targets = scanUrls
	startSummary.TotalTargets = len(scanUrls)
	// Send scan start notification
	notificationHelper.SendScanStartNotification(ctx, startSummary)

	// Execute complete scan workflow using the new shared function
	scanSessionID := time.Now().Format("20060102-150405")

	summaryData, _, reportFilePaths, workflowErr := scanner.ExecuteSingleScanWorkflowWithReporting(
		ctx,
		gCfg,
		baseLogger,
		scanUrls,
		scanSessionID,
		targetSource,
		scanMode,
	)

	// Handle workflow error or context cancellation
	if workflowErr != nil || ctx.Err() != nil {
		finalStatus := string(models.ScanStatusFailed)
		if errors.Is(workflowErr, context.Canceled) || errors.Is(workflowErr, context.DeadlineExceeded) || ctx.Err() == context.Canceled {
			baseLogger.Info().Str("scanSessionID", scanSessionID).Msg("Onetime scan workflow interrupted.")
			finalStatus = string(models.ScanStatusInterrupted)
			if !common.ContainsCancellationError(summaryData.ErrorMessages) { // summaryData is returned by ExecuteSingleScanWorkflowWithReporting
				summaryData.ErrorMessages = append(summaryData.ErrorMessages, "Onetime scan interrupted by signal or context cancellation.")
			}
		} else if workflowErr != nil {
			baseLogger.Error().Err(workflowErr).Str("scanSessionID", scanSessionID).Msg("Onetime scan workflow execution failed")
			// Error messages should already be in summaryData by ExecuteSingleScanWorkflowWithReporting
		}
		summaryData.Status = finalStatus          // Ensure status is correctly set on the summary from orchestrator
		summaryData.ScanSessionID = scanSessionID // Ensure ScanSessionID is set
		summaryData.TargetSource = targetSource   // Ensure TargetSource is set
		summaryData.Targets = scanUrls            // Ensure Targets are set
		summaryData.TotalTargets = len(scanTargets)

		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification, reportFilePaths) // reportFilePaths might be nil
		return
	}

	// If successful, summaryData is already populated by ExecuteSingleScanWorkflowWithReporting with Completed status
	baseLogger.Info().Str("scanSessionID", scanSessionID).Msg("Onetime scan workflow completed successfully via orchestrator. Sending completion notification.")
	notificationHelper.SendScanCompletionNotification(ctx, summaryData, notifier.ScanServiceNotification, reportFilePaths)

	baseLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}

func runAutomatedScan(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	scanTargetsFile string,
	scanner *scanner.Scanner,
	monitorTargetsFile string,
	monitoringService *monitor.MonitoringService,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	schedulerPtr **scheduler.Scheduler,
) {
	// Determine if the scheduler should run.
	// Scheduler runs if scan targets are provided OR if monitor targets have been loaded into the service.
	automatedModeActive := scanTargetsFile != "" || (monitoringService != nil && len(monitoringService.GetCurrentlyMonitorUrls()) > 0)

	if !automatedModeActive {
		zLogger.Info().Msg("Automated mode: Neither scan targets (-st) nor monitor targets (-mt) were provided or loaded with valid URLs. Scheduler will not start.")
		return
	}

	// Initialize scheduler
	scheduler, schedulerErr := scheduler.NewScheduler(
		gCfg,
		scanTargetsFile,
		scanner,
		monitorTargetsFile,
		monitoringService,
		zLogger,
		notificationHelper,
	)
	if schedulerErr != nil {
		criticalErrSummary := models.GetDefaultScanSummaryData()
		criticalErrSummary.Component = "SchedulerInitialization"
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", schedulerErr)}
		notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", criticalErrSummary)
		zLogger.Fatal().Err(schedulerErr).Msg("Failed to initialize scheduler")
		return
	}
	*schedulerPtr = scheduler

	// Start the scheduler. This is a blocking call.
	if err := (*schedulerPtr).Start(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
		} else {
			criticalErrSummary := models.GetDefaultScanSummaryData()
			criticalErrSummary.Component = "SchedulerRuntime"
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Scheduler error: %v", err)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerRuntime", criticalErrSummary)
			zLogger.Error().Err(err).Msg("Scheduler error")
		}
	}
	zLogger.Info().Msg("Automated mode processing finished (scheduler has exited).")

	// Ensure scheduler is stopped if context was cancelled (idempotent call)
	if *schedulerPtr != nil && ctx.Err() == context.Canceled {
		zLogger.Info().Msg("Context cancelled, ensuring scheduler is stopped post-exit.")
		(*schedulerPtr).Stop()
	}
}

func shutdownServices(
	ms *monitor.MonitoringService,
	scheduler *scheduler.Scheduler,
	monitorWg *sync.WaitGroup,
	zLogger zerolog.Logger,
	ctx context.Context,
) {
	if ms != nil {
		zLogger.Info().Msg("Waiting for file monitoring service to shut down completely...")
		monitorWg.Wait() // This ensures its goroutine finishes, which includes calling its Stop()
		zLogger.Info().Msg("File monitoring service has shut down.")
	}

	if scheduler != nil {
		zLogger.Info().Msg("Ensuring scheduler is stopped as part of final shutdown sequence...")
		scheduler.Stop() // Call Stop here if not already stopped by interrupt logic in runApplicationLogic
		zLogger.Info().Msg("Scheduler confirmed stopped in shutdownServices.")
	}

	if ctx.Err() == context.Canceled {
		zLogger.Info().Msg("Application shutting down due to context cancellation.")
	} else {
		zLogger.Info().Msg("Application finished.")
	}
}
