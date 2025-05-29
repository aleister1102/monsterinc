package common

import (
	"context"

	"github.com/rs/zerolog"
)

// Validator defines the interface for objects that can validate their configuration
type Validator interface {
	Validate() error
}

// Initializer defines the interface for objects that need initialization
type Initializer interface {
	Initialize() error
}

// Configurable defines the interface for objects that can be configured
type Configurable interface {
	Configure(config interface{}) error
}

// Startable defines the interface for services that can be started
type Startable interface {
	Start(ctx context.Context) error
}

// Stoppable defines the interface for services that can be stopped
type Stoppable interface {
	Stop() error
}

// Service combines common service behaviors
type Service interface {
	Startable
	Stoppable
}

// HealthCheckable defines the interface for objects that can report their health status
type HealthCheckable interface {
	HealthCheck() error
}

// Closeable defines the interface for objects that need cleanup
type Closeable interface {
	Close() error
}

// LoggerProvider defines the interface for objects that can provide a logger
type LoggerProvider interface {
	Logger() zerolog.Logger
}

// ConfigProvider defines the interface for objects that can provide configuration
type ConfigProvider interface {
	GetConfig() interface{}
}

// ComponentInfo defines the interface for objects that can provide component information
type ComponentInfo interface {
	Name() string
	Version() string
}

// Resettable defines the interface for objects that can be reset to their initial state
type Resettable interface {
	Reset() error
}

// Refreshable defines the interface for objects that can refresh their state
type Refreshable interface {
	Refresh() error
}

// Constructor defines a function type for creating new instances with consistent parameters
// config first, logger second, then dependencies
type Constructor[T any] func(config interface{}, logger zerolog.Logger, deps ...interface{}) (T, error)

// ServiceConstructor defines a function type for creating services
type ServiceConstructor[T Service] func(config interface{}, logger zerolog.Logger, deps ...interface{}) (T, error)

// Repository defines the interface for data repository operations
type Repository[T any] interface {
	Create(ctx context.Context, entity T) error
	GetByID(ctx context.Context, id string) (T, error)
	Update(ctx context.Context, entity T) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filters map[string]interface{}) ([]T, error)
}

// Store defines a generic storage interface
type Store[T any] interface {
	Store(ctx context.Context, key string, value T) error
	Retrieve(ctx context.Context, key string) (T, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) bool
}

// Processor defines the interface for content processors
type Processor[TInput, TOutput any] interface {
	Process(ctx context.Context, input TInput) (TOutput, error)
}

// Scanner defines the interface for content scanners
type Scanner[TInput, TOutput any] interface {
	Scan(ctx context.Context, input TInput) ([]TOutput, error)
}

// Notifier defines the interface for notification services
type Notifier interface {
	Notify(ctx context.Context, message interface{}) error
}

// EventHandler defines the interface for event handlers
type EventHandler[T any] interface {
	Handle(ctx context.Context, event T) error
}

// Publisher defines the interface for event publishers
type Publisher[T any] interface {
	Publish(ctx context.Context, event T) error
}

// Subscriber defines the interface for event subscribers
type Subscriber[T any] interface {
	Subscribe(ctx context.Context, handler EventHandler[T]) error
	Unsubscribe(ctx context.Context, handler EventHandler[T]) error
}

// Cache defines the interface for caching operations
type Cache[K comparable, V any] interface {
	Get(ctx context.Context, key K) (V, bool)
	Set(ctx context.Context, key K, value V) error
	Delete(ctx context.Context, key K) error
	Clear(ctx context.Context) error
}

// Transformer defines the interface for data transformation
type Transformer[TFrom, TTo any] interface {
	Transform(ctx context.Context, from TFrom) (TTo, error)
}

// Filter defines the interface for data filtering
type Filter[T any] interface {
	Filter(ctx context.Context, items []T) ([]T, error)
}

// Serializer defines the interface for data serialization
type Serializer[T any] interface {
	Serialize(ctx context.Context, data T) ([]byte, error)
	Deserialize(ctx context.Context, data []byte) (T, error)
}

// Metrics defines the interface for metrics collection
type Metrics interface {
	Increment(name string) error
	Gauge(name string, value float64) error
	Histogram(name string, value float64) error
	Timer(name string, duration int64) error
}

// Logger interface wraps zerolog.Logger for dependency injection patterns
type Logger interface {
	Debug() *zerolog.Event
	Info() *zerolog.Event
	Warn() *zerolog.Event
	Error() *zerolog.Event
	Fatal() *zerolog.Event
	Panic() *zerolog.Event
	With() zerolog.Context
	Level(level zerolog.Level) zerolog.Logger
}

// ComponentWithDependencies defines a component that has dependencies
type ComponentWithDependencies interface {
	Dependencies() []interface{}
	SetDependencies(deps []interface{}) error
}

// LifecycleManager manages the lifecycle of components
type LifecycleManager interface {
	Initialize(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// BackgroundWorker defines the interface for background workers
type BackgroundWorker interface {
	Service
	HealthCheckable
	IsRunning() bool
}

// RateLimited defines the interface for rate-limited operations
type RateLimited interface {
	IsAllowed(ctx context.Context) bool
	Wait(ctx context.Context) error
}

// Retryable defines the interface for operations that can be retried
type Retryable interface {
	Retry(ctx context.Context, operation func() error) error
}

// TimeoutEnabled defines the interface for operations with timeout support
type TimeoutEnabled interface {
	WithTimeout(ctx context.Context, timeout int) context.Context
}
