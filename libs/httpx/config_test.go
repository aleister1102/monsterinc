package httpx

import "testing"

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Threads <= 0 {
		t.Errorf("expected default Threads > 0, got %d", cfg.Threads)
	}
	if cfg.Timeout <= 0 {
		t.Errorf("expected default Timeout > 0, got %d", cfg.Timeout)
	}
}
