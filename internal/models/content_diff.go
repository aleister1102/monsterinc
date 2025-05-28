package models

// DiffOperation defines the type of change.
type DiffOperation int

const (
	// DiffEqual indicates an unchanged segment.
	DiffEqual DiffOperation = 0
	// DiffInsert indicates an inserted segment.
	DiffInsert DiffOperation = 1
	// DiffDelete indicates a deleted segment.
	DiffDelete DiffOperation = -1
)

// ContentDiff represents a single difference between two contents.
type ContentDiff struct {
	Operation DiffOperation `json:"operation"`
	Text      string        `json:"text"`
}

// ContentDiffResult holds the structured result of a content diff operation.
type ContentDiffResult struct {
	Timestamp        int64           `json:"timestamp"`
	ContentType      string          `json:"content_type"`
	Diffs            []ContentDiff   `json:"diffs"`
	LinesAdded       int             `json:"lines_added"`
	LinesDeleted     int             `json:"lines_deleted"`
	LinesChanged     int             `json:"lines_changed"` // Optional: if we can detect changed lines specifically
	IsIdentical      bool            `json:"is_identical"`
	ErrorMessage     string          `json:"error_message,omitempty"`
	ProcessingTimeMs int64           `json:"processing_time_ms"`
	OldHash          string          `json:"old_hash,omitempty"`
	NewHash          string          `json:"new_hash,omitempty"`
	ExtractedPaths   []ExtractedPath `json:"extracted_paths,omitempty"`
}

// DiffDisplay holds processed diffs for HTML template rendering.
