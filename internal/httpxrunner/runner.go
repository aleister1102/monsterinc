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

// Config holds the configuration for the httpx runner
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

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		CustomHeaders:        make(map[string]string),
		ExtractBody:          false,
		ExtractCNAMEs:        true,
		ExtractContentLength: true,
		ExtractContentType:   true,
		ExtractHeaders:       true,
		ExtractIPs:           true,
		ExtractLocation:      true,
		ExtractServerHeader:  true,
		ExtractStatusCode:    true,
		ExtractTitle:         true,
		FollowRedirects:      true,
		Method:               "GET",
		RateLimit:            0,
		RequestURIs:          []string{},
		Retries:              1,
		Targets:              []string{},
		TechDetect:           true,
		Threads:              25,
		Timeout:              10,
		Verbose:              false,
	}
}

// HTTPXOptionsConfigurator handles configuration of httpx options
type HTTPXOptionsConfigurator struct {
	logger zerolog.Logger
}

// NewHTTPXOptionsConfigurator creates a new options configurator
func NewHTTPXOptionsConfigurator(logger zerolog.Logger) *HTTPXOptionsConfigurator {
	return &HTTPXOptionsConfigurator{
		logger: logger.With().Str("component", "HTTPXOptionsConfigurator").Logger(),
	}
}

// ConfigureOptions applies MonsterInc configuration to httpx.Options
func (hoc *HTTPXOptionsConfigurator) ConfigureOptions(config *Config) *runner.Options {
	options := hoc.createDefaultOptions()

	if config != nil {
		hoc.applyBasicConfig(options, config)
		hoc.applyTargetConfig(options, config)
		hoc.applyPerformanceConfig(options, config)
		hoc.applyCustomHeaders(options, config)
		hoc.applyExtractionConfig(options, config)
	}

	return options
}

// createDefaultOptions creates default httpx options
func (hoc *HTTPXOptionsConfigurator) createDefaultOptions() *runner.Options {
	return &runner.Options{
		Asn:                     true,
		ContentLength:           true,
		ExtractTitle:            true,
		FollowRedirects:         true,
		Location:                true,
		MaxRedirects:            10,
		Methods:                 "GET",
		NoColor:                 true,
		OmitBody:                false,
		OutputCName:             true,
		OutputContentType:       true,
		OutputIP:                true,
		OutputServerHeader:      true,
		RateLimit:               0,
		RespectHSTS:             true,
		ResponseHeadersInStdout: true,
		Retries:                 1,
		Silent:                  true,
		StatusCode:              true,
		TechDetect:              true,
		Threads:                 25,
		Timeout:                 10,
	}
}

// applyBasicConfig applies basic configuration options
func (hoc *HTTPXOptionsConfigurator) applyBasicConfig(options *runner.Options, config *Config) {
	options.Verbose = config.Verbose
	options.Silent = !config.Verbose

	if config.Method != "" {
		options.Methods = config.Method
	}

	options.FollowRedirects = config.FollowRedirects
}

// applyTargetConfig applies target-related configuration
func (hoc *HTTPXOptionsConfigurator) applyTargetConfig(options *runner.Options, config *Config) {
	if len(config.Targets) > 0 {
		options.InputTargetHost = config.Targets
	} else {
		options.InputTargetHost = []string{}
	}

	if len(config.RequestURIs) > 0 {
		options.RequestURI = config.RequestURIs[0]
	}
}

// applyPerformanceConfig applies performance-related configuration
func (hoc *HTTPXOptionsConfigurator) applyPerformanceConfig(options *runner.Options, config *Config) {
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
}

// applyCustomHeaders applies custom headers configuration
func (hoc *HTTPXOptionsConfigurator) applyCustomHeaders(options *runner.Options, config *Config) {
	if len(config.CustomHeaders) == 0 {
		return
	}

	headers := customheader.CustomHeaders{}
	for k, v := range config.CustomHeaders {
		headerVal := k + ": " + v
		if err := headers.Set(headerVal); err != nil {
			hoc.logger.Warn().
				Str("header", headerVal).
				Err(err).
				Msg("Failed to set custom header")
			continue
		}
	}
	options.CustomHeaders = headers
}

