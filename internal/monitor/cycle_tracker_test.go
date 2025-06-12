package monitor

import "testing"

func TestCycleTracker_Basic(t *testing.T) {
	ct := NewCycleTracker("cycle-1")
	if ct.GetCurrentCycleID() != "cycle-1" {
		t.Fatalf("expected cycle ID cycle-1, got %s", ct.GetCurrentCycleID())
	}

	ct.AddChangedURL("https://example.com")

	if !ct.HasChanges() {
		t.Fatalf("expected HasChanges true")
	}
	if ct.GetChangeCount() != 1 {
		t.Fatalf("expected change count 1, got %d", ct.GetChangeCount())
	}

	urls := ct.GetChangedURLs()
	if len(urls) != 1 || urls[0] != "https://example.com" {
		t.Errorf("unexpected changed URLs: %v", urls)
	}

	ct.ClearChangedURLs()
	if ct.HasChanges() {
		t.Errorf("expected no changes after ClearChangedURLs")
	}
}
