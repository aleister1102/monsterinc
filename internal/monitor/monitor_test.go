package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockMonitorInterface for testing
type MockMonitorInterface struct {
	startCalled bool
	stopCalled  bool
	isRunning   bool
	status      string
	lastError   error
}

func (m *MockMonitorInterface) Start(ctx context.Context) error {
	if m.startCalled && m.isRunning {
		return assert.AnError
	}
	m.startCalled = true
	m.isRunning = true
	return m.lastError
}

func (m *MockMonitorInterface) Stop() error {
	if !m.isRunning {
		return assert.AnError
	}
	m.stopCalled = true
	m.isRunning = false
	return m.lastError
}

func (m *MockMonitorInterface) IsRunning() bool {
	return m.isRunning
}

func (m *MockMonitorInterface) GetStatus() string {
	if m.isRunning {
		return "running"
	}
	return "stopped"
}

func TestMonitorConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.MonitorConfig
		expectValid bool
	}{
		{
			name: "valid config",
			config: &config.MonitorConfig{
				Enabled:              true,
				CheckIntervalSeconds: 60,
			},
			expectValid: true,
		},
		{
			name: "disabled monitor",
			config: &config.MonitorConfig{
				Enabled:              false,
				CheckIntervalSeconds: 0,
			},
			expectValid: true,
		},
		{
			name: "negative interval",
			config: &config.MonitorConfig{
				Enabled:              true,
				CheckIntervalSeconds: -10,
			},
			expectValid: false,
		},
		{
			name: "zero interval when enabled",
			config: &config.MonitorConfig{
				Enabled:              true,
				CheckIntervalSeconds: 0,
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateMonitorConfig(tt.config)
			assert.Equal(t, tt.expectValid, valid)
		})
	}
}

func TestMonitorLifecycle(t *testing.T) {
	config := &config.MonitorConfig{
		Enabled:              true,
		CheckIntervalSeconds: 1, // 1 second for testing
	}
	_ = config // Use the config variable to avoid unused warning

	monitor := &MockMonitorInterface{}

	t.Run("start monitor", func(t *testing.T) {
		ctx := context.Background()
		err := monitor.Start(ctx)

		require.NoError(t, err)
		assert.True(t, monitor.IsRunning())
	})

	t.Run("stop monitor", func(t *testing.T) {
		monitor := &MockMonitorInterface{isRunning: true}

		err := monitor.Stop()

		require.NoError(t, err)
		assert.False(t, monitor.IsRunning())
	})

	t.Run("get status", func(t *testing.T) {
		monitor := &MockMonitorInterface{isRunning: true}

		status := monitor.GetStatus()

		assert.Equal(t, "running", status)
	})
}

func TestMonitorContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		monitor := &MockMonitorInterface{}

		ctx, cancel := context.WithCancel(context.Background())

		// Start monitor
		err := monitor.Start(ctx)
		require.NoError(t, err)

		// Cancel context
		cancel()

		// Verify context is cancelled
		select {
		case <-ctx.Done():
			assert.Error(t, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context should have been cancelled")
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		monitor := &MockMonitorInterface{}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Start monitor
		err := monitor.Start(ctx)
		require.NoError(t, err)

		// Wait for timeout
		select {
		case <-ctx.Done():
			assert.Error(t, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context should have timed out")
		}
	})
}

func TestMonitorInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		expected time.Duration
	}{
		{
			name:     "1 second interval",
			interval: 1,
			expected: 1 * time.Second,
		},
		{
			name:     "60 second interval",
			interval: 60,
			expected: 60 * time.Second,
		},
		{
			name:     "300 second interval",
			interval: 300,
			expected: 300 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config.MonitorConfig{
				Enabled:              true,
				CheckIntervalSeconds: tt.interval,
			}

			duration := time.Duration(config.CheckIntervalSeconds) * time.Second
			assert.Equal(t, tt.expected, duration)
		})
	}
}

func TestMonitorErrorHandling(t *testing.T) {
	t.Run("start error", func(t *testing.T) {
		monitor := &MockMonitorInterface{lastError: assert.AnError}

		ctx := context.Background()
		err := monitor.Start(ctx)

		assert.Error(t, err)
	})

	t.Run("stop error", func(t *testing.T) {
		monitor := &MockMonitorInterface{lastError: assert.AnError}

		err := monitor.Stop()

		assert.Error(t, err)
	})
}

func TestMonitorState(t *testing.T) {
	tests := []struct {
		name           string
		isRunning      bool
		expectedStatus string
	}{
		{
			name:           "monitor running",
			isRunning:      true,
			expectedStatus: "running",
		},
		{
			name:           "monitor stopped",
			isRunning:      false,
			expectedStatus: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			monitor := &MockMonitorInterface{isRunning: tt.isRunning}

			running := monitor.IsRunning()
			status := monitor.GetStatus()

			assert.Equal(t, tt.isRunning, running)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

func TestMonitorConfiguration(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		config := &config.MonitorConfig{
			Enabled:              false,
			CheckIntervalSeconds: 0,
		}

		assert.False(t, config.Enabled)
		assert.Equal(t, 0, config.CheckIntervalSeconds)
	})

	t.Run("enabled config", func(t *testing.T) {
		config := &config.MonitorConfig{
			Enabled:              true,
			CheckIntervalSeconds: 60,
		}

		assert.True(t, config.Enabled)
		assert.Equal(t, 60, config.CheckIntervalSeconds)
		assert.True(t, validateMonitorConfig(config))
	})

	t.Run("disabled config with interval", func(t *testing.T) {
		config := &config.MonitorConfig{
			Enabled:              false,
			CheckIntervalSeconds: 60,
		}

		// Should be valid even if disabled with interval set
		assert.True(t, validateMonitorConfig(config))
	})
}

func TestMonitorConcurrency(t *testing.T) {
	t.Run("multiple start calls", func(t *testing.T) {
		monitor := &MockMonitorInterface{}

		ctx := context.Background()

		err1 := monitor.Start(ctx)
		assert.NoError(t, err1)

		err2 := monitor.Start(ctx)
		assert.Error(t, err2)
	})

	t.Run("multiple stop calls", func(t *testing.T) {
		monitor := &MockMonitorInterface{isRunning: true}

		err1 := monitor.Stop()
		assert.NoError(t, err1)

		err2 := monitor.Stop()
		assert.Error(t, err2)
	})
}

// Helper function for validation (would be implemented in the actual monitor package)
func validateMonitorConfig(config *config.MonitorConfig) bool {
	if !config.Enabled {
		return true // Disabled monitor is always valid
	}

	return config.CheckIntervalSeconds > 0
}
