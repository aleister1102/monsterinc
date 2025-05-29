package common

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ContextCheckResult represents the result of a context cancellation check
type ContextCheckResult struct {
	Cancelled bool
	Error     error
}

// CheckCancellation checks if the context is cancelled and returns appropriate result
func CheckCancellation(ctx context.Context) ContextCheckResult {
	select {
	case <-ctx.Done():
		return ContextCheckResult{
			Cancelled: true,
			Error:     ctx.Err(),
		}
	default:
		return ContextCheckResult{
			Cancelled: false,
			Error:     nil,
		}
	}
}

// CheckCancellationWithLog checks for context cancellation and logs if cancelled
func CheckCancellationWithLog(ctx context.Context, logger zerolog.Logger, operation string) ContextCheckResult {
	result := CheckCancellation(ctx)
	if result.Cancelled {
		logger.Info().Str("operation", operation).Msg("Context cancelled")
	}
	return result
}

// WaitWithCancellation waits for a duration or until context is cancelled
func WaitWithCancellation(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitWithCancellationAndStop waits for duration, context cancellation, or stop signal
func WaitWithCancellationAndStop(ctx context.Context, duration time.Duration, stopChan <-chan struct{}) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-stopChan:
		return nil // Stop signal received
	}
}

// ExecuteWithTimeout creates a context with timeout and executes a function
func ExecuteWithTimeout(parentCtx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	return fn(ctx)
}

// ExecuteWithCancellationCheck executes a function with periodic cancellation checks
func ExecuteWithCancellationCheck(ctx context.Context, logger zerolog.Logger, operation string, fn func() error) error {
	// Check for cancellation before starting
	if result := CheckCancellationWithLog(ctx, logger, operation+" (before start)"); result.Cancelled {
		return result.Error
	}

	// Execute the function
	err := fn()

	// Check for cancellation after execution
	if result := CheckCancellationWithLog(ctx, logger, operation+" (after completion)"); result.Cancelled {
		return result.Error
	}

	return err
}

// CreateServiceContext creates a service-specific context with cancellation
func CreateServiceContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// CreateTimeoutContext creates a context with timeout
func CreateTimeoutContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

// RetryConfig holds configuration for retry operations
type RetryConfig struct {
	MaxRetries int
	RetryDelay time.Duration
	Operation  string
	SessionID  string
}

// RetryResult represents the result of a retry operation
type RetryResult struct {
	Attempt       int
	LastError     error
	Success       bool
	TotalDuration time.Duration
}

// RetryWithCancellation executes a function with retry logic and context cancellation support
func RetryWithCancellation(ctx context.Context, logger zerolog.Logger, config RetryConfig, fn func(attempt int) error) RetryResult {
	startTime := time.Now()
	result := RetryResult{
		Attempt: 0,
		Success: false,
	}

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		result.Attempt = attempt

		// Check for cancellation at the beginning of each attempt
		if cancelResult := CheckCancellationWithLog(ctx, logger, config.Operation+" retry attempt"); cancelResult.Cancelled {
			logger.Info().
				Str("session_id", config.SessionID).
				Int("attempt", attempt).
				Msg("Context cancelled before retry attempt")
			result.LastError = cancelResult.Error
			result.TotalDuration = time.Since(startTime)
			return result
		}

		// Apply retry delay for attempts > 0
		if attempt > 0 && config.RetryDelay > 0 {
			logger.Info().
				Int("attempt", attempt).
				Int("max_retries", config.MaxRetries).
				Dur("delay", config.RetryDelay).
				Str("operation", config.Operation).
				Msg("Retrying after delay")

			if err := WaitWithCancellation(ctx, config.RetryDelay); err != nil {
				logger.Info().
					Str("session_id", config.SessionID).
					Err(err).
					Msg("Context cancelled during retry delay")
				result.LastError = err
				result.TotalDuration = time.Since(startTime)
				return result
			}
		}

		// Execute the function
		err := fn(attempt)
		if err == nil {
			logger.Info().
				Str("session_id", config.SessionID).
				Int("attempt", attempt).
				Str("operation", config.Operation).
				Msg("Operation completed successfully")
			result.Success = true
			result.TotalDuration = time.Since(startTime)
			return result
		}

		result.LastError = err

		// Check if error is context-related and should not be retried
		if IsContextError(err) {
			logger.Info().
				Str("session_id", config.SessionID).
				Err(err).
				Str("operation", config.Operation).
				Msg("Operation interrupted by context cancellation, no further retries")
			result.TotalDuration = time.Since(startTime)
			return result
		}

		logger.Error().
			Err(err).
			Str("session_id", config.SessionID).
			Int("attempt", attempt+1).
			Int("total_attempts", config.MaxRetries+1).
			Str("operation", config.Operation).
			Msg("Operation failed")

		// Check if this was the last attempt
		if attempt == config.MaxRetries {
			logger.Error().
				Str("session_id", config.SessionID).
				Str("operation", config.Operation).
				Msg("All retry attempts exhausted, operation failed permanently")
		}
	}

	result.TotalDuration = time.Since(startTime)
	return result
}

