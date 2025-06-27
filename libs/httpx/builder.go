package httpx

import (
	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

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
	b.config = cfg
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
		return nil, NewValidationError("config", b.config, "config cannot be nil")
	}

	if b.rootTargetURL == "" {
		return nil, NewValidationError("root_target_url", b.rootTargetURL, "root target URL cannot be empty")
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
		return nil, WrapError(err, "failed to initialize httpx engine")
	}

	runnerInstance := &Runner{
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

	return runnerInstance, nil
}
