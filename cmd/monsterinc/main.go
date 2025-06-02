package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	"github.com/aleister1102/monsterinc/internal/orchestrator"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/scheduler"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

type appFlags struct {
	urlListFile        string
	monitorTargetsFile string
	globalConfigFile   string
	mode               string
}

func parseFlags() appFlags {
	urlListFile := flag.String("scan-targets", "", "Path to a text file containing seed URLs for the main scan. Used if --diff-target-file is not set. This flag is for backward compatibility.")
	urlListFileAlias := flag.String("st", "", "Alias for -scan-targets")

	monitorTargetFile := flag.String("monitor-targets", "", "Path to a text file containing JS/HTML URLs for file monitoring (only in automated mode).")
	monitorTargetFileAlias := flag.String("mt", "", "Alias for --monitor-targets")

	globalConfigFile := flag.String("globalconfig", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("gc", "", "Alias for --globalconfig")

	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	modeFlagAlias := flag.String("m", "", "Alias for --mode")

	flag.Parse()

	flags := appFlags{}

	if *urlListFile != "" {
		flags.urlListFile = *urlListFile
	} else if *urlListFileAlias != "" {
		flags.urlListFile = *urlListFileAlias
	}

	if *monitorTargetFile != "" {
		flags.monitorTargetsFile = *monitorTargetFile
	} else if *monitorTargetFileAlias != "" {
		flags.monitorTargetsFile = *monitorTargetFileAlias
	}

	if *globalConfigFile != "" {
		flags.globalConfigFile = *globalConfigFile
	} else if *globalConfigFileAlias != "" {
		flags.globalConfigFile = *globalConfigFileAlias
	}

	if *modeFlag != "" {
		flags.mode = *modeFlag
	} else if *modeFlagAlias != "" {
		flags.mode = *modeFlagAlias
	}

	// Auto-set mode to automated if using monitor-specific flags
	if flags.mode == "" {
		if flags.monitorTargetsFile != "" {
			flags.mode = "automated"
			fmt.Printf("[INFO] Mode automatically set to 'automated' due to monitor-related flags\n")
		} else {
			fmt.Fprintln(os.Stderr, "[FATAL] --mode argument is required (onetime or automated)")
			os.Exit(1)
		}
	}

	// Validate flag combinations
	if err := validateFlags(flags); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] %v\n", err)
		os.Exit(1)
	}

	return flags
}

// validateFlags validates command line flag combinations
func validateFlags(flags appFlags) error {
	if flags.monitorTargetsFile != "" && flags.mode == "onetime" {
		return fmt.Errorf("-mt (monitor targets) cannot be used with mode 'onetime'. Use 'automated' mode or omit mode flag")
	}

	return nil
}

func loadConfigurationAndLogger(flags appFlags) (*config.GlobalConfig, zerolog.Logger, error) {
	fmt.Println("[INFO] Main: Attempting to load global configuration...")
	gCfg, err := config.LoadGlobalConfig(flags.globalConfigFile)
	if err != nil {
		return nil, zerolog.Nop(), fmt.Errorf("could not load global config using path '%s': %w", flags.globalConfigFile, err)
	}
	if gCfg == nil {
		return nil, zerolog.Nop(), fmt.Errorf("loaded configuration is nil, though no error was reported. This should not happen")
	}
	fmt.Println("[INFO] Main: Global configuration loaded successfully.")

	zLogger, err := logger.New(gCfg.LogConfig)
	if err != nil {
		return gCfg, zerolog.Nop(), fmt.Errorf("could not initialize logger: %w", err)
	}
	zLogger.Info().Msg("Logger initialized successfully.")

	if flags.mode != "" {
		gCfg.Mode = flags.mode
		zLogger.Info().Str("mode", gCfg.Mode).Msg("Mode overridden by command line flag.")
	}

	if gCfg.ReporterConfig.OutputDir != "" {
		if err := os.MkdirAll(gCfg.ReporterConfig.OutputDir, 0755); err != nil {
			// Log with zLogger if available, otherwise fallback to standard log
			if zLogger.GetLevel() != zerolog.Disabled {
				zLogger.Fatal().Err(err).Str("directory", gCfg.ReporterConfig.OutputDir).Msg("Could not create default report output directory before validation")
			} else {
				fmt.Fprintf(os.Stderr, "[FATAL] Could not create default report output directory '%s': %v\n", gCfg.ReporterConfig.OutputDir, err)
				os.Exit(1)
			}
			return gCfg, zLogger, fmt.Errorf("could not create default report output directory '%s': %w", gCfg.ReporterConfig.OutputDir, err)
		}
	}

	if err := config.ValidateConfig(gCfg); err != nil {
		return gCfg, zLogger, fmt.Errorf("configuration validation failed: %w", err)
	}
	zLogger.Info().Msg("Configuration validated successfully.")

	return gCfg, zLogger, nil
}

