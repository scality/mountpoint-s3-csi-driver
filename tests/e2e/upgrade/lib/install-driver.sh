#!/bin/bash

install_old_version() {
    local version=$1
    local namespace=$2
    local s3_endpoint=${3:-http://s3.scality.com:8000}

    log_info "Installing CSI Driver version ${version}"

    log_info "Installing from OCI Helm chart"
    
    # Strip 'v' prefix from version for OCI chart (e.g., v1.2.0 -> 1.2.0)
    local chart_version="${version#v}"
    
    log_info "Chart version: ${chart_version}"
    log_info "S3 endpoint: ${s3_endpoint}"
    log_info "Namespace: ${namespace}"
    
    # Create s3-secret before Helm installation
    log_info "Creating s3-secret in namespace ${namespace}..."
    kubectl create secret generic s3-secret \
        --from-literal=accessKeyId="${ACCOUNT1_ACCESS_KEY}" \
        --from-literal=secretAccessKey="${ACCOUNT1_SECRET_KEY}" \
        --namespace="${namespace}" \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Install specific version from OCI registry with verbose logging
    log_info "Starting Helm installation..."
    if ! helm install scality-mountpoint-s3-csi-driver \
        oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
        --version "${chart_version}" \
        --namespace "${namespace}" \
        --create-namespace \
        --set s3.endpointUrl="${s3_endpoint}" \
        --set s3CredentialSecret.accessKeyId="${ACCOUNT1_ACCESS_KEY}" \
        --set s3CredentialSecret.secretAccessKey="${ACCOUNT1_SECRET_KEY}" \
        --wait --timeout 10m \
        --debug; then
        
        log_error "Helm installation failed, gathering debug information..."
        
        # Get Helm status
        helm status scality-mountpoint-s3-csi-driver -n "${namespace}" || true
        
        # Get pod status
        log_info "Pod status in namespace ${namespace}:"
        kubectl get pods -n "${namespace}" -o wide || true
        
        # Get events
        log_info "Recent events in namespace ${namespace}:"
        kubectl get events -n "${namespace}" --sort-by='.lastTimestamp' || true
        
        # Get logs from any failed pods
        log_info "Logs from CSI driver pods:"
        kubectl logs -n "${namespace}" -l app=s3-csi-node --all-containers --tail=50 || true
        kubectl logs -n "${namespace}" -l app=s3-csi-controller --all-containers --tail=50 || true
        
        return 1
    fi

    log_success "CSI Driver ${version} installed"
}

upgrade_driver() {
    local to_version=$1
    local to_image=$2
    local namespace=$3

    log_info "Upgrading CSI Driver to ${to_version}"

    local upgrade_args="--reuse-values"

    if [[ -n "${to_image}" ]]; then
        # Extract repository and tag from image
        local repo=$(echo "${to_image}" | cut -d: -f1)
        local tag=$(echo "${to_image}" | cut -d: -f2)
        upgrade_args="${upgrade_args} --set image.repository=${repo} --set image.tag=${tag}"
    fi

    if [[ "${to_version}" == "local" ]]; then
        # Upgrade to local chart
        helm upgrade scality-mountpoint-s3-csi-driver \
            ./charts/scality-mountpoint-s3-csi-driver \
            --namespace "${namespace}" \
            ${upgrade_args} \
            --wait --timeout 10m
    else
        # Strip 'v' prefix from version for OCI chart
        local chart_version="${to_version#v}"
        
        # Upgrade to specific version from OCI registry
        helm upgrade scality-mountpoint-s3-csi-driver \
            oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts/scality-mountpoint-s3-csi-driver \
            --version "${chart_version}" \
            --namespace "${namespace}" \
            ${upgrade_args} \
            --wait --timeout 10m
    fi

    log_success "CSI Driver upgraded to ${to_version}"
}
