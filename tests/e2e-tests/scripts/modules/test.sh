#!/bin/bash
# test.sh - Test functions for e2e-tests scripts

# Source common functions
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

# Default namespace value
DEFAULT_NAMESPACE="kube-system"

# Run Go tests
run_go_tests() {
  local project_root=$(get_project_root)
  local e2e_tests_dir="${project_root}/tests/e2e-tests"
  local namespace="${1:-$DEFAULT_NAMESPACE}"
  local junit_report="$2"
  local kubectl_path="$3"
  local s3_endpoint_url="$4"
  local access_key_id="$5"
  local secret_access_key="$6"
  local bucket_prefix="${7:-e2e-test}"
  local additional_args="${8:-}"
  
  log "Running Go-based end-to-end tests for Scality CSI driver in namespace: $namespace..."
  
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
  
  # Check if kubectl path is provided
  if [ -z "$kubectl_path" ]; then
    error "kubectl path is required. Please specify the --kubectl-path parameter."
    return 1
  fi
  
  # Check if kubectl exists
  if [ ! -f "$kubectl_path" ] || [ ! -x "$kubectl_path" ]; then
    error "kubectl not found at specified path: $kubectl_path. Please provide a valid path to the kubectl binary."
    return 1
  fi
  
  # Check if S3 endpoint URL is provided
  if [ -z "$s3_endpoint_url" ]; then
    error "S3 endpoint URL is required. Please specify the --s3-endpoint-url parameter."
    return 1
  fi
  
  # Check if access key ID is provided
  if [ -z "$access_key_id" ]; then
    error "S3 access key ID is required. Please specify the --access-key-id parameter."
    return 1
  fi
  
  # Check if secret access key is provided
  if [ -z "$secret_access_key" ]; then
    error "S3 secret access key is required. Please specify the --secret-access-key parameter."
    return 1
  fi
  
  # Get kubeconfig path
  local kubeconfig_path="${KUBECONFIG:-$HOME/.kube/config}"
  if [ ! -f "$kubeconfig_path" ]; then
    error "Kubeconfig file not found: $kubeconfig_path. Please ensure the KUBECONFIG environment variable is set correctly."
    return 1
  fi
  
  # Build the Go test command with enhanced verbosity
  local go_test_cmd="go test -v ./... -ginkgo.v -s3-endpoint-url=$s3_endpoint_url -access-key-id=$access_key_id -secret-access-key=$secret_access_key -kubectl-path=$kubectl_path -kubeconfig=$kubeconfig_path -bucket-prefix=$bucket_prefix $additional_args"
  
  # Add JUnit report if specified
  if [ -n "$junit_report" ]; then
    log "Using JUnit report file: $junit_report"
    
    # Handle absolute and relative paths
    local junit_absolute_path
    
    # If path is absolute, use it directly
    if [[ "$junit_report" = /* ]]; then
      junit_absolute_path="$junit_report"
    else
      # For relative paths, determine if we need to adjust the path based on the CWD
      # If path starts with ./ then make it relative to the e2e-tests directory
      if [[ "$junit_report" = ./* ]]; then
        # For paths starting with ./, keep them relative to the test directory
        junit_absolute_path="$junit_report"
        log "Using relative path from e2e-tests directory: $junit_absolute_path"
      else
        # For other paths (like just a filename), ensure they're created in the test directory
        junit_absolute_path="./$junit_report"
        log "Adjusted path to be relative to e2e-tests directory: $junit_absolute_path"
      fi
    fi
    
    # Create the output directory if it doesn't exist
    local junit_dir=$(dirname "$junit_absolute_path")
    if [ ! -d "$junit_dir" ] && [ "$junit_dir" != "." ]; then
      log "Creating output directory for JUnit report: $junit_dir"
      mkdir -p "$junit_dir"
    fi
    
    # Use the correct format for Ginkgo JUnit report
    go_test_cmd="$go_test_cmd -ginkgo.junit-report=$junit_absolute_path"
    log "Final JUnit report path: $junit_absolute_path"
  fi
  
  # Run the Go tests
  log "Executing Go tests in $e2e_tests_dir"
  log "Test command: $go_test_cmd"
  
  if ! (cd "$e2e_tests_dir" && eval "$go_test_cmd"); then
    error "Go tests failed with exit code $?"
    # List any XML files that were created
    if [ -n "$junit_report" ]; then
      log "Checking for JUnit report files:"
      (cd "$e2e_tests_dir" && find . -name "*.xml" -ls || true)
    fi
    return 1
  fi
  
  # Verify the JUnit report was created
  if [ -n "$junit_report" ]; then
    log "Checking for JUnit report file:"
    (cd "$e2e_tests_dir" && find . -name "*.xml" -ls || true)
  fi
  
  log "Go tests completed successfully."
  return 0
}

# Wait for pods to reach the Running state
wait_for_pods() {
  local namespace="${1:-$DEFAULT_NAMESPACE}"
  local max_attempts=30
  local wait_seconds=10
  local attempt=1
  local all_namespaces=false
  
  if [ "$2" = "all-namespaces" ]; then
    all_namespaces=true
  fi
  
  log "Waiting for CSI driver pods to reach Running state..."
  
  while [ $attempt -le $max_attempts ]; do
    local pods_running=false
    local pod_output=""
    
    if [ "$all_namespaces" = true ]; then
      pod_output=$(exec_cmd kubectl get pods --all-namespaces | grep -E "s3|csi" || true)
    else
      pod_output=$(exec_cmd kubectl get pods -n "$namespace" | grep -E "s3|csi" || true)
    fi
    
    if [ -z "$pod_output" ]; then
      log "Attempt $attempt/$max_attempts: No CSI driver pods found yet. Waiting ${wait_seconds}s..."
    elif echo "$pod_output" | grep -q "Running"; then
      pods_running=true
      break
    else
      log "Attempt $attempt/$max_attempts: Pods are not running yet. Current status:"
      echo "$pod_output"
      log "Waiting ${wait_seconds}s for pods to start..."
    fi
    
    sleep $wait_seconds
    attempt=$((attempt + 1))
  done
  
  if [ "$pods_running" = true ]; then
    log "CSI driver pods are now in Running state:"
    echo "$pod_output"
    return 0
  else
    error "Timed out waiting for CSI driver pods to reach Running state after $((max_attempts * wait_seconds)) seconds."
    if [ "$all_namespaces" = true ]; then
      exec_cmd kubectl get pods --all-namespaces | grep -E "s3|csi"
    else
      exec_cmd kubectl get pods -n "$namespace" | grep -E "s3|csi"
    fi
    return 1
  fi
}

# Run basic verification tests
run_verification_tests() {
  local namespace="${1:-$DEFAULT_NAMESPACE}"
  local kubectl_path="$2"
  
  log "Verifying Scality CSI driver installation in namespace: $namespace..."
  
  # Check if kubectl path is provided
  if [ -z "$kubectl_path" ]; then
    error "kubectl path is required. Please specify the --kubectl-path parameter."
    return 1
  fi
  
  # Use the specified kubectl path
  local kubectl_cmd="$kubectl_path"
  
  # Check if the CSI driver is registered
  if "$kubectl_cmd" get csidrivers | grep -q "s3.csi.scality.com"; then
    log "CSI driver is registered properly."
  else
    error "CSI driver is not registered properly."
    return 1
  fi
  
  # Wait for the CSI driver pods to reach Running state
  if ! wait_for_pods "$namespace"; then
    # If pods not found in the specified namespace, try all namespaces
    log "CSI driver pods not found in namespace $namespace. Checking all namespaces..."
    if ! wait_for_pods "$namespace" "all-namespaces"; then
      error "Failed to find running CSI driver pods in any namespace."
      return 1
    fi
  fi
  
  log "Basic verification tests passed."
  return 0
}

# Main test function that will be called from run.sh
do_test() {
  log "Starting Scality CSI driver tests..."
  
  local skip_go_tests=false
  local skip_verification=false
  local namespace="$DEFAULT_NAMESPACE"
  local junit_report=""
  local kubectl_path=""
  local s3_endpoint_url=""
  local access_key_id=""
  local secret_access_key=""
  local bucket_prefix="e2e-test"
  local additional_args=""
  
  # Parse command-line parameters
  while [[ $# -gt 0 ]]; do
    key="$1"
    case "$key" in
      --namespace)
        namespace="$2"
        shift 2
        ;;
      --skip-go-tests)
        skip_go_tests=true
        shift
        ;;
      --skip-verification)
        skip_verification=true
        shift
        ;;
      --junit-report)
        if [[ "$2" == --* ]] || [[ $# -eq 1 ]]; then
          # Handle the case where --junit-report has no value
          junit_report="junit-report.xml"
          shift
        else
          junit_report="$2"
          shift 2
        fi
        ;;
      --junit-report=*)
        # Handle the case where --junit-report=value is used
        junit_report="${1#*=}"
        shift
        ;;
      --kubectl-path)
        kubectl_path="$2"
        shift 2
        ;;
      --s3-endpoint-url)
        s3_endpoint_url="$2"
        shift 2
        ;;
      --access-key-id)
        access_key_id="$2"
        shift 2
        ;;
      --secret-access-key)
        secret_access_key="$2"
        shift 2
        ;;
      --bucket-prefix)
        bucket_prefix="$2"
        shift 2
        ;;
      --additional-args)
        additional_args="$2"
        shift 2
        ;;
      *)
        log "Unknown parameter: $1"
        shift
        ;;
    esac
  done
  
  # Check if kubectl path is provided
  if [ -z "$kubectl_path" ]; then
    error "kubectl path is required. Please specify the --kubectl-path parameter."
    show_help
    return 1
  fi
  
  # Run verification tests
  if [ "$skip_verification" != true ]; then
    log "Running verification tests..."
    if ! run_verification_tests "$namespace" "$kubectl_path"; then
      error "Verification tests failed."
      return 1
    fi
  else
    log "Skipping verification tests."
  fi
  
  # Run Go tests
  if [ "$skip_go_tests" != true ]; then
    log "Running Go tests..."
    if ! run_go_tests "$namespace" "$junit_report" "$kubectl_path" "$s3_endpoint_url" "$access_key_id" "$secret_access_key" "$bucket_prefix" "$additional_args"; then
      error "Go tests failed."
      return 1
    fi
  else
    log "Skipping Go tests."
  fi
  
  log "Scality CSI driver tests completed successfully."
  return 0
}
