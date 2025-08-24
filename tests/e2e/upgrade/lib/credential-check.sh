#!/bin/bash

verify_credential_refresh() {
    local namespace=${1:-default}
    local pod=${2:-test-pod-1}

    log_info "Verifying credential refresh"

    # Test that pods can still access S3 after credential refresh period
    local test_file="cred-refresh-test-$(date +%s).txt"

    if ! kubectl exec -n "${namespace}" "${pod}" -- \
        touch "/data/${test_file}" 2>/dev/null; then
        log_error "Cannot write after credential refresh period"
        return 1
    fi

    if ! kubectl exec -n "${namespace}" "${pod}" -- \
        ls "/data/${test_file}" 2>/dev/null; then
        log_error "Cannot read after credential refresh period"
        return 1
    fi

    log_success "Credential refresh verified"
}
