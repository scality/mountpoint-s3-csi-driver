#!/bin/bash

create_test_workloads() {
    local namespace=${1:-default}
    local s3_endpoint=${2:-http://s3.scality.com:8000}

    log_info "Creating test workloads"

    # Create test buckets first
    log_info "Creating test buckets via Kubernetes Job"
    
    # Update the Job with the correct S3 endpoint if provided
    if [[ "${s3_endpoint}" != "http://s3.scality.com:8000" ]]; then
        sed -i.bak "s|http://s3.scality.com:8000|${s3_endpoint}|g" tests/e2e/upgrade/fixtures/create-buckets-job.yaml
    fi
    
    # Create the bucket creation job
    kubectl apply -f tests/e2e/upgrade/fixtures/create-buckets-job.yaml
    
    # Wait for job to complete
    if kubectl wait --for=condition=complete job/create-test-buckets --timeout=60s; then
        log_success "Test buckets created"
    else
        log_error "Failed to create test buckets"
        kubectl logs job/create-test-buckets
    fi
    
    # Clean up the job
    kubectl delete job/create-test-buckets --ignore-not-found=true

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
