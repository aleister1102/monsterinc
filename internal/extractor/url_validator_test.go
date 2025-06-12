package extractor

import (
	"net/url"
	"testing"

	"github.com/rs/zerolog"
)

func TestURLValidator_ValidateAndResolveURL(t *testing.T) {
	validator := NewURLValidator(zerolog.Nop())

	base, _ := url.Parse("https://example.com")
	result := validator.ValidateAndResolveURL("/path", base, "https://example.com/page")

	if !result.IsValid {
		t.Fatalf("expected URL to be valid, got error: %v", result.Error)
	}

	expected := "https://example.com/path"
	if result.AbsoluteURL != expected {
		t.Errorf("expected resolved URL %s, got %s", expected, result.AbsoluteURL)
	}
}