func initializeMonitoringServices(
	gCfg *config.GlobalConfig,
	monitorTargetsFile string,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) (*monitor.MonitoringService, error) {
	var monitoringService *monitor.MonitoringService = nil
	var fileHistoryStore *datastore.ParquetFileHistoryStore = nil

	// Only initialize the monitoring service if there are monitor targets to monitor and the monitor is enabled
	if monitorTargetsFile != "" && gCfg.MonitorConfig.Enabled {
		// If monitor is enabled, initialize file history store
		var fhStoreErr error
		fileHistoryStore, fhStoreErr = datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, zLogger)
		if fhStoreErr != nil {
			zLogger.Error().Err(fhStoreErr).Msg("Failed to initialize ParquetFileHistoryStore for monitoring. Monitoring will be disabled.")
			return nil, fmt.Errorf("failed to initialize ParquetFileHistoryStore for monitoring: %w", fhStoreErr)
		}

		// Initialize HTTP client for monitor
		monitorHTTPClientTimeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
		if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
			monitorHTTPClientTimeout = 30 * time.Second
			zLogger.Warn().Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).Dur("default_timeout", monitorHTTPClientTimeout).Msg("Monitor HTTPTimeoutSeconds invalid or not set, using default timeout")
		}

		clientFactory := common.NewHTTPClientFactory(zLogger)
		monitorHTTPClient, clientErr := clientFactory.CreateMonitorClient(
			monitorHTTPClientTimeout,
			gCfg.MonitorConfig.MonitorInsecureSkipVerify,
		)
		if clientErr != nil {
			zLogger.Error().Err(clientErr).Msg("Failed to create HTTP client for monitoring. Monitoring will be disabled.")
			return nil, fmt.Errorf("failed to create HTTP client for monitoring: %w", clientErr)
		}

		// Initialize monitor service
		monitorLogger := zLogger.With().Str("service", "FileMonitor").Logger()
		monitoringService = monitor.NewMonitoringService(
			&gCfg.MonitorConfig,
			&gCfg.CrawlerConfig,
			&gCfg.ExtractorConfig,
			&gCfg.NotificationConfig,
			&gCfg.ReporterConfig,
			&gCfg.DiffReporterConfig,
			fileHistoryStore,
			monitorLogger,
			notificationHelper,
			monitorHTTPClient,
		)
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service initialized.")
	}

	return monitoringService, nil
}

