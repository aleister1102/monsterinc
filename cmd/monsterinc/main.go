package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"monsterinc/internal/httpxrunner"
	"os"
	"strings"
	// Required for CrawlerConfig RequestTimeout
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Flags
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	// The -config flag for a separate crawler config is now deprecated, global config is used.
	// crawlerConfigFile := flag.String("config", "", "Path to a JSON configuration file for the crawler. DEPRECATED")
	globalConfigFile := flag.String("globalconfig", "config.json", "Path to the global JSON configuration file.")
	flag.Parse()

	// Load Global Configuration
	log.Printf("[INFO] Main: Loading global configuration from %s", *globalConfigFile)
	gCfg, err := config.LoadGlobalConfig(*globalConfigFile)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not load global config from '%s': %v", *globalConfigFile, err)
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
			Method:               gCfg.HTTPXRunnerConfig.Method,
			RequestURIs:          gCfg.HTTPXRunnerConfig.RequestURIs,
			FollowRedirects:      gCfg.HTTPXRunnerConfig.FollowRedirects,
			Timeout:              gCfg.HTTPXRunnerConfig.TimeoutSecs, // Ensure this matches field name in httpxrunner.Config
			Retries:              gCfg.HTTPXRunnerConfig.Retries,
			Threads:              gCfg.HTTPXRunnerConfig.Threads,
			CustomHeaders:        gCfg.HTTPXRunnerConfig.CustomHeaders,
			Proxy:                gCfg.HTTPXRunnerConfig.Proxy,
			Verbose:              gCfg.HTTPXRunnerConfig.Verbose,
			TechDetect:           gCfg.HTTPXRunnerConfig.TechDetect,
			ExtractTitle:         gCfg.HTTPXRunnerConfig.ExtractTitle,
			ExtractStatusCode:    gCfg.HTTPXRunnerConfig.ExtractStatusCode,
			ExtractLocation:      gCfg.HTTPXRunnerConfig.ExtractLocation,
			ExtractContentLength: gCfg.HTTPXRunnerConfig.ExtractContentLength,
			ExtractServerHeader:  gCfg.HTTPXRunnerConfig.ExtractServerHeader,
			ExtractContentType:   gCfg.HTTPXRunnerConfig.ExtractContentType,
			ExtractIPs:           gCfg.HTTPXRunnerConfig.ExtractIPs,
			ExtractBody:          gCfg.HTTPXRunnerConfig.ExtractBody,
			ExtractHeaders:       gCfg.HTTPXRunnerConfig.ExtractHeaders,
			// RateLimit is part of gCfg.HTTPXRunnerConfig but might be used differently by the runner wrapper or httpx library itself.
			// Assuming httpxrunner.Config has a RateLimit field if it's directly used.
			// RateLimit: gCfg.HTTPXRunnerConfig.RateLimit,
		}

		probeRunner := httpxrunner.NewRunner(runnerCfg)
		runErr := probeRunner.Run()
		if runErr != nil {
			log.Printf("[ERROR] Main: HTTPX probing encountered an error: %v", runErr)
		}
		log.Println("[INFO] Main: HTTPX Probing finished.")
	}

	log.Println("MonsterInc finished.")
}
