# Scheduler Package

The scheduler package provides automated task scheduling and execution for MonsterInc security scanner. It manages periodic scans, coordinates scan and monitoring workflows, and maintains persistent scheduling state using SQLite database.

## Overview

The scheduler package enables:
- **Automated Scanning**: Schedule periodic security scans with configurable intervals
- **Task Persistence**: SQLite-based storage for reliable task management
- **Worker Coordination**: Concurrent execution of scanning and monitoring tasks
- **Retry Logic**: Automatic retry mechanisms for failed operations
- **State Management**: Track task execution history and manage schedules

## File Structure

### Core Components

- **`scheduler.go`** - Main scheduler service and orchestration
- **`db.go`** - SQLite database management and operations
- **`scan_executor.go`** - Automated scan execution and coordination
- **`monitor_workers.go`** - Monitoring task workers
- **`helpers.go`** - Utility functions and common operations

## Features

### 1. Automated Task Scheduling

**Capabilities:**
- Configurable scan intervals (minutes/hours/days)
- Persistent scheduling across application restarts
- Task priority management
- Concurrent task execution
- Flexible scheduling patterns

### 2. Database-Backed Persistence

**Features:**
- SQLite database for reliable storage
- Task execution history tracking
- Schedule state management
- Atomic operations with transactions
- Database schema migrations

### 3. Worker Management

**Coordination:**
- Separate workers for scan and monitor tasks
- Configurable worker pools
- Resource allocation management
- Graceful shutdown handling
- Load balancing across workers

## Usage Examples

### Basic Scheduler Setup

```go
import (
    "github.com/aleister1102/monsterinc/internal/scheduler"
    "github.com/aleister1102/monsterinc/internal/config"
)

// Initialize scheduler
schedulerService, err := scheduler.NewScheduler(
    globalConfig.SchedulerConfig,
    logger,
    scannerService,
    monitoringService,
    notificationHelper,
)
if err != nil {
    return fmt.Errorf("scheduler init failed: %w", err)
}

// Start scheduler
ctx := context.Background()
err = schedulerService.Start(ctx)
if err != nil {
    return fmt.Errorf("scheduler start failed: %w", err)
}

// Schedule a recurring scan
err = schedulerService.ScheduleScan(
    "daily-security-scan",
    "targets.txt",
    24*time.Hour, // Run every 24 hours
    scheduler.ScanOptions{
        EnableCrawling: true,
        EnableDiffing:  true,
    },
)
```

### Advanced Scheduling Configuration

```go
// Configure scheduler with custom settings
schedulerConfig := config.SchedulerConfig{
    CycleMinutes:  30,           // Check for tasks every 30 minutes
    RetryAttempts: 3,            // Retry failed tasks up to 3 times
    SQLiteDBPath:  "./scheduler.db", // Database path
}

// Create scheduler with custom worker pools
scheduler := scheduler.NewScheduler(schedulerConfig, logger, scanner, monitor, notifier)

// Schedule multiple scan types
err = scheduler.ScheduleMultipleTasks([]scheduler.TaskDefinition{
    {
        Name:     "morning-scan",
        Type:     scheduler.TaskTypeScan,
        Interval: 24 * time.Hour,
        StartTime: time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC), // 8 AM daily
        Config: scheduler.ScanTaskConfig{
            TargetsFile:    "targets.txt",
            EnableCrawling: true,
            EnableDiffing:  true,
        },
    },
    {
        Name:     "monitoring-cycle",
        Type:     scheduler.TaskTypeMonitor,
        Interval: 2 * time.Hour,     // Every 2 hours
        Config: scheduler.MonitorTaskConfig{
            CheckInterval: 300,      // 5 minutes
            MaxChecks:     100,
        },
    },
})
```

### Manual Task Execution

```go
// Execute immediate scan
taskID, err := scheduler.ExecuteImmediateScan(
    "emergency-scan",
    "urgent-targets.txt",
    scheduler.ScanOptions{
        EnableCrawling:     true,
        EnableDiffing:      true,
        NotifyOnCompletion: true,
    },
)

// Check task status
status, err := scheduler.GetTaskStatus(taskID)
fmt.Printf("Task %s status: %s\n", taskID, status.State)

// Cancel running task
err = scheduler.CancelTask(taskID)
```

## Configuration

### Scheduler Configuration

