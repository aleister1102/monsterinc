package telescope

import (
	"fmt"

	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

// RunnerBuilder is a fluent builder for creating a configured Runner.
// It allows for a step-by-step, readable configuration process.
type RunnerBuilder struct {
	logger        zerolog.Logger
	config        *Config
	rootTargetURL string
	errors        []error
}

// NewRunnerBuilder creates a new builder with a logger.
func NewRunnerBuilder(logger zerolog.Logger) *RunnerBuilder {
	return &RunnerBuilder{
		logger: logger.With().Str("component", "RunnerBuilder").Logger(),
	}
}

// WithConfig sets the configuration for the runner.
func (b *RunnerBuilder) WithConfig(config *Config) *RunnerBuilder {
	b.config = config
	return b
}

// WithRootTargetURL sets the root target URL for the scan.
func (b *RunnerBuilder) WithRootTargetURL(url string) *RunnerBuilder {
	b.rootTargetURL = url
	return b
}

// Build finalizes the configuration and returns a Runner.
func (b *RunnerBuilder) Build() (*Runner, error) {
	if b.config == nil {
		b.errors = append(b.errors, ErrConfigNotSet)
	}

	if b.rootTargetURL == "" {
		b.errors = append(b.errors, ErrRootURLNotSet)
	}

	if len(b.errors) > 0 {
		return nil, ErrBuilderErrors(b.errors)
	}

	// Create the result mapper and collector
	resultMapper := NewProbeResultMapper(b.logger)
	resultCollector := NewResultCollector(b.logger, resultMapper, b.rootTargetURL)

	// Create the options configurator
	optionsConfigurator := NewOptionsConfigurator(b.config, b.rootTargetURL, resultCollector.Collect)

	// Create the underlying httpx runner
	// We pass a dummy output writer because we collect results via a callback.
	httpxOptions := optionsConfigurator.GetOptions()
	httpxRunner, err := runner.New(httpxOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create httpx runner: %w", err)
	}

	// Build the runner
	runner := &Runner{
		logger:              b.logger,
		config:              b.config,
		rootTargetURL:       b.rootTargetURL,
		optionsConfigurator: optionsConfigurator,
		mapper:              resultMapper,
		collector:           resultCollector,
		httpxRunner:         httpxRunner,
	}

	return runner, nil
}
