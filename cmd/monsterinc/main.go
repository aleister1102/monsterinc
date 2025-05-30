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
	unifiedFile       string
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

	unifiedFile := flag.String("f", "", "Path to a text file containing seed URLs for unified scan + monitor")

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

	if *unifiedFile != "" {
		flags.unifiedFile = *unifiedFile
	}

	// Auto-set mode to automated if using monitor-specific flags
	if flags.mode == "" {
		if flags.monitorTargetFile != "" || flags.unifiedFile != "" {
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
	// Rule 1: If using -mt, mode should not be onetime
	if flags.monitorTargetFile != "" && flags.mode == "onetime" {
		return fmt.Errorf("-mt (monitor targets) cannot be used with mode 'onetime'. Use 'automated' mode or omit mode flag")
	}

	// Rule 2: If using -f, it must be automated mode (or no mode specified, defaults to automated)
	if flags.unifiedFile != "" {
		if flags.mode != "" && flags.mode != "automated" {
			return fmt.Errorf("-f (unified file) can only be used with 'automated' mode")
		}
		// If -f is used, cannot use -st or -mt
		if flags.urlListFile != "" {
			return fmt.Errorf("-f (unified file) cannot be used together with -st (scan targets)")
		}
		if flags.monitorTargetFile != "" {
			return fmt.Errorf("-f (unified file) cannot be used together with -mt (monitor targets)")
		}
	}

	// Rule 3: Cannot use both -st and -mt together (they should use -f instead)
	if flags.urlListFile != "" && flags.monitorTargetFile != "" {
		return fmt.Errorf("-st and -mt cannot be used together. Use -f for unified scan + monitor on the same file")
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

func initializeServices(gCfg *config.GlobalConfig, zLogger zerolog.Logger, flags appFlags, notificationHelper *notifier.NotificationHelper) (*monitor.MonitoringService, *secrets.SecretDetectorService, *scheduler.UnifiedScheduler, error) {
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
	// Initialize monitoring service for both automated and onetime modes if enabled
	if gCfg.MonitorConfig.Enabled {
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service is enabled. Initializing...")
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
	} else {
		zLogger.Info().Str("mode", gCfg.Mode).Msg("File monitoring service is disabled in configuration.")
	}

	var unifiedSchedulerInstance *scheduler.UnifiedScheduler
	// UnifiedScheduler is only initialized in automated mode
	// This logic will be handled in runApplicationLogic

	return monitoringService, secretDetector, unifiedSchedulerInstance, nil
}

func setupSignalHandling(ctx context.Context, cancel context.CancelFunc, gCfg *config.GlobalConfig, notificationHelper *notifier.NotificationHelper, monitoringService *monitor.MonitoringService, zLogger zerolog.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		zLogger.Info().Str("signal", sig.String()).Msg("Received interrupt signal, initiating graceful shutdown...")
		cancel() // Only cancel the main context. Services should listen to this context.
	}()
}

func startMonitoringService(ctx context.Context, monitoringService *monitor.MonitoringService, monitorTargetFile string, gCfg *config.GlobalConfig, zLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, monitorWg *sync.WaitGroup) {
	if monitoringService == nil {
		return
	}
	monitorWg.Add(1)
	go func() {
		defer monitorWg.Done()
		zLogger.Info().Msg("Starting file monitoring service...")

		initialMonitorURLs := []string{}
		targetSource := "config"
		if monitorTargetFile != "" {
			zLogger.Info().Str("file", monitorTargetFile).Msg("Loading initial monitor URLs from --monitor-target-file")
			urlsFromFile, err := urlhandler.ReadURLsFromFile(monitorTargetFile, zLogger)
			if err != nil {
				zLogger.Error().Err(err).Str("file", monitorTargetFile).Msg("Failed to load URLs from monitor target file. Continuing without them.")
			} else {
				initialMonitorURLs = append(initialMonitorURLs, urlsFromFile...)
				targetSource = filepath.Base(monitorTargetFile)
			}
		}

		if len(gCfg.MonitorConfig.InitialMonitorURLs) > 0 {
			zLogger.Info().Int("count", len(gCfg.MonitorConfig.InitialMonitorURLs)).Msg("Appending initial monitor URLs from config file.")
			initialMonitorURLs = append(initialMonitorURLs, gCfg.MonitorConfig.InitialMonitorURLs...)
			if targetSource == "config" {
				targetSource = "config_initial_urls"
			}
		}

		if len(initialMonitorURLs) == 0 {
			zLogger.Info().Msg("No initial URLs provided for monitoring service (via CLI flag or config). Service will start with an empty list.")
		}

		if err := monitoringService.Start(initialMonitorURLs); err != nil {
			zLogger.Error().Err(err).Msg("File monitoring service failed to start")
		}
		<-ctx.Done()
		zLogger.Info().Msg("Stopping file monitoring service due to context cancellation...")

		// Send notification about monitoring service interruption
		if len(initialMonitorURLs) > 0 && notificationHelper != nil {
			interruptSummary := models.GetDefaultScanSummaryData()
			interruptSummary.Component = "MonitoringService"
			interruptSummary.Status = string(models.ScanStatusInterrupted)
			interruptSummary.TargetSource = targetSource
			interruptSummary.Targets = initialMonitorURLs
			interruptSummary.TotalTargets = len(initialMonitorURLs)
			interruptSummary.ErrorMessages = []string{"Monitoring service was interrupted by user or system signal"}

			if notificationHelper != nil {
				// Use monitor service notification type for proper webhook routing
				notificationHelper.SendMonitorInterruptNotification(context.Background(), interruptSummary)
			}
		}
	}()
}

func runApplicationLogic(ctx context.Context, gCfg *config.GlobalConfig, flags appFlags, zLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService, monitoringService *monitor.MonitoringService, unifiedSchedulerInstancePtr **scheduler.UnifiedScheduler) {
	mainScanURLFile := ""
	if flags.urlListFile != "" {
		mainScanURLFile = flags.urlListFile
		zLogger.Info().Str("file", mainScanURLFile).Msg("Using -st for main scan targets.")
	}

	monitorURLFile := ""
	if flags.monitorTargetFile != "" {
		monitorURLFile = flags.monitorTargetFile
		zLogger.Info().Str("file", monitorURLFile).Msg("Using -mt for monitor targets.")
	}

	unifiedFile := ""
	if flags.unifiedFile != "" {
		unifiedFile = flags.unifiedFile
		zLogger.Info().Str("file", unifiedFile).Msg("Using -f for unified scan + monitor targets.")
		// For unified file, set both scan and monitor to the same file
		mainScanURLFile = unifiedFile
		// Note: We don't set monitorURLFile here because UnifiedScheduler will handle both
	}

	if gCfg.Mode == "automated" {
		// Handle unified file (-f) - both scan and monitor on same file
		if unifiedFile != "" {
			zLogger.Info().Str("unified_file", unifiedFile).Msg("Automated mode: -f provided. Initializing unified scheduler for scan + monitor on same file...")
			var errUnifiedScheduler error
			*unifiedSchedulerInstancePtr, errUnifiedScheduler = scheduler.NewUnifiedScheduler(gCfg, unifiedFile, zLogger, notificationHelper, secretDetector, monitoringService)
			if errUnifiedScheduler != nil {
				criticalErrSummary := models.GetDefaultScanSummaryData()
				criticalErrSummary.Component = "UnifiedSchedulerInitialization"
				criticalErrSummary.TargetSource = unifiedFile
				criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize unified scheduler: %v", errUnifiedScheduler)}
				notificationHelper.SendCriticalErrorNotification(context.Background(), "UnifiedSchedulerInitialization", criticalErrSummary)
				zLogger.Fatal().Err(errUnifiedScheduler).Msg("Failed to initialize unified scheduler")
			}

			if *unifiedSchedulerInstancePtr != nil {
				if err := (*unifiedSchedulerInstancePtr).Start(ctx); err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						zLogger.Info().Msg("UnifiedScheduler stopped due to context cancellation (interrupt).")
					} else {
						criticalErrSummary := models.GetDefaultScanSummaryData()
						criticalErrSummary.Component = "UnifiedSchedulerRuntime"
						criticalErrSummary.TargetSource = unifiedFile
						criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("UnifiedScheduler error: %v", err)}
						notificationHelper.SendCriticalErrorNotification(context.Background(), "UnifiedSchedulerRuntime", criticalErrSummary)
						zLogger.Error().Err(err).Msg("UnifiedScheduler error")
					}
				}
				zLogger.Info().Msg("Automated mode (with unified scheduler) completed or interrupted.")
				if *unifiedSchedulerInstancePtr != nil && ctx.Err() == context.Canceled {
					zLogger.Info().Msg("Context cancelled, ensuring unified scheduler is stopped.")
					(*unifiedSchedulerInstancePtr).Stop()
				}
			}
		} else {
			// Handle separate scan (-st) and monitor (-mt) files
			// Start UnifiedScheduler if scan targets are provided
			if mainScanURLFile != "" {
				zLogger.Info().Str("urlfile", mainScanURLFile).Msg("Automated mode: -st provided. Initializing and starting unified scheduler module...")
				var errUnifiedScheduler error
				*unifiedSchedulerInstancePtr, errUnifiedScheduler = scheduler.NewUnifiedScheduler(gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector, monitoringService)
				if errUnifiedScheduler != nil {
					criticalErrSummary := models.GetDefaultScanSummaryData()
					criticalErrSummary.Component = "UnifiedSchedulerInitialization"
					criticalErrSummary.TargetSource = mainScanURLFile
					criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("Failed to initialize unified scheduler: %v", errUnifiedScheduler)}
					notificationHelper.SendCriticalErrorNotification(context.Background(), "UnifiedSchedulerInitialization", criticalErrSummary)
					zLogger.Fatal().Err(errUnifiedScheduler).Msg("Failed to initialize unified scheduler")
				}

				if *unifiedSchedulerInstancePtr != nil {
					if err := (*unifiedSchedulerInstancePtr).Start(ctx); err != nil {
						if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
							zLogger.Info().Msg("UnifiedScheduler stopped due to context cancellation (interrupt).")
						} else {
							criticalErrSummary := models.GetDefaultScanSummaryData()
							criticalErrSummary.Component = "UnifiedSchedulerRuntime"
							criticalErrSummary.TargetSource = mainScanURLFile
							criticalErrSummary.ErrorMessages = []string{fmt.Sprintf("UnifiedScheduler error: %v", err)}
							notificationHelper.SendCriticalErrorNotification(context.Background(), "UnifiedSchedulerRuntime", criticalErrSummary)
							zLogger.Error().Err(err).Msg("UnifiedScheduler error")
						}
					}
					zLogger.Info().Msg("Automated mode (with unified scheduler) completed or interrupted.")
					if *unifiedSchedulerInstancePtr != nil && ctx.Err() == context.Canceled {
						zLogger.Info().Msg("Context cancelled, ensuring unified scheduler is stopped.")
						(*unifiedSchedulerInstancePtr).Stop()
					}
				}
			}

			// Start MonitoringService if monitor targets are provided (and no scan targets, or as additional monitoring)
			if monitorURLFile != "" && mainScanURLFile == "" {
				zLogger.Info().Str("monitor_file", monitorURLFile).Msg("Automated mode: -mt provided without -st. Starting monitoring service only...")

				// Load monitor URLs from file
				monitorURLs, targetSource := loadSeedURLs(gCfg, monitorURLFile, zLogger, notificationHelper)
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

			// If neither scan nor monitor targets provided
			if mainScanURLFile == "" && monitorURLFile == "" {
				zLogger.Info().Msg("Automated mode: Neither -st, -mt, nor -f provided. No services will be started.")
			}
		}
	} else { // Onetime mode
		zLogger.Info().Msg("Running in onetime mode...")
		runOnetimeScan(ctx, gCfg, mainScanURLFile, zLogger, notificationHelper, secretDetector)
	}
}

