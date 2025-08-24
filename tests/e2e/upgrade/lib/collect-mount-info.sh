#!/bin/bash

collect_mount_info() {
    echo "=== Mount Information Collection ==="
    echo "Time: $(date)"
    echo ""

    echo "=== CSI Driver Pods ==="
    kubectl get pods -n scality-s3-csi -o wide 2>/dev/null || \
        kubectl get pods -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o wide

    echo ""
    echo "=== Systemd Mounts on Nodes ==="
    for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
        echo "Node: $node"
        kubectl debug node/$node -it --image=busybox -- \
            sh -c "mount | grep mount-s3" 2>/dev/null || echo "  No systemd mounts"
    done

    echo ""
    echo "=== Pod Mounter Namespace ==="
    if kubectl get namespace mount-s3 2>/dev/null; then
        echo "Pod Mounter namespace exists"
        kubectl get pods -n mount-s3 -o wide 2>/dev/null || echo "  No mountpoint pods"
    else
        echo "Pod Mounter namespace not found"
    fi

    echo ""
    echo "=== Test Workloads ==="
    kubectl get pods -l test=upgrade -o wide

    echo ""
    echo "=== Mount Points in Test Pods ==="
    for pod in $(kubectl get pods -l test=upgrade -o jsonpath='{.items[*].metadata.name}'); do
        echo "Pod: $pod"
        kubectl exec $pod -- mount | grep "/data" 2>/dev/null || echo "  Mount point not found"
        kubectl exec $pod -- ls -la /data 2>/dev/null | head -5 || echo "  Cannot list /data"
    done

    echo ""
    echo "=== CSI Driver Version ==="
    kubectl get pods -n scality-s3-csi -l app=s3-csi-node -o jsonpath='{.items[0].spec.containers[0].image}' 2>/dev/null || \
        kubectl get pods -n kube-system -l app.kubernetes.io/name=scality-mountpoint-s3-csi-driver -o jsonpath='{.items[0].spec.containers[0].image}'
}

# If script is run directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    collect_mount_info
fi
