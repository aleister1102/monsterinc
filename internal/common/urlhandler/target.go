package urlhandler

// Target represents a URL to be scanned.
// It includes the original input URL and its normalized form.
// It can also store metadata about the target.
type Target struct {
	URL string // The URL as provided by the user
}
