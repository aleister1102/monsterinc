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

// initializeMonitorWorkers initializes and starts monitor workers
func (s *Scheduler) initializeMonitorWorkers() {
	if s.globalConfig.MonitorConfig.CheckIntervalSeconds <= 0 {
		s.logger.Warn().
			Int("configured_interval", s.globalConfig.MonitorConfig.CheckIntervalSeconds).
			Msg("Monitor CheckIntervalSeconds is not configured or invalid, monitor scheduling disabled.")
		return
	}

	// Initialize monitor worker channel
	numWorkers := s.globalConfig.MonitorConfig.MaxConcurrentChecks
	if numWorkers <= 0 {
		numWorkers = 1 // Default to 1 worker if not configured or invalid
		s.logger.Warn().
			Int("configured_workers", s.globalConfig.MonitorConfig.MaxConcurrentChecks).
			Int("default_workers", numWorkers).
			Msg("MaxConcurrentChecks is not configured or invalid, defaulting to 1 worker.")
	}

	// Start monitor workers
	s.monitorWorkerChan = make(chan MonitorJob, numWorkers)
	s.logger.Info().Int("num_workers", numWorkers).Msg("Starting monitor workers")
	for i := range numWorkers {
		s.monitorWorkerWG.Add(1) // Used for waiting for all workers to finish
		go s.monitorWorker(i)    // Start each worker in a goroutine for running concurrently
	}

	// Start monitor ticker for periodic monitoring
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
	// Check if monitoring service is available
	if s.monitoringService == nil {
		s.logger.Warn().Str("cycle_type", cycleType).Msg("Scheduler: Monitoring service not available for cycle. Skipping.")
		return
	}

	// Check if scheduler is stopping before starting the cycle
	select {
	case <-s.stopChan:
		s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: Stop signal received before starting monitor cycle. Skipping.")
		return
	default:
		// Continue if stopChan is not closed
	}

	targetsToCheck := s.monitoringService.GetCurrentlyMonitorUrls()
	if len(targetsToCheck) == 0 {
		s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: No targets to check in this monitor cycle. Skipping cycle end report.")
		// Do not trigger cycle end report if no targets were checked.
		return
	}

	s.logger.Info().Str("cycle_type", cycleType).Int("targets", len(targetsToCheck)).Msg("Scheduler: Starting monitor cycle.")

	var cycleWG sync.WaitGroup
	jobsDispatched, stoppedPrematurely := s.dispatchMonitorJobs(cycleType, targetsToCheck, &cycleWG)

	if stoppedPrematurely {
		s.logger.Info().
			Str("cycle_type", cycleType).
			Int("jobs_dispatched_before_stop", jobsDispatched).
			Int("targets_intended", len(targetsToCheck)).
			Msg("Scheduler: Monitor job dispatch was stopped prematurely by a signal.")
	}

	if jobsDispatched == 0 && len(targetsToCheck) > 0 {
		s.logger.Info().
			Str("cycle_type", cycleType).
			Int("targets_intended", len(targetsToCheck)).
			Bool("dispatch_stopped_prematurely", stoppedPrematurely).
			Msg("Scheduler: No monitor jobs were dispatched in this cycle.")
	}

	if jobsDispatched > 0 {
		s.logger.Info().Str("cycle_type", cycleType).Int("jobs_dispatched", jobsDispatched).Msg("Scheduler: Waiting for dispatched monitor jobs to complete.")
		cycleWG.Wait() // Wait for all successfully dispatched jobs to complete
		s.logger.Info().Str("cycle_type", cycleType).Int("jobs_completed", jobsDispatched).Msg("Scheduler: All dispatched monitor jobs completed.")
	} else {
		s.logger.Info().Str("cycle_type", cycleType).Msg("Scheduler: No monitor jobs were active in this cycle to wait for.")
	}

	// After waiting (if any jobs were dispatched), check stopChan again before triggering report
	select {
	case <-s.stopChan:
		s.logger.Info().
			Str("cycle_type", cycleType).
			Int("jobs_processed_before_stop", jobsDispatched).
			Msg("Stop signal active after monitor cycle processing. Report not triggered.")
	default:
		// Trigger report only if jobs were dispatched and completed.
		if jobsDispatched > 0 {
			s.logger.Info().
				Str("cycle_type", cycleType).
				Int("targets_processed", jobsDispatched).
				Msg("All checks for the monitor cycle completed. Triggering report.")
			s.monitoringService.TriggerCycleEndReport()
		} else {
			s.logger.Info().
				Str("cycle_type", cycleType).
				Int("targets_intended", len(targetsToCheck)).
				Int("jobs_dispatched_and_completed", jobsDispatched). // Will be 0 if this branch is hit
				Bool("dispatch_stopped_prematurely", stoppedPrematurely).
				Msg("No monitor jobs were processed to completion in this cycle. Report not triggered.")
		}
	}
}

// dispatchMonitorJobs attempts to dispatch all monitor jobs for the given targets.
// It returns the number of jobs successfully dispatched and a boolean indicating if the dispatching
// was stopped prematurely due to a signal on s.stopChan.
func (s *Scheduler) dispatchMonitorJobs(
	cycleType string,
	targetsToCheck []string,
	cycleWG *sync.WaitGroup,
) (jobsDispatched int, stoppedPrematurely bool) {
	s.logger.Debug().Str("cycle_type", cycleType).Int("targets_to_dispatch", len(targetsToCheck)).Msg("Attempting to dispatch monitor jobs.")

	for _, url := range targetsToCheck {
		// Check for stop signal before attempting to create and dispatch each job
		select {
		case <-s.stopChan:
			s.logger.Info().Str("cycle_type", cycleType).Msg("Stop signal received before dispatching next job. Halting further job dispatch for this cycle.")
			return jobsDispatched, true // Stopped prematurely
		default:
			// Continue to dispatch job
		}

		job := MonitorJob{URL: url, CycleWG: cycleWG}
		// Increment WaitGroup *before* attempting to send, to ensure Done() is called if send fails due to stop.
		cycleWG.Add(1)

		// Attempt to send job to monitor worker channel
		select {
		case s.monitorWorkerChan <- job:
			jobsDispatched++
			s.logger.Debug().Str("url", url).Str("cycle_type", cycleType).Msg("Successfully dispatched monitor job.")
		case <-s.stopChan:
			s.logger.Info().
				Str("url", url).
				Str("cycle_type", cycleType).
				Msg("Stop signal received while attempting to dispatch job. Job not dispatched.")
			cycleWG.Done()              // Decrement WG as this job won't be processed by a worker
			return jobsDispatched, true // Stopped prematurely
		}
	}
	s.logger.Debug().Str("cycle_type", cycleType).Int("jobs_dispatched", jobsDispatched).Msg("Finished dispatching monitor jobs for this cycle.")
	return jobsDispatched, false // All jobs dispatched (or targets list exhausted) without premature stop
}
