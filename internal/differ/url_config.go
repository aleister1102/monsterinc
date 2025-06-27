package differ

import (
	httpx "github.com/aleister1102/go-telescope"
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
	HistoricalURLMap map[string]httpx.ProbeResult
	CurrentURLMap    map[string]httpx.ProbeResult
}

// URLStatusCounts holds the counts for different URL statuses
type URLStatusCounts struct {
	New      int
	Existing int
	Old      int
}
