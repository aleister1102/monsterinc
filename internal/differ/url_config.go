package differ

import (
	"github.com/aleister1102/monsterinc/internal/models"
)

// URLComparerConfig holds configuration for URL comparison
type URLComparerConfig struct {
	EnableURLNormalization bool
	CaseSensitive          bool
}

// DefaultURLComparerConfig returns default configuration
func DefaultURLComparerConfig() URLComparerConfig {
	return URLComparerConfig{
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
