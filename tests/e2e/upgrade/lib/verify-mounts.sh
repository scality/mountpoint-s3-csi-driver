#!/bin/bash

verify_mounts_working() {
    local expected_type=$1
    local namespace=${2:-default}

    log_info "Verifying mounts are working (expected: ${expected_type})"

    # Get all test pods
    local pods=$(kubectl get pods -n "${namespace}" -l test=upgrade -o jsonpath='{.items[*].metadata.name}')

    if [[ -z "$pods" ]]; then
        log_error "No test pods found"
        return 1
    fi

    for pod in $pods; do
        log_info "Checking pod: ${pod}"

        # Test basic file operations
        if ! kubectl exec -n "${namespace}" "$pod" -- touch /data/test-file-$(date +%s); then
            log_error "Cannot write to mount in pod ${pod}"
            return 1
        fi

        if ! kubectl exec -n "${namespace}" "$pod" -- ls /data/ > /dev/null 2>&1; then
            log_error "Cannot list mount in pod ${pod}"
            return 1
        fi

        # Verify mount type if specified
        if [[ "$expected_type" != "any" ]]; then
            local actual_type=$(detect_mount_strategy "$pod" "$namespace")
            if [[ "$actual_type" != "$expected_type" ]]; then
                log_warning "Pod $pod has mount type: $actual_type (expected: $expected_type)"
            fi
        fi
    done

    log_success "All mounts are working"
}