```yaml
scheduler_config:
  cycle_minutes: 15              # Task check interval in minutes
  retry_attempts: 3              # Maximum retry attempts for failed tasks
  sqlite_db_path: "./scheduler.db"  # SQLite database path
  max_concurrent_scans: 2        # Maximum concurrent scan tasks
  max_concurrent_monitors: 5     # Maximum concurrent monitor tasks
  task_timeout_minutes: 120      # Task execution timeout
  cleanup_interval_hours: 24     # How often to clean up old task records
  max_task_history_days: 30      # How long to keep task history
```

### Configuration Options

- **`cycle_minutes`**: How often scheduler checks for pending tasks
- **`retry_attempts`**: Maximum retries for failed tasks
- **`sqlite_db_path`**: Path to SQLite database file
- **`max_concurrent_scans`**: Limit on concurrent scan executions
- **`max_concurrent_monitors`**: Limit on concurrent monitor operations
- **`task_timeout_minutes`**: Maximum execution time per task

## Database Schema

### Task Management Tables

```sql
-- Main tasks table
CREATE TABLE scheduled_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,           -- 'scan' or 'monitor'
    interval_minutes INTEGER NOT NULL,
    next_run_time INTEGER NOT NULL,
    config_json TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Task execution history
CREATE TABLE task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    execution_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,         -- 'pending', 'running', 'completed', 'failed'
    started_at INTEGER,
    completed_at INTEGER,
    result_json TEXT,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    FOREIGN KEY (task_id) REFERENCES scheduled_tasks (id)
);

-- Task dependencies (for future use)
CREATE TABLE task_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    depends_on_task_id INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES scheduled_tasks (id),
    FOREIGN KEY (depends_on_task_id) REFERENCES scheduled_tasks (id)
);
```

## Task Types

### 1. Scan Tasks

Execute automated security scans:

```go
type ScanTaskConfig struct {
    TargetsFile         string                 `json:"targets_file"`
    EnableCrawling      bool                   `json:"enable_crawling"`
    EnableDiffing       bool                   `json:"enable_diffing"`
    EnableNotifications bool                   `json:"enable_notifications"`
    ScanMode           string                 `json:"scan_mode"`
    CustomOptions      map[string]interface{} `json:"custom_options,omitempty"`
}

// Schedule scan task
err := scheduler.ScheduleScanTask(scheduler.ScanTaskDefinition{
    Name:     "weekly-full-scan",
    Interval: 7 * 24 * time.Hour, // Weekly
    Config: ScanTaskConfig{
        TargetsFile:         "all-targets.txt",
        EnableCrawling:      true,
        EnableDiffing:       true,
        EnableNotifications: true,
        ScanMode:           "comprehensive",
    },
})
```

### 2. Monitor Tasks

Execute monitoring cycles:

```go
type MonitorTaskConfig struct {
    MonitorTargetsFile string `json:"monitor_targets_file"`
    CheckInterval      int    `json:"check_interval"`
    MaxChecks          int    `json:"max_checks"`
    GenerateReports    bool   `json:"generate_reports"`
}

// Schedule monitor task
err := scheduler.ScheduleMonitorTask(scheduler.MonitorTaskDefinition{
    Name:     "continuous-monitoring",
    Interval: 4 * time.Hour, // Every 4 hours
    Config: MonitorTaskConfig{
        MonitorTargetsFile: "monitor-urls.txt",
        CheckInterval:      300, // 5 minutes
        MaxChecks:         200,
        GenerateReports:   true,
    },
})
```

## Worker Implementation

### Scan Executor

```go
type ScanExecutor struct {
    scanner         *scanner.Scanner
    notifier        *notifier.NotificationHelper
    logger          zerolog.Logger
    maxConcurrent   int
    activeScans     map[string]*ScanExecution
    scanSemaphore   chan struct{}
}

func (se *ScanExecutor) ExecuteScanTask(ctx context.Context, task ScheduledTask) (*TaskResult, error) {
    // Acquire semaphore for concurrency control
    select {
    case se.scanSemaphore <- struct{}{}:
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    defer func() { <-se.scanSemaphore }()
    
    // Parse scan configuration
    var config ScanTaskConfig
    if err := json.Unmarshal([]byte(task.ConfigJSON), &config); err != nil {
        return nil, fmt.Errorf("invalid scan config: %w", err)
    }
    
    // Execute scan
    scanInput := scanner.WorkflowInput{
        TargetsFile:           config.TargetsFile,
        ScanMode:              config.ScanMode,
        SessionID:             generateSessionID(task.Name),
        EnableCrawling:        config.EnableCrawling,
        EnableDiffing:         config.EnableDiffing,
        EnableReportGeneration: true,
    }
    
    summary, err := se.scanner.ExecuteWorkflow(ctx, scanInput)
    if err != nil {
        return &TaskResult{
            Status:       TaskStatusFailed,
            ErrorMessage: err.Error(),
        }, nil
    }
    
    // Send notifications if enabled
    if config.EnableNotifications {
        se.notifier.SendScanCompletionNotification(ctx, *summary, 
            notifier.ScanServiceNotification, []string{summary.ReportPath})
    }
    
    return &TaskResult{
        Status:    TaskStatusCompleted,
        ResultData: summary,
    }, nil
}
```

