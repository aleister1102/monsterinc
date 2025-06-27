package progress

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgress_New(t *testing.T) {
	p := NewProgress(ProgressTypeScan)
	require.NotNil(t, p)
	info := p.Info()
	assert.Equal(t, ProgressTypeScan, info.Type)
	assert.Equal(t, ProgressStatusIdle, info.Status)
}

func TestProgress_Update(t *testing.T) {
	p := NewProgress(ProgressTypeScan)
	p.Update(10, 100, "testing", "message")

	info := p.Info()
	assert.Equal(t, int64(10), info.Current)
	assert.Equal(t, int64(100), info.Total)
	assert.Equal(t, "testing", info.Stage)
	assert.Equal(t, "message", info.Message)
	assert.Equal(t, ProgressStatusRunning, info.Status)
	assert.NotZero(t, info.StartTime)
	assert.NotZero(t, info.LastUpdateTime)
}

func TestProgress_SetStatus(t *testing.T) {
	p := NewProgress(ProgressTypeMonitor)
	p.SetStatus(ProgressStatusComplete, "done")

	info := p.Info()
	assert.Equal(t, ProgressStatusComplete, info.Status)
	assert.Equal(t, "done", info.Message)
}

func TestProgress_UpdateBatch(t *testing.T) {
	p := NewProgress(ProgressTypeScan)
	p.UpdateBatch(2, 5)

	info := p.Info()
	require.NotNil(t, info.BatchInfo)
	assert.Equal(t, 2, info.BatchInfo.CurrentBatch)
	assert.Equal(t, 5, info.BatchInfo.TotalBatches)
}

func TestProgress_ResetBatch(t *testing.T) {
	p := NewProgress(ProgressTypeScan)
	p.Update(50, 100, "old", "old")
	time.Sleep(10 * time.Millisecond)
	oldStartTime := p.Info().StartTime

	p.ResetBatch(1, 10, "new", "new")

	info := p.Info()
	assert.Equal(t, int64(0), info.Current)
	assert.Equal(t, int64(10), info.Total)
	assert.Equal(t, "new", info.Stage)
	assert.NotEqual(t, oldStartTime, info.StartTime)
	require.NotNil(t, info.BatchInfo)
	assert.Equal(t, 1, info.BatchInfo.CurrentBatch)
	assert.Equal(t, 10, info.BatchInfo.TotalBatches)
}

func TestProgress_UpdateWorkflow(t *testing.T) {
	p := NewProgress(ProgressTypeScan)
	p.UpdateBatch(1, 2)
	p.UpdateWorkflow(5, 10, "workflow stage", "workflow message")

	info := p.Info()
	// In batch mode, current/total should not be updated by workflow
	assert.Zero(t, info.Current)
	assert.Equal(t, "workflow stage", info.Stage)
	assert.Equal(t, "workflow message", info.Message)

	// Test non-batch mode
	p = NewProgress(ProgressTypeScan)
	p.UpdateWorkflow(5, 10, "workflow stage", "workflow message")
	info = p.Info()
	assert.Equal(t, int64(5), info.Current)
	assert.Equal(t, int64(10), info.Total)
}

func TestProgressInfo_GetPercentage(t *testing.T) {
	testCases := []struct {
		name     string
		current  int64
		total    int64
		expected float64
	}{
		{"zero total", 50, 0, 0.0},
		{"zero current", 0, 100, 0.0},
		{"normal", 25, 100, 25.0},
		{"halfway", 50, 100, 50.0},
		{"full", 100, 100, 100.0},
		{"over", 150, 100, 100.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pi := &ProgressInfo{Current: tc.current, Total: tc.total}
			assert.InDelta(t, tc.expected, pi.GetPercentage(), 0.001)
		})
	}
}

