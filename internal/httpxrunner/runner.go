package httpxrunner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/aleister1102/monsterinc/internal/models"

	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
	// Thêm import cho clients và wappalyzer nếu thực sự cần và có sẵn
	// "github.com/projectdiscovery/httpx/common/httpx/clients"
	// "github.com/projectdiscovery/wappalyzergo"
)

// Runner wraps the httpx library runner and provides MonsterInc-specific functionality
type Runner struct {
	httpxRunner      *runner.Runner
	options          *runner.Options
	config           *Config // Store the passed config
	rootTargetURL    string  // Store the root target URL for this runner instance
	results          chan *models.ProbeResult
	collectedResults []*models.ProbeResult // Added to store all results
	resultsMutex     sync.Mutex            // Added to protect collectedResults
	errors           chan error
	wg               sync.WaitGroup
	logger           zerolog.Logger // Added logger field
}

// Config holds the configuration for the httpx runner
type Config struct {
	// Target configuration
	Targets []string // Can be URLs or file paths

	// HTTP configuration
	Method          string
	RequestURIs     []string
	FollowRedirects bool
	Timeout         int // Timeout in seconds
	Retries         int
	Threads         int
	RateLimit       int // Added RateLimit (requests per second)

	// Output configuration
	OutputFormat string // Currently not used by this wrapper but kept for future flexibility
	Verbose      bool

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
	ExtractBody          bool
	ExtractHeaders       bool
	ExtractCNAMEs        bool // Added for CNAME extraction
	ExtractASN           bool // Added for ASN extraction
	ExtractTLSData       bool // Added for TLS data extraction
}

// configureHttpxOptions applies the MonsterInc configuration to httpx.Options.
// It centralizes the logic for setting up httpx based on the provided Config.
func configureHttpxOptions(config *Config, logger zerolog.Logger) *runner.Options {
	options := &runner.Options{
		// Sensible defaults for library usage
		Methods:                 "GET",
		FollowRedirects:         true,
		Timeout:                 10, // Default timeout in seconds
		Retries:                 1,
		Threads:                 25,
		MaxRedirects:            10,
		RespectHSTS:             true,
		NoColor:                 true, // Recommended for programmatic use
		Silent:                  true, // Overridden by Verbose if true
		OmitBody:                false,
		ResponseHeadersInStdout: true,
		TechDetect:              true,
		OutputIP:                true,
		StatusCode:              true,
		ContentLength:           true,
		OutputContentType:       true,
		ExtractTitle:            true,
		OutputServerHeader:      true,
		Location:                true,
		RateLimit:               0,    // Default rate limit (0 often means no limit or use threads)
		OutputCName:             true, // Default to true, controlled by ExtractCNAMEs
		Asn:                     true, // Default to true, controlled by ExtractASN
		TLSProbe:                true, // Default to true, controlled by ExtractTLSData
	}

	// Apply MonsterInc's specific configuration
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
					// Use the instance logger if available, otherwise a temporary one or standard log for this rare setup case
					// Since this function is static and doesn't have access to r.logger, we can't use it directly.
					// For now, let's assume this setup error is rare and a global/temp logger might be too much overhead.
					// If this becomes problematic, we might need to pass a logger to configureHttpxOptions.
					// For the purpose of removing std log, this will be commented.
					// log.Printf("[WARN] HTTPXRunner: Failed to set custom header %s: %v", headerVal, err)
					logger.Warn().Str("component", "HttpxRunnerSetup").Str("header", headerVal).Err(err).Msg("Failed to set custom header")
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
		options.OmitBody = !config.ExtractBody
		options.ResponseHeadersInStdout = config.ExtractHeaders
		options.OutputCName = config.ExtractCNAMEs
		options.Asn = config.ExtractASN
		options.TLSProbe = config.ExtractTLSData
	}
	return options
}