// IsContextError checks if an error is context-related (cancelled or deadline exceeded)
func IsContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// ContainsCancellationError checks if error messages contain cancellation-related errors
func ContainsCancellationError(messages []string) bool {
	for _, msg := range messages {
		if containsCancellationKeywords(msg) {
			return true
		}
	}
	return false
}

// containsCancellationKeywords checks for cancellation keywords in a message
func containsCancellationKeywords(message string) bool {
	keywords := []string{
		"context canceled",
		"context deadline exceeded",
		"operation interrupted",
		"cancelled",
		"canceled",
	}

	for _, keyword := range keywords {
		if len(message) >= len(keyword) {
			for i := 0; i <= len(message)-len(keyword); i++ {
				if message[i:i+len(keyword)] == keyword {
					return true
				}
			}
		}
	}
	return false
}

// TimeoutConfig holds configuration for timeout operations
type TimeoutConfig struct {
	Timeout   time.Duration
	Operation string
	SessionID string
}

// ExecuteWithTimeoutAndRetry combines timeout and retry functionality
func ExecuteWithTimeoutAndRetry(ctx context.Context, logger zerolog.Logger, timeoutConfig TimeoutConfig, retryConfig RetryConfig, fn func(context.Context, int) error) RetryResult {
	wrappedFn := func(attempt int) error {
		return ExecuteWithTimeout(ctx, timeoutConfig.Timeout, func(timeoutCtx context.Context) error {
			return fn(timeoutCtx, attempt)
		})
	}

	return RetryWithCancellation(ctx, logger, retryConfig, wrappedFn)
}

// OperationConfig combines all operation configuration options
type OperationConfig struct {
	Operation     string
	SessionID     string
	Timeout       time.Duration
	MaxRetries    int
	RetryDelay    time.Duration
	EnableTimeout bool
	EnableRetry   bool
}

// OperationResult represents the comprehensive result of an operation
type OperationResult struct {
	Success       bool
	Error         error
	Attempts      int
	TotalDuration time.Duration
	TimedOut      bool
	Cancelled     bool
}

