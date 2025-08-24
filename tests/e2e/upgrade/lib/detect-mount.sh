#!/bin/bash

detect_mount_strategy() {
    local pod=$1
    local namespace=${2:-default}

    # Get the node where pod is running
    local node=$(kubectl get pod -n "${namespace}" "${pod}" -o jsonpath='{.spec.nodeName}' 2>/dev/null)

    if [[ -z "$node" ]]; then
        echo "unknown"
        return 1
    fi

    # Check for systemd mount by looking at mount points on the node
    # We use kubectl debug to check the node
    local mount_info=$(kubectl debug node/"${node}" -it --image=busybox -- \
        sh -c "mount | grep mount-s3" 2>/dev/null || true)

    if [[ -n "$mount_info" ]]; then
        echo "systemd"
        return 0
    fi

    # Check for Pod Mounter by looking for mountpoint pods
    local pod_uid=$(kubectl get pod -n "${namespace}" "${pod}" -o jsonpath='{.metadata.uid}' 2>/dev/null)

    if [[ -n "$pod_uid" ]]; then
        # Check if there's a mountpoint pod for this workload
        local mp_pods=$(kubectl get pods -n mount-s3 \
            -l "s3.csi.scality.com/pod-uid=${pod_uid}" \
            -o name 2>/dev/null | grep "mp-" || true)

        if [[ -n "$mp_pods" ]]; then
            echo "podmounter"
            return 0
        fi
    fi

    # Check if mount-s3 namespace exists (indicates Pod Mounter capability)
    if kubectl get namespace mount-s3 2>/dev/null; then
        echo "podmounter-capable"
        return 0
    fi

    echo "unknown"
}
