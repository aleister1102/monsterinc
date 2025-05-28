package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

// ServiceMonitor handles MonsterInc service monitoring and control
type ServiceMonitor struct {
	config *Config
}

// ServiceStatus represents the current status of MonsterInc service
type ServiceStatus struct {
	IsRunning bool
	PID       int
	Uptime    time.Duration
	Memory    string
}

// NewServiceMonitor creates a new service monitor instance
func NewServiceMonitor(config *Config) *ServiceMonitor {
	return &ServiceMonitor{
		config: config,
	}
}

// CheckServiceStatus checks if MonsterInc service is running
func (sm *ServiceMonitor) CheckServiceStatus() (*ServiceStatus, error) {
	status := &ServiceStatus{}

	// Find MonsterInc process
	pid, err := sm.findMonsterIncProcess()
	if err != nil {
		return status, fmt.Errorf("failed to find process: %v", err)
	}

	if pid == 0 {
		status.IsRunning = false
		return status, nil
	}

	status.IsRunning = true
	status.PID = pid

	// Get process info if running
	if runtime.GOOS == "windows" {
		// Windows process info
		uptime, memory, err := sm.getWindowsProcessInfo(pid)
		if err != nil {
			log.Warn().Err(err).Int("pid", pid).Msg("Failed to get process info")
		} else {
			status.Uptime = uptime
			status.Memory = memory
		}
	} else {
		// Unix process info
		uptime, memory, err := sm.getUnixProcessInfo(pid)
		if err != nil {
			log.Warn().Err(err).Int("pid", pid).Msg("Failed to get process info")
		} else {
			status.Uptime = uptime
			status.Memory = memory
		}
	}

	return status, nil
}

// TriggerScan executes a one-time scan using MonsterInc
func (sm *ServiceMonitor) TriggerScan() error {
	// Build command
	monsterIncPath := sm.config.Paths.MonsterIncBin
	urlsFilePath := filepath.Join(sm.config.Paths.TargetsDir, sm.config.Paths.URLsFile)

	// Check if binary exists
	if _, err := os.Stat(monsterIncPath); os.IsNotExist(err) {
		return fmt.Errorf("MonsterInc binary not found at: %s", monsterIncPath)
	}

	// Check if urls file exists
	if _, err := os.Stat(urlsFilePath); os.IsNotExist(err) {
		return fmt.Errorf("URLs file not found at: %s", urlsFilePath)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sm.config.Service.ExecuteTimeout)
	defer cancel()

	// Build command arguments
	args := []string{
		"-mode", "onetime",
		"-uf", urlsFilePath,
	}

	// Execute command
	cmd := exec.CommandContext(ctx, monsterIncPath, args...)
	cmd.Dir = filepath.Dir(monsterIncPath)

	log.Info().
		Str("binary", monsterIncPath).
		Str("urls_file", urlsFilePath).
		Msg("Triggering MonsterInc one-time scan")

	// Start command in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MonsterInc: %v", err)
	}

	// Don't wait for completion, let it run in background
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Error().Err(err).Msg("MonsterInc scan completed with error")
		} else {
			log.Info().Msg("MonsterInc scan completed successfully")
		}
	}()

	return nil
}

// findMonsterIncProcess finds the PID of running MonsterInc process
func (sm *ServiceMonitor) findMonsterIncProcess() (int, error) {
	processName := sm.config.Service.ProcessName

	if runtime.GOOS == "windows" {
		return sm.findWindowsProcess(processName)
	}
	return sm.findUnixProcess(processName)
}

// findWindowsProcess finds process on Windows
func (sm *ServiceMonitor) findWindowsProcess(processName string) (int, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s.exe", processName), "/FO", "CSV", "/NH")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run tasklist: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, processName) {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				pidStr := strings.Trim(parts[1], "\"")
				pid, err := strconv.Atoi(pidStr)
				if err == nil {
					return pid, nil
				}
			}
		}
	}

	return 0, nil
}

// findUnixProcess finds process on Unix systems
func (sm *ServiceMonitor) findUnixProcess(processName string) (int, error) {
	cmd := exec.Command("pgrep", "-f", processName)
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to run pgrep: %v", err)
	}

	pidStr := strings.TrimSpace(string(output))
	if pidStr == "" {
		return 0, nil
	}

	// Get first PID if multiple found
	pids := strings.Split(pidStr, "\n")
	pid, err := strconv.Atoi(pids[0])
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %v", err)
	}

	return pid, nil
}

// getWindowsProcessInfo gets process info on Windows
func (sm *ServiceMonitor) getWindowsProcessInfo(pid int) (time.Duration, string, error) {
	// Get process info using wmic
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", pid), "get", "CreationDate,WorkingSetSize", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("failed to get process info: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, strconv.Itoa(pid)) {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				// Parse memory
				memoryStr := strings.TrimSpace(parts[2])

				// Calculate uptime (simplified)
				uptime := time.Hour // Default fallback

				// Format memory
				if memory, err := strconv.ParseInt(memoryStr, 10, 64); err == nil {
					memoryMB := memory / (1024 * 1024)
					return uptime, fmt.Sprintf("%d MB", memoryMB), nil
				}
			}
		}
	}

	return time.Hour, "Unknown", nil
}

// getUnixProcessInfo gets process info on Unix systems
func (sm *ServiceMonitor) getUnixProcessInfo(pid int) (time.Duration, string, error) {
	// Get process start time
	cmd := exec.Command("ps", "-o", "etime,rss", "-p", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return 0, "", fmt.Errorf("failed to get process info: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, "", fmt.Errorf("unexpected ps output")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return 0, "", fmt.Errorf("unexpected ps fields")
	}

	// Parse elapsed time (format: [[DD-]HH:]MM:SS)
	etimeStr := fields[0]
	uptime := sm.parseElapsedTime(etimeStr)

	// Parse memory (RSS in KB)
	rssStr := fields[1]
	if rss, err := strconv.ParseInt(rssStr, 10, 64); err == nil {
		memoryMB := rss / 1024
		return uptime, fmt.Sprintf("%d MB", memoryMB), nil
	}

	return uptime, "Unknown", nil
}

// parseElapsedTime parses ps etime format
func (sm *ServiceMonitor) parseElapsedTime(etime string) time.Duration {
	// Handle different formats: MM:SS, HH:MM:SS, DD-HH:MM:SS
	var duration time.Duration

	if strings.Contains(etime, "-") {
		// Format: DD-HH:MM:SS
		parts := strings.Split(etime, "-")
		if len(parts) == 2 {
			days, _ := strconv.Atoi(parts[0])
			duration += time.Duration(days) * 24 * time.Hour
			etime = parts[1]
		}
	}

	// Parse HH:MM:SS or MM:SS
	timeParts := strings.Split(etime, ":")
	switch len(timeParts) {
	case 3: // HH:MM:SS
		hours, _ := strconv.Atoi(timeParts[0])
		minutes, _ := strconv.Atoi(timeParts[1])
		seconds, _ := strconv.Atoi(timeParts[2])
		duration += time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	case 2: // MM:SS
		minutes, _ := strconv.Atoi(timeParts[0])
		seconds, _ := strconv.Atoi(timeParts[1])
		duration += time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	}

	return duration
}

// IsProcessRunning checks if a process with given PID is running
func (sm *ServiceMonitor) IsProcessRunning(pid int) bool {
	if runtime.GOOS == "windows" {
		// Windows: Use tasklist to check if PID exists
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		output, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(output), strconv.Itoa(pid))
	} else {
		// Unix: Send signal 0 to check if process exists
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}