// applyExtractionConfig applies extraction-related configuration
func (hoc *HTTPXOptionsConfigurator) applyExtractionConfig(options *runner.Options, config *Config) {
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

// ProbeResultMapper handles mapping from httpx results to ProbeResult
type ProbeResultMapper struct {
	logger zerolog.Logger
}

// NewProbeResultMapper creates a new probe result mapper
func NewProbeResultMapper(logger zerolog.Logger) *ProbeResultMapper {
	return &ProbeResultMapper{
		logger: logger.With().Str("component", "ProbeResultMapper").Logger(),
	}
}

// MapResult converts an httpx runner.Result to a models.ProbeResult
func (prm *ProbeResultMapper) MapResult(res runner.Result, rootURL string) *models.ProbeResult {
	probeResult := prm.createBaseProbeResult(res, rootURL)

	prm.mapDuration(probeResult, res)
	prm.mapHeaders(probeResult, res)
	prm.mapTechnologies(probeResult, res)
	prm.mapNetworkInfo(probeResult, res)
	prm.mapASNInfo(probeResult, res)

	return probeResult
}

// createBaseProbeResult creates the basic probe result structure
func (prm *ProbeResultMapper) createBaseProbeResult(res runner.Result, rootURL string) *models.ProbeResult {
	return &models.ProbeResult{
		Body:          res.ResponseBody,
		ContentLength: int64(res.ContentLength),
		ContentType:   res.ContentType,
		Error:         res.Error,
		FinalURL:      res.URL,
		InputURL:      res.Input,
		Method:        res.Method,
		RootTargetURL: rootURL,
		StatusCode:    res.StatusCode,
		Timestamp:     res.Timestamp,
		Title:         res.Title,
		WebServer:     res.WebServer,
	}
}

// mapDuration maps response time to duration
func (prm *ProbeResultMapper) mapDuration(probeResult *models.ProbeResult, res runner.Result) {
	if res.ResponseTime == "" {
		return
	}

	durationStr := strings.TrimSuffix(res.ResponseTime, "s")
	if dur, err := strconv.ParseFloat(durationStr, 64); err == nil {
		probeResult.Duration = dur
	} else {
		prm.logger.Debug().
			Str("response_time", res.ResponseTime).
			Err(err).
			Msg("Failed to parse response time")
	}
}

// mapHeaders maps response headers
func (prm *ProbeResultMapper) mapHeaders(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.ResponseHeaders) == 0 {
		return
	}

	probeResult.Headers = make(map[string]string)
	for k, v := range res.ResponseHeaders {
		probeResult.Headers[k] = prm.convertHeaderValue(v, k)
	}
}

// convertHeaderValue converts header value from interface{} to string
func (prm *ProbeResultMapper) convertHeaderValue(v interface{}, headerKey string) string {
	switch val := v.(type) {
	case string:
		return val
	case []string:
		return strings.Join(val, ", ")
	case []interface{}:
		return prm.convertInterfaceSliceToString(val)
	default:
		prm.logger.Debug().
			Str("header_key", headerKey).
			Interface("value", v).
			Msg("Unknown header value type")
		return ""
	}
}

// convertInterfaceSliceToString converts []interface{} to comma-separated string
func (prm *ProbeResultMapper) convertInterfaceSliceToString(val []interface{}) string {
	var strVals []string
	for _, iv := range val {
		if sv, ok := iv.(string); ok {
			strVals = append(strVals, sv)
		}
	}
	return strings.Join(strVals, ", ")
}

// mapTechnologies maps detected technologies
func (prm *ProbeResultMapper) mapTechnologies(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.Technologies) == 0 {
		return
	}

	probeResult.Technologies = make([]models.Technology, 0, len(res.Technologies))
	for _, techName := range res.Technologies {
		tech := models.Technology{Name: techName}
		probeResult.Technologies = append(probeResult.Technologies, tech)
	}
}

// mapNetworkInfo maps network information
func (prm *ProbeResultMapper) mapNetworkInfo(probeResult *models.ProbeResult, res runner.Result) {
	if len(res.A) > 0 {
		probeResult.IPs = res.A
	}
	// CNAMEs mapping can be added here if needed
}

// mapASNInfo maps ASN information
func (prm *ProbeResultMapper) mapASNInfo(probeResult *models.ProbeResult, res runner.Result) {
	if res.ASN == nil {
		return
	}

	if res.ASN.AsNumber != "" {
		if asnNumber, err := prm.parseASNNumber(res.ASN.AsNumber); err == nil {
			probeResult.ASN = asnNumber
		}
	}

	probeResult.ASNOrg = res.ASN.AsName
}

// parseASNNumber parses ASN number from string
func (prm *ProbeResultMapper) parseASNNumber(asNumber string) (int, error) {
	cleanNumber := strings.ReplaceAll(asNumber, "AS", "")
	return strconv.Atoi(cleanNumber)
}

// ResultCollector handles collection of probe results
type ResultCollector struct {
	results []models.ProbeResult
	mutex   sync.RWMutex
	logger  zerolog.Logger
}

// NewResultCollector creates a new result collector
func NewResultCollector(logger zerolog.Logger) *ResultCollector {
	return &ResultCollector{
		results: make([]models.ProbeResult, 0),
		logger:  logger.With().Str("component", "ResultCollector").Logger(),
	}
}

// AddResult adds a result to the collection
func (rc *ResultCollector) AddResult(result *models.ProbeResult) {
	if result == nil {
		rc.logger.Warn().Msg("Attempted to add nil result")
		return
	}

	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.results = append(rc.results, *result)
	rc.logger.Debug().
		Str("input_url", result.InputURL).
		Int("status_code", result.StatusCode).
		Msg("Result added to collection")
}

// GetResults returns all collected results
func (rc *ResultCollector) GetResults() []models.ProbeResult {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	// Return copy to prevent external modifications
	resultsCopy := make([]models.ProbeResult, len(rc.results))
	copy(resultsCopy, rc.results)

	return resultsCopy
}