func initializeScheduler(
	gCfg *config.GlobalConfig,
	scanURLFile string,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	monitoringService *monitor.MonitoringService,
	schedulerPtr **scheduler.Scheduler,
) (**scheduler.Scheduler, error) {
	var schedulerErr error
	*schedulerPtr, schedulerErr = scheduler.NewScheduler(
		gCfg,
		scanURLFile,
		zLogger,
		notificationHelper,
		monitoringService,
	)
	// If scheduler initialization fails, send a critical error notification and exit
	if schedulerErr != nil {
		criticalErrSummary := models.GetDefaultScanSummaryData()
		criticalErrSummary.Component = "SchedulerInitialization"
		criticalErrSummary.TargetSource = scanURLFile
		criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", schedulerErr)}

		notificationHelper.SendCriticalErrorNotification(
			context.Background(),
			"SchedulerInitialization",
			criticalErrSummary,
		)
		zLogger.Fatal().Err(schedulerErr).Msg("Failed to initialize scheduler")
	}

	return schedulerPtr, nil
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

func runApplicationLogic(
	ctx context.Context,
	gCfg *config.GlobalConfig,
	flags appFlags,
	zLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
	monitoringService *monitor.MonitoringService,
	schedulerPtr **scheduler.Scheduler,
) {
	scanURLFile := ""
	if flags.urlListFile != "" {
		scanURLFile = flags.urlListFile
		zLogger.Info().Str("file", scanURLFile).Msg("Using -st for main scan targets.")
	}

	monitorURLFile := ""
	if flags.monitorTargetsFile != "" {
		monitorURLFile = flags.monitorTargetsFile
		zLogger.Info().Str("file", monitorURLFile).Msg("Using -mt for monitor targets.")
	}

	if gCfg.Mode == "automated" {
		// If neither scan nor monitor targets provided
		if scanURLFile == "" && monitorURLFile == "" {
			zLogger.Info().Msg("Automated mode: Neither -st nor -mt provided. No services will be started.")
		}

		// Start scheduler if scan targets are provided
		if scanURLFile != "" {
			// Initialize scheduler for scan targets in automated mode
			schedulerPtr, err := initializeScheduler(
				gCfg,
				scanURLFile,
				zLogger,
				notificationHelper,
				monitoringService,
				schedulerPtr,
			)
			if err != nil {
				zLogger.Error().Err(err).Msg("Failed to initialize scheduler")
				return
			}

			// Start the scheduler
			if *schedulerPtr != nil {
				if err := (*schedulerPtr).Start(ctx); err != nil {
					// If the scheduler is stopped due to context cancellation (interrupt), log it
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
					} else {
						criticalErrSummary := models.GetDefaultScanSummaryData()
						criticalErrSummary.Component = "SchedulerRuntime"
						criticalErrSummary.TargetSource = scanURLFile
						criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Scheduler error: %v", err)}

						notificationHelper.SendCriticalErrorNotification(
							context.Background(),
							"SchedulerRuntime",
							criticalErrSummary,
						)
						zLogger.Error().Err(err).Msg("Scheduler error")
					}
				}
				zLogger.Info().Msg("Automated mode (with scheduler) completed or interrupted.")
				if *schedulerPtr != nil && ctx.Err() == context.Canceled {
					zLogger.Info().Msg("Context cancelled, ensuring scheduler is stopped.")
					(*schedulerPtr).Stop()
				}
			}
		}

		// Start MonitoringService if monitor targets are provided
		if monitorURLFile != "" {
			// Load monitor URLs from file
			monitorURLs, targetSource := loadUrls(gCfg, monitorURLFile, zLogger, notificationHelper)
			if len(monitorURLs) == 0 {
				zLogger.Warn().Str("file", monitorURLFile).Str("target_source", targetSource).Msg("No valid monitor URLs found in file")
				return
			}

			// Start monitoring service with the loaded URLs
			if err := monitoringService.Start(monitorURLs); err != nil {
				zLogger.Error().Err(err).Msg("Failed to start monitoring service")
				return
			}
			zLogger.Info().Int("monitor_urls", len(monitorURLs)).Msg("Monitoring service started successfully")

			// Wait for context cancellation
			<-ctx.Done()
			zLogger.Info().Msg("Context cancelled, stopping monitoring service...")

			// Send notification about monitoring service interruption
			interruptSummary := models.GetDefaultScanSummaryData()
			interruptSummary.Component = "MonitoringService"
			interruptSummary.Status = string(models.ScanStatusInterrupted)
			interruptSummary.TargetSource = targetSource
			interruptSummary.Targets = monitorURLs
			interruptSummary.TotalTargets = len(monitorURLs)
			interruptSummary.ErrorMessages = []string{"Monitoring service was interrupted by user or system signal"}

			if notificationHelper != nil {
				// Use monitor service notification type for proper webhook routing
				notificationHelper.SendMonitorInterruptNotification(context.Background(), interruptSummary)
			}
		}
	} else {
		runOnetimeScan(ctx, gCfg, scanURLFile, zLogger, notificationHelper)
	}
}

