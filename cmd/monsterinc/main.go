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
	crawlerCfg := &gCfg.CrawlerSettings
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
		log.Printf("[INFO] Main: Total URLs discovered: %d", len(crawlerInstance.GetDiscoveredURLs()))
	}

	// --- HTTPX Probing Module --- //
	log.Println("-----")
	log.Println("[INFO] Main: Starting HTTPX Probing Module...")

	if len(gCfg.HTTPXSettings.Targets) == 0 {
		log.Println("[INFO] Main: No targets specified in httpx_settings of global config. Skipping HTTPX probing.")
	} else {
		probingSvc := probing.NewService()
		err = probingSvc.RunHTTPXProbes(&gCfg.HTTPXSettings) // Truyền HTTPXSettings từ GlobalConfig
		if err != nil {
			// Lỗi từ RunHTTPXProbes (nếu có) đã được log bên trong service
			// ở đây chỉ cần ghi nhận là có lỗi ở mức main nếu cần thiết
			log.Printf("[ERROR] Main: HTTPX probing encountered an error: %v", err)
		}
		log.Println("[INFO] Main: HTTPX Probing finished.")
	}

	log.Println("MonsterInc finished.")
}
