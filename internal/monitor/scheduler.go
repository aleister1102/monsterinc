package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"

	"github.com/rs/zerolog"
)

// monitorJob wraps a URL and a WaitGroup for a specific monitoring cycle.
type monitorJob struct {
	URL     string
	CycleWG *sync.WaitGroup
}

// Scheduler manages the periodic checking of URLs for the MonitoringService.
type Scheduler struct {
	logger  zerolog.Logger
	cfg     *config.MonitorConfig
	service *MonitoringService // Reference to the monitoring service to call its methods

	ctx        context.Context
	cancelFunc context.CancelFunc
	workerChan chan monitorJob // Changed to chan monitorJob
	wg         sync.WaitGroup
	active     bool
	mu         sync.Mutex // To protect access to 'active' field
}

// NewScheduler creates a new monitor scheduler.
func NewScheduler(cfg *config.MonitorConfig, logger zerolog.Logger, service *MonitoringService) *Scheduler {
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
	s.workerChan = make(chan monitorJob, numWorkers) // Buffer size can be numWorkers or a bit more

	s.logger.Info().Int("num_workers", numWorkers).Msg("Starting monitor workers")
	for i := 0; i < numWorkers; i++ {
		s.wg.Add(1)
		go s.worker(i)
		// s.logger.Info().Int("worker_id", i).Msg("Monitoring worker started") // Logged inside worker now
	}
	// s.logger.Info().Int("num_workers", numWorkers).Msg("MonitorScheduler started successfully with workers.") // Logged after initial checks

	initialURLs := s.service.GetCurrentlyMonitoredURLs()
	s.logger.Info().Int("count", len(initialURLs)).Msg("Performing initial check for monitored URLs.")
	if len(initialURLs) > 0 {
		var initialCycleWG sync.WaitGroup
		initialCycleWG.Add(len(initialURLs))
		for _, url := range initialURLs {
			job := monitorJob{URL: url, CycleWG: &initialCycleWG}
			select {
			case s.workerChan <- job:
			case <-s.ctx.Done():
				s.logger.Info().Str("url", url).Msg("Context cancelled during initial URL job submission.")
				initialCycleWG.Done() // Ensure WG is decremented if job not sent
			}
		}
		initialCycleWG.Wait() // Wait for all initial checks to complete
		s.logger.Info().Msg("Initial checks for monitored URLs completed.")
		s.service.TriggerCycleEndReport() // Trigger report after initial checks
	}

	s.logger.Info().Int("num_workers", numWorkers).Msg("MonitorScheduler started successfully with workers and initial checks complete.")

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
			s.wg.Wait()         // Wait for all worker goroutines to finish

			// Final report on shutdown if context was not cancelled before this point.
			// If context was cancelled, TriggerCycleEndReport might have already run due to earlier logic
			// or it might be redundant/report on incomplete data.
			// Consider the state of changedURLsInCycle if service was abruptly stopped.
			if s.ctx.Err() == nil { // Only trigger if not already shutting down due to cancellation that might have pre-empted the last tick's report
				s.logger.Info().Msg("Scheduler shutting down. Triggering final cycle end report.")
				s.service.TriggerCycleEndReport()
			}

			s.mu.Lock()
			s.active = false
			s.mu.Unlock()
			s.logger.Info().Msg("MonitorScheduler main loop and workers stopped.")
		}()

		for {
			select {
			case <-s.ctx.Done():
				s.logger.Info().Msg("MonitorScheduler context cancelled, main loop stopping.")
				return
			case <-ticker.C:
				s.logger.Debug().Msg("Monitor tick. Distributing checks to workers...")
				targetsToCheck := s.service.GetCurrentlyMonitoredURLs()
				s.logger.Debug().Int("count", len(targetsToCheck)).Msgf("Number of targets to check this cycle: %d", len(targetsToCheck))

				if len(targetsToCheck) > 0 {
					var currentCycleWG sync.WaitGroup
					currentCycleWG.Add(len(targetsToCheck))

					for _, url := range targetsToCheck {
						job := monitorJob{URL: url, CycleWG: &currentCycleWG}
						select {
						case s.workerChan <- job:
						case <-s.ctx.Done():
							s.logger.Info().Str("url", url).Msg("Context cancelled during job submission for current cycle.")
							currentCycleWG.Done() // Decrement if job won't be processed by a worker
						}
					}
					currentCycleWG.Wait() // Wait for all checkURL calls in this cycle to complete

					if s.ctx.Err() == nil { // Check context again before triggering report
						s.logger.Info().Int("targets_processed", len(targetsToCheck)).Msg("All checks for the current monitor cycle completed. Triggering report.")
						s.service.TriggerCycleEndReport()
					} else {
						s.logger.Info().Msg("Context cancelled during monitor cycle processing, report not triggered by this tick.")
					}
				} else {
					s.logger.Debug().Msg("No targets to check in this monitor cycle. Triggering report to clear state.")
					// Still trigger report to clear changedURLsInCycle and handle potential state issues
					// if URLs were removed, etc.
					s.service.TriggerCycleEndReport()
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
	for job := range s.workerChan { // Now receives monitorJob
		select {
		case <-s.ctx.Done():
			s.logger.Info().Int("worker_id", id).Str("url", job.URL).Msg("Context cancelled, worker stopping before processing URL.")
			if job.CycleWG != nil { // Ensure WG is decremented if worker is stopping mid-assignment
				job.CycleWG.Done()
			}
			return
		default:
			// Proceed to check URL
		}
		s.logger.Debug().Int("worker_id", id).Str("url", job.URL).Msg("Worker processing URL")
		s.service.checkURL(job.URL) // Call checkURL on the service instance
		if job.CycleWG != nil {
			job.CycleWG.Done() // Signal completion for this job in its cycle
		}
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
