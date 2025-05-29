package main

import (
	"context"
	"crypto/tls"
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

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/aleister1102/monsterinc/internal/datastore"
	"github.com/aleister1102/monsterinc/internal/logger"
	"github.com/aleister1102/monsterinc/internal/models"
	"github.com/aleister1102/monsterinc/internal/monitor"
	"github.com/aleister1102/monsterinc/internal/notifier"
	"github.com/aleister1102/monsterinc/internal/orchestrator"
	"github.com/aleister1102/monsterinc/internal/reporter"
	"github.com/aleister1102/monsterinc/internal/scheduler"
	"github.com/aleister1102/monsterinc/internal/secrets"
	"github.com/aleister1102/monsterinc/internal/urlhandler"
	"github.com/rs/zerolog"
)

type appFlags struct {
	urlListFile       string
	monitorTargetFile string
	globalConfigFile  string
	mode              string
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
		flags.monitorTargetFile = *monitorTargetFile
	} else if *monitorTargetFileAlias != "" {
		flags.monitorTargetFile = *monitorTargetFileAlias
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

	if flags.mode == "" {
		fmt.Fprintln(os.Stderr, "[FATAL] --mode argument is required (onetime or automated)")
		os.Exit(1)
	}
	return flags
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

func initializeServices(gCfg *config.GlobalConfig, zLogger zerolog.Logger, flags appFlags, notificationHelper *notifier.NotificationHelper) (*monitor.MonitoringService, *secrets.SecretDetectorService, *scheduler.Scheduler, error) {
	var secretStore datastore.SecretsStore
	var secretDetector *secrets.SecretDetectorService
	var errSecretService error

	if gCfg.SecretsConfig.Enabled {
		zLogger.Info().Msg("Secrets detection is enabled. Initializing services...")
		secretStore, errSecretService = datastore.NewParquetSecretsStore(&gCfg.StorageConfig, zLogger)
		if errSecretService != nil {
			zLogger.Error().Err(errSecretService).Msg("Failed to initialize ParquetSecretsStore. Secret detection storage will be compromised.")
			secretStore = nil
		}

		if secretStore != nil {
			secretDetector, errSecretService = secrets.NewSecretDetectorService(gCfg, secretStore, zLogger, notificationHelper)
			if errSecretService != nil {
				zLogger.Error().Err(errSecretService).Msg("Failed to initialize SecretDetectorService. Secret detection will be compromised.")
				secretDetector = nil
			} else {
				zLogger.Info().Msg("SecretDetectorService initialized successfully.")
			}
		} else {
			zLogger.Warn().Msg("ParquetSecretsStore failed to initialize. SecretDetectorService will not be initialized.")
		}
	} else {
		zLogger.Info().Msg("Secrets detection is disabled in the configuration.")
	}

	var monitoringService *monitor.MonitoringService
	if gCfg.MonitorConfig.Enabled && flags.monitorTargetFile != "" {
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service is enabled and --monitor-target-file is provided. Initializing...")
		fileHistoryStore, fhStoreErr := datastore.NewParquetFileHistoryStore(&gCfg.StorageConfig, zLogger)
		if fhStoreErr != nil {
			zLogger.Error().Err(fhStoreErr).Msg("Failed to initialize ParquetFileHistoryStore for monitoring. Monitoring will be disabled.")
		} else {
			monitorHTTPClientTimeout := time.Duration(gCfg.MonitorConfig.HTTPTimeoutSeconds) * time.Second
			if gCfg.MonitorConfig.HTTPTimeoutSeconds <= 0 {
				monitorHTTPClientTimeout = 30 * time.Second
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
			zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service initialized.")
		}
	} else if gCfg.MonitorConfig.Enabled && flags.monitorTargetFile == "" {
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service is enabled but --monitor-target-file was not provided. Monitoring will be skipped.")
	} else if !gCfg.MonitorConfig.Enabled {
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service is disabled in configuration.")
	}

	var schedulerInstance *scheduler.Scheduler
	// Scheduler is only initialized in automated mode and if a urlListFile is provided.
	// This logic will be handled in runApplicationLogic.

	return monitoringService, secretDetector, schedulerInstance, nil
}

func setupSignalHandling(ctx context.Context, cancel context.CancelFunc, gCfg *config.GlobalConfig, notificationHelper *notifier.NotificationHelper, monitoringService *monitor.MonitoringService, schedulerInstancePtr **scheduler.Scheduler, zLogger zerolog.Logger) {
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
			var interruptionServiceType notifier.NotificationServiceType = notifier.ScanServiceNotification

			// Dereference the pointer to get the actual *scheduler.Scheduler instance
			currentSchedulerInstance := *schedulerInstancePtr

			if gCfg.Mode == "automated" {
				isMonitorActive := monitoringService != nil
				isSchedulerActive := currentSchedulerInstance != nil // Use the dereferenced pointer

				if isSchedulerActive {
					interruptionServiceType = notifier.ScanServiceNotification
					if gCfg.NotificationConfig.ScanServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				} else if isMonitorActive {
					interruptionServiceType = notifier.MonitorServiceNotification
					if gCfg.NotificationConfig.MonitorServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				} else {
					zLogger.Info().Msg("Interruption in automated mode: Neither monitor nor scheduler was active. Notification will be skipped.")
					sendInterruptionNotification = false
				}
			} else { // Onetime mode
				isMonitorActive := monitoringService != nil

				if isMonitorActive {
					interruptionServiceType = notifier.MonitorServiceNotification
					if gCfg.NotificationConfig.MonitorServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				} else {
					interruptionServiceType = notifier.ScanServiceNotification
					if gCfg.NotificationConfig.ScanServiceDiscordWebhookURL != "" {
						sendInterruptionNotification = true
					}
				}
			}

			if sendInterruptionNotification {
				notificationHelper.SendScanCompletionNotification(notificationCtx, interruptionSummary, interruptionServiceType)
				zLogger.Info().Msg("Interruption notification sent.")
			} else {
				if !(gCfg.Mode == "automated" && monitoringService == nil && currentSchedulerInstance == nil) { // Use dereferenced pointer
					zLogger.Info().Str("service_type_considered", string(interruptionServiceType)).Msg("Interruption notification skipped: Webhook not configured for the determined service type or no service deemed active for notification.")
				}
			}
		}
		cancel()
	}()
}

