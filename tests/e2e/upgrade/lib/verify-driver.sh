#!/bin/bash

verify_driver_ready() {
    local namespace=$1
    local timeout=${2:-300}

    log_info "Verifying CSI driver is ready in namespace: ${namespace}"

    # Check DaemonSet is ready
    if ! kubectl rollout status daemonset/s3-csi-node -n "${namespace}" --timeout="${timeout}s"; then
        log_error "CSI node DaemonSet not ready"
        return 1
    fi

    # Check if controller is deployed and ready
    if kubectl get deployment/s3-csi-controller -n "${namespace}" 2>/dev/null; then
        if ! kubectl rollout status deployment/s3-csi-controller -n "${namespace}" --timeout="${timeout}s"; then
            log_error "CSI controller Deployment not ready"
            return 1
        fi
    fi

    # Verify CSI driver is registered
    if ! kubectl get csidrivers s3.csi.scality.com; then
        log_error "CSI driver not registered"
        return 1
    fi

    log_success "CSI driver is ready"
}
