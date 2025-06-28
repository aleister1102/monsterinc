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

	"github.com/aleister1102/monsterinc/internal/common/contextutils"
	"github.com/aleister1102/monsterinc/internal/common/httpclient"
	"github.com/aleister1102/monsterinc/internal/common/summary"
	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/logger"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/notifier/discord"
	"github.com/aleister1102/monsterinc/internal/scanner"
	"github.com/aleister1102/monsterinc/internal/scheduler"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

// Global state variables for tracking active sessions and interrupt notifications
var (
	activeScanSessionID        string
	activeScanMutex            sync.RWMutex
	interruptNotificationSent  bool
	interruptNotificationMutex sync.Mutex
)

// setActiveScanSessionID safely sets the active scan session ID
func setActiveScanSessionID(sessionID string) {
	activeScanMutex.Lock()
	defer activeScanMutex.Unlock()
	activeScanSessionID = sessionID

	// Reset interrupt notification flag when starting new scan
	if sessionID != "" {
		setInterruptNotificationSent(false)
	}
}

// getActiveScanSessionID safely gets the active scan session ID
func getActiveScanSessionID() string {
	activeScanMutex.RLock()
	defer activeScanMutex.RUnlock()
	return activeScanSessionID
}

// setInterruptNotificationSent safely sets the interrupt notification flag
func setInterruptNotificationSent(sent bool) {
	interruptNotificationMutex.Lock()
	defer interruptNotificationMutex.Unlock()
	interruptNotificationSent = sent
}

// getAndSetInterruptNotificationSent atomically checks and sets the interrupt notification flag
func getAndSetInterruptNotificationSent() bool {
	interruptNotificationMutex.Lock()
	defer interruptNotificationMutex.Unlock()

	if interruptNotificationSent {
		return true // Already sent
	}

	interruptNotificationSent = true
	return false // First time
}

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Set function pointer for scheduler to track active scans
	scheduler.SetActiveScanSessionID = setActiveScanSessionID
	scheduler.GetAndSetInterruptNotificationSent = getAndSetInterruptNotificationSent

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	discordHttpClient, err := httpclient.NewHTTPClientFactory(zLogger).CreateDiscordClient(
		20 * time.Second,
	)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to create Discord HTTP client.")
	}
	discordNotifier, err := discord.NewDiscordNotifier(&gCfg.NotificationConfig, zLogger, discordHttpClient)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier infra.")
	}
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	scanner, err := initializeScanner(gCfg, zLogger)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize scanner.")
	}

	setupSignalHandling(cancel, zLogger, notificationHelper, gCfg)

	var schedulerPtr *scheduler.Scheduler

	runApplicationLogic(ctx, gCfg, flags, zLogger, notificationHelper, scanner, &schedulerPtr)

	shutdownServices(scanner, schedulerPtr, zLogger, ctx)
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
func initializeScanner(gCfg *config.GlobalConfig, appLogger zerolog.Logger) (*scanner.Scanner, error) {
	pReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)

	pWriter, err := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if err != nil {
		return nil, fmt.Errorf("could not initialize ParquetWriter: %w", err)
	}

	appLogger.Info().Msg("Initializing scanner...")
	scannerInstance := scanner.NewScanner(gCfg, appLogger, pReader, pWriter)
	appLogger.Info().Msg("Scanner initialized successfully.")
	return scannerInstance, nil
}

