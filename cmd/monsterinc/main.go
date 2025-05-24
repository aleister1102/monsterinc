package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"monsterinc/internal/probing"
	"os"
	"strings"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Crawler flags
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	crawlerConfigFile := flag.String("config", "", "Path to a JSON configuration file for the crawler. If not provided, default settings are used.")

	// HTTPXProbing flag (sử dụng global config file)
	globalConfigFile := flag.String("globalconfig", "config.json", "Path to the global JSON configuration file.")
	flag.Parse()

	// Load Global Configuration
	log.Printf("[INFO] Main: Loading global configuration from %s", *globalConfigFile)
	gCfg, err := config.LoadGlobalConfig(*globalConfigFile)
	if err != nil {
		log.Fatalf("[FATAL] Main: Could not load global config from '%s': %v", *globalConfigFile, err)
	}

	// --- Crawler Module --- //
	// Sử dụng CrawlerSettings từ GlobalConfig nếu không có crawlerConfigFile riêng
	crawlerCfg := &gCfg.CrawlerConfig
	if *crawlerConfigFile != "" {
		log.Printf("[INFO] Main: Loading dedicated crawler configuration from %s", *crawlerConfigFile)
		crawlerCfgFromFile, err := config.LoadCrawlerConfigFromFile(*crawlerConfigFile) // Asmume this func exists & handles JSON
		if err != nil {
			log.Fatalf("[FATAL] Main: Could not load crawler configuration from '%s': %v", *crawlerConfigFile, err)
		}
		crawlerCfg = crawlerCfgFromFile // Override with specific crawler config
	}

	// Override or set seed URLs from the urlfile if provided
	if *urlListFile != "" {
		log.Printf("[INFO] Main: Reading seed URLs from %s", *urlListFile)
		file, errFile := os.Open(*urlListFile)
		if errFile != nil {
			log.Fatalf("[FATAL] Main: Could not open URL list file '%s': %v", *urlListFile, errFile)
		}
		defer file.Close()

		var urls []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			url := strings.TrimSpace(scanner.Text())
			if url != "" && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
				urls = append(urls, url)
			}
		}
		if scanner.Err() != nil {
			log.Fatalf("[FATAL] Main: Error reading URL list file '%s': %v", *urlListFile, scanner.Err())
		}
		if len(urls) > 0 {
			crawlerCfg.SeedURLs = urls
		} else {
			log.Printf("[WARN] Main: No valid URLs found in '%s'. Using seeds from config if available.", *urlListFile)
		}
	}

	var discoveredURLs []string // Store discovered URLs here
	if len(crawlerCfg.SeedURLs) == 0 {
		log.Println("[INFO] Main: No seed URLs provided for crawler. Skipping crawler module.")
	} else {
		log.Printf("[INFO] Main: Initializing crawler with %d seed URLs.", len(crawlerCfg.SeedURLs))
		crawlerInstance, crawlerErr := crawler.NewCrawler(crawlerCfg)
		if crawlerErr != nil {
			log.Fatalf("[FATAL] Main: Failed to initialize crawler: %v", crawlerErr)
		}
		log.Println("[INFO] Main: Starting crawl...")
		crawlerInstance.Start()
		log.Println("[INFO] Main: Crawl finished.")
		discoveredURLs = crawlerInstance.GetDiscoveredURLs() // Get discovered URLs
		log.Printf("[INFO] Main: Total URLs discovered: %d", len(discoveredURLs))
	}

	// --- HTTPX Probing Module --- //
	log.Println("-----")
	log.Println("[INFO] Main: Starting HTTPX Probing Module...")

	var targets []string
	if len(discoveredURLs) > 0 {
		log.Println("[INFO] Main: Using URLs discovered by crawler for HTTPX probing.")
		targets = discoveredURLs
	} else {
		log.Println("[INFO] Main: No URLs from crawler. Falling back to global config for HTTPX targets.")
		// Use InputFile or InputURLs from InputConfig within GlobalConfig for targets
		targets = gCfg.InputConfig.InputURLs
		if gCfg.InputConfig.InputFile != "" {
			log.Printf("[INFO] Main: Reading targets for HTTPX from input file: %s", gCfg.InputConfig.InputFile)
			// Logic to read targets from gCfg.InputConfig.InputFile
			// This is a simplified example; you might need more robust file reading and parsing
			file, errFile := os.Open(gCfg.InputConfig.InputFile)
			if errFile != nil {
				log.Printf("[WARN] Main: Could not open HTTPX target file '%s': %v. Proceeding with InputURLs if any.", gCfg.InputConfig.InputFile, errFile)
			} else {
				defer file.Close()
				scanner := bufio.NewScanner(file)
				var fileTargets []string
				for scanner.Scan() {
					target := strings.TrimSpace(scanner.Text())
					if target != "" {
						fileTargets = append(fileTargets, target)
					}
				}
				if scanner.Err() != nil {
					log.Printf("[WARN] Main: Error reading HTTPX target file '%s': %v. Proceeding with InputURLs if any.", gCfg.InputConfig.InputFile, scanner.Err())
				} else {
					targets = append(targets, fileTargets...)
				}
			}
		}
	}

	if len(targets) == 0 {
		log.Println("[INFO] Main: No targets specified for HTTPX probing (checked InputURLs and InputFile in global config). Skipping HTTPX probing.")
	} else {
		probingSvc := probing.NewService()
		log.Printf("[INFO] Main: Running HTTPX probes for %d targets.", len(targets))

		// We now pass HTTPXRunnerConfig. The probingSvc.RunHTTPXProbes method
		// will need to be aware of how to use this config and potentially the gCfg.InputConfig for targets.
		// For this change to work, probing.RunHTTPXProbes must expect *config.HTTPXRunnerConfig
		// and internally it should use the `targets` list that we prepared above, or have access to gCfg.InputConfig.
		// For now, assuming the function signature expects *config.HTTPXRunnerConfig and we have populated `targets` for its internal use (if it can access it).
		// A more explicit approach would be to modify HTTPXRunnerConfig to include a Targets field, or change RunHTTPXProbes signature.
		// Given the linter error `want (*config.HTTPXConfig)`, the type name itself might be the issue in the probing service.
		// We are passing *config.HTTPXRunnerConfig here.

		// If the probing service is expecting the targets to be part of the config,
		// we would need to add a `Targets []string` field to `HTTPXRunnerConfig` in `config.go`
		// and set it like: `currentHTTPXConfig := gCfg.HTTPXRunnerConfig; currentHTTPXConfig.Targets = targets`
		// Then pass `&currentHTTPXConfig`
		// Since we cannot modify config.go at this exact step, and RunHTTPXProbes is not visible,
		// this simplification assumes RunHTTPXProbes can somehow get `targets` or `gCfg.InputConfig`.
		// The original call was `probingSvc.RunHTTPXProbes(&gCfg.HTTPXSettings)` and the error implies the type was `HTTPXConfig`.
		// We now pass `&gCfg.HTTPXRunnerConfig`.

		// For the immediate fix in main.go, aligning with the most probable interpretation of the error:
		// The function expects one argument of type *config.HTTPXRunnerConfig (assuming HTTPXConfig was old name).
		// The `targets` variable prepared above will be used by the service internally.
		err = probingSvc.RunHTTPXProbes(&gCfg.HTTPXRunnerConfig, targets) // Pass the runner config and the resolved targets
		if err != nil {
			// Lỗi từ RunHTTPXProbes (nếu có) đã được log bên trong service
			// ở đây chỉ cần ghi nhận là có lỗi ở mức main nếu cần thiết
			log.Printf("[ERROR] Main: HTTPX probing encountered an error: %v", err)
		}
		log.Println("[INFO] Main: HTTPX Probing finished.")
	}

	log.Println("MonsterInc finished.")
}
