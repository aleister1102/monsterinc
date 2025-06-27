package rslimiter

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// ResourceUsage represents current system resource usage
type ResourceUsage struct {
	AllocMB              int64   // Currently allocated memory by application
	SysMB                int64   // System memory used by Go runtime
	Goroutines           int     // Number of goroutines
	GCCount              int64   // Number of GC cycles
	NextGCMB             int64   // Next GC target
	SystemMemUsedMB      int64   // System memory used (MB)
	SystemMemTotalMB     int64   // Total system memory (MB)
	SystemMemUsedPercent float64 // System memory used percentage
	CPUUsagePercent      float64 // CPU usage percentage
}

// GetResourceUsage returns current resource usage statistics
func GetResourceUsage() ResourceUsage {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	usage := ResourceUsage{
		AllocMB:    int64(m.Alloc / 1024 / 1024),
		SysMB:      int64(m.Sys / 1024 / 1024),
		Goroutines: runtime.NumGoroutine(),
		GCCount:    int64(m.NumGC),
		NextGCMB:   int64(m.NextGC / 1024 / 1024),
	}

	// Get system memory stats
	if vmStat, err := mem.VirtualMemory(); err == nil {
		usage.SystemMemUsedMB = int64(vmStat.Used / 1024 / 1024)
		usage.SystemMemTotalMB = int64(vmStat.Total / 1024 / 1024)
		usage.SystemMemUsedPercent = vmStat.UsedPercent
	}

	// Get CPU usage
	if cpuPercents, err := cpu.Percent(100*time.Millisecond, false); err == nil && len(cpuPercents) > 0 {
		usage.CPUUsagePercent = cpuPercents[0]
	}

	return usage
}
