package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/datastore"
	"monsterinc/internal/logger"
	"monsterinc/internal/models"
	"monsterinc/internal/monitor"
	"monsterinc/internal/notifier"
	"monsterinc/internal/orchestrator"
	"monsterinc/internal/reporter"
	"monsterinc/internal/scheduler"
	"monsterinc/internal/secrets"
	"monsterinc/internal/urlhandler"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Flags
	urlListFile := flag.String("scan-targets", "", "Path to a text file containing seed URLs for the main scan. Used if --diff-target-file is not set. This flag is for backward compatibility.")
	urlListFileAlias := flag.String("st", "", "Alias for -scan-targets")

	monitorTargetFile := flag.String("monitor-targets", "", "Path to a text file containing JS/HTML URLs for file monitoring (only in automated mode).")
	monitorTargetFileAlias := flag.String("mt", "", "Alias for --monitor-targets")

	globalConfigFile := flag.String("globalconfig", "", "Path to the global YAML/JSON configuration file. If not set, searches default locations.")
	globalConfigFileAlias := flag.String("gc", "", "Alias for --globalconfig")

	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (overrides config file if set)")
	modeFlagAlias := flag.String("m", "", "Alias for --mode")
	flag.Parse()

	// Consolidate alias flags
	if *urlListFile == "" && *urlListFileAlias != "" {
		*urlListFile = *urlListFileAlias
	}
	if *monitorTargetFile == "" && *monitorTargetFileAlias != "" {
		*monitorTargetFile = *monitorTargetFileAlias
	}
	if *globalConfigFile == "" && *globalConfigFileAlias != "" {
		*globalConfigFile = *globalConfigFileAlias
	}
	if *modeFlag == "" && *modeFlagAlias != "" {
		*modeFlag = *modeFlagAlias
	}

	// Check required --mode
	if *modeFlag == "" {
		log.Fatalln("[FATAL] --mode argument is required (onetime or automated)")
	}

	// Load Global Configuration (path determined by globalConfigFile flag)
	log.Println("[INFO] Main: Attempting to load global configuration...")
	gCfg, err := config.LoadGlobalConfig(*globalConfigFile)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not load global config using path '%s': %v", *globalConfigFile, err)
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
			zLogger.Fatal().Err(err).Str("directory", gCfg.ReporterConfig.OutputDir).Msg("Could not create default report output directory before validation")
		}
	}

	// Validate the loaded configuration
	if err := config.ValidateConfig(gCfg); err != nil {
		zLogger.Fatal().Err(err).Msg("Configuration validation failed")
	}
	zLogger.Info().Msg("Configuration validated successfully.")

	// Initialize DiscordNotifier (without specific webhook URL at this stage)
	discordNotifier, err := notifier.NewDiscordNotifier(zLogger, &http.Client{Timeout: 20 * time.Second})
	if err != nil {
		// This error is from NewDiscordNotifier if something fundamental fails, not webhook related anymore.
		zLogger.Fatal().Err(err).Msg("Failed to initialize DiscordNotifier infra.")
	}
	// NotificationHelper now holds the NotificationConfig and decides which webhook to use.
	notificationHelper := notifier.NewNotificationHelper(discordNotifier, gCfg.NotificationConfig, zLogger)

	// --- Secrets Service Initialization ---
	var secretStore datastore.SecretsStore
	var secretDetector *secrets.SecretDetectorService
	var errSecretService error

	if gCfg.SecretsConfig.Enabled {
		zLogger.Info().Msg("Secrets detection is enabled. Initializing services...")
		secretStore, errSecretService = datastore.NewParquetSecretsStore(&gCfg.StorageConfig, zLogger)
		if errSecretService != nil {
			zLogger.Error().Err(errSecretService).Msg("Failed to initialize ParquetSecretsStore. Secret detection storage will be compromised.")
			// Potentially send a critical notification or handle this as fatal depending on policy
			// For now, we log and secretDetector will be nil if store fails.
			secretStore = nil // Ensure store is nil if it failed
		}

		if secretStore != nil { // Only init detector if store was successful
			secretDetector, errSecretService = secrets.NewSecretDetectorService(gCfg, secretStore, zLogger, notificationHelper)
			if errSecretService != nil {
				zLogger.Error().Err(errSecretService).Msg("Failed to initialize SecretDetectorService. Secret detection will be compromised.")
				// Potentially send a critical notification
				secretDetector = nil // Ensure detector is nil if it failed
			} else {
				zLogger.Info().Msg("SecretDetectorService initialized successfully.")
			}
		} else {
			zLogger.Warn().Msg("ParquetSecretsStore failed to initialize. SecretDetectorService will not be initialized.")
		}
	} else {
		zLogger.Info().Msg("Secrets detection is disabled in the configuration.")
	}

	// --- Monitoring Service Initialization ---
	var monitoringService *monitor.MonitoringService
	var monitorWg sync.WaitGroup
	var schedulerInstance *scheduler.Scheduler // Declare schedulerInstance here to be accessible by signal handler

	// Only initialize and run monitoring service in automated mode, if enabled in config, AND if --monitor-target-file is provided
	if gCfg.Mode == "automated" && gCfg.MonitorConfig.Enabled {
		if *monitorTargetFile != "" {
			zLogger.Info().Msg("File monitoring service is enabled, in automated mode, and --monitor-target-file is provided. Initializing...")
			fileHistoryStore, fhStoreErr := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, zLogger)
			if fhStoreErr != nil {
				zLogger.Error().Err(fhStoreErr).Msg("Failed to initialize ParquetFileHistoryStore for monitoring. Monitoring will be disabled.")
				// No need to assign to monitoringService, it will remain nil
			} else {
				monitorHTTPClientTimeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
				if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
					monitorHTTPClientTimeout = 30 * time.Second // Default if not set or invalid - USER CAN CONFIRM THIS VALUE
					zLogger.Warn().Int("configured_timeout", gCfg.MonitorConfig.HTTPTimeoutSeconds).Dur("default_timeout", monitorHTTPClientTimeout).Msg("Monitor HTTPTimeoutSeconds invalid or not set, using default")
				}
				monitorHTTPClient := &http.Client{
					Timeout: monitorHTTPClientTimeout,
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					},
				}
				monitorLogger := zLogger.With().Str("service", "FileMonitor").Logger()

				if gCfg.SecretsConfig.Enabled && secretDetector != nil {
					monitoringService = monitor.NewMonitoringService(
						&gCfg.MonitorConfig,
						&gCfg.CrawlerConfig,
						&gCfg.ExtractorConfig,
						&gCfg.NotificationConfig,
						&gCfg.ReporterConfig,
						&gCfg.DiffReporterConfig,
						&gCfg.SecretsConfig,
						fileHistoryStore,
						monitorLogger,
						notificationHelper,
						monitorHTTPClient,
						secretDetector,
					)
				} else {
					monitoringService = monitor.NewMonitoringService(
						&gCfg.MonitorConfig,
						&gCfg.CrawlerConfig,
						&gCfg.ExtractorConfig,
						&gCfg.NotificationConfig,
						&gCfg.ReporterConfig,
						&gCfg.DiffReporterConfig,
						&gCfg.SecretsConfig,
						fileHistoryStore,
						monitorLogger,
						notificationHelper,
						monitorHTTPClient,
						nil,
					)
				}
				zLogger.Info().Msg("File monitoring service initialized.")
			}
		} else {
			zLogger.Info().Msg("File monitoring service is configured and enabled in automated mode, but will NOT start because the --monitor-target-file (-mtf) flag was not provided.")
		}
	} else if gCfg.MonitorConfig.Enabled { // This implies Mode is not "automated" or MonitorConfig.Enabled is false (but caught by outer if)
		if gCfg.Mode != "automated" {
			zLogger.Info().Str("current_mode", gCfg.Mode).Msg("File monitoring is enabled in config, but will only run in 'automated' mode and if --monitor-target-file is specified.")
		}
		// If MonitorConfig.Enabled is false, no message is printed here, which is fine.
	}

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
			interruptionSummary.Status = string(models.ScanStatusInterrupted)
			interruptionSummary.ErrorMessages = []string{fmt.Sprintf("Application (%s mode) interrupted by signal: %s", gCfg.Mode, sig.String())}
			notificationCtx, notificationCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer notificationCancel()

			var sendInterruptionNotification bool = false
			var interruptionServiceType notifier.NotificationServiceType = notifier.ScanServiceNotification // Default for onetime or if logic below doesn't set

			if gCfg.Mode == "automated" {
				isMonitorActive := monitoringService != nil
				isSchedulerActive := schedulerInstance != nil

				if isSchedulerActive { // If scheduler is active, it's the primary context
					interruptionServiceType = notifier.ScanServiceNotification
					if gCfg.NotificationConfig.ScanServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				} else if isMonitorActive { // Scheduler is not active, but Monitor is
					interruptionServiceType = notifier.MonitorServiceNotification
					if gCfg.NotificationConfig.MonitorServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				} else { // Neither Scheduler nor Monitor is active in automated mode
					zLogger.Info().Msg("Interruption in automated mode: Neither monitor nor scheduler was active. Notification will be skipped.")
					sendInterruptionNotification = false
				}
			} else { // Onetime mode
				// interruptionServiceType is already default ScanServiceNotification
				if gCfg.NotificationConfig.ScanServiceDiscordWebhookURL != "" {
					sendInterruptionNotification = true
				}
			}

			if sendInterruptionNotification {
				notificationHelper.SendScanCompletionNotification(notificationCtx, interruptionSummary, interruptionServiceType)
				zLogger.Info().Msg("Interruption notification sent.")
			} else {
				// Avoid double logging if already logged for automated+neither_active case
				if !(gCfg.Mode == "automated" && monitoringService == nil && schedulerInstance == nil) {
					zLogger.Info().Str("service_type_considered", string(interruptionServiceType)).Msg("Interruption notification skipped: Webhook not configured for the determined service type or no service deemed active for notification.")
				}
			}
		}
		cancel()
	}()

	// Start Monitoring Service if initialized (which implies automated mode and enabled in config)
	if monitoringService != nil {
		monitorWg.Add(1)
		go func() {
			defer monitorWg.Done()
			zLogger.Info().Msg("Starting file monitoring service...")

			initialMonitorURLs := []string{}
			// 1. Load from --monitor-target-file if provided
			if *monitorTargetFile != "" {
				zLogger.Info().Str("file", *monitorTargetFile).Msg("Loading initial monitor URLs from --monitor-target-file")
				urlsFromFile, err := urlhandler.ReadURLsFromFile(*monitorTargetFile, zLogger) // Assuming urlhandler.ReadURLsFromFile exists and is suitable
				if err != nil {
					zLogger.Error().Err(err).Str("file", *monitorTargetFile).Msg("Failed to load URLs from monitor target file. Continuing without them.")
				} else {
					initialMonitorURLs = append(initialMonitorURLs, urlsFromFile...)
				}
			}

			// 2. Append URLs from config's InitialMonitorURLs
			if len(gCfg.MonitorConfig.InitialMonitorURLs) > 0 {
				zLogger.Info().Int("count", len(gCfg.MonitorConfig.InitialMonitorURLs)).Msg("Appending initial monitor URLs from config file.")
				initialMonitorURLs = append(initialMonitorURLs, gCfg.MonitorConfig.InitialMonitorURLs...)
			}

			if len(initialMonitorURLs) == 0 {
				zLogger.Info().Msg("No initial URLs provided for monitoring service (via CLI flag or config). Service will start with an empty list.")
			}

			if err := monitoringService.Start(initialMonitorURLs); err != nil {
				zLogger.Error().Err(err).Msg("File monitoring service failed to start")
			}
			// Wait for context cancellation
			<-ctx.Done()
			zLogger.Info().Msg("Stopping file monitoring service due to context cancellation...")
			monitoringService.Stop()
			zLogger.Info().Msg("File monitoring service stopped.")
		}()
	}

	// Determine the primary URL file for main scanning/diffing
	mainScanURLFile := ""
	if *urlListFile != "" {
		mainScanURLFile = *urlListFile
		zLogger.Info().Str("file", mainScanURLFile).Msg("Using -urlfile for main scan targets.")
	} else {
		zLogger.Info().Msg("-urlfile not provided, will rely on configuration for scan targets if in onetime mode, or no initial targets for scheduler if in automated mode.")
	} // If empty, mainScanURLFile remains "", and downstream logic will handle fallback to config.

	// Main application logic based on mode
	if gCfg.Mode == "automated" {
		if mainScanURLFile == "" {
			zLogger.Info().Msg("Automated mode: -urlfile (or -uf) was not provided. Scheduler module will not be started. Monitoring service will run if enabled.")
			// Scheduler is not started, but monitoring (if enabled) and main app lifecycle (signal handling) will continue.
		} else {
			zLogger.Info().Str("urlfile", mainScanURLFile).Msg("Automated mode: -urlfile provided. Initializing and starting scheduler module...")
			var errScheduler error // Declare error variable for scheduler
			schedulerInstance, errScheduler = scheduler.NewScheduler(gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector)
			if errScheduler != nil {
				criticalErrSummary := models.GetDefaultScanSummaryData()
				criticalErrSummary.Component = "SchedulerInitialization"
				criticalErrSummary.TargetSource = mainScanURLFile // Use the determined file for source
				criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", errScheduler)}
				notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", criticalErrSummary)
				zLogger.Fatal().Err(errScheduler).Msg("Failed to initialize scheduler")
			}

			if err := schedulerInstance.Start(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					zLogger.Info().Msg("Scheduler stopped due to context cancellation (interrupt).")
				} else {
					criticalErrSummary := models.GetDefaultScanSummaryData()
					criticalErrSummary.Component = "SchedulerRuntime"
					criticalErrSummary.TargetSource = mainScanURLFile
					criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Scheduler error: %v", err)}
					notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerRuntime", criticalErrSummary)
					zLogger.Error().Err(err).Msg("Scheduler error")
				}
			}
			zLogger.Info().Msg("Automated mode (with scheduler) completed or interrupted.")
		}
	} else {
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(ctx, gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector)
	}

	// Wait for monitoring service to stop if it was started
	if monitoringService != nil {
		zLogger.Info().Msg("Waiting for file monitoring service to shut down completely...")
		monitorWg.Wait() // Wait for the monitoring service goroutine to finish
		zLogger.Info().Msg("File monitoring service has shut down.")
	}

	if ctx.Err() == context.Canceled {
		zLogger.Info().Msg("Application shutting down due to context cancellation.")
	} else {
		zLogger.Info().Msg("Application finished.")
	}
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFileArgument string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService) {
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter, secretDetector)

	var seedURLs []string
	var onetimeTargetSource string

	// 1. Use urlListFileArgument (from -urlfile) if provided
	if urlListFileArgument != "" {
		appLogger.Info().Str("file", urlListFileArgument).Msg("Attempting to read seed URLs from provided file argument for onetime scan.")
		loadedURLs, errFile := urlhandler.ReadURLsFromFile(urlListFileArgument, appLogger)
		if errFile != nil {
			appLogger.Error().Err(errFile).Str("file", urlListFileArgument).Msg("Failed to load URLs from file. See previous logs for details.")
			criticalErrSummary := models.GetDefaultScanSummaryData()
			criticalErrSummary.TargetSource = urlListFileArgument
			criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to load URLs from file '%s': %v. Check application logs.", urlListFileArgument, errFile)}
			notificationHelper.SendCriticalErrorNotification(context.Background(), "OnetimeScanURLFileLoad", criticalErrSummary)
			// Do not return yet, allow fallback to config if file load fails but wasn't strictly required to exist
		}
		seedURLs = loadedURLs
		onetimeTargetSource = filepath.Base(urlListFileArgument)
		if len(seedURLs) == 0 && errFile == nil { // File existed and was read, but was empty
			appLogger.Warn().Str("file", urlListFileArgument).Msg("Provided URL file was empty. Will attempt to use seeds from config if available.")
		} else if len(seedURLs) == 0 && errFile != nil { // File loading failed
			appLogger.Warn().Str("file", urlListFileArgument).Msg("Failed to load URLs from file. Will attempt to use seeds from config if available.")
		}
	}

	// 2. Fallback to InputConfig.InputURLs if no seeds from file argument
	if len(seedURLs) == 0 {
		if len(gCfg.InputConfig.InputURLs) > 0 {
			appLogger.Info().Int("count", len(gCfg.InputConfig.InputURLs)).Msg("Using seed URLs from global input_config.input_urls for onetime scan.")
			seedURLs = gCfg.InputConfig.InputURLs
			onetimeTargetSource = "config_input_urls"
		} else if gCfg.InputConfig.InputFile != "" { // 3. Fallback to InputConfig.InputFile
			appLogger.Info().Str("file", gCfg.InputConfig.InputFile).Msg("Using seed URLs from input_config.input_file for onetime scan.")
			loadedURLs, errFile := urlhandler.ReadURLsFromFile(gCfg.InputConfig.InputFile, appLogger)
			if errFile != nil {
				appLogger.Error().Err(errFile).Str("file", gCfg.InputConfig.InputFile).Msg("Failed to load URLs from config input_file.")
				// critical error if this was the last resort and it fails?
			} else {
				seedURLs = loadedURLs
				onetimeTargetSource = filepath.Base(gCfg.InputConfig.InputFile)
				if len(seedURLs) == 0 {
					appLogger.Warn().Str("file", gCfg.InputConfig.InputFile).Msg("Config input_file was empty.")
				}
			}
		}
	}

	// Final check for seed URLs
	if len(seedURLs) == 0 {
		noSeedsMsg := "No seed URLs provided or loaded for onetime scan. Please specify via -urlfile, or in input_config in the config file. Onetime scan will not run."
		// This is not a critical error for the application to stop, but onetime scan won't run.
		// Notification of "no seeds" might still be relevant if onetime mode was explicitly chosen.
		appLogger.Warn().Msg(noSeedsMsg) // Changed from Error to Warn

		// Send a specific type of notification or a regular completion with status "NO_TARGETS"
		noTargetsSummary := models.GetDefaultScanSummaryData()
		if onetimeTargetSource == "" {
			noTargetsSummary.TargetSource = "NotProvided"
		} else {
			noTargetsSummary.TargetSource = onetimeTargetSource // Could be 'config_input_urls' if that was tried and empty
		}
		noTargetsSummary.Status = string(models.ScanStatusNoTargets) // New status
		noTargetsSummary.ErrorMessages = []string{noSeedsMsg}
		notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, notifier.ScanServiceNotification)
		return
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
	probeResults, urlDiffResults, secretFindings, workflowErr := scanOrchestrator.ExecuteScanWorkflow(ctx, seedURLs, scanSessionID)
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

	// Log secret findings summary
	if len(secretFindings) > 0 {
		appLogger.Info().Int("secret_findings_count", len(secretFindings)).Msg("Secret detection found findings during scan")
	}

	if workflowErr != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = []string{fmt.Sprintf("Scan workflow execution failed: %v", workflowErr)}
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification)
		appLogger.Error().Err(workflowErr).Msg("Scan workflow execution failed")
		return
	}
	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification)
		appLogger.Error().Err(err).Msg("Failed to initialize HTML reporter")
		return
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode)
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	if err := htmlReporter.GenerateReport(probeResultsPtr, urlDiffResults, secretFindings, reportPath); err != nil {
		summaryData.Status = string(models.ScanStatusFailed) // Or models.ScanStatusPartialComplete
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to generate HTML report: %v", err))
		notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification)
		appLogger.Error().Err(err).Msg("Failed to generate HTML report")
		return
	}

	// Check if report file was actually created
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		appLogger.Info().Str("path", reportPath).Msg("HTML report was skipped (no data to report)")
		summaryData.ReportPath = "" // Clear report path since no file was created
	} else {
		appLogger.Info().Str("path", reportPath).Msg("HTML report generated successfully")
		summaryData.ReportPath = reportPath
	}

	summaryData.Status = string(models.ScanStatusCompleted)

	notificationHelper.SendScanCompletionNotification(ctx, summaryData, notifier.ScanServiceNotification)

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}
