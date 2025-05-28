#!/bin/bash

# MonsterInc Watchdog Script
# Monitors MonsterInc service and restarts if needed
# Sends Discord notifications on restart events

# Configuration
MONSTERINC_BIN="${MONSTERINC_BIN:-./monsterinc}"
MONSTERINC_ARGS="${MONSTERINC_ARGS:--mode scheduler}"
CHECK_INTERVAL="${CHECK_INTERVAL:-3600}"  # 60 minutes in seconds
LOG_DIR="${LOG_DIR:-$HOME/logs}"
LOG_FILE="$LOG_DIR/monsterinc-watchdog.log"
PID_FILE="/tmp/monsterinc.pid"
DISCORD_WEBHOOK_URL="${DISCORD_WEBHOOK_URL:-}"
MAX_RESTART_ATTEMPTS="${MAX_RESTART_ATTEMPTS:-3}"
RESTART_DELAY="${RESTART_DELAY:-30}"

# Create log directory if it doesn't exist
mkdir -p "$LOG_DIR"

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Send Discord notification
send_discord_notification() {
    local message="$1"
    local color="$2"  # 16711680 for red, 65280 for green, 16776960 for yellow
    
    if [[ -z "$DISCORD_WEBHOOK_URL" ]]; then
        log "Discord webhook URL not configured, skipping notification"
        return
    fi
    
    local payload=$(cat <<EOF
{
    "embeds": [{
        "title": "MonsterInc Watchdog Alert",
        "description": "$message",
        "color": $color,
        "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%S.000Z)",
        "footer": {
            "text": "MonsterInc Watchdog"
        }
    }]
}
EOF
)
    
    curl -s -H "Content-Type: application/json" \
         -d "$payload" \
         "$DISCORD_WEBHOOK_URL" > /dev/null 2>&1
    
    if [[ $? -eq 0 ]]; then
        log "Discord notification sent successfully"
    else
        log "Failed to send Discord notification"
    fi
}

# Check if MonsterInc process is running
is_monsterinc_running() {
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        if kill -0 "$pid" 2>/dev/null; then
            return 0  # Process is running
        else
            log "PID file exists but process $pid is not running"
            rm -f "$PID_FILE"
        fi
    fi
    
    # Check by process name as fallback
    if pgrep -f "monsterinc" > /dev/null; then
        log "Found MonsterInc process but no PID file, creating PID file"
        pgrep -f "monsterinc" | head -1 > "$PID_FILE"
        return 0
    fi
    
    return 1  # Process is not running
}

# Start MonsterInc service
start_monsterinc() {
    log "Starting MonsterInc service..."
    
    # Check if binary exists
    if [[ ! -f "$MONSTERINC_BIN" ]]; then
        log "ERROR: MonsterInc binary not found at $MONSTERINC_BIN"
        send_discord_notification "❌ **MonsterInc Watchdog Error**\n\nBinary not found at: \`$MONSTERINC_BIN\`" 16711680
        return 1
    fi
    
    # Start MonsterInc in background
    nohup "$MONSTERINC_BIN" $MONSTERINC_ARGS > "$LOG_DIR/monsterinc.log" 2>&1 &
    local pid=$!
    
    # Save PID
    echo "$pid" > "$PID_FILE"
    
    # Wait a moment and check if it started successfully
    sleep 5
    if kill -0 "$pid" 2>/dev/null; then
        log "MonsterInc started successfully with PID $pid"
        send_discord_notification "✅ **MonsterInc Service Started**\n\nPID: \`$pid\`\nCommand: \`$MONSTERINC_BIN $MONSTERINC_ARGS\`" 65280
        return 0
    else
        log "ERROR: MonsterInc failed to start"
        rm -f "$PID_FILE"
        send_discord_notification "❌ **MonsterInc Service Failed to Start**\n\nCheck logs for details: \`$LOG_FILE\`" 16711680
        return 1
    fi
}

