package differ

// DiffConfig holds configuration for content diffing
type DiffConfig struct {
	EnableSemanticCleanup bool
	EnableLineBasedDiff   bool
	ContextLines          int
}

// DefaultDiffConfig returns default configuration
func DefaultDiffConfig() DiffConfig {
	return DiffConfig{
		EnableSemanticCleanup: true,
		EnableLineBasedDiff:   true,
		ContextLines:          3,
	}
}
