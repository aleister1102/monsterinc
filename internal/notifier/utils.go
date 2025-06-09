package notifier

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aleister1102/monsterinc/internal/models"
)

// buildMentions creates mention strings for Discord role IDs
func buildMentions(roleIDs []string) string {
	if len(roleIDs) == 0 {
		return ""
	}
	var mentions []string
	for _, roleID := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", roleID))
	}
	return strings.Join(mentions, " ")
}

// truncateString truncates a string to maxLength with ellipsis
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}

// compressErrorMessage rút gọn error message bằng cách loại bỏ thông tin không cần thiết
func compressErrorMessage(errorMsg string) string {
	// Loại bỏ stack traces dài
	lines := strings.Split(errorMsg, "\n")
	var compressedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Loại bỏ stack trace lines (thường bắt đầu bằng whitespace + at hoặc \t)
		if strings.HasPrefix(line, "\tat ") || strings.HasPrefix(line, "    at ") {
			continue
		}

		// Loại bỏ file paths dài, chỉ giữ tên file
		re := regexp.MustCompile(`([a-zA-Z0-9_-]+/)*([\w-]+\.(go|js|html|css|json))`)
		line = re.ReplaceAllString(line, "$2")

		// Rút gọn URL dài
		urlRe := regexp.MustCompile(`https?://[^\s]{50,}`)
		line = urlRe.ReplaceAllStringFunc(line, func(url string) string {
			if len(url) > 60 {
				return url[:30] + "..." + url[len(url)-20:]
			}
			return url
		})

		// Giới hạn độ dài của mỗi line
		if len(line) > MaxSingleErrorLength {
			line = truncateString(line, MaxSingleErrorLength)
		}

		compressedLines = append(compressedLines, line)

		// Giới hạn số lượng dòng
		if len(compressedLines) >= 8 {
			break
		}
	}

	result := strings.Join(compressedLines, "\n")
	return strings.TrimSpace(result)
}

// compressMultipleErrors rút gọn multiple error messages
func compressMultipleErrors(errorMessages []string, maxLength int) string {
	if len(errorMessages) == 0 {
		return ""
	}

	var compressedErrors []string
	totalLength := 0

	for i, errMsg := range errorMessages {
		compressed := compressErrorMessage(errMsg)

		// Thêm prefix số thứ tự nếu có nhiều errors
		if len(errorMessages) > 1 {
			compressed = fmt.Sprintf("%d. %s", i+1, compressed)
		}

		// Kiểm tra độ dài tổng
		if totalLength+len(compressed) > maxLength {
			if len(compressedErrors) == 0 {
				// Nếu error đầu tiên đã quá dài, cắt nó
				compressed = truncateString(compressed, maxLength-50)
				compressedErrors = append(compressedErrors, compressed)
			}

			// Thêm thông báo có thêm errors
			remaining := len(errorMessages) - len(compressedErrors)
			if remaining > 0 {
				suffix := fmt.Sprintf("\n... and %d more errors", remaining)
				if totalLength+len(suffix) <= maxLength {
					compressedErrors = append(compressedErrors, suffix)
				}
			}
			break
		}

		compressedErrors = append(compressedErrors, compressed)
		totalLength += len(compressed) + 1 // +1 for newline
	}

	return strings.Join(compressedErrors, "\n")
}

// formatDuration formats duration truncated to seconds
func formatDuration(d time.Duration) string {
	return d.Truncate(time.Second).String()
}

// addErrorSamplesField adds error samples field to embed
func addErrorSamplesField(embedBuilder *DiscordEmbedBuilder, errors []models.MonitorFetchErrorInfo) {
	if len(errors) == 0 {
		return
	}

	var errorTexts []string
	for i, errorInfo := range errors {
		if i >= MaxErrorSampleCount {
			break
		}
		errorText := fmt.Sprintf("%s: ```%s```", errorInfo.URL, compressErrorMessage(errorInfo.Error))
		errorTexts = append(errorTexts, errorText)
	}

	fieldValue := strings.Join(errorTexts, "\n\n")
	if len(errorTexts) < len(errors) {
		fieldValue += fmt.Sprintf("\n\n... and %d more errors", len(errors)-len(errorTexts))
	}

	// Truncate nếu quá dài
	if len(fieldValue) > 900 {
		fieldValue = truncateString(fieldValue, 900)
	}

	embedBuilder.AddField("⚠️ Error", fieldValue, false)
}
