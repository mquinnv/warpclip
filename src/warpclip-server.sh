#!/usr/bin/env bash

# warpclip-server.sh
# A persistent service that listens for incoming connections and copies content to clipboard
# Part of the WarpClip solution for remote clipboard forwarding

# Configuration
PORT=8888
LOG_FILE="$HOME/.warpclip.log"
DEBUG_FILE="$HOME/.warpclip.debug.log"
MAX_RETRIES=5
RETRY_DELAY=2
CONNECTION_TIMEOUT=7200  # 2 hours in seconds
VERSION="1.0.0"
PID_FILE="$HOME/.warpclip.pid"

# Create log files if they don't exist
touch "$LOG_FILE"
touch "$DEBUG_FILE"

# Log levels: INFO, WARNING, ERROR, DEBUG
log() {
    local level="INFO"
    if [[ $# -gt 1 ]]; then
        level="$1"
        shift
    fi
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $1" >> "$LOG_FILE"
    
    # Only write DEBUG level messages to debug file
    if [[ "$level" == "DEBUG" ]]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $1" >> "$DEBUG_FILE"
    fi
    
    # Write errors to stderr for launchd to capture
    if [[ "$level" == "ERROR" ]]; then
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $1" >&2
    fi
}

# Store PID for status checking
echo $$ > "$PID_FILE"

log "INFO" "Starting warpclip server v${VERSION} on port $PORT (PID: $$)"

# Function to check status of the service
check_status() {
    if [[ -f "$PID_FILE" ]]; then
        local stored_pid=$(cat "$PID_FILE")
        if ps -p "$stored_pid" > /dev/null; then
            echo "WarpClip service is running (PID: $stored_pid, Port: $PORT)"
            
            # Show last clipboard activity if available
            if [[ -f "$HOME/.warpclip.last" ]]; then
                echo "Last clipboard activity: $(cat "$HOME/.warpclip.last")"
            fi
            
            # Check if port is actually listening
            if lsof -i ":$PORT" -sTCP:LISTEN > /dev/null 2>&1; then
                echo "Port $PORT is actively listening for connections"
            else
                echo "WARNING: Port $PORT is not listening! Service may be restarting"
            fi
            
            return 0
        else
            echo "WarpClip service appears to be dead (stale PID: $stored_pid)"
            return 1
        fi
    else
        echo "WarpClip service is not running (no PID file found)"
        return 1
    fi
}

# Handle command line arguments
if [[ "$1" == "status" ]]; then
    check_status
    exit $?
fi

check_port_available() {
    log "DEBUG" "Checking if port $PORT is available"
    if lsof -i ":$PORT" > /dev/null 2>&1; then
        local pid=$(lsof -i ":$PORT" -t)
        if [[ "$pid" == "$$" ]]; then
            log "DEBUG" "Port $PORT is used by our own process ($$)"
            return 0
        fi
        log "ERROR" "Port $PORT is already in use by process $pid. Choose a different port or stop the existing process."
        return 1
    fi
    log "DEBUG" "Port $PORT is available"
    return 0
}

check_dependencies() {
    log "DEBUG" "Checking dependencies"
    if ! command -v nc &> /dev/null; then
        log "ERROR" "netcat (nc) not found. Please install netcat to use this script."
        return 1
    fi
    
    if ! command -v pbcopy &> /dev/null; then
        log "ERROR" "pbcopy not found. This script requires macOS."
        return 1
    fi
    
    if ! command -v timeout &> /dev/null && ! command -v gtimeout &> /dev/null; then
        log "WARNING" "timeout/gtimeout command not found. Connection timeouts may not work properly."
    fi
    
    return 0
}

handle_exit() {
    log "INFO" "Shutting down warpclip server (PID: $$)"
    rm -f "$PID_FILE"
    exit 0
}

# Set up trap for clean shutdown
trap handle_exit SIGINT SIGTERM

# Check dependencies
if ! check_dependencies; then
    log "ERROR" "Missing required dependencies. Exiting."
    exit 1
fi

# Data processor function to handle clipboard content
process_data() {
    local temp_file=$(mktemp)
    cat > "$temp_file"
    
    local size=$(wc -c < "$temp_file")
    local lines=$(wc -l < "$temp_file")
    
    if [[ $size -eq 0 ]]; then
        log "WARNING" "Received empty data, nothing copied to clipboard"
        rm -f "$temp_file"
        return 1
    fi
    
    log "INFO" "Received data: $size bytes, $lines lines"
    
    # Copy to clipboard
    if cat "$temp_file" | pbcopy; then
        log "INFO" "✓ Successfully copied to clipboard"
        # Print success message to stdout (for daemon logs)
        echo "$(date '+%Y-%m-%d %H:%M:%S') ✓ Copied $size bytes to clipboard"
    else
        log "ERROR" "Failed to copy data to clipboard"
        # Clean up
        rm -f "$temp_file"
        return 1
    fi
    
    # Clean up
    rm -f "$temp_file"
    
    # Touch a timestamp file for status checking
    echo "$size bytes, $lines lines" > "$HOME/.warpclip.last"
    echo "$(date '+%Y-%m-%d %H:%M:%S')" >> "$HOME/.warpclip.last"
    
    return 0
}

# Main loop
retry_count=0
while true; do
    if ! check_port_available; then
        retry_count=$((retry_count + 1))
        if [ $retry_count -gt $MAX_RETRIES ]; then
            log "ERROR" "FATAL: Failed to start server after $MAX_RETRIES attempts. Exiting."
            exit 1
        fi
        log "WARNING" "Retrying in $RETRY_DELAY seconds (attempt $retry_count/$MAX_RETRIES)..."
        sleep $RETRY_DELAY
        continue
    fi
    
    # Reset retry counter on successful port check
    retry_count=0
    
    log "INFO" "Listening on port $PORT for incoming connections"
    
    # Check if netcat supports keep-alive
    if nc -h 2>&1 | grep -q -- "-k"; then
        log "DEBUG" "Using netcat with keep-alive support"
        has_keepalive=true
    else
        log "DEBUG" "Netcat does not support keep-alive (-k option)"
        has_keepalive=false
    fi
    
    # Create a named pipe for handling multiple connections
    pipe_file=$(mktemp -u)
    mkfifo "$pipe_file"
    
    # Set up a background process to read from the pipe and process data
    (
        while IFS= read -r line; do
            echo "$line" | process_data
        done < "$pipe_file"
    ) &
    bg_pid=$!
    
    log "DEBUG" "Started background processor with PID $bg_pid"
    
    # Use timeout for the netcat connection with keep-alive if available
    if $has_keepalive; then
        if command -v timeout &> /dev/null; then
            log "INFO" "Starting netcat listener with keep-alive (timeout controlled)"
            timeout $CONNECTION_TIMEOUT nc -k -l $PORT > "$pipe_file"
            exit_code=$?
        elif command -v gtimeout &> /dev/null; then
            log "INFO" "Starting netcat listener with keep-alive (gtimeout controlled)"
            gtimeout $CONNECTION_TIMEOUT nc -k -l $PORT > "$pipe_file"
            exit_code=$?
        else
            log "INFO" "Starting netcat listener with keep-alive (no timeout control)"
            nc -k -l $PORT > "$pipe_file"
            exit_code=$?
        fi
    else
        # Without keep-alive, just use regular nc
        if command -v timeout &> /dev/null; then
            log "INFO" "Starting netcat listener without keep-alive (timeout controlled)"
            timeout $CONNECTION_TIMEOUT nc -l $PORT > "$pipe_file"
            exit_code=$?
        elif command -v gtimeout &> /dev/null; then
            log "INFO" "Starting netcat listener without keep-alive (gtimeout controlled)"
            gtimeout $CONNECTION_TIMEOUT nc -l $PORT > "$pipe_file"
            exit_code=$?
        else
            log "INFO" "Starting netcat listener without keep-alive (no timeout control)"
            nc -l $PORT > "$pipe_file"
            exit_code=$?
        fi
    fi
    
    # Clean up
    kill $bg_pid 2>/dev/null || true
    rm -f "$pipe_file"
    
    if [ $exit_code -eq 124 ] || [ $exit_code -eq 143 ]; then
        # Timeout expired (normal behavior for periodic reconnect)
        log "INFO" "Listener refreshing after $CONNECTION_TIMEOUT seconds (planned refresh)"
    elif [ $exit_code -ne 0 ]; then
        log "WARNING" "nc exited with code $exit_code. Reconnecting..."
        log "DEBUG" "Sleeping for 5 seconds before reconnecting"
        sleep 5  # Longer pause on error to prevent CPU thrashing
    else
        log "INFO" "Connection closed normally"
    fi
    
    # Brief pause before restarting to avoid hammering CPU if there's an issue
    log "DEBUG" "Pausing before restart"
    sleep 3
done

