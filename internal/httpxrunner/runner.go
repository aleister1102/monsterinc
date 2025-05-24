package httpxrunner

import (
	"fmt"
	"monsterinc/internal/models"
	"strconv"
	"strings"
	"sync"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
	// Thêm import cho clients và wappalyzer nếu thực sự cần và có sẵn
	// "github.com/projectdiscovery/httpx/common/httpx/clients"
	// "github.com/projectdiscovery/wappalyzergo"
)

// Runner wraps the httpx library runner and provides MonsterInc-specific functionality
type Runner struct {
	httpxRunner *runner.Runner
	options     *runner.Options
	results     chan *models.ProbeResult
	errors      chan error
	wg          sync.WaitGroup
}

// Config holds the configuration for the httpx runner
type Config struct {
	// Target configuration
	Targets []string

	// HTTP configuration
	Method          string
	RequestURIs     []string
	FollowRedirects bool
	Timeout         int
	Retries         int
	Threads         int

	// Output configuration
	OutputFormat string

	// Headers and proxy
	CustomHeaders map[string]string
	Proxy         string

	// Data extraction flags - mapped to httpx.Options
	TechDetect           bool
	ExtractTitle         bool
	ExtractStatusCode    bool
	ExtractLocation      bool
	ExtractContentLength bool
	ExtractServerHeader  bool
	ExtractContentType   bool
	ExtractIPs           bool
	// ExtractCNAMEs        bool // Loại bỏ theo yêu cầu
	// ExtractASN           bool // Loại bỏ theo yêu cầu
	ExtractBody    bool
	ExtractHeaders bool
	// ExtractTLSData       bool // Loại bỏ theo yêu cầu
}

// NewRunner creates a new instance of the httpx runner wrapper
func NewRunner(config *Config) *Runner {
	resultsChan := make(chan *models.ProbeResult, 100)
	options := &runner.Options{
		// Sensible defaults for library usage
		Methods:                 "GET",
		FollowRedirects:         true,
		Timeout:                 10,
		Retries:                 1,
		Threads:                 25,
		MaxRedirects:            10,
		RespectHSTS:             true,
		NoColor:                 true,
		Silent:                  true,
		OmitBody:                false,
		ResponseHeadersInStdout: true,
		// TLSProbe:                false, // Tắt TLSProbe vì không thu thập TLS nữa
		TechDetect: true,
		// Asn:                     false, // Tắt ASN vì không thu thập ASN nữa
		OutputIP: true,
		// OutputCName:             false, // Tắt OutputCName vì không thu thập CNAME nữa
		StatusCode:         true,
		ContentLength:      true,
		OutputContentType:  true,
		ExtractTitle:       true,
		OutputServerHeader: true,
		Location:           true,
	}

	// Apply MonsterInc's specific configuration
	if config != nil {
		if config.Method != "" {
			options.Methods = config.Method
		}
		if len(config.RequestURIs) > 0 {
			options.RequestURI = config.RequestURIs[0]
		}
		options.FollowRedirects = config.FollowRedirects
		if config.Timeout > 0 {
			options.Timeout = config.Timeout
		}
		if config.Retries >= 0 {
			options.Retries = config.Retries
		}
		if config.Threads > 0 {
			options.Threads = config.Threads
		}
		if len(config.CustomHeaders) > 0 {
			headers := customheader.CustomHeaders{}
			for k, v := range config.CustomHeaders {
				headerVal := k + ": " + v
				if err := headers.Set(headerVal); err != nil {
					continue
				}
			}
			options.CustomHeaders = headers
		}
		if config.Proxy != "" {
			options.Proxy = config.Proxy
		}
		options.TechDetect = config.TechDetect
		options.ExtractTitle = config.ExtractTitle
		options.StatusCode = config.ExtractStatusCode
		options.Location = config.ExtractLocation
		options.ContentLength = config.ExtractContentLength
		options.OutputServerHeader = config.ExtractServerHeader
		options.OutputContentType = config.ExtractContentType
		options.OutputIP = config.ExtractIPs
		// options.OutputCName = config.ExtractCNAMEs // Loại bỏ
		// options.Asn = config.ExtractASN             // Loại bỏ
		options.OmitBody = !config.ExtractBody
		options.ResponseHeadersInStdout = config.ExtractHeaders
		// options.TLSProbe = config.ExtractTLSData    // Loại bỏ
	}

	// OnResult Callback: Maps httpx.Result to our ProbeResult
	options.OnResult = func(res runner.Result) {
		probeResult := &models.ProbeResult{
			InputURL:      res.URL,
			Method:        res.Method,
			Timestamp:     res.Timestamp,
			StatusCode:    res.StatusCode,
			ContentLength: int64(res.ContentLength),
			ContentType:   res.ContentType,
			Error:         res.Error,
			FinalURL:      res.FinalURL,
			Title:         res.Title,
			WebServer:     res.WebServer,
			Body:          res.ResponseBody,
		}

		if res.ResponseTime != "" {
			durationStr := strings.TrimSuffix(res.ResponseTime, "s")
			if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
				probeResult.Duration = dur
			}
		}

		if len(res.ResponseHeaders) > 0 {
			probeResult.Headers = make(map[string]string)
			for k, v := range res.ResponseHeaders {
				switch val := v.(type) {
				case string:
					probeResult.Headers[k] = val
				case []string:
					probeResult.Headers[k] = strings.Join(val, ", ")
				case []interface{}:
					var strVals []string
					for _, iv := range val {
						if sv, ok := iv.(string); ok {
							strVals = append(strVals, sv)
						}
					}
					probeResult.Headers[k] = strings.Join(strVals, ", ")
				default:
				}
			}
		}

		// TLSData, CNAMEs, ASN processing is removed as per request.

		if len(res.Technologies) > 0 {
			probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
			for _, techName := range res.Technologies {
				// Tạm thời chỉ lấy Name. Version và Category sẽ được cập nhật sau khi có cấu trúc chính xác.
				tech := models.Technology{Name: techName}
				// if res.TechnologyDetails != nil {
				// 	if appInfo, ok := res.TechnologyDetails[techName]; ok {
				// 		// Logic để lấy Version và Category sẽ được thêm ở đây
				// 	}
				// }
				probeResult.Technologies = append(probeResult.Technologies, tech)
			}
		}

		if len(res.A) > 0 {
			probeResult.IPs = res.A
		}
		// CNAMEs and ASN are removed here.

		resultsChan <- probeResult
	}

	return &Runner{
		options: options,
		results: resultsChan,
		errors:  make(chan error, 10),
	}
}

