package scheduler

import (
	"context"
	"sync"
	"time"
)

// MonitorJob wraps a URL and a WaitGroup for a specific monitoring cycle.
type MonitorJob struct {
	URL     string
	CycleWG *sync.WaitGroup
}

type monitorConfig struct {
	intervalSeconds     int
	maxConcurrentChecks int
}

// initializeMonitorWorkers initializes and starts monitor workers
func (s *Scheduler) initializeMonitorWorkers(ctx context.Context) {
	config := s.getMonitorConfig()
	if !s.isValidMonitorConfig(config) {
		return
	}

	s.startMonitorWorkers(ctx, config.maxConcurrentChecks)
	s.startMonitorTicker(ctx, config.intervalSeconds)
}

func (s *Scheduler) getMonitorConfig() monitorConfig {
	return monitorConfig{
		intervalSeconds:     s.globalConfig.MonitorConfig.CheckIntervalSeconds,
		maxConcurrentChecks: s.globalConfig.MonitorConfig.MaxConcurrentChecks,
	}
}

func (s *Scheduler) isValidMonitorConfig(config monitorConfig) bool {
	if config.intervalSeconds <= 0 {
		s.logger.Error().
			Int("configured_interval", config.intervalSeconds).
			Msg("Monitor CheckIntervalSeconds is not configured or invalid")
		return false
	}
	return true
}

func (s *Scheduler) startMonitorWorkers(ctx context.Context, maxWorkers int) {
	numWorkers := s.normalizeWorkerCount(maxWorkers)
	// Use a larger buffer to avoid blocking when dispatching jobs
	// Buffer size should be larger than the typical number of URLs to monitor
	bufferSize := numWorkers * 10 // Allow queuing multiple jobs per worker
	if bufferSize < 100 {
		bufferSize = 100 // Minimum buffer size for better throughput
	}
	s.monitorWorkerChan = make(chan MonitorJob, bufferSize)

	for i := 0; i < numWorkers; i++ {
		s.monitorWorkerWG.Add(1)
		go s.monitorWorker(ctx, i)
	}
}

func (s *Scheduler) normalizeWorkerCount(configured int) int {
	if configured <= 0 {
		return DefaultWorkerCount
	}
	return configured
}

func (s *Scheduler) startMonitorTicker(ctx context.Context, intervalSeconds int) {
	intervalDuration := time.Duration(intervalSeconds) * time.Second
	s.monitorTicker = time.NewTicker(intervalDuration)
	s.monitorWorkerWG.Add(1)

	// Trigger immediate first cycle
	go func() {
		s.executeMonitoringCycle(ctx, "initial")
		s.runMonitorTicker(ctx)
	}()
}

func (s *Scheduler) runMonitorTicker(ctx context.Context) {
	defer s.monitorWorkerWG.Done()
	s.logger.Info().Msg("Monitor ticker goroutine started")

	for {
		select {
		case <-s.monitorTicker.C:
			s.logger.Info().Msg("Monitor ticker fired, executing cycle")
			s.executeMonitoringCycle(ctx, "periodic")
		case <-s.stopChan:
			s.logger.Info().Msg("Monitor ticker received stop signal")
			s.monitorTicker.Stop()
			return
		case <-ctx.Done():
			s.logger.Info().Msg("Monitor ticker received context cancellation")
			s.monitorTicker.Stop()
			return
		}
	}
}

// monitorWorker processes monitor jobs
func (s *Scheduler) monitorWorker(ctx context.Context, id int) {
	defer s.monitorWorkerWG.Done()
	// s.logger.Info().Int("worker_id", id).Msg("Monitor worker started")

	for {
		select {
		// Receive job from monitor worker channel
		case job, ok := <-s.monitorWorkerChan:
			if !ok {
				s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping as channel closed.")
				return
			}

			// Check if we should stop before processing
			select {
			case <-s.stopChan:
				s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to scheduler's stop signal.")
				job.CycleWG.Done()
				return
			case <-ctx.Done():
				s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to context cancellation.")
				job.CycleWG.Done()
				return
			default:
			}

			s.logger.Info().Int("worker_id", id).Str("url", job.URL).Msg("Monitor worker processing job.")
			// Ensure monitoringService is not nil before calling CheckURL
			if s.monitoringService != nil {
				s.monitoringService.CheckURL(job.URL) // This should be a blocking call per URL
			} else {
				s.logger.Warn().Int("worker_id", id).Str("url", job.URL).Msg("Monitoring service is nil, cannot check URL.")
			}
			job.CycleWG.Done()
			// s.logger.Info().Int("worker_id", id).Str("url", job.URL).Msg("Monitor worker finished job.")
		case <-s.stopChan: // Listen for the scheduler's main stop signal
			s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to scheduler's stop signal.")
			return
		case <-ctx.Done():
			s.logger.Info().Int("worker_id", id).Msg("Monitor worker stopping due to context cancellation.")
			return
		}
	}
}

