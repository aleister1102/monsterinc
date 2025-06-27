package reporter

import (
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
	"github.com/rs/zerolog"
)

// Since processProbeResults is a method on HtmlReporter, we need to instantiate it.
// We can use a minimal configuration for this test.
func newTestHtmlReporter(t *testing.T) *HtmlReporter {
	logger := zerolog.Nop()
	// Pass a pointer to the config
	cfg := config.NewDefaultReporterConfig()
	reporter, err := NewHtmlReporter(&cfg, logger)
	if err != nil {
		t.Fatalf("Failed to create HtmlReporter: %v", err)
	}
	return reporter
}
