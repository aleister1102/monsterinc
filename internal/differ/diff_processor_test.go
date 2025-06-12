package differ

import "testing"

func TestDiffProcessor_ProcessDiff(t *testing.T) {
	dp := NewDiffProcessor(DefaultDiffConfig())

	left := "hello world"
	right := "hello gopher"

	diffs := dp.ProcessDiff(left, right)
	if len(diffs) == 0 {
		t.Errorf("expected non-empty diffs")
	}
}
