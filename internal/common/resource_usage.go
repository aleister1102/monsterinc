package common

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
