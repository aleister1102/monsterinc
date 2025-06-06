package reporter

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/aleister1102/monsterinc/internal/models"
)

// DiffUtils contains utility functions for diff processing
type DiffUtils struct{}

// NewDiffUtils creates a new DiffUtils
func NewDiffUtils() *DiffUtils {
	return &DiffUtils{}
}

// GenerateDiffHTML creates HTML representation of diffs
func (du *DiffUtils) GenerateDiffHTML(diffs []models.ContentDiff) template.HTML {
	var htmlBuilder strings.Builder
	for _, d := range diffs {
		// Escape HTML characters to prevent XSS and rendering issues
		escapedText := template.HTMLEscapeString(d.Text)

		switch d.Operation {
		case models.DiffInsert:
			htmlBuilder.WriteString(fmt.Sprintf(`<ins style="background:#e6ffe6; text-decoration: none;">%s</ins>`, escapedText))
		case models.DiffDelete:
			htmlBuilder.WriteString(fmt.Sprintf(`<del style="background:#f8d7da; text-decoration: none;">%s</del>`, escapedText))
		case models.DiffEqual:
			htmlBuilder.WriteString(escapedText)
		}
	}
	return template.HTML(htmlBuilder.String())
}

// CreateDiffSummary creates text summary of diff
func (du *DiffUtils) CreateDiffSummary(diffs []models.ContentDiff) string {
	insertions := 0
	deletions := 0
	for _, d := range diffs {
		switch d.Operation {
		case models.DiffInsert:
			insertions++
		case models.DiffDelete:
			deletions++
		}
	}
	if insertions == 0 && deletions == 0 {
		return "No textual changes detected."
	}
	return fmt.Sprintf("%d insertions (+), %d deletions (-).", insertions, deletions)
}

// TruncateHash truncates hash for shorter display
func (du *DiffUtils) TruncateHash(hash string) string {
	if len(hash) <= HashLength {
		return hash
	}
	return hash[:HashLength]
}

// MinInt returns the smaller of two integers
func (du *DiffUtils) MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MinInt global function for convenience
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