# Stop MonsterInc service
stop_monsterinc() {
    if [[ -f "$PID_FILE" ]]; then
        local pid=$(cat "$PID_FILE")
        log "Stopping MonsterInc service (PID: $pid)..."
        
        if kill -TERM "$pid" 2>/dev/null; then
            # Wait for graceful shutdown
            local count=0
            while kill -0 "$pid" 2>/dev/null && [[ $count -lt 30 ]]; do
                sleep 1
                ((count++))
            done
            
            # Force kill if still running
            if kill -0 "$pid" 2>/dev/null; then
                log "Graceful shutdown failed, force killing process"
                kill -KILL "$pid" 2>/dev/null
            fi
        fi
        
        rm -f "$PID_FILE"
        log "MonsterInc service stopped"
    fi
}

# Main monitoring loop
monitor_service() {
    local restart_count=0
    
    log "Starting MonsterInc watchdog monitoring (interval: ${CHECK_INTERVAL}s)"
    send_discord_notification "🔍 **MonsterInc Watchdog Started**\n\nMonitoring interval: \`${CHECK_INTERVAL}s\`\nMax restart attempts: \`$MAX_RESTART_ATTEMPTS\`" 16776960
    
    while true; do
        if is_monsterinc_running; then
            log "MonsterInc service is running (PID: $(cat "$PID_FILE" 2>/dev/null || echo "unknown"))"
            restart_count=0  # Reset restart count on successful check
        else
            log "MonsterInc service is not running"
            
            if [[ $restart_count -lt $MAX_RESTART_ATTEMPTS ]]; then
                ((restart_count++))
                log "Attempting to restart MonsterInc (attempt $restart_count/$MAX_RESTART_ATTEMPTS)"
                
                if start_monsterinc; then
                    log "MonsterInc restarted successfully"
                    restart_count=0
                else
                    log "Failed to restart MonsterInc (attempt $restart_count/$MAX_RESTART_ATTEMPTS)"
                    if [[ $restart_count -lt $MAX_RESTART_ATTEMPTS ]]; then
                        log "Waiting ${RESTART_DELAY}s before next attempt..."
                        sleep "$RESTART_DELAY"
                    fi
                fi
            else
                log "ERROR: Maximum restart attempts ($MAX_RESTART_ATTEMPTS) reached"
                send_discord_notification "🚨 **MonsterInc Watchdog Critical Error**\n\nMaximum restart attempts reached: \`$MAX_RESTART_ATTEMPTS\`\nWatchdog will continue monitoring but won't attempt more restarts.\nManual intervention required." 16711680
                restart_count=0  # Reset to allow future restart attempts after manual intervention
            fi
        fi
        
        sleep "$CHECK_INTERVAL"
    done
}

# Signal handlers
cleanup() {
    log "Watchdog received shutdown signal"
    send_discord_notification "⏹️ **MonsterInc Watchdog Stopped**\n\nWatchdog service has been shut down." 16776960
    exit 0
}

# Set up signal handlers
trap cleanup SIGTERM SIGINT

# Command line argument handling
case "${1:-monitor}" in
    "start")
        start_monsterinc
        ;;
    "stop")
        stop_monsterinc
        ;;
    "restart")
        stop_monsterinc
        sleep 2
        start_monsterinc
        ;;
    "status")
        if is_monsterinc_running; then
            echo "MonsterInc is running (PID: $(cat "$PID_FILE" 2>/dev/null || echo "unknown"))"
            exit 0
        else
            echo "MonsterInc is not running"
            exit 1
        fi
        ;;
    "monitor")
        monitor_service
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|monitor}"
        echo ""
        echo "Commands:"
        echo "  start   - Start MonsterInc service"
        echo "  stop    - Stop MonsterInc service"
        echo "  restart - Restart MonsterInc service"
        echo "  status  - Check MonsterInc service status"
        echo "  monitor - Start watchdog monitoring (default)"
        echo ""
        echo "Environment variables:"
        echo "  MONSTERINC_BIN      - Path to MonsterInc binary (default: ./monsterinc)"
        echo "  MONSTERINC_ARGS     - Arguments for MonsterInc (default: -mode scheduler)"
        echo "  CHECK_INTERVAL      - Check interval in seconds (default: 3600)"
        echo "  LOG_DIR             - Log directory (default: \$HOME/logs)"
        echo "  DISCORD_WEBHOOK_URL - Discord webhook for notifications"
        echo "  MAX_RESTART_ATTEMPTS - Maximum restart attempts (default: 3)"
        echo "  RESTART_DELAY       - Delay between restart attempts in seconds (default: 30)"
        exit 1
        ;;
esac 