// mapHttpxResultToProbeResult converts an httpx runner.Result to a models.ProbeResult.
func mapHttpxResultToProbeResult(res runner.Result, rootURL string) *models.ProbeResult {
	probeResult := &models.ProbeResult{
		InputURL:      res.Input,
		Method:        res.Method,
		Timestamp:     res.Timestamp,
		StatusCode:    res.StatusCode,
		ContentLength: int64(res.ContentLength),
		ContentType:   res.ContentType,
		Error:         res.Error,
		FinalURL:      res.URL, // res.URL is the final URL after redirects
		Title:         res.Title,
		WebServer:     res.WebServer,
		Body:          res.ResponseBody, // Assuming OmitBody=false allows this
		RootTargetURL: rootURL,          // Assign the root target URL
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
				// Optionally log or handle other unexpected types for header values
				// log.Printf("[WARN] HTTPXRunner: Unexpected type for header '%s': %T", k, v)
			}
		}
	}

	if len(res.Technologies) > 0 {
		probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
		for _, techName := range res.Technologies {
			tech := models.Technology{Name: techName}
			// Version/Category might come from wappalyzer integration if enabled/configured
			probeResult.Technologies = append(probeResult.Technologies, tech)
		}
	}

	if len(res.A) > 0 {
		probeResult.IPs = res.A
	}
	if len(res.CNAMEs) > 0 { // Map CNAMEs if available
		probeResult.CNAMEs = res.CNAMEs
	}

	// Map ASN if available
	if res.ASN != nil {
		if res.ASN.AsNumber != "" {
			asnNumber, err := strconv.Atoi(strings.ReplaceAll(res.ASN.AsNumber, "AS", ""))
			if err == nil {
				probeResult.ASN = asnNumber
			}
		}
		probeResult.ASNOrg = res.ASN.AsName // Corrected to AsName
	}

	if res.TLSData != nil { // Map TLS data if available
		probeResult.TLSVersion = res.TLSData.Version // Direct field from clients.Response
		probeResult.TLSCipher = res.TLSData.Cipher   // Direct field from clients.Response

		// Mapping for certificate details depends on the exact structure of res.TLSData.CertificateResponse
		// Assuming res.TLSData has a field like `CertificateResponse` which is a struct/pointer.
		// Example structure: res.TLSData.CertificateResponse.IssuerCN []string and res.TLSData.CertificateResponse.NotAfter time.Time
		// This is a common pattern but needs to be verified against the exact `clients.Response` structure used by `httpx`.
		if certData := res.TLSData.CertificateResponse; certData != nil { // Check if CertificateResponse exists
			if certData.IssuerCN != "" {
				probeResult.TLSCertIssuer = certData.IssuerCN // Directly assign if it's a string and not empty
			}
			// Check if NotAfter is a non-zero time
			if !certData.NotAfter.IsZero() {
				probeResult.TLSCertExpiry = certData.NotAfter
			}
		}
	}
	return probeResult
}

// NewRunner creates a new instance of the httpx runner wrapper
// rootTargetForThisInstance is the primary target URL this runner instance is responsible for.
func NewRunner(cfg *Config, rootTargetForThisInstance string, appLogger zerolog.Logger) (*Runner, error) {
	moduleLogger := appLogger.With().Str("module", "HttpxRunner").Str("root_target", rootTargetForThisInstance).Logger()

	if cfg == nil {
		moduleLogger.Error().Msg("Configuration for HttpxRunner cannot be nil.")
		return nil, fmt.Errorf("httpxrunner: config cannot be nil")
	}

	r := &Runner{
		// Initialize collectedResults and other fields early if they are needed by OnResult closure
		collectedResults: make([]*models.ProbeResult, 0),
		resultsMutex:     sync.Mutex{},
		config:           cfg,
		rootTargetURL:    rootTargetForThisInstance,
		results:          make(chan *models.ProbeResult, (cfg.Threads*2)+10), // Adjusted buffer size
		errors:           make(chan error, 1),
		logger:           moduleLogger,
	}

	options := configureHttpxOptions(cfg, moduleLogger)
	options.OnResult = func(result runner.Result) {
		// This callback will be invoked for each result by the httpx engine.
		probeRes := mapHttpxResultToProbeResult(result, r.rootTargetURL) // Use r.rootTargetURL from the outer scope
		if probeRes != nil {
			r.resultsMutex.Lock()
			r.collectedResults = append(r.collectedResults, probeRes)
			r.resultsMutex.Unlock()

			// Optionally, if r.results channel is still intended for real-time streaming (though GetResults is primary)
			// select {
			// case r.results <- probeRes:
			// default:
			// 	r.logger.Warn().Str("url", probeRes.InputURL).Msg("Results channel full or closed, dropping result for streaming.")
			// }
		}
	}

	httpxRun, err := runner.New(options)
	if err != nil {
		moduleLogger.Error().Err(err).Msg("Failed to initialize httpx engine.")
		return nil, fmt.Errorf("httpxrunner: failed to initialize httpx engine: %w", err)
	}

	r.httpxRunner = httpxRun // Assign httpxRun to r after options (and its OnResult) are fully set up.
	r.options = options      // Store options as well

	moduleLogger.Info().Msg("HttpxRunner initialized successfully.")
	return r, nil
}

