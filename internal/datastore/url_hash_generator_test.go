package datastore

import "testing"

func TestURLHashGenerator_GenerateHash(t *testing.T) {
	gen := NewURLHashGenerator(8)
	h1 := gen.GenerateHash("https://example.com")
	h2 := gen.GenerateHash("https://example.com")

	if len(h1) != 8 {
		t.Fatalf("expected hash length 8, got %d", len(h1))
	}
	if h1 != h2 {
		t.Errorf("expected deterministic hash, got %s and %s", h1, h2)
	}
}