// executeMonitoringCycle performs a monitoring cycle for all monitored URLs
func (s *Scheduler) executeMonitoringCycle(ctx context.Context, cycleType string) {
	s.logger.Info().Str("cycle_type", cycleType).Msg("Starting monitor cycle")

	if !s.canExecuteMonitorCycle(ctx) {
		return
	}

	cycleID := s.initializeMonitorCycle()
	targets := s.monitoringService.GetCurrentlyMonitorUrls()

	s.logger.Info().
		Str("cycle_type", cycleType).
		Str("cycle_id", cycleID).
		Int("target_count", len(targets)).
		Msg("Retrieved targets for monitoring cycle")

	if len(targets) == 0 {
		s.logger.Info().Str("cycle_type", cycleType).Str("cycle_id", cycleID).Msg("No targets to check in this monitor cycle. Skipping cycle end report.")
		return
	}

	s.notificationHelper.SendMonitoredUrlsNotification(ctx, targets, cycleID)

	jobsDispatched, stopped := s.dispatchMonitorJobs(ctx, targets)

	s.logger.Info().
		Str("cycle_type", cycleType).
		Str("cycle_id", cycleID).
		Int("jobs_dispatched", jobsDispatched).
		Bool("stopped_early", stopped).
		Msg("Monitor cycle completed")

	if jobsDispatched > 0 {
		s.waitForJobsAndTriggerReport(jobsDispatched, stopped)
	}
}

func (s *Scheduler) canExecuteMonitorCycle(ctx context.Context) bool {
	if s.monitoringService == nil {
		s.logger.Warn().Str("cycle_type", "periodic").Msg("Scheduler: Monitoring service not available for cycle. Skipping.")
		return false
	}

	select {
	case <-s.stopChan:
		s.logger.Info().Str("cycle_type", "periodic").Msg("Scheduler: Stop signal received before starting monitor cycle. Skipping.")
		return false
	case <-ctx.Done():
		s.logger.Info().Str("cycle_type", "periodic").Msg("Scheduler: Context cancelled before starting monitor cycle. Skipping.")
		return false
	default:
		return true
	}
}

func (s *Scheduler) initializeMonitorCycle() string {
	cycleID := s.monitoringService.GenerateNewCycleID()
	s.monitoringService.SetCurrentCycleID(cycleID)
	s.logger.Info().Str("cycle_type", "periodic").Str("cycle_id", cycleID).Msg("Starting new monitoring cycle with generated ID.")
	return cycleID
}

func (s *Scheduler) waitForJobsAndTriggerReport(jobsDispatched int, stoppedPrematurely bool) {
	select {
	case <-s.stopChan:
		return
	default:
		if jobsDispatched > 0 && !stoppedPrematurely {
			s.monitoringService.TriggerCycleEndReport()
		}
	}
}

func (s *Scheduler) dispatchMonitorJobs(
	ctx context.Context,
	targets []string,
) (jobsDispatched int, stoppedPrematurely bool) {
	var cycleWG sync.WaitGroup

	s.logger.Info().Int("total_targets", len(targets)).Msg("Starting to dispatch monitor jobs")

	stopped := false
	for _, url := range targets {
		if s.shouldStopDispatching(ctx) {
			s.logger.Warn().Int("jobs_dispatched", jobsDispatched).Int("total_targets", len(targets)).Msg("Stopping job dispatch early")
			stopped = true
			break
		}

		if s.tryDispatchJob(url, &cycleWG, ctx) {
			jobsDispatched++
		}
	}

	s.logger.Info().Int("jobs_dispatched", jobsDispatched).Int("total_targets", len(targets)).Msg("All jobs dispatched, waiting for completion")
	cycleWG.Wait()
	s.logger.Info().Int("jobs_completed", jobsDispatched).Msg("All monitor jobs completed")

	return jobsDispatched, stopped
}

func (s *Scheduler) shouldStopDispatching(ctx context.Context) bool {
	select {
	case <-s.stopChan:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (s *Scheduler) tryDispatchJob(url string, cycleWG *sync.WaitGroup, ctx context.Context) bool {
	job := MonitorJob{
		URL:     url,
		CycleWG: cycleWG,
	}

	cycleWG.Add(1)

	select {
	case s.monitorWorkerChan <- job:
		return true
	case <-s.stopChan:
		s.logger.Debug().Str("url", url).Msg("Job dispatch cancelled due to stop signal")
		cycleWG.Done()
		return false
	case <-ctx.Done():
		s.logger.Debug().Str("url", url).Msg("Job dispatch cancelled due to context cancellation")
		cycleWG.Done()
		return false
	}
}