func startMonitoringService(ctx context.Context, monitoringService *monitor.MonitoringService, monitorTargetFile string, gCfg *config.GlobalConfig, zLogger zerolog.Logger, monitorWg *sync.WaitGroup) {
	if monitoringService == nil {
		return
	}
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()
		zLogger.Info().Msg("Starting file monitoring service...")

		initialMonitorURLs := []string{}
		if monitorTargetFile != "" {
			zLogger.Info().Str("file", monitorTargetFile).Msg("Loading initial monitor URLs from --monitor-target-file")
			urlsFromFile, err := urlhandler.ReadURLsFromFile(monitorTargetFile, zLogger)
			if err != nil {
				zLogger.Error().Err(err).Str("file", monitorTargetFile).Msg("Failed to load URLs from monitor target file. Continuing without them.")
			} else {
				initialMonitorURLs = append(initialMonitorURLs, urlsFromFile...)
			}
		}

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
		<-ctx.Done()
		zLogger.Info().Msg("Stopping file monitoring service due to context cancellation...")
		monitoringService.Stop()
		zLogger.Info().Msg("File monitoring service stopped.")
	}()
}

func runApplicationLogic(ctx context.Context, gCfg *config.GlobalConfig, flags appFlags, zLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService, monitoringService *monitor.MonitoringService, schedulerInstancePtr **scheduler.Scheduler) {
	mainScanURLFile := ""
	if flags.urlListFile != "" {
		mainScanURLFile = flags.urlListFile
		zLogger.Info().Str("file", mainScanURLFile).Msg("Using -urlfile for main scan targets.")
	} else {
		zLogger.Info().Msg("-urlfile not provided, will rely on configuration for scan targets if in onetime mode, or no initial targets for scheduler if in automated mode.")
	}

	// var schedulerInstance *scheduler.Scheduler // No longer declare here, use the passed pointer

	if gCfg.Mode == "automated" {
		if mainScanURLFile == "" {
			zLogger.Info().Msg("Automated mode: -urlfile (or -uf) was not provided. Scheduler module will not be started. Monitoring service will run if enabled.")
		} else {
			zLogger.Info().Str("urlfile", mainScanURLFile).Msg("Automated mode: -urlfile provided. Initializing and starting scheduler module...")
			var errScheduler error
			// Assign to the dereferenced pointer
			*schedulerInstancePtr, errScheduler = scheduler.NewScheduler(gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector)
			if errScheduler != nil {
				criticalErrSummary := models.GetDefaultScanSummaryData()
				criticalErrSummary.Component = "SchedulerInitialization"
				criticalErrSummary.TargetSource = mainScanURLFile
				criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize scheduler: %v", errScheduler)}
				notificationHelper.SendCriticalErrorNotification(context.Background(), "SchedulerInitialization", criticalErrSummary)
				zLogger.Fatal().Err(errScheduler).Msg("Failed to initialize scheduler")
			}

			if *schedulerInstancePtr != nil { // Check the dereferenced pointer
				if err := (*schedulerInstancePtr).Start(ctx); err != nil { // Call Start on the dereferenced pointer
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
		}
	} else {
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(ctx, gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector)
		if monitoringService != nil {
			zLogger.Info().Msg("Onetime scan completed. Stopping monitoring service...")
			monitoringService.Stop()
		}
	}
	// No longer return schedulerInstance
}

func shutdownServices(monitoringService *monitor.MonitoringService, monitorWg *sync.WaitGroup, zLogger zerolog.Logger, ctx context.Context) {
	if monitoringService != nil {
		zLogger.Info().Msg("Waiting for file monitoring service to shut down completely...")
		monitorWg.Wait()
		zLogger.Info().Msg("File monitoring service has shut down.")
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

	monitoringService, secretDetector, _, err := initializeServices(gCfg, zLogger, flags, notificationHelper)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize services")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var schedulerInstance *scheduler.Scheduler // Declare schedulerInstance for the signal handler

	// Setup signal handling first, passing the address of schedulerInstance
	setupSignalHandling(ctx, cancel, gCfg, notificationHelper, monitoringService, &schedulerInstance, zLogger)

	// Now run application logic, which might populate schedulerInstance
	runApplicationLogic(ctx, gCfg, flags, zLogger, notificationHelper, secretDetector, monitoringService, &schedulerInstance)

	var monitorWg sync.WaitGroup
	startMonitoringService(ctx, monitoringService, flags.monitorTargetFile, gCfg, zLogger, &monitorWg)

	shutdownServices(monitoringService, &monitorWg, zLogger, ctx)
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFileArgument string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService) {
	// Initialize components
	scanOrchestrator := initializeScanOrchestrator(gCfg, appLogger, secretDetector)

	// Load seed URLs
	seedURLs, targetSource := loadSeedURLs(gCfg, urlListFileArgument, appLogger, notificationHelper)
	if len(seedURLs) == 0 {
		return // Error already handled in loadSeedURLs
	}

	// Send start notification
	sendScanStartNotification(ctx, seedURLs, targetSource, "onetime", notificationHelper, appLogger)

	// Execute complete scan workflow
	scanSessionID := time.Now().Format("20060102-150405")
	summaryData, probeResults, urlDiffResults, secretFindings, workflowErr := scanOrchestrator.ExecuteCompleteScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)

	if ctx.Err() == context.Canceled {
		appLogger.Info().Msg("Onetime scan workflow interrupted.")
		return
	}

	// Log secret findings summary
	if len(secretFindings) > 0 {
		appLogger.Info().Int("secret_findings_count", len(secretFindings)).Msg("Secret detection found findings during scan")
	}

	// Handle workflow errors
	if workflowErr != nil {
		handleWorkflowError(summaryData, workflowErr, "onetime", notificationHelper, appLogger)
		return
	}

	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	// Generate and send completion notification
	generateReportAndNotify(ctx, gCfg, summaryData, probeResults, urlDiffResults, secretFindings, scanSessionID, "onetime", notificationHelper, appLogger)
}

func initializeScanOrchestrator(gCfg *config.GlobalConfig, appLogger zerolog.Logger, secretDetector *secrets.SecretDetectorService) *orchestrator.ScanOrchestrator {
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Error().Err(parquetErr).Msg("Failed to initialize ParquetWriter for orchestrator. Parquet writing will be disabled.")
		parquetWriter = nil
	}

	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter, secretDetector)
	return scanOrchestrator
}

