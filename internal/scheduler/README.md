# Scheduler Package

The scheduler package provides automated task scheduling and execution for MonsterInc security operations. It manages periodic scans, coordinates monitoring workflows, and maintains persistent scheduling state using SQLite database.

## Package Role in MonsterInc
As the automation engine, this package:
- **Scanner Automation**: Schedules and executes automated security scans at configurable intervals
- **Monitor Orchestration**: Coordinates monitoring cycles and file change detection workflows
- **Task Persistence**: Provides reliable SQLite-based task storage with execution history
- **Resource Management**: Manages concurrent task execution and system resource allocation
- **Integration Hub**: Coordinates Scanner and Monitor services with automated execution

## Overview

The scheduler package enables:
- **Automated Scanning**: Schedule periodic security scans with flexible timing configurations
- **Task Persistence**: SQLite-based storage for reliable task management across restarts
- **Worker Coordination**: Concurrent execution of scanning and monitoring tasks with resource limits
- **Retry Logic**: Automatic retry mechanisms for failed operations with exponential backoff
- **State Management**: Track task execution history and manage schedule states

## Integration with MonsterInc Components

### With Scanner Service

```go
// Scheduler executes automated scans
scanExecutor := scheduler.GetScanExecutor()
taskResult, err := scanExecutor.ExecuteScheduledScan(ctx, taskConfig)

if err != nil {
    logger.Error().Err(err).Msg("Scheduled scan failed")
    // Scheduler handles retry logic automatically
    return scheduler.MarkTaskForRetry(taskID)
}

// Scanner workflow integration
summary := &models.ScanSummaryData{
    ScanSessionID: taskResult.SessionID,
    ScanMode:      "scheduled",
    Status:        "COMPLETED",
}

// Notifier integration through scheduler
notifier.SendScanCompletionNotification(ctx, summary, 
    notifier.ScanServiceNotification, taskResult.ReportPaths)
```

### With Monitor Service

```go  
// Scheduler coordinates monitoring cycles
monitorExecutor := scheduler.GetMonitorExecutor()
err := monitorExecutor.ExecuteMonitoringCycle(ctx, monitorConfig)

if err != nil {
    logger.Error().Err(err).Msg("Monitoring cycle failed")
    // Scheduler manages retry and error reporting
    return scheduler.HandleMonitorError(err, cycleID)
}

// Monitor integration with batch processing
batchManager := monitor.GetBatchURLManager()
changedURLs, err := batchManager.ProcessBatch(ctx, urlBatch)

// Scheduler tracks monitoring statistics
scheduler.UpdateMonitorStats(cycleID, changedURLs, err)
```

### With Datastore Integration

```go
// Scheduler maintains task persistence
db := scheduler.GetDatabase()

// Task scheduling with database backing
task := &models.ScheduledTask{
    Name:         "daily-security-scan",
    Type:         scheduler.TaskTypeScan,
    Interval:     24 * time.Hour,
    NextRunTime:  time.Now().Add(24 * time.Hour),
    ConfigJSON:   configJSON,
}

err = db.SaveScheduledTask(task)
if err != nil {
    return fmt.Errorf("failed to save scheduled task: %w", err)
}

// Execution history tracking
execution := &models.TaskExecution{
    TaskID:     task.ID,
    StartTime:  time.Now(),
    Status:     "RUNNING",
    SessionID:  sessionID,
}

db.SaveTaskExecution(execution)
```

## File Structure

### Core Components

- **`scheduler_core.go`** - Main scheduler service and orchestration logic
- **`scheduler_executor.go`** - Task execution coordinator and worker management
- **`scan_executor.go`** - Automated scan execution and Scanner service integration
- **`db.go`** - SQLite database management and persistence operations
- **`helpers.go`** - Utility functions and common operations

## Features

### 1. Automated Task Scheduling

**Capabilities:**
- **Flexible Intervals**: Configure scan intervals in minutes, hours, or days
- **Cron-like Scheduling**: Support for complex scheduling patterns
- **Task Priorities**: Manage task execution order and resource allocation
- **Concurrent Execution**: Multiple tasks running simultaneously with limits
- **Dynamic Scheduling**: Add, modify, or remove schedules at runtime