func TestProgressInfo_UpdateETA(t *testing.T) {
	// Need to control time for reliable ETA tests
	now := time.Now()
	startTime := now.Add(-10 * time.Second) // 10 seconds ago

	// Calculate expected duration for "almost done" case explicitly
	// to avoid untyped float constant conversion issues.
	rateAlmostDone := 99.0 / 10.0 // items/sec
	etaSecondsAlmostDone := 1.0 / rateAlmostDone
	expectedDurationAlmostDone := time.Duration(etaSecondsAlmostDone * float64(time.Second))

	testCases := []struct {
		name     string
		info     ProgressInfo
		expected time.Duration
	}{
		{
			name:     "not running",
			info:     ProgressInfo{Status: ProgressStatusIdle, Current: 10, Total: 100, StartTime: startTime},
			expected: 0,
		},
		{
			name:     "zero total",
			info:     ProgressInfo{Status: ProgressStatusRunning, Current: 10, Total: 0, StartTime: startTime},
			expected: 0,
		},
		{
			name:     "zero current",
			info:     ProgressInfo{Status: ProgressStatusRunning, Current: 0, Total: 100, StartTime: startTime},
			expected: 0,
		},
		{
			name:     "no time elapsed",
			info:     ProgressInfo{Status: ProgressStatusRunning, Current: 1, Total: 100, StartTime: time.Now()},
			expected: 0,
		},
		{
			name:     "halfway done",
			info:     ProgressInfo{Status: ProgressStatusRunning, Current: 50, Total: 100, StartTime: startTime},
			expected: 10 * time.Second, // 50 items in 10s -> 5 items/s -> 50 remaining -> 10s
		},
		{
			name:     "almost done",
			info:     ProgressInfo{Status: ProgressStatusRunning, Current: 99, Total: 100, StartTime: startTime},
			expected: expectedDurationAlmostDone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Override start time for test stability if needed
			if !tc.info.StartTime.IsZero() {
				// To simulate time.Since(pi.StartTime)
				// we need to set a static start time and test against it.
				// This is tricky without mocking time. The calculation is what matters.
			}
			tc.info.UpdateETA()
			// Use a small delta for duration comparisons due to float math
			assert.InDelta(t, tc.expected.Seconds(), tc.info.EstimatedETA.Seconds(), 0.1)
		})
	}
}

func TestProgress_Update_EdgeCases(t *testing.T) {
	t.Run("first update", func(t *testing.T) {
		p := NewProgress(ProgressTypeScan)
		assert.Equal(t, ProgressStatusIdle, p.Info().Status)
		p.Update(1, 10, "", "")
		info := p.Info()
		assert.Equal(t, ProgressStatusRunning, info.Status)
		assert.NotZero(t, info.StartTime)
	})

	t.Run("update with zero total", func(t *testing.T) {
		p := NewProgress(ProgressTypeScan)
		p.Update(10, 0, "stage", "")
		info := p.Info()
		assert.Equal(t, int64(10), info.Current)
		assert.Equal(t, int64(0), info.Total)
		info.UpdateETA()
		assert.Zero(t, info.EstimatedETA)
	})

	t.Run("update resets start time for new batch", func(t *testing.T) {
		p := NewProgress(ProgressTypeScan)
		p.UpdateBatch(1, 2)
		p.Update(0, 100, "stage1", "") // initial state for batch
		firstStartTime := p.Info().StartTime
		time.Sleep(2 * time.Millisecond)
		// Simulating processing within the same batch
		p.Update(50, 100, "stage1", "")
		assert.Equal(t, firstStartTime, p.Info().StartTime)
		time.Sleep(2 * time.Millisecond)

		// Simulating start of a new batch by resetting current to 0
		p.Update(0, 50, "stage2", "")
		p.Update(1, 50, "stage2", "")
		assert.NotEqual(t, firstStartTime, p.Info().StartTime)
	})
}

func TestProgress_Initialization(t *testing.T) {
	t.Run("UpdateBatchWithURLs initializes correctly", func(t *testing.T) {
		p := NewProgress(ProgressTypeScan)
		require.Nil(t, p.Info().BatchInfo)
		p.UpdateBatchWithURLs(1, 10, 5, 50, 25)
		info := p.Info()
		require.NotNil(t, info.BatchInfo)
		assert.Equal(t, 1, info.BatchInfo.CurrentBatch)
		assert.Equal(t, 10, info.BatchInfo.TotalBatches)
		assert.Equal(t, 5, info.BatchInfo.CurrentBatchURLs)
		assert.Equal(t, 50, info.BatchInfo.TotalURLs)
		assert.Equal(t, 25, info.BatchInfo.ProcessedURLs)
	})

	t.Run("UpdateMonitorStats initializes correctly", func(t *testing.T) {
		p := NewProgress(ProgressTypeMonitor)
		require.Nil(t, p.Info().MonitorInfo)
		p.UpdateMonitorStats(10, 2, 8)
		info := p.Info()
		require.NotNil(t, info.MonitorInfo)
		assert.Equal(t, 10, info.MonitorInfo.ProcessedURLs)
		assert.Equal(t, 2, info.MonitorInfo.FailedURLs)
		assert.Equal(t, 8, info.MonitorInfo.CompletedURLs)
	})
}
