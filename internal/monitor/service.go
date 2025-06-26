package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/monsterinc/httpclient"
	"github.com/monsterinc/limiter"
	"github.com/monsterinc/logger"
	"github.com/rs/zerolog"

	"github.com/aleister1102/monsterinc/internal/notifier"
)

// Service defines the monitoring service
type Service struct {
	config          *config.MonitorConfig
	logger          zerolog.Logger
	resourceLimiter *limiter.ResourceLimiter
	httpClient      *httpclient.HTTPClient
	urlManager      *URLManager
	cycleTracker    *CycleTracker
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	isRunning       bool
	mutex           sync.RWMutex
}

// NewService creates a new monitoring service
func NewService(
	cfg *config.MonitorConfig,
	appLogger zerolog.Logger,
	resourceLimiter *limiter.ResourceLimiter,
	httpClient *httpclient.HTTPClient,
) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		config:          cfg,
		logger:          appLogger.With().Str("service", "Monitor").Logger(),
		resourceLimiter: resourceLimiter,
		httpClient:      httpClient,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins the monitoring service
func (s *Service) Start(initialTargets []string) error {
	s.mutex.Lock()
	if s.isRunning {
		s.mutex.Unlock()
		return fmt.Errorf("monitoring service is already running")
	}

	s.logger.Info().Msg("Starting monitoring service...")

	// Initialize components
	s.initializeComponents(initialTargets)

	s.isRunning = true
	s.mutex.Unlock()

	// Start the main monitoring loop in a goroutine
	s.wg.Add(1)
	go s.monitoringLoop()

	s.logger.Info().Msg("Monitoring service started successfully.")
	return nil
}

// Stop gracefully stops the monitoring service
func (s *Service) Stop() {
	s.mutex.Lock()
	if !s.isRunning {
		s.mutex.Unlock()
		return
	}
	s.isRunning = false
	s.mutex.Unlock()

	s.logger.Info().Msg("Stopping monitoring service...")
	s.cancel() // Signal all goroutines to stop
	s.wg.Wait() // Wait for all goroutines to finish
	s.logger.Info().Msg("Monitoring service stopped.")
}

// initializeComponents sets up the necessary components for the service
func (s *Service) initializeComponents(initialTargets []string) {
	s.cycleTracker = NewCycleTracker(s.config.MaxCycles)
	s.urlManager = NewURLManager(s.logger, initialTargets)
	// Other initializations can go here
}

// monitoringLoop is the main loop for the monitoring service
func (s *Service) monitoringLoop() {
	defer s.wg.Done()

	// Initial cycle
	s.executeCycle()

	// Subsequent cycles based on ticker
	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.shouldStartNewCycle() {
				s.executeCycle()
			}
		case <-s.ctx.Done():
			s.logger.Info().Msg("Monitoring loop terminated.")
			return
		}
	}
}

// shouldStartNewCycle checks if a new monitoring cycle should be initiated
func (s *Service) shouldStartNewCycle() bool {
	if !s.cycleTracker.ShouldContinue() {
		s.logger.Info().Int("max_cycles", s.config.MaxCycles).Msg("Reached max monitoring cycles, stopping.")
		go s.Stop() // Stop the service in a new goroutine to avoid deadlock
		return false
	}
	return true
}

// executeCycle runs a single monitoring cycle
func (s *Service) executeCycle() {
	s.cycleTracker.StartCycle()
	cycleID := s.cycleTracker.GetCurrentCycleID()
	s.logger.Info().Str("cycle_id", cycleID).Msg("Starting new monitoring cycle")

	urlsToCheck := s.urlManager.GetURLsForCycle()
	if len(urlsToCheck) == 0 {
		s.logger.Info().Msg("No URLs to check in this cycle.")
		s.cycleTracker.EndCycle()
		return
	}

	// Create a content processor for this cycle
	processor := NewContentProcessor(s.config, s.logger, s.httpClient, s.resourceLimiter)

	// Process URLs in batches
	batchManager := NewBatchURLManager(urlsToCheck, s.config.BatchSize)
	for batchManager.HasNext() {
		batch := batchManager.NextBatch()
		processor.ProcessBatch(s.ctx, batch, cycleID)
	}

	// Cycle cleanup and reporting
	s.urlManager.UpdateWithCycleResults(processor.GetDiscoveredAssets())
	s.cycleTracker.EndCycle()
	s.logger.Info().Str("cycle_id", cycleID).Msg("Monitoring cycle finished.")
}
