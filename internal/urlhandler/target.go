package urlhandler

// Target represents a URL to be scanned.
// It includes the original input URL and its normalized form.
// It can also store metadata about the target.
type Target struct {
	OriginalURL   string // The URL as provided by the user
	NormalizedURL string // The URL after normalization (e.g., adding scheme, lowercasing domain)
	// Add other metadata fields if needed, for example:
	// SourceFile string    // The file from which this target was loaded
	// AddedAt    time.Time // Timestamp when the target was added
}