func setupSignalHandling(
	cancel context.CancelFunc,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	gCfg *config.GlobalConfig,
) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		zLogger.Warn().Str("signal", sig.String()).Msg("🚨 INTERRUPT SIGNAL RECEIVED - Initiating immediate shutdown...")

		// Create dedicated context with timeout for interrupt notifications
		// This ensures notifications can be sent even if main context is cancelled
		notificationCtx, notificationCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer notificationCancel()

		// Track if any notification was sent
		notificationSent := false

		// Send scan interrupt notification if there's an active scan
		currentActiveScanID := getActiveScanSessionID()
		zLogger.Debug().Str("active_scan_id", currentActiveScanID).Msg("Checking for active scan session")

		if currentActiveScanID != "" && notificationHelper != nil {
			// Check if notification already sent to avoid duplicates
			if !getAndSetInterruptNotificationSent() {
				interruptSummary := summary.GetDefaultScanSummaryData()
				interruptSummary.ScanSessionID = currentActiveScanID
				interruptSummary.ScanMode = gCfg.Mode
				interruptSummary.TargetSource = "global_interrupt"
				interruptSummary.Status = string(summary.ScanStatusInterrupted)
				interruptSummary.ErrorMessages = []string{fmt.Sprintf("Scan interrupted by user signal (%s)", sig.String())}
				interruptSummary.Component = "SignalHandler"

				zLogger.Info().Str("scan_session_id", currentActiveScanID).Msg("Sending scan interrupt notification for active scan")
				notificationHelper.SendScanInterruptNotification(notificationCtx, interruptSummary)
				notificationSent = true
			} else {
				zLogger.Info().Str("scan_session_id", currentActiveScanID).Msg("Scan interrupt notification already sent, skipping duplicate")
				notificationSent = true
			}
		}

		// Send general interrupt notification if no specific notification was sent
		if !notificationSent && notificationHelper != nil {
			zLogger.Info().Msg("No active scan or monitor found, sending general interrupt notification")

			generalInterruptSummary := summary.GetDefaultScanSummaryData()
			generalInterruptSummary.ScanMode = gCfg.Mode
			generalInterruptSummary.TargetSource = "system_interrupt"
			generalInterruptSummary.Status = string(summary.ScanStatusInterrupted)
			generalInterruptSummary.ErrorMessages = []string{fmt.Sprintf("MonsterInc service interrupted by user signal (%s)", sig.String())}
			generalInterruptSummary.Component = "SystemService"
			generalInterruptSummary.ScanSessionID = fmt.Sprintf("system-%s", time.Now().Format("20060102-150405"))

			notificationHelper.SendScanInterruptNotification(notificationCtx, generalInterruptSummary)
		} else if !notificationSent {
			zLogger.Warn().Msg("No notification sent for interrupt - notification helper is not available")
		} else {
			zLogger.Debug().Msg("Specific interrupt notification already sent, skipping general notification")
		}

		// Wait a moment for notifications to be sent before cancelling context
		time.Sleep(1 * time.Second)

		// Cancel context after notifications are sent - this will propagate to all components
		cancel()

		zLogger.Info().Msg("Cancellation signal sent to all components. Please wait for graceful shutdown...")

		// Set up a secondary signal handler for force quit
		go func() {
			sig2 := <-sigChan
			zLogger.Error().Str("signal", sig2.String()).Msg("🛑 SECOND INTERRUPT SIGNAL - Force quitting application!")
			os.Exit(1)
		}()
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
	schedulerPtr **scheduler.Scheduler,
) {
	scanTargetsFile := ""
	if flags.ScanTargetsFile != "" {
		scanTargetsFile = flags.ScanTargetsFile
		zLogger.Info().Str("file", scanTargetsFile).Msg("Using -st for main scan targets.")
	}

	// Set notification helper for scanner
	scanner.SetNotificationHelper(notificationHelper)

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
	scannerInstance *scanner.Scanner,
) {
	// Create a new context for this scan that we can cancel.
	scanCtx, scanCancel := context.WithCancel(ctx)
	defer scanCancel() // Ensure it's cancelled on return

	// Load seed URLs using TargetManager
	targetManager := urlhandler.NewTargetManager(baseLogger)
	scanTargets, targetSource, err := targetManager.LoadAndSelectTargets(scanTargetsFile)

	if err != nil {
		baseLogger.Error().Err(err).Msg("Failed to load seed URLs for onetime scan.")

		criticalErrSummary := summary.GetDefaultScanSummaryData()
		criticalErrSummary.ScanMode = gCfg.Mode
		criticalErrSummary.TargetSource = scanTargetsFile
		if criticalErrSummary.TargetSource == "" {
			criticalErrSummary.TargetSource = "config"
		}
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to load seed URLs: %v", err)}
		notificationHelper.SendScanCompletionNotification(context.Background(), criticalErrSummary, nil)
		return
	}

	if len(scanTargets) == 0 {
		baseLogger.Info().Msg("Onetime scan: No seed URLs loaded, scan will not run.")

		if targetSource != "" && targetSource != "no_input" {
			noTargetsSummary := summary.GetDefaultScanSummaryData()
			noTargetsSummary.TargetSource = targetSource
			noTargetsSummary.Status = string(summary.ScanStatusNoTargets)
			noTargetsSummary.ErrorMessages = []string{"No URLs provided or loaded."}
			notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, nil)
		} else {
			baseLogger.Info().Msg("No targets specified for onetime scan via CLI or config. 'NO_TARGETS' notification will be skipped.")
		}
		return
	}

	// Send start notification
	scanMode := "onetime"
	scanUrls := targetManager.GetTargetStrings(scanTargets) // Convert to string slice for notification
	baseLogger.Info().Int("count", len(scanUrls)).Str("source", targetSource).Msg("Starting onetime scan with seed URLs.")

	// Create single session ID for the entire scan
	scanSessionID := time.Now().Format("20060102-150405")

	// Track active scan session ID for interrupt handling
	setActiveScanSessionID(scanSessionID)

	// Create scan logger with scanID for organized logging
	scanLogger, err := logger.NewWithScanID(gCfg.LogConfig, scanSessionID)
	if err != nil {
		baseLogger.Warn().Err(err).Str("scan_session_id", scanSessionID).Msg("Failed to create scan logger, using default logger")
		scanLogger = baseLogger
	}

	// Prepare scan summary data for start notification
	startSummary := summary.GetDefaultScanSummaryData()
	startSummary.ScanSessionID = scanSessionID
	startSummary.TargetSource = targetSource
	startSummary.ScanMode = scanMode
	startSummary.Targets = scanUrls
	startSummary.TotalTargets = len(scanUrls)
	// Send scan start notification
	notificationHelper.SendScanStartNotification(ctx, startSummary)

	// Create batch workflow orchestrator
	batchOrchestrator := scanner.NewBatchWorkflowOrchestrator(gCfg, scannerInstance, scanLogger)

	// Execute batch scan
	batchResult, workflowErr := batchOrchestrator.ExecuteBatchScan(
		scanCtx,
		gCfg,
		scanTargetsFile,
		scanSessionID,
		targetSource,
		scanMode,
	)

	// Clear active scan session when done
	setActiveScanSessionID("")

	var summaryData summary.ScanSummaryData
	var reportFilePaths []string

	if batchResult != nil {
		summaryData = batchResult.SummaryData
		reportFilePaths = batchResult.ReportFilePaths

		// Log batch processing information
		if batchResult.UsedBatching {
			baseLogger.Info().
				Int("total_batches", batchResult.TotalBatches).
				Int("processed_batches", batchResult.ProcessedBatches).
				Bool("interrupted", batchResult.InterruptedAt > 0).
				Msg("Batch scan workflow completed")
		}
	} else {
		// Fallback to default summary if batch result is nil
		summaryData = summary.GetDefaultScanSummaryData()
		summaryData.ScanSessionID = scanSessionID
		summaryData.TargetSource = targetSource
		summaryData.ScanMode = scanMode
		summaryData.Targets = scanUrls
		summaryData.TotalTargets = len(scanTargets)
		summaryData.Status = string(summary.ScanStatusFailed)
		if workflowErr != nil {
			summaryData.ErrorMessages = []string{workflowErr.Error()}
		}
	}

	// Handle workflow error or context cancellation
	if workflowErr != nil || ctx.Err() != nil {
		finalStatus := string(summary.ScanStatusFailed)
		if errors.Is(workflowErr, context.Canceled) || errors.Is(workflowErr, context.DeadlineExceeded) || ctx.Err() == context.Canceled {
			baseLogger.Info().Str("scanSessionID", scanSessionID).Msg("Onetime scan workflow interrupted.")
			finalStatus = string(summary.ScanStatusInterrupted)
			if !contextutils.ContainsCancellationError(summaryData.ErrorMessages) { // summaryData is returned by ExecuteSingleScanWorkflowWithReporting
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

		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, reportFilePaths) // reportFilePaths might be nil

		// Shutdown scanner even on error to clean up singleton crawler with timeout
		baseLogger.Info().Msg("Shutting down scanner after onetime scan error")
		shutdownDone := make(chan struct{})
		go func() {
			defer close(shutdownDone)
			scannerInstance.Shutdown()
		}()

		select {
		case <-shutdownDone:
			baseLogger.Info().Msg("Scanner shutdown completed successfully after error")
		case <-time.After(10 * time.Second):
			baseLogger.Warn().Msg("Scanner shutdown timeout reached after error, forcing exit")
		}

		return
	}

	// If successful, summaryData is already populated by ExecuteSingleScanWorkflowWithReporting with Completed status
	baseLogger.Info().Str("scanSessionID", scanSessionID).Msg("Onetime scan workflow completed successfully via orchestrator. Sending completion notification.")
	notificationHelper.SendScanCompletionNotification(ctx, summaryData, reportFilePaths)

	// Shutdown scanner to clean up singleton crawler with timeout
	baseLogger.Info().Msg("Shutting down scanner after onetime scan completion")
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		scannerInstance.Shutdown()
	}()

	select {
	case <-shutdownDone:
		baseLogger.Info().Msg("Scanner shutdown completed successfully")
	case <-time.After(10 * time.Second):
		baseLogger.Warn().Msg("Scanner shutdown timeout reached, forcing exit")
	}

	baseLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")

	// Force exit for onetime mode to prevent hanging - this might not be needed after the fix.
	// We'll leave it for now to be safe.
	os.Exit(0)
}

