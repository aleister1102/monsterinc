package common

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ServiceState represents the current state of a service
type ServiceState string

const (
	ServiceStateStopped  ServiceState = "stopped"
	ServiceStateStarting ServiceState = "starting"
	ServiceStateRunning  ServiceState = "running"
	ServiceStateStopping ServiceState = "stopping"
	ServiceStateError    ServiceState = "error"
)

// ServiceInfo contains metadata about a service
type ServiceInfo struct {
	Name    string
	Version string
	Type    string
}

// HealthStatus represents service health check result
type HealthStatus struct {
	Healthy   bool
	Message   string
	Details   map[string]interface{}
	CheckedAt time.Time
}

// ServiceLifecycle defines the interface for service lifecycle operations
type ServiceLifecycle interface {
	Start(ctx context.Context) error
	Stop() error
	Health() HealthStatus
	Info() ServiceInfo
	State() ServiceState
}

// ResourceCleanup defines cleanup operations for service resources
type ResourceCleanup interface {
	Cleanup() error
}

// ServiceLifecycleManager provides standardized service lifecycle management
type ServiceLifecycleManager struct {
	info           ServiceInfo
	state          ServiceState
	stateMutex     sync.RWMutex
	logger         zerolog.Logger
	ctx            context.Context
	cancelFunc     context.CancelFunc
	wg             sync.WaitGroup
	isStarted      bool
	startedMutex   sync.RWMutex
	shutdownChan   chan struct{}
	resources      []ResourceCleanup
	resourcesMutex sync.RWMutex

	// Lifecycle hooks
	onStart  func(context.Context) error
	onStop   func() error
	onHealth func() HealthStatus
}

// ServiceLifecycleConfig holds configuration for service lifecycle manager
type ServiceLifecycleConfig struct {
	ServiceInfo     ServiceInfo
	Logger          zerolog.Logger
	OnStart         func(context.Context) error
	OnStop          func() error
	OnHealth        func() HealthStatus
	ShutdownTimeout time.Duration
}

// NewServiceLifecycleManager creates a new service lifecycle manager
func NewServiceLifecycleManager(config ServiceLifecycleConfig) *ServiceLifecycleManager {
	ctx, cancel := context.WithCancel(context.Background())

	serviceLogger := config.Logger.With().
		Str("service", config.ServiceInfo.Name).
		Str("service_type", config.ServiceInfo.Type).
		Logger()

	return &ServiceLifecycleManager{
		info:         config.ServiceInfo,
		state:        ServiceStateStopped,
		logger:       serviceLogger,
		ctx:          ctx,
		cancelFunc:   cancel,
		shutdownChan: make(chan struct{}),
		resources:    make([]ResourceCleanup, 0),
		onStart:      config.OnStart,
		onStop:       config.OnStop,
		onHealth:     config.OnHealth,
	}
}

// Start starts the service with standardized lifecycle management
func (slm *ServiceLifecycleManager) Start(parentCtx context.Context) error {
	slm.startedMutex.Lock()
	defer slm.startedMutex.Unlock()

	if slm.isStarted {
		slm.logger.Warn().Msg("Service is already started")
		return fmt.Errorf("service %s is already started", slm.info.Name)
	}

	slm.setState(ServiceStateStarting)
	slm.logger.Info().Msg("Starting service")

	// Create service context as child of parent
	slm.ctx, slm.cancelFunc = context.WithCancel(parentCtx)

	// Execute custom start logic if provided
	if slm.onStart != nil {
		if err := slm.onStart(slm.ctx); err != nil {
			slm.setState(ServiceStateError)
			slm.logger.Error().Err(err).Msg("Failed to start service")
			return fmt.Errorf("failed to start service %s: %w", slm.info.Name, err)
		}
	}

	slm.isStarted = true
	slm.setState(ServiceStateRunning)
	slm.logger.Info().Msg("Service started successfully")

	return nil
}

// Stop stops the service gracefully with proper resource cleanup
func (slm *ServiceLifecycleManager) Stop() error {
	slm.startedMutex.Lock()
	defer slm.startedMutex.Unlock()

	if !slm.isStarted {
		slm.logger.Info().Msg("Service is not started")
		return nil
	}

	slm.setState(ServiceStateStopping)
	slm.logger.Info().Msg("Stopping service")

	// Signal shutdown to any waiting goroutines
	select {
	case <-slm.shutdownChan:
		// Already closed
	default:
		close(slm.shutdownChan)
	}

	// Cancel service context to stop operations
	if slm.cancelFunc != nil {
		slm.cancelFunc()
	}

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		slm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slm.logger.Debug().Msg("All service goroutines finished")
	case <-time.After(10 * time.Second):
		slm.logger.Warn().Msg("Timeout waiting for service goroutines to finish")
	}

	// Execute custom stop logic if provided
	if slm.onStop != nil {
		if err := slm.onStop(); err != nil {
			slm.logger.Error().Err(err).Msg("Error during custom stop logic")
			// Don't return error, continue with cleanup
		}
	}

	// Cleanup all registered resources
	slm.cleanupResources()

	slm.isStarted = false
	slm.setState(ServiceStateStopped)
	slm.logger.Info().Msg("Service stopped successfully")

	return nil
}

