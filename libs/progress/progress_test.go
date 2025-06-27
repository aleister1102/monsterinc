package progress

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgressDisplayManager_New(t *testing.T) {
	logger := zerolog.Nop()
	config := &ProgressDisplayConfig{
		DisplayInterval:   5 * time.Second,
		EnableProgress:    true,
		ShowETAEstimation: true,
	}
	pdm := NewProgressDisplayManager(logger, config)

	require.NotNil(t, pdm)
	assert.Equal(t, 5*time.Second, pdm.config.DisplayInterval)
	assert.True(t, pdm.config.EnableProgress)
	assert.True(t, pdm.config.ShowETAEstimation)
	assert.NotNil(t, pdm.scanProgress)
	assert.NotNil(t, pdm.monitorProgress)
}

func TestProgressDisplayManager_NewWithNilConfig(t *testing.T) {
	logger := zerolog.Nop()
	pdm := NewProgressDisplayManager(logger, nil)

	require.NotNil(t, pdm)
	assert.NotNil(t, pdm.config)
	assert.Equal(t, 3*time.Second, pdm.config.DisplayInterval)
	assert.True(t, pdm.config.EnableProgress)
}

func TestProgressInfo_GetPercentage(t *testing.T) {
	pi := &ProgressInfo{Current: 50, Total: 200}
	assert.InDelta(t, 25.0, pi.GetPercentage(), 0.01)

	pi.Total = 0
	assert.Equal(t, 0.0, pi.GetPercentage())

	pi.Current = 100
	pi.Total = 100
	assert.InDelta(t, 100.0, pi.GetPercentage(), 0.01)
}

func TestProgressInfo_UpdateETA(t *testing.T) {
	pi := &ProgressInfo{
		Current:   50,
		Total:     100,
		StartTime: time.Now().Add(-10 * time.Second),
	}
	pi.UpdateETA()
	assert.InDelta(t, (10 * time.Second).Seconds(), pi.EstimatedETA.Seconds(), float64(time.Second), "ETA should be around 10 seconds")

	// Test case where current is 0
	pi.Current = 0
	pi.UpdateETA()
	assert.Equal(t, time.Duration(0), pi.EstimatedETA)

	// Test case where elapsed time is very short
	pi.Current = 1
	pi.StartTime = time.Now()
	pi.UpdateETA()
	assert.Equal(t, time.Duration(0), pi.EstimatedETA)
}

func TestProgressDisplayManager_UpdateScanProgress(t *testing.T) {
	pdm := NewProgressDisplayManager(zerolog.Nop(), nil)
	pdm.UpdateScanProgress(10, 100, "testing", "message")

	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	assert.Equal(t, int64(10), pdm.scanProgress.Current)
	assert.Equal(t, int64(100), pdm.scanProgress.Total)
	assert.Equal(t, "testing", pdm.scanProgress.Stage)
	assert.Equal(t, "message", pdm.scanProgress.Message)
	assert.Equal(t, ProgressStatusRunning, pdm.scanProgress.Status)
}

func TestProgressDisplayManager_SetScanStatus(t *testing.T) {
	pdm := NewProgressDisplayManager(zerolog.Nop(), nil)
	pdm.SetScanStatus(ProgressStatusComplete, "done")

	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	assert.Equal(t, ProgressStatusComplete, pdm.scanProgress.Status)
	assert.Equal(t, "done", pdm.scanProgress.Message)
}

func TestProgressDisplayManager_UpdateBatchProgress(t *testing.T) {
	pdm := NewProgressDisplayManager(zerolog.Nop(), nil)
	pdm.UpdateBatchProgress(ProgressTypeScan, 2, 5)

	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	require.NotNil(t, pdm.scanProgress.BatchInfo)
	assert.Equal(t, 2, pdm.scanProgress.BatchInfo.CurrentBatch)
	assert.Equal(t, 5, pdm.scanProgress.BatchInfo.TotalBatches)
}

func TestProgressDisplayManager_ResetBatchProgress(t *testing.T) {
	pdm := NewProgressDisplayManager(zerolog.Nop(), nil)
	pdm.UpdateScanProgress(50, 100, "old", "old")
	time.Sleep(10 * time.Millisecond)
	oldStartTime := pdm.scanProgress.StartTime

	pdm.ResetBatchProgress(ProgressTypeScan, 1, 10, "new", "new")

	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	assert.Equal(t, int64(0), pdm.scanProgress.Current)
	assert.Equal(t, int64(10), pdm.scanProgress.Total)
	assert.Equal(t, "new", pdm.scanProgress.Stage)
	assert.NotEqual(t, oldStartTime, pdm.scanProgress.StartTime)
	require.NotNil(t, pdm.scanProgress.BatchInfo)
	assert.Equal(t, 1, pdm.scanProgress.BatchInfo.CurrentBatch)
	assert.Equal(t, 10, pdm.scanProgress.BatchInfo.TotalBatches)
}

func TestProgressDisplayManager_GetMonitorProgress(t *testing.T) {
	pdm := NewProgressDisplayManager(zerolog.Nop(), nil)
	pdm.UpdateMonitorProgress(5, 10, "checking", "")

	progressCopy := pdm.GetMonitorProgress()
	require.NotNil(t, progressCopy)
	assert.Equal(t, int64(5), progressCopy.Current)
	assert.Equal(t, int64(10), progressCopy.Total)

	// Modify the copy and check that the original is not affected
	progressCopy.Current = 99
	originalProgress := pdm.monitorProgress
	assert.Equal(t, int64(5), originalProgress.Current)
} 