// ExecuteOperation provides a unified interface for executing operations with timeout, retry, and cancellation
func ExecuteOperation(ctx context.Context, logger zerolog.Logger, config OperationConfig, fn func(context.Context, int) error) OperationResult {
	startTime := time.Now()
	result := OperationResult{}

	// Simple execution without retry or timeout
	if !config.EnableTimeout && !config.EnableRetry {
		err := fn(ctx, 0)
		result.Success = err == nil
		result.Error = err
		result.Attempts = 1
		result.TotalDuration = time.Since(startTime)
		result.Cancelled = IsContextError(err)
		return result
	}

	// Execution with timeout but no retry
	if config.EnableTimeout && !config.EnableRetry {
		err := ExecuteWithTimeout(ctx, config.Timeout, func(timeoutCtx context.Context) error {
			return fn(timeoutCtx, 0)
		})
		result.Success = err == nil
		result.Error = err
		result.Attempts = 1
		result.TotalDuration = time.Since(startTime)
		result.TimedOut = errors.Is(err, context.DeadlineExceeded)
		result.Cancelled = IsContextError(err)
		return result
	}

	// Execution with retry but no timeout
	if !config.EnableTimeout && config.EnableRetry {
		retryConfig := RetryConfig{
			MaxRetries: config.MaxRetries,
			RetryDelay: config.RetryDelay,
			Operation:  config.Operation,
			SessionID:  config.SessionID,
		}
		retryResult := RetryWithCancellation(ctx, logger, retryConfig, func(attempt int) error {
			return fn(ctx, attempt)
		})
		result.Success = retryResult.Success
		result.Error = retryResult.LastError
		result.Attempts = retryResult.Attempt + 1
		result.TotalDuration = retryResult.TotalDuration
		result.Cancelled = IsContextError(retryResult.LastError)
		return result
	}

	// Execution with both timeout and retry
	timeoutConfig := TimeoutConfig{
		Timeout:   config.Timeout,
		Operation: config.Operation,
		SessionID: config.SessionID,
	}
	retryConfig := RetryConfig{
		MaxRetries: config.MaxRetries,
		RetryDelay: config.RetryDelay,
		Operation:  config.Operation,
		SessionID:  config.SessionID,
	}
	retryResult := ExecuteWithTimeoutAndRetry(ctx, logger, timeoutConfig, retryConfig, fn)
	result.Success = retryResult.Success
	result.Error = retryResult.LastError
	result.Attempts = retryResult.Attempt + 1
	result.TotalDuration = retryResult.TotalDuration
	result.TimedOut = errors.Is(retryResult.LastError, context.DeadlineExceeded)
	result.Cancelled = IsContextError(retryResult.LastError)
	return result
}

// ExecuteStepWithCancellation executes a workflow step with cancellation check before and after
func ExecuteStepWithCancellation(ctx context.Context, logger zerolog.Logger, stepName string, sessionID string, fn func() error) error {
	// Check before execution
	if cancelled := CheckCancellationWithLog(ctx, logger, stepName+" (before start)"); cancelled.Cancelled {
		logger.Info().Str("step", stepName).Str("session_id", sessionID).Msg("Step cancelled before start")
		return cancelled.Error
	}

	logger.Debug().Str("step", stepName).Str("session_id", sessionID).Msg("Starting workflow step")

	// Execute the step
	err := fn()

	// Check after execution
	if cancelled := CheckCancellationWithLog(ctx, logger, stepName+" (after completion)"); cancelled.Cancelled {
		logger.Info().Str("step", stepName).Str("session_id", sessionID).Msg("Step cancelled after completion")
		return cancelled.Error
	}

	if err != nil {
		logger.Error().Err(err).Str("step", stepName).Str("session_id", sessionID).Msg("Workflow step failed")
	} else {
		logger.Debug().Str("step", stepName).Str("session_id", sessionID).Msg("Workflow step completed successfully")
	}

	return err
}

// WorkflowStep represents a single step in a workflow
type WorkflowStep struct {
	Name        string
	Description string
	Function    func() error
	Required    bool // If false, failure won't stop the workflow
}

// SimpleWorkflowConfig holds configuration for simple workflow execution
type SimpleWorkflowConfig struct {
	WorkflowName string
	SessionID    string
	Steps        []WorkflowStep
	StopOnError  bool // If true, stop workflow on first error from required step
}

// SimpleWorkflowResult represents the result of simple workflow execution
type SimpleWorkflowResult struct {
	Success         bool
	CompletedSteps  int
	TotalSteps      int
	StepResults     map[string]error
	FirstError      error
	TotalDuration   time.Duration
	CancelledAtStep string
}