### 2. Database-Backed Persistence

**Features:**
- **SQLite Storage**: Lightweight, reliable database for task persistence
- **Execution History**: Complete audit trail of task executions
- **Schedule State**: Persistent schedule state across application restarts
- **Atomic Operations**: ACID compliance with transaction support
- **Schema Migrations**: Automatic database schema updates

### 3. Worker Management

**Coordination:**
- **Resource Pools**: Separate worker pools for scan and monitor tasks
- **Load Balancing**: Smart distribution of tasks across available workers
- **Graceful Shutdown**: Proper cleanup and task completion on shutdown
- **Resource Limits**: Configurable limits to prevent system overload
- **Health Monitoring**: Track worker health and performance metrics

## Usage Examples

### Basic Scheduler Setup and Configuration

```go
import (
    "github.com/aleister1102/monsterinc/internal/scheduler"
    "github.com/aleister1102/monsterinc/internal/config"
)

// Initialize scheduler with dependencies
schedulerService, err := scheduler.NewScheduler(
    cfg.SchedulerConfig,
    logger,
    scannerService,  // Scanner service for automated scans
    monitorService,  // Monitor service for monitoring cycles
    notificationHelper, // Notifier for task completion alerts
)
if err != nil {
    return fmt.Errorf("scheduler initialization failed: %w", err)
}

// Start scheduler with context
ctx := context.Background()
err = schedulerService.Start(ctx)
if err != nil {
    return fmt.Errorf("scheduler start failed: %w", err)
}

// Schedule recurring security scan
err = schedulerService.ScheduleRecurringScan(
    &scheduler.ScanScheduleConfig{
        Name:           "daily-security-scan",
        TargetsFile:    "targets.txt",
        Interval:       24 * time.Hour,
        StartTime:      time.Now().Add(1 * time.Hour), // Start in 1 hour
        EnableCrawling: true,
        EnableDiffing:  true,
        MaxConcurrent:  2,
    },
)

if err != nil {
    return fmt.Errorf("failed to schedule scan: %w", err)
}
```

### Advanced Scheduling Patterns

```go
// Configure scheduler with custom worker settings
schedulerConfig := config.SchedulerConfig{
    CycleMinutes:          15,              // Check for tasks every 15 minutes
    RetryAttempts:         3,               // Retry failed tasks up to 3 times
    SQLiteDBPath:          "./scheduler.db", // Database path
    MaxConcurrentScans:    2,               // Limit concurrent scans
    MaxConcurrentMonitors: 5,               // Limit concurrent monitor tasks
    TaskTimeoutMinutes:    120,             // 2-hour task timeout
}

// Schedule multiple task types with different patterns
tasks := []scheduler.TaskDefinition{
    {
        Name:     "morning-vulnerability-scan",
        Type:     scheduler.TaskTypeScan,
        Schedule: "0 8 * * *", // 8 AM daily (cron format)
        Config: scheduler.ScanTaskConfig{
            TargetsFile:       "production-targets.txt",
            EnableCrawling:    true,
            EnableDiffing:     true,
            NotifyOnCompletion: true,
        },
    },
    {
        Name:     "continuous-monitoring",
        Type:     scheduler.TaskTypeMonitor,
        Interval: 30 * time.Minute, // Every 30 minutes
        Config: scheduler.MonitorTaskConfig{
            BatchSize:     50,
            CheckInterval: 300, // 5 minutes per check
            MaxRetries:    2,
        },
    },
    {
        Name:     "weekend-deep-scan",
        Type:     scheduler.TaskTypeScan,
        Schedule: "0 2 * * 6", // 2 AM every Saturday
        Config: scheduler.ScanTaskConfig{
            TargetsFile:    "comprehensive-targets.txt",
            EnableCrawling: true,
            EnableDiffing:  true,
            DeepScanMode:   true,
        },
    },
}

err = schedulerService.ScheduleMultipleTasks(tasks)
if err != nil {
    return fmt.Errorf("failed to schedule tasks: %w", err)
}
```

### Manual Task Management