### Monitor Workers

```go
type MonitorWorkers struct {
    monitor       *monitor.MonitoringService
    logger        zerolog.Logger
    maxConcurrent int
    workerPool    chan struct{}
}

func (mw *MonitorWorkers) ExecuteMonitorTask(ctx context.Context, task ScheduledTask) (*TaskResult, error) {
    // Acquire worker from pool
    select {
    case mw.workerPool <- struct{}{}:
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    defer func() { <-mw.workerPool }()
    
    // Parse monitor configuration
    var config MonitorTaskConfig
    if err := json.Unmarshal([]byte(task.ConfigJSON), &config); err != nil {
        return nil, fmt.Errorf("invalid monitor config: %w", err)
    }
    
    // Load monitor targets
    err := mw.monitor.LoadAndMonitorFromSources(config.MonitorTargetsFile)
    if err != nil {
        return &TaskResult{
            Status:       TaskStatusFailed,
            ErrorMessage: fmt.Sprintf("failed to load targets: %v", err),
        }, nil
    }
    
    // Execute monitoring cycle
    cycleID := mw.monitor.GenerateNewCycleID()
    mw.monitor.SetCurrentCycleID(cycleID)
    
    // Run monitoring for specified duration
    monitorCtx, cancel := context.WithTimeout(ctx, 
        time.Duration(config.CheckInterval)*time.Second*time.Duration(config.MaxChecks))
    defer cancel()
    
    // Trigger monitoring cycle
    mw.monitor.TriggerCycleEndReport()
    
    // Wait for completion or timeout
    <-monitorCtx.Done()
    
    return &TaskResult{
        Status: TaskStatusCompleted,
        ResultData: map[string]interface{}{
            "cycle_id":       cycleID,
            "checks_performed": config.MaxChecks,
        },
    }, nil
}
```

## Error Handling and Retry Logic

### Automatic Retry Mechanism

```go
func (s *Scheduler) executeTaskWithRetry(ctx context.Context, task ScheduledTask) error {
    maxRetries := s.config.RetryAttempts
    var lastErr error
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        // Create execution record
        execution := &TaskExecution{
            TaskID:      task.ID,
            ExecutionID: generateExecutionID(),
            Status:      TaskStatusRunning,
            StartedAt:   time.Now(),
            RetryCount:  attempt,
        }
        
        // Save execution record
        if err := s.db.SaveTaskExecution(execution); err != nil {
            s.logger.Error().Err(err).Msg("Failed to save task execution")
        }
        
        // Execute task
        result, err := s.executeTask(ctx, task)
        if err == nil {
            // Success - update execution record
            execution.Status = TaskStatusCompleted
            execution.CompletedAt = time.Now()
            execution.ResultJSON = marshalResult(result)
            s.db.UpdateTaskExecution(execution)
            return nil
        }
        
        lastErr = err
        
        // Failed - check if we should retry
        if attempt < maxRetries && s.shouldRetry(err) {
            retryDelay := s.calculateRetryDelay(attempt)
            s.logger.Warn().
                Err(err).
                Int("attempt", attempt+1).
                Int("max_retries", maxRetries+1).
                Dur("retry_delay", retryDelay).
                Msg("Task failed, retrying")
            
            // Update execution with failure
            execution.Status = TaskStatusFailed
            execution.ErrorMessage = err.Error()
            execution.CompletedAt = time.Now()
            s.db.UpdateTaskExecution(execution)
            
            // Wait before retry
            select {
            case <-time.After(retryDelay):
            case <-ctx.Done():
                return ctx.Err()
            }
            
            continue
        }
        
        // Final failure
        execution.Status = TaskStatusFailed
        execution.ErrorMessage = err.Error()
        execution.CompletedAt = time.Now()
        s.db.UpdateTaskExecution(execution)
        break
    }
    
    return fmt.Errorf("task failed after %d attempts: %w", maxRetries+1, lastErr)
}
```