func shutdownServices(
	monitoringService *monitor.MonitoringService,
	scheduler *scheduler.Scheduler,
	monitorWg *sync.WaitGroup,
	zLogger zerolog.Logger,
	ctx context.Context,
) {
	if monitoringService != nil {
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

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	flags := parseFlags()

	gCfg, zLogger, err := loadConfigurationAndLogger(flags)
	if err != nil {
		if gCfg == nil || zLogger.GetLevel() == zerolog.Disabled {
			fmt.Fprintf(os.Stderr, "[FATAL] Main: %v\n", err)
			os.Exit(1)
		} else {
			zLogger.Fatal().Err(err).Msg("Initialization error")
		}
	}

	discordNotifier, err := notifier.NewDiscordNotifier(zLogger, &http.Client{Timeout: 20 * time.Second})
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier infra.")
	}
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	if flags.monitorTargetsFile != "" && gCfg.Mode == "onetime" {
		errMsg := "Invalid combination: --monitor-targets (-mt) cannot be used with --mode onetime (-m onetime). File monitoring is only available in automated mode."
		if zLogger.GetLevel() != zerolog.Disabled {
			zLogger.Fatal().Str("monitor_target_file", flags.monitorTargetsFile).Str("mode", gCfg.Mode).Msg(errMsg)
		} else {
			fmt.Fprintln(os.Stderr, "[FATAL] "+errMsg)
		}
		os.Exit(1)
	}

	monitoringService, err := initializeMonitoringServices(gCfg, flags.monitorTargetsFile, zLogger, notificationHelper)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize services")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandling(cancel, zLogger)

	var schedulerPtr *scheduler.Scheduler
	runApplicationLogic(ctx, gCfg, flags, zLogger, notificationHelper, monitoringService, &schedulerPtr)

	var monitorWg sync.WaitGroup
	shutdownServices(monitoringService, schedulerPtr, &monitorWg, zLogger, ctx)
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFileArgument string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper) {
	scanOrchestrator := initializeScanOrchestrator(gCfg, appLogger)

	// Load seed URLs
	seedURLs, targetSource := loadUrls(gCfg, urlListFileArgument, appLogger, notificationHelper)
	if len(seedURLs) == 0 {
		appLogger.Info().Msg("Onetime scan: No seed URLs loaded, scan will not run. Monitoring service (if active) will continue until application termination.")
		return // Error already handled in loadSeedURLs, or no targets were specified.
	}

	// Send start notification
	sendScanStartNotification(ctx, seedURLs, targetSource, "onetime", notificationHelper, appLogger)

	// Execute complete scan workflow
	scanSessionID := time.Now().Format("20060102-150405")
	// summaryData is used to populate the interruption summary if the scan is interrupted.
	var summaryData models.ScanSummaryData = models.GetDefaultScanSummaryData()
	var probeResults []models.ProbeResult
	var workflowErr error

	summaryData, probeResults, _, workflowErr = scanOrchestrator.ExecuteCompleteScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)

	// Check workflow error first, then check context cancellation
	if errors.Is(workflowErr, context.Canceled) || ctx.Err() == context.Canceled {
		appLogger.Info().Msg("Onetime scan workflow interrupted.")
		interruptionSummary := models.GetDefaultScanSummaryData()
		interruptionSummary.ScanSessionID = scanSessionID
		interruptionSummary.TargetSource = targetSource
		interruptionSummary.Targets = seedURLs
		interruptionSummary.TotalTargets = len(seedURLs)
		interruptionSummary.Status = string(models.ScanStatusInterrupted)
		interruptionSummary.ScanMode = "onetime"
		interruptionSummary.ErrorMessages = []string{"Onetime scan interrupted by signal or context cancellation."}

		// If workflowErr is not context.Canceled (e.g., another error occurred before cancellation was fully processed)
		// or if summaryData seems to have been partially populated, try to use it.
		if workflowErr != nil && !errors.Is(workflowErr, context.Canceled) {
			interruptionSummary.ErrorMessages = append(interruptionSummary.ErrorMessages, fmt.Sprintf("Additional error during interruption: %v", workflowErr))
		}

		// Populate with any partial data if available from summaryData returned by ExecuteCompleteScanWorkflow
		// Check a field like TargetSource which should be set if summaryData was meaningfully populated.
		if summaryData.TargetSource != "" {
			if summaryData.ProbeStats.TotalProbed > 0 || summaryData.ProbeStats.SuccessfulProbes > 0 || summaryData.ProbeStats.FailedProbes > 0 {
				interruptionSummary.ProbeStats = summaryData.ProbeStats
			}
			if summaryData.DiffStats.New > 0 || summaryData.DiffStats.Old > 0 || summaryData.DiffStats.Existing > 0 {
				interruptionSummary.DiffStats = summaryData.DiffStats
			}
			// if summaryData.ScanDuration > 0 { // Duration might be very short if interrupted early
			// 	interruptionSummary.ScanDuration = summaryData.ScanDuration
			// }
		}

		appLogger.Info().Interface("interruption_summary", interruptionSummary).Msg("Attempting to send onetime scan interruption notification.")
		notificationHelper.SendScanCompletionNotification(context.Background(), interruptionSummary, notifier.ScanServiceNotification, nil)
		return
	}

	// Handle other workflow errors (nếu không phải là Canceled)
	if workflowErr != nil {
		handleWorkflowError(summaryData, workflowErr, "onetime", notificationHelper, appLogger)
		return
	}

	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	// Generate and send completion notification
	generateReportAndNotify(ctx, gCfg, summaryData, probeResults, scanSessionID, "onetime", notificationHelper, appLogger)

	// Note: monitoringService.Stop() was removed from here.
	// The monitoring service will be stopped when the main context is cancelled.
	appLogger.Info().Msg("Onetime scan specific tasks finished. Application will now exit.")
}

