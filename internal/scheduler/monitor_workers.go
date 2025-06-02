package scheduler

import (
	"sync"
	"time"
)

// MonitorJob wraps a URL and a WaitGroup for a specific monitoring cycle.
type MonitorJob struct {
	URL     string
	CycleWG *sync.WaitGroup
}

// initializeAndStartMonitorWorkersIfNeeded initializes and starts monitor workers if the monitoring service is available.
func (s *Scheduler) initializeAndStartMonitorWorkersIfNeeded() {
	if s.monitoringService != nil {
		s.logger.Info().Msg("Scheduler: Monitoring service is available, starting monitor workers.")
		s.initializeAndStartMonitorWorkers() // This is the existing function that starts workers and ticker
	} else {
		s.logger.Warn().Msg("Scheduler: Monitoring service is not available, monitor workers will not be started.")
	}
}

// initializeAndStartMonitorWorkers initializes and starts monitor workers
func (s *Scheduler) initializeAndStartMonitorWorkers() {
	s.logger.Info().Msg("Scheduler: initializeAndStartMonitorWorkers called.")

	if s.globalConfig.MonitorConfig.CheckIntervalSeconds <= 0 {
		s.logger.Warn().Int("configured_interval", s.globalConfig.MonitorConfig.CheckIntervalSeconds).Msg("Monitor CheckIntervalSeconds is not configured or invalid, monitor scheduling disabled.")
		return
	}

	// Initialize monitor worker channel
	numWorkers := s.globalConfig.MonitorConfig.MaxConcurrentChecks
	if numWorkers <= 0 {
		numWorkers = 1 // Default to 1 worker if not configured or invalid
		s.logger.Warn().Int("configured_workers", s.globalConfig.MonitorConfig.MaxConcurrentChecks).Int("default_workers", numWorkers).Msg("MaxConcurrentChecks is not configured or invalid, defaulting to 1 worker.")
	}
	s.monitorWorkerChan = make(chan MonitorJob, numWorkers)

	// Start monitor workers
	s.logger.Info().Int("num_workers", numWorkers).Msg("Starting monitor workers")
	for i := range numWorkers {
		s.monitorWorkerWG.Add(1)
		go s.monitorWorker(i)
	}

	// Start monitor ticker
	intervalDuration := time.Duration(s.globalConfig.MonitorConfig.CheckIntervalSeconds) * time.Second
	s.logger.Info().Dur("interval", intervalDuration).Msg("Starting monitor ticker.")
	s.monitorTicker = time.NewTicker(intervalDuration)
	s.monitorWorkerWG.Add(1) // Add to WG for the ticker goroutine as well

	go func() {
		defer s.monitorWorkerWG.Done()
		s.logger.Info().Msg("Monitor ticker goroutine started.")
		for {
			select {
			case <-s.monitorTicker.C:
				s.logger.Info().Msg("Monitor ticker event received.")
				s.executeMonitoringCycle("periodic")
			case <-s.stopChan: // Listen for the scheduler's main stop signal
				s.logger.Info().Msg("Monitor ticker stopping due to scheduler's stop signal.")
				s.monitorTicker.Stop() // Ensure ticker is stopped
				return
			}
		}
	}()

	s.logger.Info().Msg("Monitor workers and ticker started successfully.")
}

// monitorWorker processes monitor jobs
func (s *Scheduler) monitorWorker(id int) {
	defer s.monitorWorkerWG.Done()
	s.logger.Info().Int("worker_id", id).Msg("Monitor worker started")

	for {
		select {
		// Receive job from monitor worker channel
		case job, ok := <-s.monitorWorkerChan:
			if !ok {
				s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping as channel closed.")
				return
			}
			s.logger.Info().Int("worker_id", id).Str("url", job.URL).Msg("Monitor worker processing job.")
			// Ensure monitoringService is not nil before calling CheckURL
			if s.monitoringService != nil {
				s.monitoringService.CheckURL(job.URL) // This should be a blocking call per URL
			} else {
				s.logger.Warn().Int("worker_id", id).Str("url", job.URL).Msg("Monitoring service is nil, cannot check URL.")
			}
			job.CycleWG.Done()
			s.logger.Info().Int("worker_id", id).Str("url", job.URL).Msg("Monitor worker finished job.")
		case <-s.stopChan: // Listen for the scheduler's main stop signal
			s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to scheduler's stop signal.")
			return
		}
	}
}

