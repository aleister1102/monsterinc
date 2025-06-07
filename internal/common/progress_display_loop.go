package common

import (
	"fmt"
	"strings"
	"time"
)

// displayLoop vòng lặp hiển thị progress
func (pdm *ProgressDisplayManager) displayLoop() {
	for {
		select {
		case <-pdm.ctx.Done():
			return
		case <-pdm.stopChan:
			return
		case <-pdm.displayTicker.C:
			pdm.displayProgress()
		}
	}
}

// displayProgress hiển thị progress hiện tại
func (pdm *ProgressDisplayManager) displayProgress() {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	var output strings.Builder

	// Scan progress
	if pdm.scanProgress.Status != ProgressStatusIdle {
		percentage := pdm.scanProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.scanProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 20)

		if pdm.scanProgress.BatchInfo != nil {
			output.WriteString(fmt.Sprintf("🔍 Scan [Batch %d/%d]: %s %s %.1f%% (%d/%d)",
				pdm.scanProgress.BatchInfo.CurrentBatch,
				pdm.scanProgress.BatchInfo.TotalBatches,
				icon,
				progressBar,
				percentage,
				pdm.scanProgress.Current,
				pdm.scanProgress.Total))
		} else {
			output.WriteString(fmt.Sprintf("🔍 Scan: %s %s %.1f%% (%d/%d)",
				icon,
				progressBar,
				percentage,
				pdm.scanProgress.Current,
				pdm.scanProgress.Total))
		}

		if pdm.scanProgress.Stage != "" {
			output.WriteString(fmt.Sprintf(" | %s", pdm.scanProgress.Stage))
		}

		if pdm.scanProgress.EstimatedETA > 0 && pdm.scanProgress.Status == ProgressStatusRunning {
			output.WriteString(fmt.Sprintf(" | ETA: %s", pdm.formatDuration(pdm.scanProgress.EstimatedETA)))
		}

		if pdm.scanProgress.Message != "" {
			output.WriteString(fmt.Sprintf(" | %s", pdm.scanProgress.Message))
		}
	}

	// Monitor progress
	if pdm.monitorProgress.Status != ProgressStatusIdle {
		if output.Len() > 0 {
			output.WriteString(" | ")
		}

		percentage := pdm.monitorProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.monitorProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 15) // Shorter bar for monitor

		output.WriteString(fmt.Sprintf("👁 Monitor: %s %s %.1f%% (%d/%d)",
			icon,
			progressBar,
			percentage,
			pdm.monitorProgress.Current,
			pdm.monitorProgress.Total))

		if pdm.monitorProgress.MonitorInfo != nil {
			output.WriteString(fmt.Sprintf(" | P:%d F:%d C:%d",
				pdm.monitorProgress.MonitorInfo.ProcessedURLs,
				pdm.monitorProgress.MonitorInfo.FailedURLs,
				pdm.monitorProgress.MonitorInfo.CompletedURLs))
		}

		if pdm.monitorProgress.Message != "" {
			output.WriteString(fmt.Sprintf(" | %s", pdm.monitorProgress.Message))
		}
	}

	// Only display if content has changed
	currentOutput := output.String()
	if currentOutput != "" && currentOutput != pdm.lastDisplayed {
		// Clear current line and print new progress
		fmt.Printf("\r\033[K%s", currentOutput)
		pdm.lastDisplayed = currentOutput
	}
}

// getStatusIcon trả về icon tương ứng với status
func (pdm *ProgressDisplayManager) getStatusIcon(status ProgressStatus) string {
	switch status {
	case ProgressStatusRunning:
		return "⏳"
	case ProgressStatusComplete:
		return "✅"
	case ProgressStatusError:
		return "❌"
	case ProgressStatusCancelled:
		return "🚫"
	case ProgressStatusIdle:
		return "💤"
	default:
		return "❓"
	}
}

// createProgressBar tạo thanh progress bar
func (pdm *ProgressDisplayManager) createProgressBar(percentage float64, width int) string {
	if width <= 0 {
		return ""
	}

	filled := int((percentage / 100.0) * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s]", bar)
}

// formatDuration định dạng duration thành chuỗi readable
func (pdm *ProgressDisplayManager) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}