func initializeScanOrchestrator(gCfg *config.GlobalConfig, appLogger zerolog.Logger) *orchestrator.Orchestrator {
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	scanOrchestrator := orchestrator.NewOrchestrator(gCfg, appLogger, parquetReader, parquetWriter)
	return scanOrchestrator
}

func loadUrls(
	gCfg *config.GlobalConfig,
	urlFileArgument string,
	appLogger zerolog.Logger,
	notificationHelper *notifier.NotificationHelper,
) ([]string, string) {
	var Urls []string
	var targetSource string

	// 1. Use urlListFileArgument (from -scan-targets or -monitor-targets) if provided
	if urlFileArgument != "" {
		loadedURLs, errFile := urlhandler.ReadURLsFromFile(urlFileArgument, appLogger)
		if errFile != nil {
			appLogger.Error().Err(errFile).Str("file", urlFileArgument).Msg("Failed to load URLs from file. See previous logs for details.")

			criticalErrSummary := models.GetDefaultScanSummaryData()
			criticalErrSummary.ScanMode = gCfg.Mode
			criticalErrSummary.TargetSource = urlFileArgument
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to load URLs from file '%s': %v. Check application logs.", urlFileArgument, errFile)}

			notificationHelper.SendCriticalErrorNotification(
				context.Background(),
				"URLFileLoad",
				criticalErrSummary,
			)
		}

		Urls = loadedURLs
		targetSource = filepath.Base(urlFileArgument)

		if len(Urls) == 0 && errFile == nil { // File existed and was read, but was empty
			appLogger.Warn().Str("file", urlFileArgument).Msg("Provided URL file was empty. Will attempt to use URLs from config if available.")
		} else if len(Urls) == 0 && errFile != nil { // File loading failed
			appLogger.Warn().Str("file", urlFileArgument).Msg("Failed to load URLs from file. Will attempt to use URLs from config if available.")
		}
	}

	// 2. Fallback to InputConfig.InputURLs if no seeds from file argument
	if len(Urls) == 0 {
		if len(gCfg.InputConfig.InputURLs) > 0 {
			appLogger.Info().Int("count", len(gCfg.InputConfig.InputURLs)).Msg("Using seed URLs from global input_config.input_urls")
			Urls = gCfg.InputConfig.InputURLs
			targetSource = "config_input_urls"
		} else if gCfg.InputConfig.InputFile != "" { // 3. Fallback to InputConfig.InputFile
			appLogger.Info().Str("file", gCfg.InputConfig.InputFile).Msg("Using seed URLs from input_config.input_file")
			loadedURLs, errFile := urlhandler.ReadURLsFromFile(gCfg.InputConfig.InputFile, appLogger)
			if errFile != nil {
				appLogger.Error().Err(errFile).Str("file", gCfg.InputConfig.InputFile).Msg("Failed to load URLs from config input_file.")
			} else {
				Urls = loadedURLs
				targetSource = filepath.Base(gCfg.InputConfig.InputFile)
				if len(Urls) == 0 {
					appLogger.Warn().Str("file", gCfg.InputConfig.InputFile).Msg("Config input_file was empty.")
				}
			}
		}
	}

	// Final check for seed URLs
	if len(Urls) == 0 {
		if targetSource != "" {
			noTargetsSummary := models.GetDefaultScanSummaryData()
			noTargetsSummary.TargetSource = targetSource
			noTargetsSummary.Status = string(models.ScanStatusNoTargets)
			noTargetsSummary.ErrorMessages = []string{"No URLs provided or loaded."}
			notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, notifier.ScanServiceNotification, nil)
		} else {
			appLogger.Info().Msg("No targets specified for onetime scan via CLI or config. 'NO_TARGETS' notification will be skipped.")
		}
		return nil, ""
	}

	return Urls, targetSource
}