```go
// Execute immediate emergency scan
taskID, err := schedulerService.ExecuteImmediateScan(
    &scheduler.ImmediateScanConfig{
        Name:        "emergency-vulnerability-scan",
        TargetsFile: "critical-targets.txt",
        Priority:    scheduler.HighPriority,
        Options: scheduler.ScanOptions{
            EnableCrawling:     true,
            EnableDiffing:      true,
            NotifyOnCompletion: true,
            OverrideRateLimit:  true, // For emergency scans
        },
    },
)

if err != nil {
    return fmt.Errorf("immediate scan failed: %w", err)
}

// Monitor task progress
ticker := time.NewTicker(30 * time.Second)
defer ticker.Stop()

for {
    select {
    case <-ticker.C:
        status, err := schedulerService.GetTaskStatus(taskID)
        if err != nil {
            logger.Error().Err(err).Msg("Failed to get task status")
            continue
        }
        
        logger.Info().
            Str("task_id", taskID).
            Str("status", status.State).
            Int("progress", status.Progress).
            Msg("Task progress update")
            
        if status.State == "COMPLETED" || status.State == "FAILED" {
            break
        }
        
    case <-ctx.Done():
        // Cancel task on context cancellation
        schedulerService.CancelTask(taskID)
        return ctx.Err()
    }
}
```

### Task History and Analytics

```go
// Query task execution history
history, err := schedulerService.GetTaskHistory(&scheduler.HistoryQuery{
    TaskName:  "daily-security-scan",
    StartDate: time.Now().AddDate(0, -1, 0), // Last month
    EndDate:   time.Now(),
    Status:    []string{"COMPLETED", "FAILED"},
    Limit:     100,
})

if err != nil {
    return fmt.Errorf("failed to get task history: %w", err)
}

// Generate execution statistics
stats := scheduler.CalculateTaskStats(history)
logger.Info().
    Int("total_executions", stats.TotalExecutions).
    Int("successful_executions", stats.SuccessfulExecutions).
    Float64("success_rate", stats.SuccessRate).
    Dur("avg_duration", stats.AverageDuration).
    Msg("Task execution statistics")

// Clean up old task records
cleanupConfig := scheduler.CleanupConfig{
    MaxHistoryDays:    30,  // Keep 30 days of history
    MaxExecutionsPerTask: 100, // Keep last 100 executions per task
}

err = schedulerService.CleanupTaskHistory(cleanupConfig)
if err != nil {
    logger.Error().Err(err).Msg("Task history cleanup failed")
}
```

## Configuration

### Scheduler Configuration

```yaml
scheduler_config:
  # Core scheduling settings
  cycle_minutes: 15              # Task check interval in minutes
  retry_attempts: 3              # Maximum retry attempts for failed tasks
  sqlite_db_path: "./scheduler.db"  # SQLite database path
  
  # Resource management
  max_concurrent_scans: 2        # Maximum concurrent scan tasks
  max_concurrent_monitors: 5     # Maximum concurrent monitor tasks
  task_timeout_minutes: 120      # Task execution timeout (2 hours)
  
  # Maintenance settings
  cleanup_interval_hours: 24     # How often to clean up old records
  max_task_history_days: 30      # How long to keep task execution history
  max_executions_per_task: 200   # Maximum executions to keep per task
  
  # Performance tuning
  worker_pool_size: 10           # Worker pool size for task execution
  queue_buffer_size: 100         # Task queue buffer size
  health_check_interval: 300     # Health check interval in seconds
  
  # Error handling
  retry_backoff_multiplier: 2.0  # Exponential backoff multiplier
  max_retry_delay_minutes: 60    # Maximum retry delay
  enable_dead_letter_queue: true # Failed task tracking
```

### Configuration Structure

```go
type SchedulerConfig struct {
    CycleMinutes              int     `yaml:"cycle_minutes"`
    RetryAttempts             int     `yaml:"retry_attempts"`
    SQLiteDBPath              string  `yaml:"sqlite_db_path"`
    MaxConcurrentScans        int     `yaml:"max_concurrent_scans"`
    MaxConcurrentMonitors     int     `yaml:"max_concurrent_monitors"`
    TaskTimeoutMinutes        int     `yaml:"task_timeout_minutes"`
    CleanupIntervalHours      int     `yaml:"cleanup_interval_hours"`
    MaxTaskHistoryDays        int     `yaml:"max_task_history_days"`
    WorkerPoolSize            int     `yaml:"worker_pool_size"`
    RetryBackoffMultiplier    float64 `yaml:"retry_backoff_multiplier"`
    MaxRetryDelayMinutes      int     `yaml:"max_retry_delay_minutes"`
}
```

