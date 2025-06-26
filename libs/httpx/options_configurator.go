package httpx

import (
	"github.com/projectdiscovery/httpx/common/customheader"
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

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
	options.Asn = config.ExtractASN
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