// Run executes the httpx probing against the configured targets.
func (r *Runner) Run(ctx context.Context) error {
	if r.httpxRunner == nil {
		r.logger.Error().Msg("Httpx engine is not initialized. Call NewRunner first.")
		return fmt.Errorf("httpx engine not initialized")
	}

	if len(r.config.Targets) == 0 {
		r.logger.Info().Msg("No targets configured for HttpxRunner. Nothing to do.")
		close(r.results) // Close channels as there's nothing to process
		close(r.errors)
		return nil
	}

	r.logger.Info().Int("target_count", len(r.config.Targets)).Msg("Starting httpx enumeration")

	// httpx.Runner.RunEnumeration() is blocking.
	// It will use the targets provided during options setup (e.g. options.InputTargetHost or via options.ScanInput + strings.NewReader)
	// For this setup, we assume targets are already in r.config.Targets and were used to set r.options.InputTargetHost in configureHttpxOptions.
	// If using a different method like ScanInput, that needs to be set in configureHttpxOptions.

	// The context handling with httpx.RunEnumeration is a bit tricky as it doesn't directly accept a context.
	// httpx internally might handle SIGINT, but for programmatic cancellation:
	// One common pattern is to call httpxRunner.Close() from another goroutine when the context is done.
	done := make(chan struct{})
	go func() {
		r.httpxRunner.RunEnumeration()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info().Msg("Httpx RunEnumeration completed.")
	case <-ctx.Done():
		r.logger.Info().Msg("Context cancelled during httpx RunEnumeration. Attempting to close httpx runner.")
		r.httpxRunner.Close() // This should interrupt RunEnumeration
		<-done                // Wait for RunEnumeration to actually finish after Close
		r.logger.Info().Msg("Httpx runner closed due to context cancellation.")
		// Send error to r.errors channel if it's not already closed
		select {
		case r.errors <- ctx.Err():
		default:
			r.logger.Warn().Err(ctx.Err()).Msg("Failed to send context cancellation error to error channel (full or closed)")
		}
		// Close results and errors channels here as the run was prematurely stopped.
		close(r.results)
		close(r.errors)
		return ctx.Err()
	}

	// Close the results and error channels as the run is complete.
	close(r.results)
	close(r.errors)
	r.logger.Info().Int("result_count", len(r.collectedResults)).Msg("Httpx probing finished.")
	return nil
}

// GetResults returns all collected probe results after the run is complete.
func (r *Runner) GetResults() []models.ProbeResult {
	r.resultsMutex.Lock()
	defer r.resultsMutex.Unlock()
	// Convert []*models.ProbeResult to []models.ProbeResult
	// Create a new slice of the non-pointer type
	actualResults := make([]models.ProbeResult, len(r.collectedResults))
	for i, ptrResult := range r.collectedResults {
		if ptrResult != nil { // Ensure pointer is not nil before dereferencing
			actualResults[i] = *ptrResult
		}
		// else: handle nil pointer case if necessary, e.g., skip or log
	}
	return actualResults
}
