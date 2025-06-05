package httpxrunner

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/common"
	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

// Runner wraps the httpx library runner
// Refactored ✅
type Runner struct {
	collectedResults []*models.ProbeResult
	config           *Config
	httpxRunner      *runner.Runner
	logger           zerolog.Logger
	options          *runner.Options
	resultsMutex     sync.Mutex // Added to protect collectedResults
	rootTargetURL    string     // Store the root target URL for this runner instance
	wg               sync.WaitGroup
}

// Config holds the configuration for the httpx runner
// Refactored ✅
type Config struct {
	CustomHeaders        map[string]string
	ExtractBody          bool
	ExtractCNAMEs        bool
	ExtractContentLength bool
	ExtractContentType   bool
	ExtractHeaders       bool
	ExtractIPs           bool
	ExtractLocation      bool
	ExtractServerHeader  bool
	ExtractStatusCode    bool
	ExtractTitle         bool
	FollowRedirects      bool
	Method               string
	RateLimit            int
	RequestURIs          []string
	Retries              int
	Targets              []string
	TechDetect           bool
	Threads              int
	Timeout              int // In seconds
	Verbose              bool
}

// NewRunner creates a new HTTPX runner instance
// Refactored ✅
func NewRunner(cfg *Config, rootTargetForThisInstance string, appLogger zerolog.Logger) (*Runner, error) {
	if cfg == nil {
		return nil, common.NewValidationError("config", cfg, "config cannot be nil")
	}

	runnerLogger := appLogger.With().Str("component", "HTTPXRunner").Logger()

	// Initialize Runner
	r := &Runner{
		config:           cfg,
		rootTargetURL:    rootTargetForThisInstance,
		collectedResults: make([]*models.ProbeResult, 0),
		logger:           runnerLogger,
	}

	// Configure httpx options
	options := configureHttpxOptions(cfg, runnerLogger)
	options.OnResult = func(result runner.Result) {
		// This callback will be invoked for each result by the httpx engine.
		probeRes := mapHttpxResultToProbeResult(result, r.rootTargetURL) // Use r.rootTargetURL from the outer scope
		if probeRes != nil {
			r.resultsMutex.Lock()
			r.collectedResults = append(r.collectedResults, probeRes)
			r.resultsMutex.Unlock()
		}
	}

	// Create httpx runner
	httpxRunner, err := runner.New(options)
	if err != nil {
		return nil, common.WrapError(err, "failed to initialize httpx engine")
	}

	// Update httpx runner and options of the wrapper runner
	r.httpxRunner = httpxRunner
	r.options = options

	runnerLogger.Info().
		Str("root_target", rootTargetForThisInstance).
		Int("threads", cfg.Threads).
		Int("timeout", cfg.Timeout).
		Msg("HTTPX runner initialized")

	return r, nil
}

