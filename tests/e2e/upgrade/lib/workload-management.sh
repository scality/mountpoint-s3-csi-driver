#!/bin/bash

create_test_workloads() {
    local namespace=${1:-default}

    log_info "Creating test workloads"

    # Apply old workload fixture
    kubectl apply -f tests/e2e/upgrade/fixtures/old-workload.yaml

    # Wait for pod to be ready
    if ! kubectl wait --for=condition=Ready pod/test-pod-1 \
        --namespace "${namespace}" --timeout=300s; then
        log_error "Test pod 1 not ready"
        return 1
    fi

    log_success "Test workloads created"
}

create_new_workload() {
    local namespace=${1:-default}

    log_info "Creating new workload (post-upgrade)"

    # Apply new workload fixture
    kubectl apply -f tests/e2e/upgrade/fixtures/new-workload.yaml

    # Wait for pod to be ready
    if ! kubectl wait --for=condition=Ready pod/test-pod-2 \
        --namespace "${namespace}" --timeout=300s; then
        log_error "Test pod 2 not ready"
        return 1
    fi

    log_success "New workload created"
}

cleanup_workloads() {
    log_info "Cleaning up test workloads"

    kubectl delete -f tests/e2e/upgrade/fixtures/ --ignore-not-found=true

    log_success "Workloads cleaned up"
}