func sendScanStartNotification(ctx context.Context, seedURLs []string, targetSource string, scanMode string, notificationHelper *notifier.NotificationHelper, appLogger zerolog.Logger) {
	appLogger.Info().Int("count", len(seedURLs)).Str("source", targetSource).Msg("Starting onetime scan with seed URLs.")

	startSummary := models.GetDefaultScanSummaryData()
	startSummary.ScanSessionID = time.Now().Format("20060102-150405")
	startSummary.TargetSource = targetSource
	startSummary.ScanMode = scanMode
	startSummary.Targets = seedURLs
	startSummary.TotalTargets = len(seedURLs)
	notificationHelper.SendScanStartNotification(ctx, startSummary)
}

func handleWorkflowError(summaryData models.ScanSummaryData, workflowErr error, scanMode string, notificationHelper *notifier.NotificationHelper, appLogger zerolog.Logger) {
	summaryData.Status = string(models.ScanStatusFailed)
	summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
	summaryData.ScanMode = scanMode
	notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification, nil)
	appLogger.Error().Err(workflowErr).Msg("Scan workflow execution failed")
}

func generateReportAndNotify(ctx context.Context, gCfg *config.GlobalConfig, summaryData models.ScanSummaryData, probeResults []models.ProbeResult, scanSessionID string, scanMode string, notificationHelper *notifier.NotificationHelper, appLogger zerolog.Logger) {
	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		summaryData.ScanMode = scanMode
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification, nil)
		appLogger.Error().Err(err).Msg("Failed to initialize HTML reporter")
		return
	}

	// Base output path, parts will be derived from this
	baseReportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode)
	baseReportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, baseReportFilename)

	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	// Correctly assign two return values from GenerateReport
	reportFilePaths, reportGenErr := htmlReporter.GenerateReport(probeResultsPtr, baseReportPath)
	if reportGenErr != nil {
		summaryData.Status = string(models.ScanStatusFailed) // Or models.ScanStatusPartialComplete
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to generate HTML report(s): %v", reportGenErr))
		summaryData.ScanMode = scanMode
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification, nil)
		appLogger.Error().Err(reportGenErr).Msg("Failed to generate HTML report(s)")
		return
	}

	if len(reportFilePaths) == 0 {
		appLogger.Info().Msg("HTML report generation resulted in no files (e.g., no data and generate_empty_report is false).")
		summaryData.ReportPath = "" // Ensure it's clear no report was generated
	} else {
		appLogger.Info().Strs("paths", reportFilePaths).Msg("HTML report(s) generated successfully.")
		// For summary, we can point to the first part or a general message
		if len(reportFilePaths) == 1 {
			summaryData.ReportPath = reportFilePaths[0]
		} else {
			summaryData.ReportPath = fmt.Sprintf("Multiple report files generated (%d parts), see notifications.", len(reportFilePaths))
		}
	}

	summaryData.Status = string(models.ScanStatusCompleted)
	summaryData.ScanMode = scanMode

	notificationHelper.SendScanCompletionNotification(ctx, summaryData, notifier.ScanServiceNotification, reportFilePaths)

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}
