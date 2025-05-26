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
	"os"
	"path/filepath"
	"strings"
	"time"
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
	appLogger := log.Default() // Using standard logger for all components for now

	// Check mode and run appropriate logic
	if gCfg.Mode == "automated" {
		// Automated mode - initialize and run scheduler
		appLogger.Println("[INFO] Main: Running in automated mode...")

		schedulerInstance, err := scheduler.NewScheduler(gCfg, *urlListFile, appLogger)
		if err != nil {
			appLogger.Fatalf("[FATAL] Main: Failed to initialize scheduler: %v", err)
		}

		// Start scheduler (this will block until scheduler stops)
		if err := schedulerInstance.Start(); err != nil {
			appLogger.Fatalf("[FATAL] Main: Scheduler error: %v", err)
		}

		// Scheduler has stopped
		appLogger.Println("[INFO] Main: Automated mode completed.")

	} else {
		// Onetime mode - run single scan
		appLogger.Println("[INFO] Main: Running in onetime mode...")
		runOnetimeScan(gCfg, *urlListFile, appLogger)
	}
}

func runOnetimeScan(gCfg *config.GlobalConfig, urlListFile string, appLogger *log.Logger) {
	// --- Initialize ParquetReader & Writer (needed for orchestrator) --- //
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)
	parquetWriter, parquetErr := datastore.NewParquetWriter(&gCfg.StorageConfig, appLogger)
	if parquetErr != nil {
		appLogger.Printf("[ERROR] Main: Failed to initialize ParquetWriter for orchestrator: %v. Parquet writing will be disabled.", parquetErr)
		parquetWriter = nil
	}

	// --- Initialize ScanOrchestrator --- //
	scanOrchestrator := orchestrator.NewScanOrchestrator(gCfg, appLogger, parquetReader, parquetWriter)

	// --- Determine Seed URLs --- //
	var seedURLs []string
	if urlListFile != "" {
		appLogger.Printf("[INFO] Main: Reading seed URLs from %s", urlListFile)
		file, errFile := os.Open(urlListFile)
		if errFile != nil {
			appLogger.Fatalf("[FATAL] Main: Could not open URL list file '%s': %v", urlListFile, errFile)
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
			appLogger.Fatalf("[FATAL] Main: Error reading URL list file '%s': %v", scanner.Err(), urlListFile)
		}
		if len(seedURLs) == 0 {
			appLogger.Printf("[WARN] Main: No valid URLs found in '%s'. Using seeds from config if available.", urlListFile)
		}
	}

	if len(seedURLs) == 0 { // If file was empty or not provided, try config
		if len(gCfg.InputConfig.InputURLs) > 0 {
			appLogger.Printf("[INFO] Main: Using %d seed URLs from global input_config.input_urls", len(gCfg.InputConfig.InputURLs))
			seedURLs = gCfg.InputConfig.InputURLs
		} else {
			appLogger.Fatalf("[FATAL] Main: No seed URLs provided. Please specify via -urlfile or in input_config.input_urls in the config file.")
		}
	}

	appLogger.Printf("[INFO] Main: Starting onetime scan with %d seed URLs.", len(seedURLs))

	// --- Generate Scan Session ID --- //
	scanSessionID := time.Now().Format("20060102-150405")

	// --- Execute Scan Workflow via Orchestrator --- //
	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: Executing scan workflow via orchestrator...")
	probeResults, urlDiffResults, workflowErr := scanOrchestrator.ExecuteScanWorkflow(seedURLs, scanSessionID)
	if workflowErr != nil {
		// For onetime scan, a workflow error is typically fatal.
		appLogger.Fatalf("[FATAL] Main: Scan workflow execution failed: %v", workflowErr)
	}
	appLogger.Println("[INFO] Main: Scan workflow completed via orchestrator.")

	// Note: The orchestrator.ExecuteScanWorkflow now returns all probe results, including those that might not have been part of a diff.
	// The `updatedProbeResults` logic from the old main.go might need re-evaluation if specific filtering was intended before reporting.
	// For now, we directly use `probeResults` from the orchestrator for the report.

	// --- HTML Report Generation --- //
	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		log.Fatalf("[FATAL] Main: Failed to initialize HTML reporter: %v", err)
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode) // gCfg.Mode will be "onetime"
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	// Create a slice of pointers to models.ProbeResult for the reporter
	probeResultsPtr := make([]*models.ProbeResult, len(probeResults))
	for i := range probeResults {
		probeResultsPtr[i] = &probeResults[i]
	}

	if err := htmlReporter.GenerateReport(probeResultsPtr, urlDiffResults, reportPath); err != nil {
		log.Fatalf("[FATAL] Main: Failed to generate HTML report: %v", err)
	}
	appLogger.Printf("[INFO] Main: HTML report generated successfully: %s", reportPath)

	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: MonsterInc Crawler finished (onetime mode).")
}