## Database Schema

### Task Management Tables

```sql
-- Main scheduled tasks table
CREATE TABLE scheduled_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL,           -- 'scan' or 'monitor'
    schedule_type TEXT NOT NULL,  -- 'interval' or 'cron'
    schedule_value TEXT NOT NULL, -- interval duration or cron expression
    next_run_time INTEGER NOT NULL,
    config_json TEXT NOT NULL,
    is_active BOOLEAN DEFAULT 1,
    priority INTEGER DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Task execution history
CREATE TABLE task_executions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    session_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,         -- 'RUNNING', 'COMPLETED', 'FAILED', 'CANCELLED'
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    error_message TEXT,
    result_json TEXT,
    retry_count INTEGER DEFAULT 0,
    FOREIGN KEY (task_id) REFERENCES scheduled_tasks (id)
);

-- Task metrics and statistics
CREATE TABLE task_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_id INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    metric_value REAL NOT NULL,
    recorded_at INTEGER NOT NULL,
    FOREIGN KEY (execution_id) REFERENCES task_executions (id)
);

-- Indexes for performance
CREATE INDEX idx_scheduled_tasks_next_run ON scheduled_tasks(next_run_time, is_active);
CREATE INDEX idx_task_executions_status ON task_executions(status, start_time);
CREATE INDEX idx_task_executions_session ON task_executions(session_id);
```

## Task Types and Configuration

### Scan Task Configuration

```go
type ScanTaskConfig struct {
    TargetsFile        string        `json:"targets_file"`
    EnableCrawling     bool         `json:"enable_crawling"`
    EnableDiffing      bool         `json:"enable_diffing"`
    MaxConcurrent      int          `json:"max_concurrent"`
    TimeoutMinutes     int          `json:"timeout_minutes"`
    NotifyOnCompletion bool         `json:"notify_on_completion"`
    OutputDirectory    string       `json:"output_directory"`
    ScanMode          string       `json:"scan_mode"` // "fast", "normal", "deep"
}
```

### Monitor Task Configuration

```go
type MonitorTaskConfig struct {
    BatchSize         int           `json:"batch_size"`
    CheckInterval     int           `json:"check_interval"`
    MaxRetries        int           `json:"max_retries"`
    NotifyOnChanges   bool         `json:"notify_on_changes"`
    DiffReporting     bool         `json:"diff_reporting"`
    TargetFilter      string       `json:"target_filter"` // Filter expression
}
```

## Dependencies

- **github.com/aleister1102/monsterinc/internal/scanner** - Scanner service integration
- **github.com/aleister1102/monsterinc/internal/monitor** - Monitor service integration
- **github.com/aleister1102/monsterinc/internal/notifier** - Notification delivery
- **github.com/aleister1102/monsterinc/internal/models** - Data structures
- **github.com/aleister1102/monsterinc/internal/config** - Configuration management
- **github.com/mattn/go-sqlite3** - SQLite database driver
- **github.com/rs/zerolog** - Structured logging
- **database/sql** - SQL database interface

## Best Practices

### Task Scheduling
- Use cron expressions for complex scheduling patterns
- Set appropriate timeout values based on expected task duration
- Implement proper retry logic with exponential backoff
- Monitor task execution success rates and adjust schedules accordingly

### Resource Management
- Configure concurrent limits based on system resources
- Monitor system performance during peak task execution
- Implement graceful degradation when resources are constrained
- Use priority queues for critical tasks

### Database Management
- Regularly clean up old task execution records
- Monitor database size and performance
- Implement proper indexing for frequently queried columns
- Use transactions for atomic operations

### Error Handling and Monitoring
- Log all task execution attempts and outcomes
- Implement alerting for persistent task failures
- Track task execution metrics and performance trends
- Provide operational dashboards for task monitoring 