func shutdownServices(monitoringService *monitor.MonitoringService, unifiedScheduler *scheduler.UnifiedScheduler, monitorWg *sync.WaitGroup, zLogger zerolog.Logger, ctx context.Context) {
	if monitoringService != nil {
		zLogger.Info().Msg("Waiting for file monitoring service to shut down completely...")
		monitorWg.Wait() // This ensures its goroutine finishes, which includes calling its Stop()
		zLogger.Info().Msg("File monitoring service has shut down.")
	}

	// UnifiedScheduler is stopped in runApplicationLogic if interrupted.
	// If not interrupted, its main loop finishes and Stop() might have been called or not needed if already fully stopped.
	// However, for a clean shutdown sequence, ensure Stop is called if it exists and hasn't been.
	// This is more of a fallback or explicit finalization if not handled by interrupt logic.
	if unifiedScheduler != nil {
		zLogger.Info().Msg("Ensuring unified scheduler is stopped as part of final shutdown sequence...")
		unifiedScheduler.Stop() // Call Stop here if not already stopped by interrupt logic in runApplicationLogic
		zLogger.Info().Msg("UnifiedScheduler confirmed stopped in shutdownServices.")
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

	// Check for invalid flag combination: -mt with onetime mode
	if flags.monitorTargetFile != "" && gCfg.Mode == "onetime" {
		// Use zLogger if available and initialized, otherwise fmt to stderr
		errMsg := "Invalid combination: --monitor-targets (-mt) cannot be used with --mode onetime (-m onetime). File monitoring is only available in automated mode."
		if zLogger.GetLevel() != zerolog.Disabled {
			zLogger.Fatal().Str("monitor_target_file", flags.monitorTargetFile).Str("mode", gCfg.Mode).Msg(errMsg)
		} else {
			fmt.Fprintln(os.Stderr, "[FATAL] "+errMsg)
		}
		os.Exit(1)
	}

	monitoringService, secretDetector, unifiedSchedulerInstance, err := initializeServices(gCfg, zLogger, flags, notificationHelper)
	if err != nil {
		zLogger.Fatal().Err(err).Msg("Failed to initialize services")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var monitorWg sync.WaitGroup

	// Setup signal handling
	setupSignalHandling(ctx, cancel, gCfg, notificationHelper, monitoringService, zLogger)

	// Start monitoring service early if enabled and in onetime mode
	if gCfg.Mode == "onetime" && monitoringService != nil {
		startMonitoringService(ctx, monitoringService, flags.monitorTargetFile, gCfg, zLogger, notificationHelper, &monitorWg)
	}

	// Now run application logic, which might populate unifiedSchedulerInstance
	runApplicationLogic(ctx, gCfg, flags, zLogger, notificationHelper, secretDetector, monitoringService, &unifiedSchedulerInstance)

	shutdownServices(monitoringService, unifiedSchedulerInstance, &monitorWg, zLogger, ctx)
}

func runOnetimeScan(ctx context.Context, gCfg *config.GlobalConfig, urlListFileArgument string, appLogger zerolog.Logger, notificationHelper *notifier.NotificationHelper, secretDetector *secrets.SecretDetectorService) {
	// Initialize components
	scanOrchestrator := initializeScanOrchestrator(gCfg, appLogger, secretDetector)

	// Load seed URLs
	seedURLs, targetSource := loadSeedURLs(gCfg, urlListFileArgument, appLogger, notificationHelper)
	if len(seedURLs) == 0 {
		appLogger.Info().Msg("Onetime scan: No seed URLs loaded, scan will not run. Monitoring service (if active) will continue until application termination.")
		return // Error already handled in loadSeedURLs, or no targets were specified.
	}

	// Send start notification
	sendScanStartNotification(ctx, seedURLs, targetSource, "onetime", notificationHelper, appLogger)

	// Execute complete scan workflow
	scanSessionID := time.Now().Format("20060102-150405")
	// summaryData được khởi tạo ở đây để nếu ExecuteCompleteScanWorkflow bị ngắt sớm, chúng ta vẫn có các giá trị mặc định.
	var summaryData models.ScanSummaryData = models.GetDefaultScanSummaryData()
	var probeResults []models.ProbeResult
	var urlDiffResults map[string]models.URLDiffResult
	var secretFindings []models.SecretFinding
	var workflowErr error

	summaryData, probeResults, urlDiffResults, secretFindings, workflowErr = scanOrchestrator.ExecuteCompleteScanWorkflow(ctx, seedURLs, scanSessionID, targetSource)

	// Ưu tiên kiểm tra lỗi từ workflow trước, sau đó mới đến context chung
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

		// Nếu workflowErr không phải là context.Canceled (ví dụ, một lỗi khác xảy ra trước khi cancel được xử lý đầy đủ)
		// hoặc nếu summaryData có vẻ đã được điền một phần, hãy cố gắng sử dụng nó.
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
			if summaryData.SecretStats.TotalFindings > 0 {
				interruptionSummary.SecretStats = summaryData.SecretStats
			}
			// if summaryData.ScanDuration > 0 { // Duration might be very short if interrupted early
			// 	interruptionSummary.ScanDuration = summaryData.ScanDuration
			// }
		}

		appLogger.Info().Interface("interruption_summary", interruptionSummary).Msg("Attempting to send onetime scan interruption notification.")
		notificationHelper.SendScanCompletionNotification(context.Background(), interruptionSummary, notifier.ScanServiceNotification)
		return
	}

	// Log secret findings summary
	if len(secretFindings) > 0 {
		appLogger.Info().Int("secret_findings_count", len(secretFindings)).Msg("Secret detection found findings during scan")
	}

	// Handle other workflow errors (nếu không phải là Canceled)
	if workflowErr != nil {
		handleWorkflowError(summaryData, workflowErr, "onetime", notificationHelper, appLogger)
		return
	}

	appLogger.Info().Msg("Scan workflow completed via orchestrator.")

	// Generate and send completion notification
	generateReportAndNotify(ctx, gCfg, summaryData, probeResults, urlDiffResults, secretFindings, scanSessionID, "onetime", notificationHelper, appLogger)

	// Note: monitoringService.Stop() was removed from here.
	// The monitoring service will be stopped when the main context is cancelled.
	appLogger.Info().Msg("Onetime scan specific tasks finished. Monitoring service (if active) continues.")
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
		appLogger.Warn().Msg(noSeedsMsg) // Keep the log

		// Only send notification if onetimeTargetSource was set, implying an attempt to provide targets.
		// If onetimeTargetSource is still "", it means no target source was specified by the user.
		if onetimeTargetSource != "" {
			noTargetsSummary := models.GetDefaultScanSummaryData()
			noTargetsSummary.ScanMode = "onetime"
			// Since onetimeTargetSource != "", it will be set correctly here.
			noTargetsSummary.TargetSource = onetimeTargetSource
			noTargetsSummary.Status = string(models.ScanStatusNoTargets)
			noTargetsSummary.ErrorMessages = []string{noSeedsMsg}
			notificationHelper.SendScanCompletionNotification(context.Background(), noTargetsSummary, notifier.ScanServiceNotification)
		} else {
			appLogger.Info().Msg("No targets specified for onetime scan via CLI or config. 'NO_TARGETS' notification will be skipped.")
		}
		return nil, "" // Still return, as no scan can run
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
