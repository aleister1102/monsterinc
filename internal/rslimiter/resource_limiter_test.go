package rslimiter

import (
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceLimiter_New(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	rl := NewResourceLimiter(config, logger)

	require.NotNil(t, rl)
	assert.Equal(t, config.MaxMemoryMB, rl.config.MaxMemoryMB)
	assert.Equal(t, config.MaxGoroutines, rl.config.MaxGoroutines)
	assert.Equal(t, config.CheckInterval, rl.config.CheckInterval)
	assert.True(t, rl.config.EnableAutoShutdown)
}

func TestResourceLimiter_StartAndStop(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	rl := NewResourceLimiter(config, logger)

	rl.Start()
	assert.True(t, rl.isRunning, "ResourceLimiter should be running after Start()")

	rl.Stop()
	// Allow some time for the monitoring goroutine to stop
	time.Sleep(10 * time.Millisecond)
	assert.False(t, rl.isRunning, "ResourceLimiter should be stopped after Stop()")
}

func TestResourceLimiter_ShutdownCallback(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	config.EnableAutoShutdown = true
	rl := NewResourceLimiter(config, logger)

	var shutdownCalled bool
	var mu sync.Mutex

	rl.SetShutdownCallback(func() {
		mu.Lock()
		shutdownCalled = true
		mu.Unlock()
	})

	// Manually trigger the shutdown to test the callback mechanism
	rl.triggerGracefulShutdown()

	mu.Lock()
	assert.True(t, shutdownCalled, "Shutdown callback should have been called")
	mu.Unlock()
}

func TestResourceLimiter_CheckGoroutineLimit(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	// Set a very high limit that should not be reached
	config.MaxGoroutines = 100000
	rl := NewResourceLimiter(config, logger)

	err := rl.CheckGoroutineLimit()
	assert.NoError(t, err)
}

func TestResourceLimiter_GetResourceUsage(t *testing.T) {
	_ = NewResourceLimiter(DefaultResourceLimiterConfig(), zerolog.Nop())
	usage := GetResourceUsage()

	assert.NotZero(t, usage.SysMB, "System memory should be reported")
	assert.NotZero(t, usage.Goroutines, "Goroutine count should be reported")
}

func TestNewResourceLimiter_Defaults(t *testing.T) {
	logger := zerolog.Nop()
	// Create a config with zero values for fields that have defaults
	config := ResourceLimiterConfig{}
	rl := NewResourceLimiter(config, logger)

	require.NotNil(t, rl)
	assert.Equal(t, 30*time.Second, rl.config.CheckInterval)
	assert.Equal(t, 0.8, rl.config.MemoryThreshold)
	assert.Equal(t, 0.8, rl.config.GoroutineWarning)
	assert.Equal(t, 0.9, rl.config.SystemMemThreshold)
	assert.Equal(t, 0.9, rl.config.CPUThreshold)
}

func TestResourceLimiter_ShutdownNoCallback(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	rl := NewResourceLimiter(config, logger)

	// Should not panic
	assert.NotPanics(t, func() {
		rl.triggerGracefulShutdown()
	}, "triggerGracefulShutdown should not panic without a callback")
}

func TestResourceLimiter_GoroutineLimitExceeded(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	config.MaxGoroutines = 1 // Set a very low limit
	rl := NewResourceLimiter(config, logger)

	exceeded, reason := rl.goroutineChecker()
	assert.True(t, exceeded)
	assert.Contains(t, reason, "goroutine limit exceeded")
}

func TestResourceLimiter_Idempotency(t *testing.T) {
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	rl := NewResourceLimiter(config, logger)

	// Start multiple times
	rl.Start()
	initialRunningState := rl.isRunning
	rl.Start()
	assert.Equal(t, initialRunningState, rl.isRunning, "Start() should be idempotent")

	// Stop multiple times
	rl.Stop()
	initialStoppedState := rl.isRunning
	rl.Stop()
	assert.Equal(t, initialStoppedState, rl.isRunning, "Stop() should be idempotent")
}