// executeMonitoringCycle performs a monitoring cycle for all monitored URLs
func (s *Scheduler) executeMonitoringCycle(cycleType string) {
	s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: Executing monitor cycle function.")
	// Check if monitoring service is available and stopChan is not closed
	if s.monitoringService == nil {
		s.logger.Warn().Str("cycle_type", cycleType).Msg("Scheduler: Monitoring service not available for cycle.")
		return
	}
	select {
	case <-s.stopChan:
		s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: Stop signal received before starting monitor cycle.")
		return
	default:
		// Continue if stopChan is not closed
	}

	targetsToCheck := s.monitoringService.GetCurrentlyMonitoredURLs()
	s.logger.Info().Str("cycle_type", cycleType).Int("count", len(targetsToCheck)).Msg("Scheduler: Performing monitor cycle")

	if len(targetsToCheck) == 0 {
		s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: No targets to check in this monitor cycle. Skipping cycle end report.")
		// Do not trigger cycle end report if no targets were checked.
		return
	}

	s.logger.Info().Str("cycle_type", cycleType).Int("targets", len(targetsToCheck)).Msg("Scheduler: Starting monitor cycle with targets.")

	var cycleWG sync.WaitGroup
	jobsDispatched := 0

	for _, url := range targetsToCheck {
		// Check for stop signal before dispatching each job
		select {
		case <-s.stopChan:
			s.logger.Info().Str("url", url).Str("cycle_type", cycleType).Msg("Stop signal received during job submission for monitor cycle. No more jobs will be dispatched for this cycle.")
			// If stop is signaled, we might not want to submit more jobs.
			// We need to ensure cycleWG correctly reflects the number of jobs that *were* added.
			goto endJobLoop // Exit the loop if stop signal is received
		default:
			// Continue to dispatch job
		}

		job := MonitorJob{URL: url, CycleWG: &cycleWG}
		cycleWG.Add(1) // Add to WG *before* sending to channel or checking stop signal for this job.

		// Send job to monitor worker channel
		select {
		case s.monitorWorkerChan <- job:
			jobsDispatched++
		case <-s.stopChan: // Check again, in case it was closed while trying to send
			s.logger.Info().Str("url", url).Str("cycle_type", cycleType).Msg("Stop signal received while attempting to dispatch job. Job not dispatched.")
			cycleWG.Done()  // Decrement WG as this job won't be processed
			goto endJobLoop // Exit the loop
		}
	}

endJobLoop:
	if jobsDispatched == 0 && len(targetsToCheck) > 0 {
		s.logger.Info().Str("cycle_type", cycleType).Int("targets_intended", len(targetsToCheck)).Msg("Scheduler: No monitor jobs were dispatched in this cycle (possibly due to early stop signal).")
		// If no jobs were dispatched but there were targets, it's likely due to an early stop.
		// In this case, TriggerCycleEndReport should probably not be called.
		// The cycleWG.Wait() will be a no-op if jobsDispatched is 0.
		// However, if some jobs were dispatched before stop, we wait for them.
	}

	if jobsDispatched > 0 {
		s.logger.Info().Str("cycle_type", cycleType).Int("jobs_dispatched", jobsDispatched).Msg("Scheduler: Waiting for all dispatched monitor jobs to complete.")
		cycleWG.Wait() // Wait for all checkURL calls in this cycle to complete
		s.logger.Info().Str("cycle_type", cycleType).Int("jobs_completed", jobsDispatched).Msg("Scheduler: All dispatched monitor jobs completed.")
	}

	// After waiting, check stopChan again before triggering report
	select {
	case <-s.stopChan:
		s.logger.Info().Str("cycle_type", cycleType).Msg("Stop signal received after monitor cycle processing, report not triggered.")
	default:
		if jobsDispatched > 0 { // Only trigger report if jobs were actually processed
			s.logger.Info().Str("cycle_type", cycleType).Int("targets_processed", jobsDispatched).Msg("All checks for the monitor cycle completed. Triggering report.")
			s.monitoringService.TriggerCycleEndReport()
		} else if len(targetsToCheck) > 0 { // Targets existed, but no jobs dispatched (likely stopped)
			s.logger.Info().Str("cycle_type", cycleType).Msg("Monitor cycle had targets but no jobs were dispatched (likely stopped early), report not triggered.")
		} else { // No targets to begin with
			s.logger.Info().Str("cycle_type", cycleType).Msg("Monitor cycle had no targets and no jobs dispatched, report not triggered.")
		}
	}
}
