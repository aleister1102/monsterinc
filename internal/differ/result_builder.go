package differ

// URLDiffResultBuilder builds URLDiffResult objects
type URLDiffResultBuilder struct {
	result URLDiffResult
}

// NewURLDiffResultBuilder creates a new result builder
func NewURLDiffResultBuilder(rootTarget string) *URLDiffResultBuilder {
	return &URLDiffResultBuilder{
		result: URLDiffResult{
			RootTargetURL: rootTarget,
			Results:       make([]DiffedURL, 0),
		},
	}
}

// WithError sets an error on the result
func (rb *URLDiffResultBuilder) WithError(err error) *URLDiffResultBuilder {
	rb.result.Error = err.Error()
	return rb
}

// WithResults sets the diff results and counts
func (rb *URLDiffResultBuilder) WithResults(results []DiffedURL, counts URLStatusCounts) *URLDiffResultBuilder {
	rb.result.Results = results
	rb.result.New = counts.New
	rb.result.Existing = counts.Existing
	rb.result.Old = counts.Old
	return rb
}

// AddResults adds additional results to the existing results
func (rb *URLDiffResultBuilder) AddResults(additionalResults []DiffedURL, additionalOldCount int) *URLDiffResultBuilder {
	rb.result.Results = append(rb.result.Results, additionalResults...)
	rb.result.Old += additionalOldCount
	return rb
}

// Build creates the final URLDiffResult
func (rb *URLDiffResultBuilder) Build() *URLDiffResult {
	return &rb.result
}
