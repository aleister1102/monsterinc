package monitor

import (
	"context"
	"github.com/aleister1102/monsterinc/internal/config"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Scheduler manages the periodic checking of URLs for the MonitoringService.
type Scheduler struct {
	logger  zerolog.Logger
	cfg     *config.MonitorConfig
	service *MonitoringService // Reference to the monitoring service to call its methods

	ctx        context.Context
	cancelFunc context.CancelFunc
	workerChan chan string
	wg         sync.WaitGroup
	active     bool
	mu         sync.Mutex // To protect access to 'active' field
}

// NewScheduler creates a new monitor scheduler.
func NewScheduler(logger zerolog.Logger, cfg *config.MonitorConfig, service *MonitoringService) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	schedLogger := logger.With().Str("component", "MonitorScheduler").Logger()

	return &Scheduler{
		logger:     schedLogger,
		cfg:        cfg,
		service:    service,
		ctx:        ctx,
		cancelFunc: cancel,
		// workerChan will be created in Start()
	}
}

// Start begins the monitoring scheduler loop and worker pool.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		s.logger.Warn().Msg("MonitorScheduler already active.")
		return nil
	}
	s.active = true
	s.mu.Unlock()

	s.logger.Info().Msg("Starting MonitorScheduler...")

	// Initialize workerChan before starting workers and the main loop
	numWorkers := s.cfg.MaxConcurrentChecks
	if numWorkers <= 0 {
		numWorkers = 1
		s.logger.Warn().Int("configured_workers", s.cfg.MaxConcurrentChecks).Msg("MaxConcurrentChecks is not configured or invalid, defaulting to 1 worker.")
	}
	s.workerChan = make(chan string, numWorkers) // Buffer size can be numWorkers or a bit more

	s.logger.Info().Int("num_workers", numWorkers).Msg("Starting monitor workers")
	for i := 0; i < numWorkers; i++ {
		s.wg.Add(1)
		go s.worker(i)
		s.logger.Info().Int("worker_id", i).Msg("Monitoring worker started")
	}
	s.logger.Info().Int("num_workers", numWorkers).Msg("MonitorScheduler started successfully with workers.")

	// Initial population of workerChan with all currently monitored URLs
	// This ensures an immediate first check for all targets.
	initialURLs := s.service.GetCurrentlyMonitoredURLs()
	s.logger.Info().Int("count", len(initialURLs)).Msg("Performing initial check for monitored URLs.")
	for _, url := range initialURLs {
		select {
		case s.workerChan <- url:
		case <-s.ctx.Done():
			s.logger.Info().Msg("Context cancelled during initial URL population for workers.")
			return nil // Stop if context is cancelled
		}
	}

	var ticker *time.Ticker
	if s.cfg.CheckIntervalSeconds <= 0 {
		s.logger.Warn().Int("configured_interval", s.cfg.CheckIntervalSeconds).Msg("CheckIntervalSeconds is not configured or invalid, defaulting to 1 hour.")
		ticker = time.NewTicker(3600 * time.Second)
	} else {
		ticker = time.NewTicker(time.Duration(s.cfg.CheckIntervalSeconds) * time.Second)
	}

	go func() {
		defer func() {
			ticker.Stop()
			close(s.workerChan) // Signal workers to stop
			s.wg.Wait()         // Wait for all workers to finish
			s.mu.Lock()
			s.active = false
			s.mu.Unlock()
			s.logger.Info().Msg("MonitorScheduler main loop and workers stopped.")
		}()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.logger.Debug().Msg("Monitor tick. Distributing checks to workers...")
				targetsToCheck := s.service.GetCurrentlyMonitoredURLs() // Get URLs from service
				s.logger.Debug().Int("count", len(targetsToCheck)).Msgf("Number of targets to check this cycle: %d", len(targetsToCheck))
				for _, url := range targetsToCheck {
					select {
					case s.workerChan <- url:
					case <-s.ctx.Done():
						s.logger.Info().Msg("Context cancelled during job distribution to workers.")
						return
					}
				}
			}
		}
	}()

	return nil
}

// worker is a goroutine that listens on workerChan for URLs to check.
func (s *Scheduler) worker(id int) {
	defer s.wg.Done()
	s.logger.Info().Int("worker_id", id).Msg("Monitoring worker started")
	for url := range s.workerChan {
		select {
		case <-s.ctx.Done():
			s.logger.Info().Int("worker_id", id).Msg("Context cancelled, worker stopping.")
			return
		default:
			// Proceed to check URL
		}
		s.logger.Debug().Int("worker_id", id).Str("url", url).Msg("Worker processing URL")
		s.service.checkURL(url) // Call checkURL on the service instance
	}
	s.logger.Info().Int("worker_id", id).Msg("Monitoring worker stopped as channel closed.")
}

// Stop signals the MonitorScheduler to shut down gracefully.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		s.logger.Info().Msg("MonitorScheduler was not active.")
		return
	}
	s.mu.Unlock() // Unlock early as cancelFunc and active checks are separate

	s.logger.Info().Msg("Attempting to stop MonitorScheduler...")

	if s.cancelFunc != nil {
		s.cancelFunc() // Signal the context to cancel
	}

	// Wait for the scheduler to become inactive (main loop and workers to finish)
	// The main loop sets active = false after wg.Wait()
	shutdownTimeout := 10 * time.Second // Increased timeout for workers to finish
	checkInterval := 200 * time.Millisecond

	start := time.Now()
	for {
		s.mu.Lock()
		isActive := s.active
		s.mu.Unlock()

		if !isActive {
			s.logger.Info().Msg("MonitorScheduler stopped successfully.")
			return
		}

		if time.Since(start) > shutdownTimeout {
			s.logger.Warn().Msg("MonitorScheduler did not stop gracefully within the timeout.")
			return
		}
		time.Sleep(checkInterval)
	}
}