// configureHttpxOptions applies the MonsterInc configuration to httpx.Options.
// It centralizes the logic for setting up httpx based on the provided Config.
// Refactored ✅
func configureHttpxOptions(config *Config, logger zerolog.Logger) *runner.Options {
	// Defaults options
	options := &runner.Options{
		Asn:                     true,
		ContentLength:           true,
		ExtractTitle:            true,
		FollowRedirects:         true,
		Location:                true,
		MaxRedirects:            10,
		Methods:                 "GET",
		NoColor:                 true, // Recommended for programmatic use
		OmitBody:                false,
		OutputCName:             true, // Default to true, controlled by ExtractCNAMEs
		OutputContentType:       true,
		OutputIP:                true,
		OutputServerHeader:      true,
		RateLimit:               0, // Default rate limit (0 often means no limit or use threads)
		RespectHSTS:             true,
		ResponseHeadersInStdout: true,
		Retries:                 1,
		Silent:                  true, // Overridden by Verbose if true
		StatusCode:              true,
		TechDetect:              true,
		Threads:                 25,
		Timeout:                 10, // Default timeout in seconds
	}

	// Apply specific configuration
	if config != nil {
		options.Verbose = config.Verbose // Use config's Verbose setting
		options.Silent = !config.Verbose // If Verbose is true, Silent should be false

		if len(config.Targets) > 0 {
			options.InputTargetHost = config.Targets
		} else {
			options.InputTargetHost = []string{} // Explicitly set to empty slice
		}
		if config.Method != "" {
			options.Methods = config.Method
		}
		if len(config.RequestURIs) > 0 {
			options.RequestURI = config.RequestURIs[0] // httpx seems to take one primary request URI
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
		if config.RateLimit > 0 {
			options.RateLimit = config.RateLimit
		}
		if len(config.CustomHeaders) > 0 {
			headers := customheader.CustomHeaders{}
			for k, v := range config.CustomHeaders {
				headerVal := k + ": " + v
				if err := headers.Set(headerVal); err != nil {
					logger.Warn().Str("component", "HttpxRunnerSetup").Str("header", headerVal).Err(err).Msg("Failed to set custom header")
					continue
				}
			}
			options.CustomHeaders = headers
		}

		options.ContentLength = config.ExtractContentLength
		options.ExtractTitle = config.ExtractTitle
		options.Location = config.ExtractLocation
		options.OmitBody = !config.ExtractBody
		options.OutputCName = config.ExtractCNAMEs
		options.OutputContentType = config.ExtractContentType
		options.OutputIP = config.ExtractIPs
		options.OutputServerHeader = config.ExtractServerHeader
		options.ResponseHeadersInStdout = config.ExtractHeaders
		options.StatusCode = config.ExtractStatusCode
		options.TechDetect = config.TechDetect
	}
	return options
}

// mapHttpxResultToProbeResult converts an httpx runner.Result to a models.ProbeResult.
// Refactored ✅
func mapHttpxResultToProbeResult(res runner.Result, rootURL string) *models.ProbeResult {
	probeResult := &models.ProbeResult{
		Body:          res.ResponseBody, // Assuming OmitBody=false allows this
		ContentLength: int64(res.ContentLength),
		ContentType:   res.ContentType,
		Error:         res.Error,
		FinalURL:      res.URL, // res.URL is the final URL after redirects
		InputURL:      res.Input,
		Method:        res.Method,
		RootTargetURL: rootURL, // Assign the root target URL
		StatusCode:    res.StatusCode,
		Timestamp:     res.Timestamp,
		Title:         res.Title,
		WebServer:     res.WebServer,
	}

	if res.ResponseTime != "" {
		durationStr := strings.TrimSuffix(res.ResponseTime, "s")
		if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
			probeResult.Duration = dur
		}
	}

	if len(res.ResponseHeaders) > 0 {
		probeResult.Headers = make(map[string]string)
		for k, v := range res.ResponseHeaders { // v is interface{}
			switch val := v.(type) {
			case string:
				probeResult.Headers[k] = val
			case []string:
				probeResult.Headers[k] = strings.Join(val, ", ")
			case []interface{}: // Handle cases where it might be []interface{} containing strings
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

	if len(res.Technologies) > 0 {
		probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
		for _, techName := range res.Technologies {
			tech := models.Technology{Name: techName}
			probeResult.Technologies = append(probeResult.Technologies, tech)
		}
	}

	if len(res.A) > 0 {
		probeResult.IPs = res.A
	}
	if len(res.CNAMEs) > 0 {
	}

	if res.ASN != nil {
		if res.ASN.AsNumber != "" {
			asnNumber, err := strconv.Atoi(strings.ReplaceAll(res.ASN.AsNumber, "AS", ""))
			if err == nil {
				probeResult.ASN = asnNumber
			}
		}
		probeResult.ASNOrg = res.ASN.AsName // Corrected to AsName
	}

	return probeResult
}

// Run executes the HTTPX runner with context support
// Refactored ✅
func (r *Runner) Run(ctx context.Context) error {
	if r.httpxRunner == nil {
		return common.NewError("httpx engine not initialized")
	}

	r.logger.Info().Msg("Starting HTTPX runner execution")

	// Execute httpx runner
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		r.httpxRunner.RunEnumeration()
	}()

	// Wait for completion or cancellation
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info().Int("results_collected", len(r.collectedResults)).Msg("HTTPX runner completed")
		return nil
	case <-ctx.Done():
		result := common.CheckCancellationWithLog(ctx, r.logger, "HTTPX runner execution")
		if result.Cancelled {
			r.logger.Info().Msg("HTTPX runner cancelled")
			return result.Error
		}
		return nil
	}
}

// GetResults returns all collected probe results after the run is complete.
// No need to refactor ✅
func (r *Runner) GetResults() []models.ProbeResult {
	r.resultsMutex.Lock()
	defer r.resultsMutex.Unlock()
	actualResults := make([]models.ProbeResult, len(r.collectedResults))
	for i, ptrResult := range r.collectedResults {
		if ptrResult != nil {
			actualResults[i] = *ptrResult
		}
	}
	return actualResults
}