// Initialize sets up the httpx runner with the provided options
func (r *Runner) Initialize() error {
	httpxRunner, err := runner.New(r.options)
	if err != nil {
		return fmt.Errorf("failed to initialize httpx runner: %w", err)
	}
	r.httpxRunner = httpxRunner
	return nil
}

// Close performs cleanup of the httpx runner
func (r *Runner) Close() {
	if r.httpxRunner != nil {
		r.httpxRunner.Close()
	}
	close(r.results)
	close(r.errors)
}

// SetTargets sets the target hosts to probe
func (r *Runner) SetTargets(targets []string) {
	r.options.InputTargetHost = targets
}

// GetOptions returns the current options configuration
func (r *Runner) GetOptions() *runner.Options {
	return r.options
}

// Run executes the httpx probing with the configured targets
func (r *Runner) Run() error {
	if r.httpxRunner == nil {
		return fmt.Errorf("runner not initialized")
	}

	// RunEnumeration in httpx v1.7.0 does not return an error.
	// It handles errors internally and sends results/errors via callbacks or logs.
	r.httpxRunner.RunEnumeration()

	// Any critical error that stops RunEnumeration prematurely would likely be a panic
	// or would be handled by httpx internal logging if not a panic.
	// The design with OnResult callback implies results (and errors per result) are streamed.
	// If there's a need to signal a global failure of RunEnumeration, httpx would need
	// to provide a mechanism for that (e.g., a returned error or a specific callback).
	// For now, assume RunEnumeration completes its course or panics on unrecoverable errors.
	return nil
}

// Results returns a channel that receives probe results
func (r *Runner) Results() <-chan *models.ProbeResult {
	return r.results
}

// Errors returns a channel that receives errors during probing
// This channel was intended for global errors from RunEnumeration.
// Since RunEnumeration doesn't return an error, this channel might not receive data
// unless we adapt httpx or use it for other types of runner-level errors.
func (r *Runner) Errors() <-chan error {
	return r.errors
}

// processResults processes results from the httpx runner
func (r *Runner) processResults() {
	defer r.wg.Done()

	// TODO: Implement result processing using httpx's HTTPX() method
	// This will require additional research into how to properly handle results
	// from the httpx library
}

// processErrors processes errors from the httpx runner
func (r *Runner) processErrors() {
	defer r.wg.Done()

	// TODO: Implement error processing using httpx's error handling
	// This will require additional research into how to properly handle errors
	// from the httpx library
}
