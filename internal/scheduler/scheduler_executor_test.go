package scheduler

import (
	"testing"
	"time"

	"github.com/aleister1102/monsterinc/internal/config"
)

func TestScheduler_CalculateNextScanTime(t *testing.T) {
	s := &Scheduler{
		globalConfig: &config.GlobalConfig{
			SchedulerConfig: config.SchedulerConfig{CycleMinutes: 10},
		},
	}

	next, err := s.calculateNextScanTime()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if next.Before(time.Now()) {
		t.Errorf("expected next scan time in the future, got %v", next)
	}
}
