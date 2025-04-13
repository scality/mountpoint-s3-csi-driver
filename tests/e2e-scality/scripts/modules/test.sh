#!/bin/bash
# test.sh - Test functions for e2e-scality scripts

# Source common functions
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

# Get the project root directory
get_project_root() {
  # Four levels up from this script (modules -> scripts -> e2e-scality -> tests -> root)
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../" && pwd)"
}

# Run Go tests
run_go_tests() {
  local project_root=$(get_project_root)
  local e2e_tests_dir="${project_root}/tests/e2e-scality/e2e-tests"
  local test_args="$1"
  local focus_pattern="$2"
  local skip_pattern="$3"
  local tags="$4"
  local namespace="$5"
  
  log "Running Go-based end-to-end tests for Scality CSI driver..."
  
  # Check if Go is installed
  if ! command -v go &> /dev/null; then
    error "Go is not installed. Please install Go to run the tests."
    return 1
  fi
  
  # Check if the tests directory exists
  if [ ! -d "$e2e_tests_dir" ]; then
    error "End-to-end tests directory not found: $e2e_tests_dir"
    return 1
  fi
  
  # Use default tags if not specified
  if [ -z "$tags" ]; then
    tags="e2e"
  fi
  
  # Build the go test command
  local go_test_cmd="go test -v -tags=$tags"
  
  # Add focus pattern if provided
  if [ -n "$focus_pattern" ]; then
    go_test_cmd="$go_test_cmd -ginkgo.focus=\"$focus_pattern\""
  fi
  
  # Add skip pattern if provided
  if [ -n "$skip_pattern" ]; then
    go_test_cmd="$go_test_cmd -ginkgo.skip=\"$skip_pattern\""
  fi
  
  # Add namespace if provided
  if [ -n "$namespace" ]; then
    go_test_cmd="$go_test_cmd -namespace=\"$namespace\""
  fi
  
  # Add any additional test arguments
  if [ -n "$test_args" ]; then
    go_test_cmd="$go_test_cmd $test_args"
  fi
  
  # Add the test directory
  go_test_cmd="$go_test_cmd ./..."
  
  # Run the Go tests
  log "Executing Go tests in $e2e_tests_dir"
  log "Test command: $go_test_cmd"
  
  if ! (cd "$e2e_tests_dir" && eval "$go_test_cmd"); then
    error "Go tests failed with exit code $?"
    return 1
  fi
  
  log "Go tests completed successfully."
  return 0
}

# Run basic verification tests
run_verification_tests() {
  log "Verifying Scality CSI driver installation..."
  
  # Check if the CSI driver pods are running
  if exec_cmd kubectl get pods -n mount-s3 | grep -q "Running"; then
    log "CSI driver pods are running properly."
  else
    error "Some CSI driver pods are not in Running state."
    exec_cmd kubectl get pods -n mount-s3
    return 1
  fi
  
  # Check if the CSI driver is registered
  if exec_cmd kubectl get csidrivers | grep -q "s3.csi.aws.com"; then
    log "CSI driver is registered properly."
  else
    error "CSI driver is not registered properly."
    return 1
  fi
  
  log "Basic verification tests passed."
  return 0
}

# Main test function that will be called from run.sh
do_test() {
  log "Starting Scality CSI driver tests..."
  
  local skip_go_tests=false
  local skip_verification=false
  local go_test_args=""
  local focus_pattern=""
  local skip_pattern=""
  local tags=""
  local namespace=""
  
  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-go-tests)
        skip_go_tests=true
        shift
        ;;
      --skip-verification)
        skip_verification=true
        shift
        ;;
      --go-test-args)
        go_test_args="$2"
        shift 2
        ;;
      --focus)
        focus_pattern="$2"
        shift 2
        ;;
      --skip)
        skip_pattern="$2"
        shift 2
        ;;
      --tags)
        tags="$2"
        shift 2
        ;;
      --namespace)
        namespace="$2"
        shift 2
        ;;
      *)
        error "Unknown parameter: $1"
        shift
        ;;
    esac
  done
  
  # Run basic verification tests unless skipped
  if [ "$skip_verification" != "true" ]; then
    if ! run_verification_tests; then
      error "Verification tests failed. Cannot proceed with Go tests."
      return 1
    fi
  else
    log "Skipping verification tests as requested."
  fi
  
  # Run Go-based tests if not skipped
  if [ "$skip_go_tests" != "true" ]; then
    if ! run_go_tests "$go_test_args" "$focus_pattern" "$skip_pattern" "$tags" "$namespace"; then
      error "Go tests failed."
      return 1
    fi
  else
    log "Skipping Go-based end-to-end tests as requested."
  fi
  
  log "All tests completed successfully."
}
