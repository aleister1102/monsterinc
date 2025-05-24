package probing

import (
	"fmt"
	"log"
	"monsterinc/internal/config"
	"monsterinc/internal/httpxrunner" // Import httpxrunner
	"strings"
	"sync"
)

// Service handles the orchestration of HTTPX probing tasks.
type Service struct {
	// Dependencies can be added here later, e.g., a logger instance
}

// NewService creates a new ProbingService.
func NewService() *Service {
	return &Service{}
}

// RunHTTPXProbes executes HTTPX probing based on the provided application configuration.
// It now expects *config.HTTPXRunnerConfig.
func (s *Service) RunHTTPXProbes(runnerCfg *config.HTTPXRunnerConfig, targets []string) error {
	log.Println("[INFO] ProbingService: Starting HTTPX probes...")

	if runnerCfg == nil {
		return fmt.Errorf("httpx runner configuration is nil")
	}
	if len(targets) == 0 {
		log.Println("[INFO] ProbingService: No targets provided for HTTPX probing.")
		return nil
	}

	// Create a new httpxrunner.Config from the global HTTPXRunnerConfig
	// This mapping is necessary because httpxrunner.Config is specific to the runner library wrapper.
	httpxLibConfig := &httpxrunner.Config{
		Targets:         targets, // Use the passed targets
		Threads:         runnerCfg.Threads,
		Timeout:         runnerCfg.Timeout,
		Retries:         runnerCfg.Retries,
		FollowRedirects: runnerCfg.FollowRedirects,
		CustomHeaders:   runnerCfg.CustomHeaders,
		Proxy:           runnerCfg.Proxy,
		// Map other relevant fields from runnerCfg to httpxLibConfig
		// For example, if httpxrunner.Config had fields for TechDetect, ExtractTitle, etc., map them here.
		// Based on the current httpxrunner.Config, many of these are booleans set by default or can be mapped.
		TechDetect:           true, // Default, or map from runnerCfg if available
		ExtractTitle:         true, // Default, or map from runnerCfg if available
		ExtractStatusCode:    true, // Default, or map from runnerCfg if available
		ExtractLocation:      true, // Default, or map from runnerCfg if available
		ExtractContentLength: true, // Default, or map from runnerCfg if available
		ExtractServerHeader:  true, // Default, or map from runnerCfg if available
		ExtractContentType:   true, // Default, or map from runnerCfg if available
		ExtractIPs:           true, // Default, or map from runnerCfg if available
		ExtractBody:          true, // Default, or map from runnerCfg if available (assuming we want body)
		ExtractHeaders:       true, // Default, or map from runnerCfg if available
	}

	runner := httpxrunner.NewRunner(httpxLibConfig)

	err := runner.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize httpx runner: %w", err)
	}
	defer runner.Close()

	var wg sync.WaitGroup
	resultsCount := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range runner.Results() {
			resultsCount++
			if result.Error != "" {
				log.Printf("[RESULT] HTTPX Probe for %s FAILED: %s (Status: %d)", result.InputURL, result.Error, result.StatusCode)
			} else {
				log.Printf("[RESULT] HTTPX Probe for %s SUCCESS: Status %d, Length %d, Type %s, FinalURL: %s, Title: %s, WebServer: %s",
					result.InputURL, result.StatusCode, result.ContentLength, result.ContentType, result.FinalURL, result.Title, result.WebServer)
				if len(result.IPs) > 0 {
					log.Printf("    IPs: %s", strings.Join(result.IPs, ", "))
				}
				if result.HasTechnologies() {
					var techNames []string
					for _, tech := range result.Technologies {
						name := tech.Name
						if tech.Version != "" { // Vẫn giữ logic hiển thị version nếu có
							name += " (" + tech.Version + ")"
						}
						techNames = append(techNames, name)
					}
					log.Printf("    Technologies: %s", strings.Join(techNames, ", "))
				}
			}
		}
	}()

	go func() {
		for err := range runner.Errors() {
			log.Printf("[ERROR] HTTPX Runner global error: %v", err)
		}
	}()

	log.Println("[INFO] ProbingService: Starting HTTPX probing run...")
	runErr := runner.Run()
	if runErr != nil {
		log.Printf("[ERROR] ProbingService: HTTPX runner execution failed: %v", runErr)
	}

	wg.Wait()

	log.Printf("[INFO] ProbingService: HTTPX Probing finished.")
	log.Printf("[INFO] ProbingService: Summary - Targets Attempted: %d, Results Processed: %d",
		len(targets),
		resultsCount)
	return runErr
}
