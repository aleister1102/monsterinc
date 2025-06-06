package differ

import (
	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffProcessor handles the core diffing logic
type DiffProcessor struct {
	dmp    *diffmatchpatch.DiffMatchPatch
	config DiffConfig
}

// NewDiffProcessor creates a new diff processor
func NewDiffProcessor(config DiffConfig) *DiffProcessor {
	return &DiffProcessor{
		dmp:    diffmatchpatch.New(),
		config: config,
	}
}

// ProcessDiff generates diff between two content strings
func (dp *DiffProcessor) ProcessDiff(text1, text2 string) []diffmatchpatch.Diff {
	diffs := dp.dmp.DiffMain(text1, text2, dp.config.EnableLineBasedDiff)

	if dp.config.EnableSemanticCleanup {
		diffs = dp.dmp.DiffCleanupSemantic(diffs)
	}

	return diffs
}

// DiffStatistics holds diff calculation results
type DiffStatistics struct {
	LinesAdded   int
	LinesDeleted int
	LinesChanged int
	IsIdentical  bool
}

// DiffStatsCalculator calculates statistics from diff results
type DiffStatsCalculator struct{}

// NewDiffStatsCalculator creates a new diff stats calculator
func NewDiffStatsCalculator() *DiffStatsCalculator {
	return &DiffStatsCalculator{}
}

// CalculateStats computes statistics from diff results
func (dsc *DiffStatsCalculator) CalculateStats(diffs []diffmatchpatch.Diff, oldHash, newHash string) DiffStatistics {
	stats := DiffStatistics{}

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			stats.LinesAdded++
		case diffmatchpatch.DiffDelete:
			stats.LinesDeleted++
		}
	}

	stats.IsIdentical = dsc.isContentIdentical(diffs, oldHash, newHash)
	return stats
}

// isContentIdentical checks if content is identical
func (dsc *DiffStatsCalculator) isContentIdentical(diffs []diffmatchpatch.Diff, oldHash, newHash string) bool {
	if oldHash != "" && newHash != "" && oldHash != newHash {
		return false
	}

	if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		return true
	}

	for _, diff := range diffs {
		if diff.Type != diffmatchpatch.DiffEqual {
			return false
		}
	}

	return true
}
