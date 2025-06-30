package summary

// DiffStats holds statistics related to the diffing phase of a scan.
type DiffStats struct {
	New      int
	Old      int
	Existing int
	Changed  int // (If StatusChanged is implemented)
}

// DiffStatsBuilder handles building diff stats
type DiffStatsBuilder struct {
	stats DiffStats
}

// NewDiffStatsBuilder creates a new diff stats builder
func NewDiffStatsBuilder() *DiffStatsBuilder {
	return &DiffStatsBuilder{
		stats: DiffStats{},
	}
}

// WithNew sets new count
func (dsb *DiffStatsBuilder) WithNew(count int) *DiffStatsBuilder {
	dsb.stats.New = count
	return dsb
}

// WithOld sets old count
func (dsb *DiffStatsBuilder) WithOld(count int) *DiffStatsBuilder {
	dsb.stats.Old = count
	return dsb
}

// WithExisting sets existing count
func (dsb *DiffStatsBuilder) WithExisting(count int) *DiffStatsBuilder {
	dsb.stats.Existing = count
	return dsb
}

// WithChanged sets changed count
func (dsb *DiffStatsBuilder) WithChanged(count int) *DiffStatsBuilder {
	dsb.stats.Changed = count
	return dsb
}

// Build returns the constructed diff stats
func (dsb *DiffStatsBuilder) Build() DiffStats {
	return dsb.stats
}
