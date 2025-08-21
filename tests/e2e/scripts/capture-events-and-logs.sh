#!/bin/bash
set -euo pipefail

# capture-events-and-logs.sh - Capture Kubernetes events and correlate with resource logs
# Usage: ./capture-events-and-logs.sh [output_directory] [start|stop]

OUTPUT_DIR="${1:-./debug-capture}"
CAPTURE_PID_FILE="$OUTPUT_DIR/capture.pid"

# Create output directory structure
mkdir -p "$OUTPUT_DIR"/{events,resources,logs,describes}

# Function to log with timestamp
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$OUTPUT_DIR/capture.log"
}

# Function to capture resource context when events occur
capture_resource_context() {
    local namespace="$1"
    local kind="$2"
    local name="$3"
    local event_type="$4"
    local reason="$5"
    local message="$6"

    log "Capturing context for $kind/$name in namespace $namespace due to $event_type: $reason"

    # Create safe filename
    local safe_name="${namespace}-${kind,,}-${name//\//_}"

    # Capture resource YAML
    if kubectl get "$kind" "$name" -n "$namespace" -o yaml > "$OUTPUT_DIR/resources/${safe_name}.yaml" 2>/dev/null; then
        log "✓ Captured resource YAML: $safe_name.yaml"
    fi

    # Capture resource description (includes events)
    if kubectl describe "$kind" "$name" -n "$namespace" > "$OUTPUT_DIR/describes/${safe_name}.txt" 2>/dev/null; then
        log "✓ Captured resource description: $safe_name.txt"
    fi

    # Capture logs if it's a Pod
    if [[ "$kind" == "Pod" ]]; then
        # Current logs
        if kubectl logs -n "$namespace" "$name" --all-containers=true --timestamps=true > "$OUTPUT_DIR/logs/${safe_name}-current.log" 2>/dev/null; then
            log "✓ Captured current pod logs: ${safe_name}-current.log"
        fi

        # Previous logs (if pod restarted)
        if kubectl logs -n "$namespace" "$name" --all-containers=true --previous=true --timestamps=true > "$OUTPUT_DIR/logs/${safe_name}-previous.log" 2>/dev/null; then
            log "✓ Captured previous pod logs: ${safe_name}-previous.log"
        fi
    fi

    # Capture related events for this specific resource
    kubectl get events -n "$namespace" \
        --field-selector "involvedObject.name=$name,involvedObject.kind=$kind" \
        -o json > "$OUTPUT_DIR/events/${safe_name}-events.json" 2>/dev/null || true
}

# Function to start monitoring
start_monitoring() {
    log "Starting Kubernetes event and log capture in: $OUTPUT_DIR"

    # Initialize event stream file
    echo > "$OUTPUT_DIR/events/all-events.jsonl"

    # Start watching events
    kubectl get events --all-namespaces -w -o json | while read -r line; do
        # Skip empty lines
        [[ -z "$line" ]] && continue

        # Write event to stream file
        echo "$line" >> "$OUTPUT_DIR/events/all-events.jsonl"

        # Try to parse the event
        if ! event_json=$(echo "$line" | jq -r 'select(.type == "ADDED" or .type == "MODIFIED") | .object' 2>/dev/null); then
            continue
        fi

        # Skip if no event object
        [[ "$event_json" == "null" || -z "$event_json" ]] && continue

        # Extract event details
        if ! event_type=$(echo "$event_json" | jq -r '.type // ""' 2>/dev/null) ||
           ! reason=$(echo "$event_json" | jq -r '.reason // ""' 2>/dev/null) ||
           ! message=$(echo "$event_json" | jq -r '.message // ""' 2>/dev/null) ||
           ! namespace=$(echo "$event_json" | jq -r '.involvedObject.namespace // "default"' 2>/dev/null) ||
           ! kind=$(echo "$event_json" | jq -r '.involvedObject.kind // ""' 2>/dev/null) ||
           ! name=$(echo "$event_json" | jq -r '.involvedObject.name // ""' 2>/dev/null); then
            continue
        fi

        # Skip empty values
        [[ -z "$kind" || -z "$name" ]] && continue

        # Capture context for Warning and Error events
        if [[ "$event_type" == "Warning" || "$event_type" == "Error" ]]; then
            capture_resource_context "$namespace" "$kind" "$name" "$event_type" "$reason" "$message"
        fi

        # Also capture context for certain critical reasons regardless of type
        case "$reason" in
            "Failed"|"FailedMount"|"FailedAttachVolume"|"VolumeFailedMount"|"BackOff"|"CrashLoopBackOff"|"ImagePullBackOff"|"ErrImagePull"|"InvalidImageName")
                capture_resource_context "$namespace" "$kind" "$name" "$event_type" "$reason" "$message"
                ;;
        esac

    done &

    # Store the background process PID
    echo $! > "$CAPTURE_PID_FILE"
    log "Event monitoring started with PID: $(cat "$CAPTURE_PID_FILE")"

    # Also capture periodic cluster state snapshots
    while true; do
        sleep 30
        log "Taking periodic cluster state snapshot..."

        # Capture current pod states
        kubectl get pods --all-namespaces -o json > "$OUTPUT_DIR/snapshots/pods-$(date +%s).json" 2>/dev/null || true

        # Capture current PVC states
        kubectl get pvc --all-namespaces -o json > "$OUTPUT_DIR/snapshots/pvcs-$(date +%s).json" 2>/dev/null || true

        # Capture current events summary
        kubectl get events --all-namespaces --sort-by='.lastTimestamp' > "$OUTPUT_DIR/snapshots/events-$(date +%s).txt" 2>/dev/null || true

    done &

    # Store the snapshot process PID
    echo $! > "$OUTPUT_DIR/snapshot.pid"
}

# Function to stop monitoring
stop_monitoring() {
    log "Stopping Kubernetes event and log capture..."

    # Stop event monitoring
    if [[ -f "$CAPTURE_PID_FILE" ]]; then
        local capture_pid=$(cat "$CAPTURE_PID_FILE")
        if kill "$capture_pid" 2>/dev/null; then
            log "✓ Stopped event monitoring (PID: $capture_pid)"
        fi
        rm -f "$CAPTURE_PID_FILE"
    fi

    # Stop snapshot monitoring
    if [[ -f "$OUTPUT_DIR/snapshot.pid" ]]; then
        local snapshot_pid=$(cat "$OUTPUT_DIR/snapshot.pid")
        if kill "$snapshot_pid" 2>/dev/null; then
            log "✓ Stopped snapshot monitoring (PID: $snapshot_pid)"
        fi
        rm -f "$OUTPUT_DIR/snapshot.pid"
    fi

    # Take final cluster state snapshot
    log "Taking final cluster state snapshot..."
    kubectl cluster-info dump --all-namespaces --output-directory="$OUTPUT_DIR/final-state" 2>/dev/null || true

    # Capture final events
    kubectl get events --all-namespaces -o json > "$OUTPUT_DIR/events/final-events.json" 2>/dev/null || true

    log "Capture complete. Output saved to: $OUTPUT_DIR"
}

# Main execution
case "${2:-start}" in
    "start")
        # Create necessary subdirectories
        mkdir -p "$OUTPUT_DIR"/{events,resources,logs,describes,snapshots}
        start_monitoring
        ;;
    "stop")
        stop_monitoring
        ;;
    *)
        echo "Usage: $0 <output_directory> [start|stop]"
        echo "  start: Begin capturing events and logs (default)"
        echo "  stop:  Stop capturing and take final snapshot"
        exit 1
        ;;
esac