### Retry Strategy

```go
func (s *Scheduler) shouldRetry(err error) bool {
    // Don't retry certain types of errors
    if errors.Is(err, context.Canceled) {
        return false
    }
    if errors.Is(err, context.DeadlineExceeded) {
        return false
    }
    
    // Check for specific error patterns
    errStr := err.Error()
    nonRetryablePatterns := []string{
        "invalid configuration",
        "permission denied",
        "file not found",
    }
    
    for _, pattern := range nonRetryablePatterns {
        if strings.Contains(errStr, pattern) {
            return false
        }
    }
    
    return true
}

func (s *Scheduler) calculateRetryDelay(attempt int) time.Duration {
    // Exponential backoff with jitter
    baseDelay := time.Minute
    maxDelay := 15 * time.Minute
    
    delay := baseDelay * time.Duration(1<<uint(attempt))
    if delay > maxDelay {
        delay = maxDelay
    }
    
    // Add jitter (±25%)
    jitter := time.Duration(rand.Float64() * 0.5 * float64(delay))
    if rand.Float64() < 0.5 {
        delay -= jitter
    } else {
        delay += jitter
    }
    
    return delay
}
```

## Integration Examples

### With Main Application

```go
// Main application integration
func main() {
    // Initialize components
    globalConfig, _ := config.LoadGlobalConfig("config.yaml", logger)
    scanner := scanner.NewScanner(globalConfig, logger)
    monitor := monitor.NewMonitoringService(globalConfig, logger, notifier)
    
    // Initialize scheduler
    scheduler, err := scheduler.NewScheduler(
        globalConfig.SchedulerConfig,
        logger,
        scanner,
        monitor,
        notifier,
    )
    if err != nil {
        log.Fatal("Failed to initialize scheduler:", err)
    }
    
    // Start scheduler
    ctx := context.Background()
    if err := scheduler.Start(ctx); err != nil {
        log.Fatal("Failed to start scheduler:", err)
    }
    
    // Schedule default tasks
    scheduler.ScheduleDefaultTasks()
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan
    
    // Graceful shutdown
    scheduler.Stop()
}
```

### REST API Integration

```go
// HTTP handlers for scheduler management
func (api *API) scheduleTaskHandler(w http.ResponseWriter, r *http.Request) {
    var req ScheduleTaskRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    taskID, err := api.scheduler.ScheduleTask(req.ToTaskDefinition())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "task_id": taskID,
        "status":  "scheduled",
    })
}

func (api *API) getTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
    taskID := r.URL.Query().Get("task_id")
    status, err := api.scheduler.GetTaskStatus(taskID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    
    json.NewEncoder(w).Encode(status)
}
```

## Performance Considerations

### Database Optimization
- Indexed queries for fast task lookup
- Connection pooling for concurrent access
- Batch operations for bulk updates
- Regular cleanup of old execution records

### Memory Management
- Limited concurrent task execution
- Stream processing for large result sets
- Proper cleanup of completed tasks
- Resource monitoring and alerting

### Concurrency Control
- Semaphore-based worker pools
- Graceful shutdown with context cancellation
- Deadlock prevention in database operations
- Proper synchronization of shared resources

## Dependencies

- **database/sql** - SQL database interface
- **github.com/mattn/go-sqlite3** - SQLite driver
- **github.com/aleister1102/monsterinc/internal/scanner** - Scan execution
- **github.com/aleister1102/monsterinc/internal/monitor** - Monitoring execution
- **github.com/aleister1102/monsterinc/internal/notifier** - Notifications
- **github.com/aleister1102/monsterinc/internal/config** - Configuration

## Thread Safety

- All scheduler operations are thread-safe
- Database operations use proper locking
- Worker pools prevent resource conflicts
- Context propagation for cancellation
- Atomic updates for task state changes

## Best Practices

### Scheduling
- Use appropriate intervals based on target sensitivity
- Monitor task execution duration and adjust timeouts
- Implement proper error handling and notifications
- Regular cleanup of completed task history

### Database Management
- Regular database maintenance and optimization
- Backup scheduling data regularly
- Monitor database size and performance
- Use transactions for atomic operations 