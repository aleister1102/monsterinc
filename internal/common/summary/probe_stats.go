package summary

// ProbeStats holds statistics related to the probing phase of a scan.
type ProbeStats struct {
	TotalProbed       int // Total URLs sent to the prober
	SuccessfulProbes  int // Number of probes that returned a successful response (e.g., 2xx)
	FailedProbes      int // Number of probes that failed or returned error codes
	DiscoverableItems int // e.g. number of items from httpx
}

// ProbeStatsBuilder handles building probe stats
type ProbeStatsBuilder struct {
	stats ProbeStats
}

// NewProbeStatsBuilder creates a new probe stats builder
func NewProbeStatsBuilder() *ProbeStatsBuilder {
	return &ProbeStatsBuilder{
		stats: ProbeStats{},
	}
}

// WithTotalProbed sets total probed count
func (psb *ProbeStatsBuilder) WithTotalProbed(total int) *ProbeStatsBuilder {
	psb.stats.TotalProbed = total
	return psb
}

// WithSuccessfulProbes sets successful probes count
func (psb *ProbeStatsBuilder) WithSuccessfulProbes(successful int) *ProbeStatsBuilder {
	psb.stats.SuccessfulProbes = successful
	return psb
}

// WithFailedProbes sets failed probes count
func (psb *ProbeStatsBuilder) WithFailedProbes(failed int) *ProbeStatsBuilder {
	psb.stats.FailedProbes = failed
	return psb
}

// WithDiscoverableItems sets discoverable items count
func (psb *ProbeStatsBuilder) WithDiscoverableItems(items int) *ProbeStatsBuilder {
	psb.stats.DiscoverableItems = items
	return psb
}

// Build returns the constructed probe stats
func (psb *ProbeStatsBuilder) Build() ProbeStats {
	return psb.stats
}
