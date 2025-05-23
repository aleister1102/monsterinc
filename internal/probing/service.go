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

// RunHTTPXProbes thực hiện probing sử dụng httpxrunner dựa trên cấu hình được cung cấp.
func (s *Service) RunHTTPXProbes(appConfig *config.HTTPXConfig) error {
	log.Printf("[INFO] ProbingService: Initializing HTTPX probing with %d targets.", len(appConfig.Targets))

	// Chuyển đổi HTTPXConfig của ứng dụng sang Config của runner
	// Lưu ý: httpxrunner.Config là struct được định nghĩa trong package httpxrunner.
	runnerCfg := &httpxrunner.Config{
		Targets:         appConfig.Targets,
		Method:          appConfig.Method,
		RequestURIs:     appConfig.RequestURIs,
		FollowRedirects: appConfig.FollowRedirects,
		Timeout:         appConfig.Timeout,
		Retries:         appConfig.Retries,
		Threads:         appConfig.Threads,
		CustomHeaders:   appConfig.CustomHeaders,
		Proxy:           appConfig.Proxy,

		// Ánh xạ các cờ trích xuất dữ liệu từ appConfig sang runnerConfig
		// Đảm bảo rằng tên các trường trong appConfig (ví dụ config.HTTPXConfig) khớp
		TechDetect:           appConfig.ExtractTech,
		ExtractTitle:         appConfig.ExtractTitle,
		ExtractStatusCode:    appConfig.ExtractStatus,
		ExtractLocation:      appConfig.ExtractFinalURL,
		ExtractContentLength: appConfig.ExtractLength,
		ExtractServerHeader:  appConfig.ExtractServer,
		ExtractContentType:   appConfig.ExtractType,
		ExtractIPs:           appConfig.ExtractIP,
		ExtractBody:          true, // Mặc định lấy body, hoặc cấu hình trong appConfig
		ExtractHeaders:       true, // Mặc định lấy headers, hoặc cấu hình trong appConfig
	}

	runnerInstance := httpxrunner.NewRunner(runnerCfg)

	err := runnerInstance.Initialize()
	if err != nil {
		return fmt.Errorf("failed to initialize httpx runner: %w", err)
	}
	defer runnerInstance.Close()

	var wg sync.WaitGroup
	resultsCount := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range runnerInstance.Results() {
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
		for err := range runnerInstance.Errors() {
			log.Printf("[ERROR] HTTPX Runner global error: %v", err)
		}
	}()

	log.Println("[INFO] ProbingService: Starting HTTPX probing run...")
	runErr := runnerInstance.Run()
	if runErr != nil {
		log.Printf("[ERROR] ProbingService: HTTPX runner execution failed: %v", runErr)
	}

	wg.Wait()

	log.Printf("[INFO] ProbingService: HTTPX Probing finished.")
	log.Printf("[INFO] ProbingService: Summary - Targets Attempted: %d, Results Processed: %d",
		len(appConfig.Targets),
		resultsCount)
	return runErr
}
