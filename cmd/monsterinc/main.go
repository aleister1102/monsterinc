package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"monsterinc/internal/datastore"
	"monsterinc/internal/differ"
	"monsterinc/internal/httpxrunner"
	"monsterinc/internal/models"
	"monsterinc/internal/reporter"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	// Required for CrawlerConfig RequestTimeout
)

// Helper function to find the root target for a given discovered URL
// For simplicity, this assumes a discovered URL belongs to the seed URL with the same hostname.
// A more robust solution might involve tracking the crawl path or using scope rules.
func getRootTargetForURL(discoveredURL string, seedURLs []string) string {
	discoveredHost := ""
	parsedDiscoveredURL, err := url.Parse(discoveredURL)
	if err == nil {
		discoveredHost = parsedDiscoveredURL.Hostname()
	}

	for _, seed := range seedURLs {
		parsedSeed, err := url.Parse(seed)
		if err == nil && parsedSeed.Hostname() == discoveredHost {
			return seed // Return the original seed URL as the root target
		}
	}
	if len(seedURLs) > 0 {
		return seedURLs[0] // Fallback to the first seed if no direct match (less ideal)
	}
	return discoveredURL // Fallback if absolutely no seeds
}

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

	// --- Initialize Logger (Example - you might have a dedicated logger package) ---
	// Based on gCfg.LogConfig, set up your logger. For now, using standard log.
	appLogger := log.Default() // Using standard logger for all components for now

	// --- Initialize ParquetReader --- //
	// ParquetReader is needed by UrlDiffer, so initialize it early.
	parquetReader := datastore.NewParquetReader(&gCfg.StorageConfig, appLogger)

	// --- Initialize ParquetWriter --- //
	storageCfg := &gCfg.StorageConfig
	parquetWriter, parquetErr := datastore.NewParquetWriter(storageCfg, appLogger)
	if parquetErr != nil {
		appLogger.Printf("[ERROR] Main: Failed to initialize ParquetWriter: %v. Parquet writing will be disabled.", parquetErr)
		parquetWriter = nil // Ensure it's nil so we can check later
	}

	// --- Crawler Module --- //
	// Use crawler configuration directly from global config
	crawlerCfg := &gCfg.CrawlerConfig

	// Override or set seed URLs from the urlfile if provided
	if *urlListFile != "" {
		appLogger.Printf("[INFO] Main: Reading seed URLs from %s", *urlListFile)
		file, errFile := os.Open(*urlListFile)
		if errFile != nil {
			appLogger.Fatalf("[FATAL] Main: Could not open URL list file '%s': %v", *urlListFile, errFile)
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
			appLogger.Fatalf("[FATAL] Main: Error reading URL list file '%s': %v", *urlListFile, scanner.Err())
		}
		if len(urls) > 0 {
			crawlerCfg.SeedURLs = urls
		} else {
			appLogger.Printf("[WARN] Main: No valid URLs found in '%s'. Using seeds from config if available.", *urlListFile)
		}
	} else if len(gCfg.InputConfig.InputURLs) > 0 {
		// If no urlListFile is given, but InputURLs are in config, use them for the crawler.
		appLogger.Printf("[INFO] Main: Using %d seed URLs from global input_config.input_urls", len(gCfg.InputConfig.InputURLs))
		crawlerCfg.SeedURLs = gCfg.InputConfig.InputURLs
	}

	// Determine the primary root target for this scan session.
	// This will be used for naming Parquet files and for diffing.
	var primaryRootTargetURL string
	if len(crawlerCfg.SeedURLs) > 0 {
		primaryRootTargetURL = crawlerCfg.SeedURLs[0] // Assuming the first seed is the primary target
		// TODO: Consider normalization of this URL if not already done
	} else {
		// If no seeds, we might not be able to meaningfully diff or store per-target Parquet.
		// For now, let it be empty; subsequent logic will handle it (e.g., skip diffing/Parquet for this root).
		appLogger.Println("[WARN] Main: No seed URLs provided. Diffing and Parquet storage might be affected or use a generic target.")
		primaryRootTargetURL = "unknown_target_" + time.Now().Format("20060102") // Fallback if no seeds
	}

	var discoveredURLs []string
	if len(crawlerCfg.SeedURLs) == 0 {
		appLogger.Println("[INFO] Main: No seed URLs provided for crawler. Skipping crawler module.")
	} else {
		appLogger.Printf("[INFO] Main: Initializing crawler with %d seed URLs. Primary target for this session: %s", len(crawlerCfg.SeedURLs), primaryRootTargetURL)
		crawlerInstance, crawlerErr := crawler.NewCrawler(crawlerCfg)
		if crawlerErr != nil {
			appLogger.Fatalf("[FATAL] Main: Failed to initialize crawler: %v", crawlerErr)
		}
		appLogger.Println("[INFO] Main: Starting crawl...")
		crawlerInstance.Start()
		appLogger.Println("[INFO] Main: Crawl finished.")
		discoveredURLs = crawlerInstance.GetDiscoveredURLs()
		appLogger.Printf("[INFO] Main: Total URLs discovered: %d", len(discoveredURLs))
	}

	// --- HTTPX Probing Module --- //
	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: Starting HTTPX Probing Module...")

	var allProbeResults []models.ProbeResult // Combined list of all results for the main report table

	if len(discoveredURLs) == 0 {
		appLogger.Println("[INFO] Main: No URLs discovered by crawler (or crawler skipped). Skipping HTTPX probing module.")
	} else {
		appLogger.Printf("[INFO] Main: Preparing to run HTTPX probes for %d URLs.", len(discoveredURLs))
		runnerCfg := &httpxrunner.Config{
			Targets:              discoveredURLs,
			Method:               gCfg.HttpxRunnerConfig.Method,
			RequestURIs:          gCfg.HttpxRunnerConfig.RequestURIs,
			FollowRedirects:      gCfg.HttpxRunnerConfig.FollowRedirects,
			Timeout:              gCfg.HttpxRunnerConfig.TimeoutSecs,
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
			// RateLimit: gCfg.HttpxRunnerConfig.RateLimit,
		}

		probeRunner, newRunnerErr := httpxrunner.NewRunner(runnerCfg, primaryRootTargetURL)
		if newRunnerErr != nil {
			appLogger.Fatalf("[FATAL] Main: Failed to create HTTPX runner: %v", newRunnerErr)
		}

		runErr := probeRunner.Run()
		if runErr != nil {
			appLogger.Printf("[ERROR] Main: HTTPX probing encountered an error: %v", runErr)
		}
		appLogger.Println("[INFO] Main: HTTPX Probing finished.")

		probeResultsFromRunner := probeRunner.GetResults()
		resultMap := make(map[string]models.ProbeResult)
		for _, r := range probeResultsFromRunner {
			resultMap[r.InputURL] = r
		}

		for _, urlString := range discoveredURLs {
			rootTargetForThisURL := getRootTargetForURL(urlString, crawlerCfg.SeedURLs)
			if r, ok := resultMap[urlString]; ok {
				// Assign the determined root target
				actualResult := r // make a copy
				actualResult.RootTargetURL = rootTargetForThisURL
				allProbeResults = append(allProbeResults, actualResult)
			} else {
				allProbeResults = append(allProbeResults, models.ProbeResult{
					InputURL:      urlString,
					Error:         "No response or error during httpx probe",
					Timestamp:     time.Now(),
					RootTargetURL: rootTargetForThisURL,
				})
			}
		}
	}

	appLogger.Printf("[INFO] Main: Total probe results processed (including fallbacks): %d", len(allProbeResults))

	// --- Group results by RootTargetURL ---
	resultsByRootTarget := make(map[string][]models.ProbeResult)
	for _, pr := range allProbeResults {
		resultsByRootTarget[pr.RootTargetURL] = append(resultsByRootTarget[pr.RootTargetURL], pr)
	}

	updatedProbeResults := make([]models.ProbeResult, 0, len(allProbeResults)) // Initialize with capacity

	// --- URL Diffing, Parquet Storage (per root target) --- //
	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: Starting URL Diffing and Parquet Storage...")
	urlDiffer := differ.NewUrlDiffer(parquetReader, appLogger)
	allURLDiffResults := make(map[string]models.URLDiffResult)

	scanSessionID := time.Now().Format("20060102-150405") // Unique ID for this scan session

	for rootTgt, resultsForRoot := range resultsByRootTarget {
		if rootTgt == "" || len(resultsForRoot) == 0 {
			appLogger.Printf("[WARN] Main: Skipping diffing/storage for empty root target or no results for a root target: '%s'", rootTgt)
			continue
		}
		appLogger.Printf("[INFO] Main: Processing diff and storage for root target: %s (%d results)", rootTgt, len(resultsForRoot))

		diffResult, diffErr := urlDiffer.Compare(resultsForRoot, rootTgt)
		if diffErr != nil {
			appLogger.Printf("[ERROR] Main: Failed to compare URLs for root target %s: %v", rootTgt, diffErr)
			// Continue to next root target, but don't store this diff result
		} else if diffResult != nil {
			allURLDiffResults[rootTgt] = *diffResult
			appLogger.Printf("[INFO] Main: URL Diffing complete for %s. New: %d, Old: %d, Existing: %d",
				rootTgt, countStatuses(diffResult, models.StatusNew), countStatuses(diffResult, models.StatusOld), countStatuses(diffResult, models.StatusExisting))

			// Add the updated results from this root target to the overall list
			updatedProbeResults = append(updatedProbeResults, resultsForRoot...)
		} else {
			appLogger.Printf("[WARN] Main: DiffResult was nil for %s, though no explicit error.", rootTgt)
			// Even if diff is nil, resultsForRoot might still be useful for parquet, but they won't have URLStatus from diffing.
			// Consider if these should be added to updatedProbeResults or if they should have a default status.
			// For now, only adding if diffResult is not nil to ensure URLStatus is populated.
		}

		// Parquet Storage for this root target's results
		if parquetWriter != nil {
			if err := parquetWriter.Write(resultsForRoot, scanSessionID, rootTgt); err != nil {
				appLogger.Printf("[ERROR] Main: Failed to write Parquet data for root target %s: %v", rootTgt, err)
			}
		} else {
			appLogger.Println("[INFO] Main: ParquetWriter is not initialized. Skipping Parquet storage.")
		}
	}

	// --- HTML Report Generation --- //
	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: Generating HTML report...")
	htmlReporter, err := reporter.NewHtmlReporter(&gCfg.ReporterConfig, appLogger)
	if err != nil {
		appLogger.Fatalf("[FATAL] Main: Failed to initialize HTML reporter: %v", err)
	}

	reportFilename := fmt.Sprintf("%s_%s_report.html", scanSessionID, gCfg.Mode)
	reportPath := filepath.Join(gCfg.ReporterConfig.OutputDir, reportFilename)

	if err := os.MkdirAll(gCfg.ReporterConfig.OutputDir, 0755); err != nil {
		appLogger.Fatalf("[FATAL] Main: Could not create report output directory '%s': %v", gCfg.ReporterConfig.OutputDir, err)
	}

	if err := htmlReporter.GenerateReport(updatedProbeResults, allURLDiffResults, reportPath); err != nil {
		appLogger.Fatalf("[FATAL] Main: Failed to generate HTML report: %v", err)
	}
	appLogger.Printf("[INFO] Main: HTML report generated successfully: %s", reportPath)

	appLogger.Println("-----")
	appLogger.Println("[INFO] Main: MonsterInc Crawler finished.")
}

func countStatuses(diffResult *models.URLDiffResult, status models.URLStatus) int {
	if diffResult == nil {
		return 0
	}
	count := 0
	for _, r := range diffResult.Results {
		if r.Status == status {
			count++
		}
	}
	return count
}
