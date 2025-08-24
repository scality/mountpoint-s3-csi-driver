#!/bin/bash

run_stability_test() {
    local duration_minutes=$1
    local io_pid=$2
    local namespace=${3:-default}

    log_info "Running stability test for ${duration_minutes} minutes"

    local end_time=$(($(date +%s) + duration_minutes * 60))
    local check_interval=60  # Check every minute
    local checks_performed=0

    while [[ $(date +%s) -lt $end_time ]]; do
        checks_performed=$((checks_performed + 1))

        log_info "Health check ${checks_performed} at $(date)"

        # Perform periodic health checks
        periodic_health_check "${namespace}" "${io_pid}"

        # Check for credential refresh at specific intervals
        # After 70 minutes and 130 minutes (credential refresh happens every hour)
        if [[ $checks_performed -eq 70 ]] || [[ $checks_performed -eq 130 ]]; then
            log_info "Checking credential refresh..."
            verify_credential_refresh "${namespace}"
        fi

        sleep $check_interval
    done

    log_success "Stability test completed"
}

periodic_health_check() {
    local namespace=$1
    local io_pid=$2

    # Check mounts are still working
    if ! verify_mounts_working "any" "${namespace}"; then
        log_error "Mount verification failed during stability test"
        return 1
    fi

    # Check data integrity
    if ! verify_data_integrity "${namespace}"; then
        log_error "Data integrity check failed during stability test"
        return 1
    fi

    # Check I/O continuity if provided
    if [[ -n "${io_pid}" ]]; then
        if ! verify_io_continuity "${io_pid}" "${namespace}"; then
            log_error "I/O continuity check failed during stability test"
            return 1
        fi
    fi

    log_info "Health check passed"
}
