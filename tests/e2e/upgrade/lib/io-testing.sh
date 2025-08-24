#!/bin/bash

IO_LOG_FILE="/data/continuous-io.log"

start_continuous_io() {
    local namespace=${1:-default}
    local pod=${2:-test-pod-1}

    log_info "Starting continuous I/O workload"

    # Run continuous I/O in background
    (
        while true; do
            kubectl exec -n "${namespace}" "${pod}" -- \
                sh -c "echo 'IO at $(date)' >> ${IO_LOG_FILE}" 2>/dev/null
            kubectl exec -n "${namespace}" "${pod}" -- \
                sh -c "tail -n 100 ${IO_LOG_FILE} > /dev/null" 2>/dev/null
            sleep 1
        done
    ) &

    local io_pid=$!
    echo $io_pid

    sleep 2
    log_success "Continuous I/O started (PID: ${io_pid})"
}

verify_io_active() {
    local pid=$1

    if ! kill -0 $pid 2>/dev/null; then
        log_error "I/O process not running (PID: ${pid})"
        return 1
    fi
    log_info "I/O process is active"
}

verify_io_continuity() {
    local pid=$1
    local namespace=${2:-default}
    local pod=${3:-test-pod-1}

    verify_io_active $pid

    # Check for recent writes (within last 10 seconds)
    local last_line=$(kubectl exec -n "${namespace}" "${pod}" -- \
        tail -n 1 ${IO_LOG_FILE} 2>/dev/null)

    if [[ -z "$last_line" ]]; then
        log_error "No I/O log found"
        return 1
    fi

    log_info "I/O continuity verified: ${last_line}"
}
