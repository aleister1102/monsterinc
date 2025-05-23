package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/crawler"
	"os"
	"strings"
)

func main() {
	fmt.Println("MonsterInc Crawler starting...")

	// Define a command-line flag for the URL list file
	urlListFile := flag.String("urlfile", "", "Path to a text file containing seed URLs (one URL per line)")
	configFile := flag.String("config", "", "Path to a JSON configuration file for the crawler. If not provided, default settings are used.")
	flag.Parse()

	var cfg *config.CrawlerConfig
	var err error

	if *configFile != "" {
		log.Printf("[INFO] Main: Loading configuration from %s", *configFile)
		cfg, err = config.LoadCrawlerConfigFromFile(*configFile)
		if err != nil {
			log.Fatalf("[FATAL] Main: Could not load configuration from '%s': %v", *configFile, err)
		}
	} else {
		log.Printf("[INFO] Main: No configuration file provided, using default settings.")
		cfg = config.NewDefaultCrawlerConfig()
	}

	// Override or set seed URLs from the urlfile if provided
	if *urlListFile != "" {
		log.Printf("[INFO] Main: Reading seed URLs from %s", *urlListFile)
		file, err := os.Open(*urlListFile)
		if err != nil {
			log.Fatalf("[FATAL] Main: Could not open URL list file '%s': %v", *urlListFile, err)
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

		if err := scanner.Err(); err != nil {
			log.Fatalf("[FATAL] Main: Error reading URL list file '%s': %v", *urlListFile, err)
		}

		if len(urls) == 0 {
			log.Fatalf("[FATAL] Main: No valid URLs found in '%s'", *urlListFile)
		}
		cfg.SeedURLs = urls // Override seed URLs from the file
	} else if len(cfg.SeedURLs) == 0 {
		// If no config file with seeds and no urlfile is provided
		log.Fatalf("[FATAL] Main: No seed URLs provided. Use -urlfile or provide seeds in a config file.")
	}

	log.Printf("[INFO] Main: Initializing crawler with %d seed URLs.", len(cfg.SeedURLs))

	crawlerInstance, err := crawler.NewCrawler(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Main: Failed to initialize crawler: %v", err)
	}

	log.Println("[INFO] Main: Starting crawl...")
	crawlerInstance.Start()

	log.Println("[INFO] Main: Crawl finished.")
	log.Printf("[INFO] Main: Total URLs discovered: %d", len(crawlerInstance.GetDiscoveredURLs()))

	// Example: Print all discovered URLs
	// for _, u := range crawlerInstance.GetDiscoveredURLs() {
	// 	fmt.Println(u)
	// }
}
