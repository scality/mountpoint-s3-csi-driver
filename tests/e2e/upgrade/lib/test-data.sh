#!/bin/bash

TEST_FILES_LIST="/tmp/upgrade-test-files.txt"
TEST_CHECKSUMS="/tmp/upgrade-test-checksums.txt"

write_test_data() {
    local namespace=${1:-default}
    local pod=${2:-test-pod-1}

    log_info "Writing test data"

    > "${TEST_FILES_LIST}"

    # Write test files with timestamps
    for i in {1..10}; do
        local filename="test-data-${i}-$(date +%s).txt"
        local content="Test data ${i} written at $(date)"

        kubectl exec -n "${namespace}" "${pod}" -- \
            sh -c "echo '${content}' > /data/${filename}"

        echo "${filename}" >> "${TEST_FILES_LIST}"
    done

    log_success "Test data written"
}

calculate_checksums() {
    local namespace=${1:-default}
    local pod=${2:-test-pod-1}

    log_info "Calculating checksums"

    > "${TEST_CHECKSUMS}"

    while read filename; do
        local checksum=$(kubectl exec -n "${namespace}" "${pod}" -- \
            sh -c "md5sum /data/${filename}" | awk '{print $1}')
        echo "${filename}:${checksum}" >> "${TEST_CHECKSUMS}"
    done < "${TEST_FILES_LIST}"

    log_success "Checksums calculated"
}

verify_data_integrity() {
    local namespace=${1:-default}
    local pod=${2:-test-pod-1}

    log_info "Verifying data integrity"

    while IFS=: read filename checksum; do
        local actual=$(kubectl exec -n "${namespace}" "${pod}" -- \
            sh -c "md5sum /data/${filename}" 2>/dev/null | awk '{print $1}')

        if [[ "$checksum" != "$actual" ]]; then
            log_error "Checksum mismatch for ${filename}"
            return 1
        fi
    done < "${TEST_CHECKSUMS}"

    log_success "Data integrity verified"
}