func runAutomatedScan(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	scanTargetsFile string,
	scanner *scanner.Scanner,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	schedulerPtr **scheduler.Scheduler,
) {
	// Determine if the scheduler should run.
	// Scheduler runs if scan targets are provided OR if monitor targets have been loaded into the service.
	automatedModeActive := scanTargetsFile != ""

	if !automatedModeActive {
		zLogger.Info().Msg("Automated mode: Scan targets (-st) were not provided. Scheduler will not start.")
		return
	}

	// Initialize scheduler
	scheduler, schedulerErr := scheduler.NewScheduler(
		gCfg,
		scanTargetsFile,
		scanner,
		zLogger,
		notificationHelper,
	)
	if schedulerErr != nil {
		criticalErrSummary := summary.GetDefaultScanSummaryData()
		criticalErrSummary.Component = "SchedulerInitialization"
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", schedulerErr)}
		notificationHelper.SendScanCompletionNotification(context.Background(), criticalErrSummary, nil)
		zLogger.Fatal().Err(schedulerErr).Msg("Failed to initialize scheduler")
		return
	}
	*schedulerPtr = scheduler

	// Start the scheduler. This is a blocking call.
	if err := (*schedulerPtr).Start(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
		} else {
			criticalErrSummary := summary.GetDefaultScanSummaryData()
			criticalErrSummary.Component = "SchedulerRuntime"
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Scheduler error: %v", err)}
			notificationHelper.SendScanCompletionNotification(context.Background(), criticalErrSummary, nil)
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
	scanner *scanner.Scanner,
	scheduler *scheduler.Scheduler,
	zLogger zerolog.Logger,
	ctx context.Context,
) {
	shutdownTimeout := 30 * time.Second // Tăng timeout lên 30 giây
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Create a done channel to signal completion
	done := make(chan struct{})

	go func() {
		defer close(done)

		// Stop scheduler first (it will stop monitoring service internally)
		if scheduler != nil {
			zLogger.Info().Msg("Stopping scheduler...")
			scheduler.Stop()
			zLogger.Info().Msg("Scheduler stopped.")
		}

		// Shutdown scanner (which will shutdown the singleton crawler)
		if scanner != nil {
			zLogger.Info().Msg("Shutting down scanner...")
			scanner.Shutdown()
			zLogger.Info().Msg("Scanner shutdown completed.")
		}

		// Give a bit of time for final cleanup
		time.Sleep(1 * time.Second)
		zLogger.Info().Msg("Shutdown sequence completed.")
	}()

	// Wait for either shutdown completion or timeout
	select {
	case <-done:
		if ctx.Err() == context.Canceled {
			zLogger.Info().Msg("Application shutting down due to context cancellation.")
		} else {
			zLogger.Info().Msg("Application finished.")
		}
	case <-shutdownCtx.Done():
		zLogger.Warn().Dur("timeout", shutdownTimeout).Msg("Shutdown timeout reached, forcing exit")
	}
}
