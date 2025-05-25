package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"monsterinc/internal/datastore"
	"monsterinc/internal/httpxrunner"
	"monsterinc/internal/models"
	"monsterinc/internal/reporter"
	"os"
	"strings"
	"time"
	// Required for CrawlerConfig RequestTimeout
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Flags
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	urlListFileAlias := flag.String("u", "", "Alias for -urlfile")
	globalConfigFile := flag.String("globalconfig", "config.json", "Path to the global JSON configuration file.")
	modeFlag := flag.String("mode", "", "Mode to run the tool: onetime or automated (required)")
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

	// Override mode if --mode is set
	if *modeFlag != "" {
		gCfg.Mode = *modeFlag
	}

	// --- Initialize ParquetWriter --- //
	storageCfg := &gCfg.StorageConfig
	parquetWriter, parquetErr := datastore.NewParquetWriter(storageCfg, log.Default()) // Using standard logger
	if parquetErr != nil {
		log.Printf("[ERROR] Main: Failed to initialize ParquetWriter: %v. Parquet writing will be disabled.", parquetErr)
		parquetWriter = nil // Ensure it's nil so we can check later
	}

	// --- Initialize Logger (Example - you might have a dedicated logger package) ---
	// Based on gCfg.LogConfig, set up your logger. For now, using standard log.
	// e.g., logger.Init(gCfg.LogConfig)

	// --- Crawler Module --- //
	// Use crawler configuration directly from global config
	crawlerCfg := &gCfg.CrawlerConfig

	// Override or set seed URLs from the urlfile if provided
	if *urlListFile != "" {
		log.Printf("[INFO] Main: Reading seed URLs from %s", *urlListFile)
		file, errFile := os.Open(*urlListFile)
		if errFile != nil {
			log.Fatalf("[FATAL] Main: Could not open URL list file '%s': %v", *urlListFile, errFile)
		}

		var urls []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
				urls = append(urls, url)
			}
		}
		file.Close()
		if scanner.Err() != nil {
			log.Fatalf("[FATAL] Main: Error reading URL list file '%s': %v", *urlListFile, scanner.Err())
		}
		if len(urls) > 0 {
			crawlerCfg.SeedURLs = urls
		} else {
			log.Printf("[WARN] Main: No valid URLs found in '%s'. Using seeds from config if available.", *urlListFile)
		}
	} else if len(gCfg.InputConfig.InputURLs) > 0 {
		// If no urlListFile is given, but InputURLs are in config, use them for the crawler.
		log.Printf("[INFO] Main: Using %d seed URLs from global input_config.input_urls", len(gCfg.InputConfig.InputURLs))
		crawlerCfg.SeedURLs = gCfg.InputConfig.InputURLs
	}

	var discoveredURLs []string
	if len(crawlerCfg.SeedURLs) == 0 {
		log.Println("[INFO] Main: No seed URLs provided for crawler. Skipping crawler module.")
	} else {
		log.Printf("[INFO] Main: Initializing crawler with %d seed URLs.", len(crawlerCfg.SeedURLs))

		// Convert RequestTimeoutSecs from int to time.Duration for the crawler package
		// Assuming the crawler package expects time.Duration. If it expects int seconds, this conversion is not needed.
		// For this example, let's assume crawler.NewCrawler was updated or always expected a struct that now has int.
		// If crawler.NewCrawler expects config.CrawlerConfig directly, and CrawlerConfig.RequestTimeoutSecs is int, no conversion here.
		// Let's assume the crawler initialization will handle the int seconds directly or has been updated.

		crawlerInstance, crawlerErr := crawler.NewCrawler(crawlerCfg) // Pass the sub-config
		if crawlerErr != nil {
			log.Fatalf("[FATAL] Main: Failed to initialize crawler: %v", crawlerErr)
		}
		log.Println("[INFO] Main: Starting crawl...")
		crawlerInstance.Start()
		log.Println("[INFO] Main: Crawl finished.")
		discoveredURLs = crawlerInstance.GetDiscoveredURLs()
		log.Printf("[INFO] Main: Total URLs discovered: %d", len(discoveredURLs))
	}

	// --- HTTPX Probing Module --- //
	log.Println("-----")
	log.Println("[INFO] Main: Starting HTTPX Probing Module...")

	if len(discoveredURLs) == 0 {
		log.Println("[INFO] Main: No URLs discovered by crawler (or crawler skipped). Skipping HTTPX probing module.")
	} else {
		log.Printf("[INFO] Main: Preparing to run HTTPX probes for %d URLs.", len(discoveredURLs))

		// Create httpxrunner.Config directly from gCfg.HTTPXRunnerConfig
		// The fields in httpxrunner.Config should now match gCfg.HTTPXRunnerConfig
		runnerCfg := &httpxrunner.Config{
			Targets:              discoveredURLs,
			Method:               gCfg.HttpxRunnerConfig.Method,
			RequestURIs:          gCfg.HttpxRunnerConfig.RequestURIs,
			FollowRedirects:      gCfg.HttpxRunnerConfig.FollowRedirects,
			Timeout:              gCfg.HttpxRunnerConfig.TimeoutSecs, // Ensure this matches field name in httpxrunner.Config
			Retries:              gCfg.HttpxRunnerConfig.Retries,
			Threads:              gCfg.HttpxRunnerConfig.Threads,
			CustomHeaders:        gCfg.HttpxRunnerConfig.CustomHeaders,
			Proxy:                gCfg.HttpxRunnerConfig.Proxy,
			Verbose:              gCfg.HttpxRunnerConfig.Verbose,
			TechDetect:           gCfg.HttpxRunnerConfig.TechDetect,
			ExtractTitle:         gCfg.HttpxRunnerConfig.ExtractTitle,
			ExtractStatusCode:    gCfg.HttpxRunnerConfig.ExtractStatusCode,
			ExtractLocation:      gCfg.HttpxRunnerConfig.ExtractLocation,
			ExtractContentLength: gCfg.HttpxRunnerConfig.ExtractContentLength,
			ExtractServerHeader:  gCfg.HttpxRunnerConfig.ExtractServerHeader,
			ExtractContentType:   gCfg.HttpxRunnerConfig.ExtractContentType,
			ExtractIPs:           gCfg.HttpxRunnerConfig.ExtractIPs,
			ExtractBody:          gCfg.HttpxRunnerConfig.ExtractBody,
			ExtractHeaders:       gCfg.HttpxRunnerConfig.ExtractHeaders,
			// RateLimit is part of gCfg.HTTPXRunnerConfig but might be used differently by the runner wrapper or httpx library itself.
			// Assuming httpxrunner.Config has a RateLimit field if it's directly used.
			// RateLimit: gCfg.HTTPXRunnerConfig.RateLimit,
		}

		probeRunner, newRunnerErr := httpxrunner.NewRunner(runnerCfg)
		if newRunnerErr != nil {
			log.Fatalf("[FATAL] Main: Failed to create HTTPX runner: %v", newRunnerErr)
		}

		runErr := probeRunner.Run()
		if runErr != nil {
			log.Printf("[ERROR] Main: HTTPX probing encountered an error: %v", runErr)
		}
		log.Println("[INFO] Main: HTTPX Probing finished.")

		// --- HTML Report Generation --- //
		probeResults := probeRunner.GetResults() // Assuming this method exists

		// Bổ sung các URL không có kết quả probe
		resultMap := make(map[string]models.ProbeResult)
		for _, r := range probeResults {
			resultMap[r.InputURL] = r
		}
		var finalResults []models.ProbeResult
		for _, url := range discoveredURLs {
			if r, ok := resultMap[url]; ok {
				finalResults = append(finalResults, r)
			} else {
				finalResults = append(finalResults, models.ProbeResult{
					InputURL: url,
					Error:    "No response from httpx",
				})
			}
		}

		if len(finalResults) > 0 {
			log.Printf("[INFO] Main: Preparing to generate HTML report for %d results.", len(finalResults))
			// Use ReporterConfig from global config
			reporterCfg := &gCfg.ReporterConfig
			htmlReporter, reporterErr := reporter.NewHtmlReporter(reporterCfg, log.Default()) // Using standard logger for now
			if reporterErr != nil {
				log.Printf("[ERROR] Main: Failed to initialize HTML reporter: %v", reporterErr)
			} else {
				outputFile := reporterCfg.DefaultOutputHTMLPath
				if gCfg.Mode == "onetime" {
					t := finalResults[0].Timestamp
					if t.IsZero() {
						t = time.Now()
					}
					outputFile = fmt.Sprintf("reports/%s.html", t.Format("2006-01-02-15-04-05"))
				} else if gCfg.Mode == "automated" {
					t := finalResults[0].Timestamp
					if t.IsZero() {
						t = time.Now()
					}
					outputFile = fmt.Sprintf("reports/%s.html", t.Format("2006-01-02"))
				}
				if outputFile == "" {
					outputFile = "monsterinc_report.html" // Default filename if not in config
					log.Printf("[WARN] Main: ReporterConfig.DefaultOutputHTMLPath is not set. Using default: %s", outputFile)
				}
				err := htmlReporter.GenerateReport(finalResults, outputFile)
				if err != nil {
					log.Printf("[ERROR] Main: Failed to generate HTML report: %v", err)
				} else {
					log.Printf("[INFO] Main: HTML report generated successfully: %s", outputFile)
				}
			}

			// --- Parquet Writing --- //
			if parquetWriter != nil && len(finalResults) > 0 {
				log.Printf("[INFO] Main: Preparing to write %d results to Parquet.", len(finalResults))
				scanSessionID := time.Now().Format("20060102-150405") // Generate a session ID

				var rootTargetForParquet string
				if len(crawlerCfg.SeedURLs) > 0 {
					rootTargetForParquet = crawlerCfg.SeedURLs[0]
				} else if len(discoveredURLs) > 0 {
					rootTargetForParquet = discoveredURLs[0] // Fallback to first discovered if no seeds
				} else {
					rootTargetForParquet = "unknown_scan_root" // Default if no other info
				}

				err := parquetWriter.Write(finalResults, scanSessionID, rootTargetForParquet)
				if err != nil {
					log.Printf("[ERROR] Main: Failed to write Parquet file: %v", err)
				} else {
					log.Printf("[INFO] Main: Parquet file generated successfully for session %s.", scanSessionID)
				}
			} else if parquetWriter == nil {
				log.Println("[WARN] Main: ParquetWriter not initialized, skipping Parquet writing.")
			} else {
				log.Println("[INFO] Main: No probe results from HTTPX to write to Parquet. Skipping Parquet writing.")
			}

		} else {
			log.Println("[INFO] Main: No probe results from HTTPX to report or write. Skipping HTML report and Parquet generation.")
		}
	}

	log.Println("MonsterInc finished.")
}
