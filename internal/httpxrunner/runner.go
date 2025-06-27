package httpxrunner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/projectdiscovery/httpx/runner"
	"github.com/rs/zerolog"
)

// Runner wraps the httpx library runner
type Runner struct {
	config              *Config
	httpxRunner         *runner.Runner
	logger              zerolog.Logger
	rootTargetURL       string
	wg                  sync.WaitGroup
	optionsConfigurator *OptionsConfigurator
	mapper              *ProbeResultMapper
	collector           *ResultCollector
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
		return errors.New("httpx engine not initialized")
	}

	if r.collector == nil {
		return errors.New("result collector not initialized")
	}

	return nil
}

// executeRunner executes the httpx runner in a goroutine with improved cancellation handling
func (r *Runner) executeRunner(ctx context.Context) {
	defer r.wg.Done()

	// Run enumeration in a separate goroutine to allow cancellation
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.httpxRunner.RunEnumeration()
	}()

	// Wait for either completion or cancellation with frequent checks
	ticker := time.NewTicker(500 * time.Millisecond) // Check cancellation every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-done:
			r.logger.Debug().Msg("HTTPX enumeration completed")
			return
		case <-ctx.Done():
			r.logger.Info().Err(ctx.Err()).Msg("Context cancelled, stopping HTTPX enumeration")
			// Note: httpx doesn't support graceful shutdown, so we just log the cancellation
			return
		case <-ticker.C:
			// Check if context is cancelled during long-running operations
			if err := IsContextCancelled(ctx); err != nil {
				r.logger.Info().Err(err).Msg("Context cancelled during HTTPX execution")
				return
			}
		}
	}
}

// waitForCompletion waits for runner completion or context cancellation with immediate response
func (r *Runner) waitForCompletion(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Debug().Msg("HTTPX runner completed successfully")
		return nil
	case <-ctx.Done():
		err := ctx.Err()
		r.logger.Info().Err(err).Msg("HTTPX runner cancelled immediately")
		// Give a shorter grace period for immediate response
		select {
		case <-done:
			r.logger.Debug().Msg("HTTPX runner completed during short grace period")
		case <-time.After(1 * time.Second): // Reduced from 5 seconds to 1 second
			r.logger.Warn().Msg("HTTPX runner did not complete within short grace period, forcing termination")
		}
		return fmt.Errorf("http runner execution cancelled: %w", err)
	}
}

// Run executes the HTTPX runner with context support
func (r *Runner) Run(ctx context.Context) error {
	// Validate runner state
	if err := r.validateRunState(); err != nil {
		return fmt.Errorf("failed to validate runner state: %w", err)
	}

	r.logger.Info().
		Str("root_target", r.rootTargetURL).
		Int("target_count", len(r.config.Targets)).
		Msg("Starting HTTPX runner execution")

	// Execute runner
	r.wg.Add(1)
	go r.executeRunner(ctx)

	// Wait for completion
	return r.waitForCompletion(ctx)
}

// GetResults returns all collected probe results after the run is complete
func (r *Runner) GetResults() []ProbeResult {
	return r.collector.GetResults()
}
