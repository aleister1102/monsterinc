package progress

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

// displayProgress hiển thị progress hiện tại dưới dạng log thông thường
func (pdm *ProgressDisplayManager) displayProgress() {
	pdm.mutex.RLock()
	defer pdm.mutex.RUnlock()

	var output string

	// Scan progress - chỉ hiển thị khi có progress thực sự (current > 0 hoặc có batch info)
	if pdm.scanProgress.Status != ProgressStatusIdle && (pdm.scanProgress.Current > 0 || pdm.scanProgress.BatchInfo != nil) {
		percentage := pdm.scanProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.scanProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 20)

		if pdm.scanProgress.BatchInfo != nil {
			displayBatch := pdm.scanProgress.BatchInfo.CurrentBatch
			if displayBatch == 0 {
				displayBatch = 1
			}

			// Hiển thị với thông tin URL nếu có
			if pdm.scanProgress.BatchInfo.TotalURLs > 0 {
				output = fmt.Sprintf("🔍 Scan [Batch %d/%d]: %s %s %.1f%% (%d/%d) | URLs: %d/%d",
					displayBatch,
					pdm.scanProgress.BatchInfo.TotalBatches,
					icon,
					progressBar,
					percentage,
					pdm.scanProgress.Current,
					pdm.scanProgress.Total,
					pdm.scanProgress.BatchInfo.ProcessedURLs,
					pdm.scanProgress.BatchInfo.TotalURLs)
			} else {
				output = fmt.Sprintf("🔍 Scan [Batch %d/%d]: %s %s %.1f%% (%d/%d)",
					displayBatch,
					pdm.scanProgress.BatchInfo.TotalBatches,
					icon,
					progressBar,
					percentage,
					pdm.scanProgress.Current,
					pdm.scanProgress.Total)
			}
		} else {
			output = fmt.Sprintf("🔍 Scan: %s %s %.1f%% (%d/%d)",
				icon,
				progressBar,
				percentage,
				pdm.scanProgress.Current,
				pdm.scanProgress.Total)
		}

		if pdm.scanProgress.Stage != "" {
			output += fmt.Sprintf(" | %s", pdm.scanProgress.Stage)
		}

		if pdm.config.ShowETAEstimation && pdm.scanProgress.EstimatedETA > 0 && pdm.scanProgress.Status == ProgressStatusRunning {
			output += fmt.Sprintf(" | ETA: %s", pdm.formatDuration(pdm.scanProgress.EstimatedETA))
		}

		if pdm.scanProgress.Message != "" {
			output += fmt.Sprintf(" | %s", pdm.scanProgress.Message)
		}
	}

	// Monitor progress
	if pdm.monitorProgress.Status != ProgressStatusIdle {
		if output != "" {
			output += " | "
		}

		percentage := pdm.monitorProgress.GetPercentage()
		icon := pdm.getStatusIcon(pdm.monitorProgress.Status)
		progressBar := pdm.createProgressBar(percentage, 15)

		// Hiển thị với thông tin batch nếu có
		if pdm.monitorProgress.BatchInfo != nil && pdm.monitorProgress.BatchInfo.TotalBatches > 1 {
			output += fmt.Sprintf("👁 Monitor [Batch %d/%d]: %s %s %.1f%% (%d/%d)",
				pdm.monitorProgress.BatchInfo.CurrentBatch,
				pdm.monitorProgress.BatchInfo.TotalBatches,
				icon,
				progressBar,
				percentage,
				pdm.monitorProgress.Current,
				pdm.monitorProgress.Total)
		} else {
			output += fmt.Sprintf("👁 Monitor: %s %s %.1f%% (%d/%d)",
				icon,
				progressBar,
				percentage,
				pdm.monitorProgress.Current,
				pdm.monitorProgress.Total)
		}

		if pdm.monitorProgress.MonitorInfo != nil {
			output += fmt.Sprintf(" | P:%d F:%d C:%d",
				pdm.monitorProgress.MonitorInfo.ProcessedURLs,
				pdm.monitorProgress.MonitorInfo.FailedURLs,
				pdm.monitorProgress.MonitorInfo.CompletedURLs)
		}

		if pdm.monitorProgress.Message != "" {
			output += fmt.Sprintf(" | %s", pdm.monitorProgress.Message)
		}
	}

	// Chỉ hiển thị nếu có nội dung và nội dung đã thay đổi
	if output != "" && output != pdm.lastDisplayed {
		pdm.logProgressAsInfoMessage(output)
		pdm.lastDisplayed = output
	}
}

// logProgressAsInfoMessage log progress như một info message thông thường
func (pdm *ProgressDisplayManager) logProgressAsInfoMessage(content string) {
	pdm.logger.Info().Msg(content)
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
