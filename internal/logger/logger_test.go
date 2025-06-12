package logger

import (
	"testing"

	"github.com/aleister1102/monsterinc/internal/config"
)

func TestNew_DefaultLogger(t *testing.T) {
	cfg := config.NewDefaultLogConfig()
	log, err := New(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_ = log // ensure variable is used
}