// ExecuteWorkflowWithCancellation executes a workflow with cancellation support
func ExecuteWorkflowWithCancellation(ctx context.Context, logger zerolog.Logger, config SimpleWorkflowConfig) SimpleWorkflowResult {
	startTime := time.Now()
	result := SimpleWorkflowResult{
		TotalSteps:  len(config.Steps),
		StepResults: make(map[string]error),
		Success:     true,
	}

	logger.Info().
		Str("workflow", config.WorkflowName).
		Str("session_id", config.SessionID).
		Int("total_steps", result.TotalSteps).
		Msg("Starting workflow execution")

	for i, step := range config.Steps {
		// Check for cancellation before each step
		if cancelled := CheckCancellationWithLog(ctx, logger, config.WorkflowName+" step: "+step.Name); cancelled.Cancelled {
			result.CancelledAtStep = step.Name
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			logger.Info().
				Str("workflow", config.WorkflowName).
				Str("step", step.Name).
				Str("session_id", config.SessionID).
				Msg("Workflow cancelled")
			return result
		}

		logger.Debug().
			Str("workflow", config.WorkflowName).
			Str("step", step.Name).
			Str("description", step.Description).
			Int("step_number", i+1).
			Int("total_steps", result.TotalSteps).
			Msg("Executing workflow step")

		err := step.Function()
		result.StepResults[step.Name] = err

		if err != nil {
			logger.Error().
				Err(err).
				Str("workflow", config.WorkflowName).
				Str("step", step.Name).
				Bool("required", step.Required).
				Msg("Workflow step failed")

			if result.FirstError == nil {
				result.FirstError = err
			}

			if step.Required && config.StopOnError {
				result.Success = false
				result.TotalDuration = time.Since(startTime)
				logger.Error().
					Str("workflow", config.WorkflowName).
					Str("step", step.Name).
					Msg("Workflow stopped due to required step failure")
				return result
			}

			if step.Required {
				result.Success = false
			}
		} else {
			result.CompletedSteps++
			logger.Debug().
				Str("workflow", config.WorkflowName).
				Str("step", step.Name).
				Msg("Workflow step completed successfully")
		}
	}

	result.TotalDuration = time.Since(startTime)

	if result.Success {
		logger.Info().
			Str("workflow", config.WorkflowName).
			Str("session_id", config.SessionID).
			Int("completed_steps", result.CompletedSteps).
			Dur("duration", result.TotalDuration).
			Msg("Workflow completed successfully")
	} else {
		logger.Warn().
			Str("workflow", config.WorkflowName).
			Str("session_id", config.SessionID).
			Int("completed_steps", result.CompletedSteps).
			Int("total_steps", result.TotalSteps).
			Dur("duration", result.TotalDuration).
			Msg("Workflow completed with errors")
	}

	return result
}

// ConcurrentExecutor handles concurrent operations with context cancellation
type ConcurrentExecutor struct {
	ctx           context.Context
	logger        zerolog.Logger
	maxConcurrent int
	semaphore     chan struct{}
}

