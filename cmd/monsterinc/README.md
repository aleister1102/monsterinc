# cmd/monsterinc

This package contains the main entry point for the MonsterInc application.

## Overview

The `cmd/monsterinc` package provides the command-line interface and orchestrates the execution of different operational modes (onetime and automated).

## Files

### main.go
The main application file that handles:
- Command-line argument parsing
- Configuration loading and validation
- Mode selection and execution
- Graceful shutdown handling
- Global initialization of services

## Command-Line Arguments

### Required Arguments
- `--mode <onetime|automated>`: Execution mode

### Optional Arguments
- `-u, --urlfile <path>`: Path to file containing seed URLs
- `--mtf, --monitor-target-file <path>`: File containing URLs to monitor (automated mode)
- `--gc, --globalconfig <path>`: Path to configuration file (default: config.yaml)

## Execution Modes

### Onetime Mode
Executes a single scan cycle and exits:
1. Load targets from file or configuration
2. Execute complete scan workflow
3. Generate report and send notifications
4. Exit

### Automated Mode
Runs continuously with scheduled scans:
1. Initialize scheduler and monitoring services
2. Execute scan cycles at configured intervals
3. Handle retries for failed scans
4. Maintain scan history in SQLite database
5. Optional file monitoring for real-time change detection

## Error Handling

The main function implements comprehensive error handling:
- Configuration validation errors
- Service initialization failures
- Graceful shutdown on interrupt signals
- Proper cleanup of resources

## Dependencies

- `internal/config`: Configuration management
- `internal/logger`: Logging infrastructure
- `internal/scheduler`: Automated scan scheduling
- `internal/orchestrator`: Workflow orchestration
- `internal/notifier`: Notification services
- `internal/secrets`: Secret detection services

## Usage Examples

```bash
# Run single scan with URL file
./monsterinc --mode onetime -u targets.txt

# Run automated mode with monitoring
./monsterinc --mode automated --mtf monitor_targets.txt

# Use custom configuration
./monsterinc --mode onetime --gc custom_config.yaml -u targets.txt
``` 