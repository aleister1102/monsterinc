package differ

import (
	"github.com/aleister1102/monsterinc/internal/models"
)

// URLDifferConfig holds configuration for URL comparison
type URLDifferConfig struct {
	EnableURLNormalization bool
	CaseSensitive          bool
}

// DefaultURLDifferConfig returns default configuration
func DefaultURLDifferConfig() URLDifferConfig {
	return URLDifferConfig{
		EnableURLNormalization: false,
		CaseSensitive:          true,
	}
}

// URLMaps holds the mapping data for URL comparison
type URLMaps struct {
	HistoricalURLMap map[string]models.ProbeResult
	CurrentURLMap    map[string]models.ProbeResult
}

// URLStatusCounts holds the counts for different URL statuses
type URLStatusCounts struct {
	New      int
	Existing int
	Old      int
}