// NewConcurrentExecutor creates a new concurrent executor
func NewConcurrentExecutor(ctx context.Context, logger zerolog.Logger, maxConcurrent int) *ConcurrentExecutor {
	return &ConcurrentExecutor{
		ctx:           ctx,
		logger:        logger,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// Execute runs a function concurrently with cancellation support
func (ce *ConcurrentExecutor) Execute(operation string, fn func() error) error {
	// Check for cancellation before acquiring semaphore
	if cancelled := CheckCancellation(ce.ctx); cancelled.Cancelled {
		ce.logger.Debug().Str("operation", operation).Msg("Operation cancelled before execution")
		return cancelled.Error
	}

	// Acquire semaphore
	select {
	case ce.semaphore <- struct{}{}:
		defer func() { <-ce.semaphore }()
	case <-ce.ctx.Done():
		ce.logger.Debug().Str("operation", operation).Msg("Operation cancelled while waiting for semaphore")
		return ce.ctx.Err()
	}

	// Check for cancellation after acquiring semaphore
	if cancelled := CheckCancellation(ce.ctx); cancelled.Cancelled {
		ce.logger.Debug().Str("operation", operation).Msg("Operation cancelled after acquiring semaphore")
		return cancelled.Error
	}

	// Execute the function
	return fn()
}

// GracefulShutdown handles graceful shutdown with context and stop channel
type GracefulShutdown struct {
	stopChan chan struct{}
	logger   zerolog.Logger
}

// NewGracefulShutdown creates a new graceful shutdown handler
func NewGracefulShutdown(logger zerolog.Logger) *GracefulShutdown {
	return &GracefulShutdown{
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

// Stop signals the shutdown
func (gs *GracefulShutdown) Stop() {
	select {
	case <-gs.stopChan:
		// Already stopped
		gs.logger.Debug().Msg("Stop signal already sent")
	default:
		close(gs.stopChan)
		gs.logger.Info().Msg("Graceful shutdown initiated")
	}
}

// Wait waits for either context cancellation or stop signal
func (gs *GracefulShutdown) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		gs.logger.Info().Msg("Shutdown due to context cancellation")
		return ctx.Err()
	case <-gs.stopChan:
		gs.logger.Info().Msg("Shutdown due to stop signal")
		return nil
	}
}

// StopChan returns the stop channel for external use
func (gs *GracefulShutdown) StopChan() <-chan struct{} {
	return gs.stopChan
}

// WaitWithTimeout waits for shutdown with a timeout
func (gs *GracefulShutdown) WaitWithTimeout(ctx context.Context, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
			gs.logger.Warn().Dur("timeout", timeout).Msg("Graceful shutdown timed out")
			return timeoutCtx.Err()
		}
		gs.logger.Info().Msg("Shutdown due to context cancellation")
		return ctx.Err()
	case <-gs.stopChan:
		gs.logger.Info().Msg("Shutdown due to stop signal")
		return nil
	}
}

// IsShutdownRequested returns true if shutdown has been requested
func (gs *GracefulShutdown) IsShutdownRequested() bool {
	select {
	case <-gs.stopChan:
		return true
	default:
		return false
	}
}

// ShutdownManager manages multiple services with graceful shutdown
type ShutdownManager struct {
	services map[string]*GracefulShutdown
	logger   zerolog.Logger
	mu       sync.RWMutex
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(logger zerolog.Logger) *ShutdownManager {
	return &ShutdownManager{
		services: make(map[string]*GracefulShutdown),
		logger:   logger,
	}
}

// RegisterService registers a service for graceful shutdown
func (sm *ShutdownManager) RegisterService(name string, service *GracefulShutdown) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.services[name] = service
	sm.logger.Info().Str("service", name).Msg("Service registered for graceful shutdown")
}

// ShutdownAll initiates shutdown for all registered services
func (sm *ShutdownManager) ShutdownAll() {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sm.logger.Info().Int("service_count", len(sm.services)).Msg("Initiating shutdown for all services")

	for name, service := range sm.services {
		sm.logger.Info().Str("service", name).Msg("Stopping service")
		service.Stop()
	}
}