func loadSeedURLs(gCfg *config.GlobalConfig, urlListFileArgument string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper) ([]string, string) {
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
		noTargetsSummary.ScanMode = "onetime"
		if onetimeTargetSource == "" {
			noTargetsSummary.TargetSource = "NotProvided"
		} else {
			noTargetsSummary.TargetSource = onetimeTargetSource // Could be 'config_input_urls' if that was tried and empty
		}
		noTargetsSummary.Status = string(models.ScanStatusNoTargets) // New status
		noTargetsSummary.ErrorMessages = []string{noSeedsMsg}
		notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, notifier.ScanServiceNotification)
		return nil, ""
	}

	return seedURLs, onetimeTargetSource
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
	notificationHelper.SendScanCompletionNotification(context.Background(), summaryData, notifier.ScanServiceNotification)
	appLogger.Error().Err(workflowErr).Msg("Scan workflow execution failed")
}

func generateReportAndNotify(ctx context.Context, gCfg *config.GlobalConfig, summaryData models.ScanSummaryData, probeResults []models.ProbeResult, urlDiffResults map[string]models.URLDiffResult, secretFindings []models.SecretFinding, scanSessionID string, scanMode string, notificationHelper *notifier.NotificationHelper, appLogger zerolog.Logger) {
	appLogger.Info().Msg("Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		summaryData.Status = string(models.ScanStatusFailed)
		summaryData.ErrorMessages = append(summaryData.ErrorMessages, fmt.Sprintf("Failed to initialize HTML reporter: %v", err))
		summaryData.ScanMode = scanMode
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
		summaryData.ScanMode = scanMode
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
	summaryData.ScanMode = scanMode

	notificationHelper.SendScanCompletionNotification(ctx, summaryData, notifier.ScanServiceNotification)

	appLogger.Info().Msg("MonsterInc Crawler finished (onetime mode).")
}
