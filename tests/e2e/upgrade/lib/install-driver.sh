#!/bin/bash

install_old_version() {
    local version=$1
    local namespace=$2
    local s3_endpoint=${3:-http://s3.scality.com:8000}

    log_info "Installing CSI Driver version ${version}"

    # Add Helm repo if not exists
    helm repo add scality https://scality.github.io/mountpoint-s3-csi-driver 2>/dev/null || true
    helm repo update

    # Install specific version
    helm install scality-mountpoint-s3-csi-driver \
        scality/scality-mountpoint-s3-csi-driver \
        --version "${version}" \
        --namespace "${namespace}" \
        --create-namespace \
        --set s3.endpointUrl="${s3_endpoint}" \
        --set s3CredentialSecret.accessKeyId="${ACCOUNT1_ACCESS_KEY}" \
        --set s3CredentialSecret.secretAccessKey="${ACCOUNT1_SECRET_KEY}" \
        --wait --timeout 5m

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
        # Upgrade to specific version
        helm upgrade scality-mountpoint-s3-csi-driver \
            scality/scality-mountpoint-s3-csi-driver \
            --version "${to_version}" \
            --namespace "${namespace}" \
            ${upgrade_args} \
            --wait --timeout 10m
    fi

    log_success "CSI Driver upgraded to ${to_version}"
}