// WaitForAll waits for all services to shutdown
func (sm *ShutdownManager) WaitForAll(ctx context.Context, timeout time.Duration) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.services) == 0 {
		sm.logger.Info().Msg("No services to wait for")
		return nil
	}

	sm.logger.Info().Int("service_count", len(sm.services)).Dur("timeout", timeout).Msg("Waiting for all services to shutdown")

	// Create channels to track completion
	done := make(chan struct{})
	var wg sync.WaitGroup

	// Wait for each service
	for name, service := range sm.services {
		wg.Add(1)
		go func(serviceName string, svc *GracefulShutdown) {
			defer wg.Done()
			if err := svc.WaitWithTimeout(ctx, timeout); err != nil {
				sm.logger.Error().Err(err).Str("service", serviceName).Msg("Service shutdown error")
			} else {
				sm.logger.Info().Str("service", serviceName).Msg("Service shutdown completed")
			}
		}(name, service)
	}

	// Signal when all are done
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for completion or context cancellation
	select {
	case <-done:
		sm.logger.Info().Msg("All services shutdown completed")
		return nil
	case <-ctx.Done():
		sm.logger.Warn().Msg("Shutdown manager context cancelled")
		return ctx.Err()
	}
}

// ServiceManager combines service lifecycle with graceful shutdown
type ServiceManager struct {
	name       string
	isRunning  bool
	mu         sync.RWMutex
	shutdown   *GracefulShutdown
	logger     zerolog.Logger
	startFunc  func(context.Context) error
	stopFunc   func() error
	healthFunc func() error
}

// ServiceConfig holds configuration for a service manager
type ServiceConfig struct {
	Name       string
	StartFunc  func(context.Context) error
	StopFunc   func() error
	HealthFunc func() error
}

// NewServiceManager creates a new service manager
func NewServiceManager(config ServiceConfig, logger zerolog.Logger) *ServiceManager {
	return &ServiceManager{
		name:       config.Name,
		shutdown:   NewGracefulShutdown(logger.With().Str("service", config.Name).Logger()),
		logger:     logger.With().Str("service", config.Name).Logger(),
		startFunc:  config.StartFunc,
		stopFunc:   config.StopFunc,
		healthFunc: config.HealthFunc,
	}
}

// Start starts the service
func (sm *ServiceManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	if sm.isRunning {
		sm.mu.Unlock()
		sm.logger.Warn().Msg("Service is already running")
		return fmt.Errorf("service %s is already running", sm.name)
	}
	sm.isRunning = true
	sm.mu.Unlock()

	sm.logger.Info().Msg("Starting service")

	if sm.startFunc != nil {
		if err := sm.startFunc(ctx); err != nil {
			sm.mu.Lock()
			sm.isRunning = false
			sm.mu.Unlock()
			sm.logger.Error().Err(err).Msg("Failed to start service")
			return fmt.Errorf("failed to start service %s: %w", sm.name, err)
		}
	}

	sm.logger.Info().Msg("Service started successfully")
	return nil
}

// Stop stops the service gracefully
func (sm *ServiceManager) Stop() error {
	sm.mu.Lock()
	if !sm.isRunning {
		sm.mu.Unlock()
		sm.logger.Info().Msg("Service is not running")
		return nil
	}
	sm.mu.Unlock()

	sm.logger.Info().Msg("Stopping service")

	// Signal shutdown
	sm.shutdown.Stop()

	// Call custom stop function if provided
	if sm.stopFunc != nil {
		if err := sm.stopFunc(); err != nil {
			sm.logger.Error().Err(err).Msg("Error during service stop")
			return fmt.Errorf("error stopping service %s: %w", sm.name, err)
		}
	}

	sm.mu.Lock()
	sm.isRunning = false
	sm.mu.Unlock()

	sm.logger.Info().Msg("Service stopped successfully")
	return nil
}

// IsRunning returns whether the service is running
func (sm *ServiceManager) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.isRunning
}

// Health checks the health of the service
func (sm *ServiceManager) Health() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.isRunning {
		return fmt.Errorf("service %s is not running", sm.name)
	}

	if sm.healthFunc != nil {
		return sm.healthFunc()
	}

	return nil
}

// GetShutdown returns the graceful shutdown handler
func (sm *ServiceManager) GetShutdown() *GracefulShutdown {
	return sm.shutdown
}

// Name returns the service name
func (sm *ServiceManager) Name() string {
	return sm.name
}
