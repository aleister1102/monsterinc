package limiter

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
	assert.Equal(t, config.MaxMemoryMB, rl.maxMemoryMB)
	assert.Equal(t, config.MaxGoroutines, rl.maxGoroutines)
	assert.Equal(t, config.CheckInterval, rl.checkInterval)
	assert.True(t, rl.enableAutoShutdown)
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
	logger := zerolog.Nop()
	config := DefaultResourceLimiterConfig()
	rl := NewResourceLimiter(config, logger)

	usage := rl.GetResourceUsage()

	assert.NotZero(t, usage.SysMB, "System memory should be reported")
	assert.NotZero(t, usage.Goroutines, "Goroutine count should be reported")
} 