package httpxrunner

import (
	"fmt"
	"sync"
	"time"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
)

// Runner wraps the httpx library runner and provides MonsterInc-specific functionality
type Runner struct {
	httpxRunner *runner.Runner
	options     *runner.Options
	results     chan *ProbeResult
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
}

// NewRunner creates a new instance of the httpx runner wrapper
func NewRunner(config *Config) *Runner {
	resultsChan := make(chan *ProbeResult, 100)
	options := &runner.Options{
		// Set default values
		Methods:         "GET",
		FollowRedirects: true,
		Timeout:         5,
		Retries:         1,
		Threads:         40,
		Output:          "json",
		OnResult: func(res runner.Result) {
			probeResult := &ProbeResult{
				URL:           res.URL,
				Method:        res.Method,
				Timestamp:     time.Now(),
				StatusCode:    res.StatusCode,
				ContentLength: int64(res.ContentLength),
				ContentType:   res.ContentType,
				Error:         res.Error,
			}

			// Technologies (nếu có)
			if len(res.Technologies) > 0 {
				probeResult.Technologies = make([]Technology, len(res.Technologies))
				for i, tech := range res.Technologies {
					probeResult.Technologies[i] = Technology{
						Name: tech,
					}
				}
			}

			resultsChan <- probeResult
		},
	}

	// Apply custom configuration if provided
	if config != nil {
		if config.Method != "" {
			options.Methods = config.Method
		}
		if len(config.RequestURIs) > 0 {
			options.RequestURIs = config.RequestURIs[0] // Use first URI as default
		}
		options.FollowRedirects = config.FollowRedirects
		if config.Timeout > 0 {
			options.Timeout = config.Timeout
		}
		if config.Retries > 0 {
			options.Retries = config.Retries
		}
		if config.Threads > 0 {
			options.Threads = config.Threads
		}
		if config.OutputFormat != "" {
			options.Output = config.OutputFormat
		}
		if len(config.CustomHeaders) > 0 {
			headers := customheader.CustomHeaders{}
			for k, v := range config.CustomHeaders {
				header := k + ": " + v
				if err := headers.Set(header); err != nil {
					// Log error but continue
					continue
				}
			}
			options.CustomHeaders = headers
		}
		if config.Proxy != "" {
			options.Proxy = config.Proxy
		}
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

	// Chạy httpx, callback sẽ tự động đẩy kết quả vào channel
	r.httpxRunner.RunEnumeration()
	close(r.results)
	return nil
}

// Results returns a channel that receives probe results
func (r *Runner) Results() <-chan *ProbeResult {
	return r.results
}

// Errors returns a channel that receives errors during probing
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
