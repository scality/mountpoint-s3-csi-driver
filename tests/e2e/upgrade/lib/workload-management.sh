#!/bin/bash

create_test_workloads() {
    local namespace=${1:-default}
    local s3_endpoint=${2:-http://s3.scality.com:8000}

    log_info "Creating test workloads"

    # Create test buckets first
    log_info "Creating test buckets"
    AWS_ACCESS_KEY_ID="${ACCOUNT1_ACCESS_KEY}" AWS_SECRET_ACCESS_KEY="${ACCOUNT1_SECRET_KEY}" \
        aws s3 mb s3://upgrade-test-bucket-1 --endpoint-url "${s3_endpoint}" || true
    AWS_ACCESS_KEY_ID="${ACCOUNT1_ACCESS_KEY}" AWS_SECRET_ACCESS_KEY="${ACCOUNT1_SECRET_KEY}" \
        aws s3 mb s3://upgrade-test-bucket-2 --endpoint-url "${s3_endpoint}" || true

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