// Health returns the current health status of the service
func (slm *ServiceLifecycleManager) Health() HealthStatus {
	slm.stateMutex.RLock()
	currentState := slm.state
	slm.stateMutex.RUnlock()

	// Default health check based on state
	defaultHealth := HealthStatus{
		Healthy:   currentState == ServiceStateRunning,
		Message:   fmt.Sprintf("Service is %s", currentState),
		Details:   map[string]interface{}{"state": currentState},
		CheckedAt: time.Now(),
	}

	// Execute custom health check if provided
	if slm.onHealth != nil {
		customHealth := slm.onHealth()
		// Combine default and custom health info
		defaultHealth.Details["custom_check"] = customHealth
		if !customHealth.Healthy {
			defaultHealth.Healthy = false
			defaultHealth.Message = customHealth.Message
		}
	}

	return defaultHealth
}

// Info returns service information
func (slm *ServiceLifecycleManager) Info() ServiceInfo {
	return slm.info
}

// State returns the current service state
func (slm *ServiceLifecycleManager) State() ServiceState {
	slm.stateMutex.RLock()
	defer slm.stateMutex.RUnlock()
	return slm.state
}

// Context returns the service context for operations
func (slm *ServiceLifecycleManager) Context() context.Context {
	return slm.ctx
}

// AddWorker adds a worker goroutine to the wait group
func (slm *ServiceLifecycleManager) AddWorker() {
	slm.wg.Add(1)
}

// WorkerDone signals that a worker goroutine has finished
func (slm *ServiceLifecycleManager) WorkerDone() {
	slm.wg.Done()
}

// ShutdownChan returns a channel that closes when shutdown is initiated
func (slm *ServiceLifecycleManager) ShutdownChan() <-chan struct{} {
	return slm.shutdownChan
}

// RegisterResource registers a resource for cleanup during service stop
func (slm *ServiceLifecycleManager) RegisterResource(resource ResourceCleanup) {
	slm.resourcesMutex.Lock()
	defer slm.resourcesMutex.Unlock()
	slm.resources = append(slm.resources, resource)
}

// IsStarted returns whether the service is currently started
func (slm *ServiceLifecycleManager) IsStarted() bool {
	slm.startedMutex.RLock()
	defer slm.startedMutex.RUnlock()
	return slm.isStarted
}

// WaitForShutdown waits for shutdown signal or context cancellation
func (slm *ServiceLifecycleManager) WaitForShutdown(ctx context.Context) error {
	select {
	case <-ctx.Done():
		slm.logger.Info().Msg("Service shutdown due to context cancellation")
		return ctx.Err()
	case <-slm.shutdownChan:
		slm.logger.Info().Msg("Service shutdown due to stop signal")
		return nil
	}
}

// setState sets the service state with proper locking
func (slm *ServiceLifecycleManager) setState(state ServiceState) {
	slm.stateMutex.Lock()
	defer slm.stateMutex.Unlock()

	if slm.state != state {
		oldState := slm.state
		slm.state = state
		slm.logger.Debug().
			Str("old_state", string(oldState)).
			Str("new_state", string(state)).
			Msg("Service state changed")
	}
}

// cleanupResources cleans up all registered resources
func (slm *ServiceLifecycleManager) cleanupResources() {
	slm.resourcesMutex.RLock()
	resources := make([]ResourceCleanup, len(slm.resources))
	copy(resources, slm.resources)
	slm.resourcesMutex.RUnlock()

	for i, resource := range resources {
		if resource != nil {
			if err := resource.Cleanup(); err != nil {
				slm.logger.Error().Err(err).Int("resource_index", i).Msg("Failed to cleanup resource")
			} else {
				slm.logger.Debug().Int("resource_index", i).Msg("Resource cleaned up successfully")
			}
		}
	}

	// Clear resources after cleanup
	slm.resourcesMutex.Lock()
	slm.resources = slm.resources[:0]
	slm.resourcesMutex.Unlock()
}

// BaseService provides a base implementation with common service patterns
type BaseService struct {
	*ServiceLifecycleManager
	name string
}

// NewBaseService creates a new base service with lifecycle management
func NewBaseService(name string, logger zerolog.Logger) *BaseService {
	config := ServiceLifecycleConfig{
		ServiceInfo: ServiceInfo{
			Name: name,
			Type: "base_service",
		},
		Logger: logger,
	}

	return &BaseService{
		ServiceLifecycleManager: NewServiceLifecycleManager(config),
		name:                    name,
	}
}

