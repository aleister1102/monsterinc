package scanner

import (
	"context"
	"fmt"
	"sync"

	"github.com/monsterinc/httpx"
	"github.com/rs/zerolog"
)

// HTTPXManager manages a singleton httpx runner instance for reuse across batches
type HTTPXManager struct {
	logger         zerolog.Logger
	runnerInstance *httpx.Runner
	mutex          sync.RWMutex
	initialized    bool
	lastConfig     *httpx.Config
	lastRootTarget string
}

// NewHTTPXManager creates a new httpx manager
func NewHTTPXManager(logger zerolog.Logger) *HTTPXManager {
	return &HTTPXManager{
		logger: logger.With().Str("component", "HTTPXManager").Logger(),
	}
}

// GetOrCreateRunner returns the existing runner instance or creates a new one if config changed
func (hm *HTTPXManager) GetOrCreateRunner(config *httpx.Config, rootTargetURL string) (*httpx.Runner, error) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	// Check if we need to recreate the runner (config or root target changed)
	if hm.needsRecreation(config, rootTargetURL) {
		if err := hm.createNewRunner(config, rootTargetURL); err != nil {
			return nil, err
		}
	}

	return hm.runnerInstance, nil
}

// needsRecreation checks if the runner needs to be recreated
func (hm *HTTPXManager) needsRecreation(config *httpx.Config, rootTargetURL string) bool {
	if !hm.initialized || hm.runnerInstance == nil {
		return true
	}

	// Check if config changed (key configuration fields)
	if hm.lastConfig == nil ||
		hm.lastConfig.Threads != config.Threads ||
		hm.lastConfig.Timeout != config.Timeout ||
		hm.lastConfig.RateLimit != config.RateLimit ||
		hm.lastConfig.Retries != config.Retries ||
		hm.lastConfig.FollowRedirects != config.FollowRedirects ||
		hm.lastRootTarget != rootTargetURL {
		return true
	}

	return false
}

// createNewRunner creates a new httpx runner instance
func (hm *HTTPXManager) createNewRunner(config *httpx.Config, rootTargetURL string) error {
	hm.logger.Debug().
		Str("root_target", rootTargetURL).
		Int("threads", config.Threads).
		Int("timeout", config.Timeout).
		Msg("Creating new HTTPXRunner instance")

	runner, err := httpx.NewRunner(config, rootTargetURL, hm.logger)
	if err != nil {
		return fmt.Errorf("failed to create httpx runner: %w", err)
	}

	hm.runnerInstance = runner
	hm.lastConfig = config
	hm.lastRootTarget = rootTargetURL
	hm.initialized = true

	hm.logger.Info().
		Str("root_target", rootTargetURL).
		Int("threads", config.Threads).
		Int("timeout", config.Timeout).
		Msg("HTTPXRunner instance created successfully")

	return nil
}

// ExecuteRunnerBatch executes httpx runner for a specific batch
func (hm *HTTPXManager) ExecuteRunnerBatch(ctx context.Context, config *httpx.Config, rootTargetURL, scanSessionID string) ([]httpx.ProbeResult, error) {
	runner, err := hm.GetOrCreateRunner(config, rootTargetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get httpx runner: %w", err)
	}

	hm.logger.Debug().
		Str("session_id", scanSessionID).
		Str("root_target", rootTargetURL).
		Msg("Executing HTTPXRunner batch")

	if err := runner.Run(ctx); err != nil {
		hm.logger.Warn().
			Err(err).
			Str("session_id", scanSessionID).
			Msg("HTTPXRunner execution encountered errors")

		// Continue with partial results unless cancelled
		if ctx.Err() == context.Canceled {
			hm.logger.Info().
				Str("session_id", scanSessionID).
				Msg("HTTPXRunner cancelled")
			return runner.GetResults(), ctx.Err()
		}

		return runner.GetResults(), fmt.Errorf("httpx execution failed for session %s: %w", scanSessionID, err)
	}

	results := runner.GetResults()
	hm.logger.Debug().
		Int("result_count", len(results)).
		Str("session_id", scanSessionID).
		Msg("HTTPXRunner batch completed")

	return results, nil
}

// Shutdown gracefully shuts down the managed httpx runner
func (hm *HTTPXManager) Shutdown() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if hm.runnerInstance != nil {
		hm.logger.Info().Msg("Shutting down managed HTTPXRunner instance")
		// HTTPXRunner doesn't have explicit shutdown method, just clean up reference
		hm.runnerInstance = nil
		hm.initialized = false
		hm.lastConfig = nil
		hm.lastRootTarget = ""
		hm.logger.Info().Msg("HTTPXRunner shutdown complete")
	}
}