// GetResultsCount returns the number of collected results
func (rc *ResultCollector) GetResultsCount() int {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	return len(rc.results)
}

// Runner wraps the httpx library runner
type Runner struct {
	config        *Config
	httpxRunner   *runner.Runner
	logger        zerolog.Logger
	options       *runner.Options
	rootTargetURL string
	wg            sync.WaitGroup
	configurator  *HTTPXOptionsConfigurator
	mapper        *ProbeResultMapper
	collector     *ResultCollector
}

// RunnerBuilder provides a fluent interface for creating Runner
type RunnerBuilder struct {
	config        *Config
	rootTargetURL string
	logger        zerolog.Logger
}

// NewRunnerBuilder creates a new builder
func NewRunnerBuilder(logger zerolog.Logger) *RunnerBuilder {
	return &RunnerBuilder{
		config: DefaultConfig(),
		logger: logger.With().Str("component", "HTTPXRunner").Logger(),
	}
}

// WithConfig sets the configuration
func (b *RunnerBuilder) WithConfig(cfg *Config) *RunnerBuilder {
	if cfg != nil {
		b.config = cfg
	}
	return b
}

// WithRootTargetURL sets the root target URL
func (b *RunnerBuilder) WithRootTargetURL(rootURL string) *RunnerBuilder {
	b.rootTargetURL = rootURL
	return b
}

// Build creates a new Runner instance
func (b *RunnerBuilder) Build() (*Runner, error) {
	if b.config == nil {
		return nil, common.NewValidationError("config", b.config, "config cannot be nil")
	}

	if b.rootTargetURL == "" {
		return nil, common.NewValidationError("root_target_url", b.rootTargetURL, "root target URL cannot be empty")
	}

	// Create components
	configurator := NewHTTPXOptionsConfigurator(b.logger)
	mapper := NewProbeResultMapper(b.logger)
	collector := NewResultCollector(b.logger)

	// Configure httpx options
	options := configurator.ConfigureOptions(b.config)

	// Set up result callback
	options.OnResult = func(result runner.Result) {
		probeRes := mapper.MapResult(result, b.rootTargetURL)
		collector.AddResult(probeRes)
	}

	// Create httpx runner
	httpxRunner, err := runner.New(options)
	if err != nil {
		return nil, common.WrapError(err, "failed to initialize httpx engine")
	}

	runner := &Runner{
		config:        b.config,
		httpxRunner:   httpxRunner,
		logger:        b.logger,
		options:       options,
		rootTargetURL: b.rootTargetURL,
		configurator:  configurator,
		mapper:        mapper,
		collector:     collector,
	}

	b.logger.Info().
		Str("root_target", b.rootTargetURL).
		Int("threads", b.config.Threads).
		Int("timeout", b.config.Timeout).
		Msg("HTTPX runner initialized successfully")

	return runner, nil
}

// NewRunner creates a new HTTPX runner instance using builder pattern
func NewRunner(cfg *Config, rootTargetForThisInstance string, appLogger zerolog.Logger) (*Runner, error) {
	return NewRunnerBuilder(appLogger).
		WithConfig(cfg).
		WithRootTargetURL(rootTargetForThisInstance).
		Build()
}

// validateRunState validates the runner state before execution
func (r *Runner) validateRunState() error {
	if r.httpxRunner == nil {
		return common.NewError("httpx engine not initialized")
	}

	if r.collector == nil {
		return common.NewError("result collector not initialized")
	}

	return nil
}

// executeRunner executes the httpx runner in a goroutine
func (r *Runner) executeRunner() {
	defer r.wg.Done()

	r.logger.Debug().Msg("Starting httpx enumeration")
	r.httpxRunner.RunEnumeration()
	r.logger.Debug().Msg("Httpx enumeration completed")
}

// waitForCompletion waits for runner completion or context cancellation
func (r *Runner) waitForCompletion(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		resultCount := r.collector.GetResultsCount()
		r.logger.Info().
			Int("results_collected", resultCount).
			Msg("HTTPX runner completed successfully")
		return nil
	case <-ctx.Done():
		result := common.CheckCancellationWithLog(ctx, r.logger, "HTTPX runner execution")
		if result.Cancelled {
			r.logger.Info().Msg("HTTPX runner cancelled by context")
			return result.Error
		}
		return nil
	}
}

// Run executes the HTTPX runner with context support
func (r *Runner) Run(ctx context.Context) error {
	// Validate runner state
	if err := r.validateRunState(); err != nil {
		return common.WrapError(err, "failed to validate runner state")
	}

	r.logger.Info().
		Str("root_target", r.rootTargetURL).
		Int("target_count", len(r.config.Targets)).
		Msg("Starting HTTPX runner execution")

	// Execute runner
	r.wg.Add(1)
	go r.executeRunner()

	// Wait for completion
	return r.waitForCompletion(ctx)
}

// GetResults returns all collected probe results after the run is complete
func (r *Runner) GetResults() []models.ProbeResult {
	return r.collector.GetResults()
}
