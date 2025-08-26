#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source all helper libraries
source "${SCRIPT_DIR}/lib/logging.sh"
source "${SCRIPT_DIR}/lib/phase-tracker.sh"
source "${SCRIPT_DIR}/lib/verify-driver.sh"
source "${SCRIPT_DIR}/lib/detect-mount.sh"
source "${SCRIPT_DIR}/lib/verify-mounts.sh"
source "${SCRIPT_DIR}/lib/test-data.sh"
source "${SCRIPT_DIR}/lib/io-testing.sh"
source "${SCRIPT_DIR}/lib/credential-check.sh"
source "${SCRIPT_DIR}/lib/install-driver.sh"
source "${SCRIPT_DIR}/lib/workload-management.sh"
source "${SCRIPT_DIR}/lib/stability-test.sh"

# Default values
FROM_VERSION="${FROM_VERSION:-v1.2.0}"
TO_VERSION="${TO_VERSION:-local}"
TO_IMAGE=""
TEST_DURATION_MINUTES="${TEST_DURATION_MINUTES:-30}"
NAMESPACE="scality-s3-csi"
S3_ENDPOINT_URL="${S3_ENDPOINT_URL:-http://s3.scality.com:8000}"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --from-version)
            FROM_VERSION="$2"
            shift 2
            ;;
        --to-version)
            TO_VERSION="$2"
            shift 2
            ;;
        --to-image)
            TO_IMAGE="$2"
            shift 2
            ;;
        --test-duration)
            TEST_DURATION_MINUTES="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --s3-endpoint-url)
            S3_ENDPOINT_URL="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --from-version VERSION    Version to upgrade from (default: v1.2.0)"
            echo "  --to-version VERSION      Version to upgrade to (default: local)"
            echo "  --to-image IMAGE          Docker image for upgrade"
            echo "  --test-duration MINUTES   Test duration in minutes (default: 30)"
            echo "  --namespace NAMESPACE     Kubernetes namespace (default: scality-s3-csi)"
            echo "  --s3-endpoint-url URL     S3 endpoint URL"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Main test execution
main() {
    log_title "Upgrade Test: ${FROM_VERSION} → ${TO_VERSION}"
    log_info "Test duration: ${TEST_DURATION_MINUTES} minutes"
    log_info "S3 endpoint: ${S3_ENDPOINT_URL}"
    log_info "Namespace: ${NAMESPACE}"

    # Phase 1: Install old version
    log_phase "1. Installing CSI Driver ${FROM_VERSION}"
    install_old_version "${FROM_VERSION}" "${NAMESPACE}" "${S3_ENDPOINT_URL}"
    verify_driver_ready "${NAMESPACE}"
    record_phase "install_old_version" "SUCCESS"

    # Phase 2: Create test workloads
    log_phase "2. Creating test workloads"
    create_test_workloads default "${S3_ENDPOINT_URL}"
    verify_mounts_working "systemd"
    record_phase "create_workloads" "SUCCESS"

    # Phase 3: Write test data
    log_phase "3. Writing test data"
    write_test_data
    calculate_checksums
    record_phase "write_test_data" "SUCCESS"

    # Phase 4: Start continuous I/O
    log_phase "4. Starting continuous I/O"
    IO_PID=$(start_continuous_io)
    sleep 5  # Let I/O start
    verify_io_active $IO_PID
    record_phase "start_io" "SUCCESS"

    # Phase 5: Perform upgrade
    log_phase "5. Upgrading to ${TO_VERSION}"
    upgrade_driver "${TO_VERSION}" "${TO_IMAGE}" "${NAMESPACE}"
    verify_driver_ready "${NAMESPACE}"
    record_phase "upgrade_driver" "SUCCESS"

    # Phase 6: Verify existing mounts
    log_phase "6. Verifying existing mounts post-upgrade"
    verify_mounts_working "systemd"  # Old mounts should still use systemd
    verify_data_integrity
    verify_io_continuity $IO_PID
    record_phase "verify_existing_mounts" "SUCCESS"

    # Phase 7: Create new workload
    log_phase "7. Creating new workload"
    create_new_workload
    NEW_MOUNT_STRATEGY=$(detect_mount_strategy "test-pod-2")
    log_info "New workload using: ${NEW_MOUNT_STRATEGY}"
    record_phase "create_new_workload" "SUCCESS"

    # Phase 8: Run stability test
    log_phase "8. Running stability test (${TEST_DURATION_MINUTES} minutes)"
    run_stability_test "${TEST_DURATION_MINUTES}" "${IO_PID}"
    record_phase "stability_test" "SUCCESS"

    # Phase 9: Cleanup
    log_phase "9. Cleanup"
    kill $IO_PID 2>/dev/null || true
    cleanup_workloads
    record_phase "cleanup" "SUCCESS"

    # Print summary
    print_test_summary
    log_success "Upgrade test completed successfully!"
}

# Run main function
main "$@"
