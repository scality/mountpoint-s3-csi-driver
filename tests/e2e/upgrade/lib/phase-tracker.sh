#!/bin/bash

# Track test phases for reporting
PHASE_RESULTS=()

record_phase() {
    local phase=$1
    local status=$2
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    PHASE_RESULTS+=("${timestamp}|${phase}|${status}")

    if [[ "$status" == "FAILED" ]]; then
        log_error "Phase failed: ${phase}"
        print_test_summary
        exit 1
    fi
    log_success "Phase completed: ${phase}"
}

print_test_summary() {
    echo ""
    echo "================================"
    echo "Upgrade Test Summary"
    echo "================================"
    echo "Timestamp|Phase|Status"
    echo "---------|-----|------"
    for result in "${PHASE_RESULTS[@]}"; do
        echo "$result"
    done
    echo "================================"
}