// StartWithMainLoop starts service with a main loop goroutine
func (bs *BaseService) StartWithMainLoop(parentCtx context.Context, mainLoop func(context.Context)) error {
	// Set custom start logic to run the main loop
	bs.onStart = func(ctx context.Context) error {
		bs.AddWorker()
		go func() {
			defer bs.WorkerDone()
			mainLoop(ctx)
		}()
		return nil
	}

	return bs.Start(parentCtx)
}

// DatabaseResource wraps database connection for resource cleanup
type DatabaseResource struct {
	closer interface{ Close() error }
	name   string
	logger zerolog.Logger
}

// NewDatabaseResource creates a new database resource for cleanup
func NewDatabaseResource(closer interface{ Close() error }, name string, logger zerolog.Logger) *DatabaseResource {
	return &DatabaseResource{
		closer: closer,
		name:   name,
		logger: logger,
	}
}

// Cleanup closes the database connection
func (dr *DatabaseResource) Cleanup() error {
	dr.logger.Info().Str("database", dr.name).Msg("Closing database connection")

	if dr.closer != nil {
		if err := dr.closer.Close(); err != nil {
			dr.logger.Error().Err(err).Str("database", dr.name).Msg("Error closing database")
			return err
		}
		dr.logger.Info().Str("database", dr.name).Msg("Database closed successfully")
	}

	return nil
}

// ChannelResource wraps channel for resource cleanup
type ChannelResource struct {
	channel interface{}
	name    string
	logger  zerolog.Logger
}

// NewChannelResource creates a new channel resource for cleanup
func NewChannelResource(channel interface{}, name string, logger zerolog.Logger) *ChannelResource {
	return &ChannelResource{
		channel: channel,
		name:    name,
		logger:  logger,
	}
}

// Cleanup closes the channel if it's a chan struct{}
func (cr *ChannelResource) Cleanup() error {
	cr.logger.Debug().Str("channel", cr.name).Msg("Closing channel")

	if ch, ok := cr.channel.(chan struct{}); ok {
		select {
		case <-ch:
			// Already closed
		default:
			close(ch)
		}
	}

	return nil
}

// ServiceRegistry manages multiple services
type ServiceRegistry struct {
	services map[string]ServiceLifecycle
	logger   zerolog.Logger
	mu       sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(logger zerolog.Logger) *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]ServiceLifecycle),
		logger:   logger.With().Str("component", "ServiceRegistry").Logger(),
	}
}

// Register registers a service with the registry
func (sr *ServiceRegistry) Register(name string, service ServiceLifecycle) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.services[name] = service
	sr.logger.Info().Str("service", name).Msg("Service registered")
}

// Unregister removes a service from the registry
func (sr *ServiceRegistry) Unregister(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	delete(sr.services, name)
	sr.logger.Info().Str("service", name).Msg("Service unregistered")
}

// StartAll starts all registered services
func (sr *ServiceRegistry) StartAll(ctx context.Context) error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	sr.logger.Info().Int("service_count", len(sr.services)).Msg("Starting all services")

	for name, service := range sr.services {
		if err := service.Start(ctx); err != nil {
			sr.logger.Error().Err(err).Str("service", name).Msg("Failed to start service")
			return fmt.Errorf("failed to start service %s: %w", name, err)
		}
		sr.logger.Info().Str("service", name).Msg("Service started")
	}

	return nil
}

// StopAll stops all registered services
func (sr *ServiceRegistry) StopAll() error {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	sr.logger.Info().Int("service_count", len(sr.services)).Msg("Stopping all services")

	var errors []error
	for name, service := range sr.services {
		if err := service.Stop(); err != nil {
			sr.logger.Error().Err(err).Str("service", name).Msg("Failed to stop service")
			errors = append(errors, fmt.Errorf("failed to stop service %s: %w", name, err))
		} else {
			sr.logger.Info().Str("service", name).Msg("Service stopped")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping services: %v", errors)
	}

	return nil
}

// HealthCheck returns health status for all services
func (sr *ServiceRegistry) HealthCheck() map[string]HealthStatus {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	health := make(map[string]HealthStatus)
	for name, service := range sr.services {
		health[name] = service.Health()
	}

	return health
}

// GetService returns a service by name
func (sr *ServiceRegistry) GetService(name string) (ServiceLifecycle, bool) {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	service, exists := sr.services[name]
	return service, exists
}

// ListServices returns a list of all registered service names
func (sr *ServiceRegistry) ListServices() []string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	names := make([]string, 0, len(sr.services))
	for name := range sr.services {
		names = append(names, name)
	}

	return names
}
