#!/bin/bash
# run.sh - Main entry point for e2e-scality scripts
set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
MODULES_DIR="${SCRIPT_DIR}/modules"

# Source common functions
source "${MODULES_DIR}/common.sh"

# Show help message
show_help() {
  echo "Usage: $0 [COMMAND] [OPTIONS]"
  echo
  echo "Commands:"
  echo "  install   Install and verify the Scality CSI driver (default)"
  echo "  test      Run end-to-end tests against the installed driver"
  echo "  go-test   Run only Go-based e2e tests (skips verification checks)"
  echo "  all       Install driver and run tests"
  echo "  uninstall Uninstall the Scality CSI driver"
  echo "  help      Show this help message"
  echo
  echo "Options for install command:"
  echo "  --image-tag VALUE         Specify custom image tag for the CSI driver"
  echo "  --endpoint-url VALUE      Specify custom S3 endpoint URL (REQUIRED)"
  echo "  --access-key-id VALUE     Specify S3 access key ID for authentication (REQUIRED)"
  echo "  --secret-access-key VALUE Specify S3 secret access key for authentication (REQUIRED)"
  echo "  --validate-s3             Validate S3 endpoint and credentials before installation"
  echo
  echo "Options for test and go-test commands:"
  echo "  --skip-go-tests           Skip executing Go-based end-to-end tests (test command only)"
  echo "  --go-test-args \"ARGS\"     Pass additional arguments to go test command"
  echo "  --focus \"PATTERN\"         Focus on tests matching the given pattern (Ginkgo)"
  echo "  --skip \"PATTERN\"          Skip tests matching the given pattern (Ginkgo)"
  echo "  --tags \"TAGS\"             Specify Go build tags (default: e2e)"
  echo "  --namespace \"NS\"          Specify the namespace to test (default: mount-s3)"
  echo
  echo "Options for uninstall command:"
  echo "  --delete-ns               Delete the mount-s3 namespace without prompting"
  echo "  --force                   Force delete all resources including CSI driver registration"
  echo
  echo "Examples:"
  echo "  $0 install --endpoint-url https://s3.example.com --access-key-id AKIAXXXXXXXX --secret-access-key xxxxxxxx"
  echo "  $0 install --image-tag v1.14.0 --endpoint-url https://s3.example.com --access-key-id AKIAXXXXXXXX --secret-access-key xxxxxxxx"
  echo "  $0 install --validate-s3 --endpoint-url https://s3.example.com --access-key-id AKIAXXXXXXXX --secret-access-key xxxxxxxx"
  echo "  $0 test                                 # Run all tests including Go-based e2e tests"
  echo "  $0 test --skip-go-tests                 # Run only basic verification tests"
  echo "  $0 test --focus \"Mount\"                 # Run only tests with 'Mount' in their name"
  echo "  $0 go-test                              # Run Go tests directly (skips verification)"
  echo "  $0 go-test --focus \"Basic Functionality\" # Run only Go tests matching the pattern"
  echo "  $0 go-test --namespace \"custom-ns\"      # Test with custom namespace"
  echo "  $0 all                                  # Install driver and run tests"
  echo "  $0 uninstall                            # Uninstall driver (interactive mode)"
  echo "  $0 uninstall --delete-ns                # Uninstall driver and delete namespace"
  echo "  $0 uninstall --force                    # Force delete all resources"
  echo "  $0 help                                 # Show this help message"
}

parse_install_parameters() {
  local params=""
  local has_endpoint_url=false
  local has_access_key_id=false
  local has_secret_access_key=false

  # Process options
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --image-tag)
        IMAGE_TAG="$2"
        params="$params --image-tag $2"
        shift 2
        ;;
      --endpoint-url)
        ENDPOINT_URL="$2"
        params="$params --endpoint-url $2"
        has_endpoint_url=true
        shift 2
        ;;
      --access-key-id)
        ACCESS_KEY_ID="$2"
        params="$params --access-key-id $2"
        has_access_key_id=true
        shift 2
        ;;
      --secret-access-key)
        SECRET_ACCESS_KEY="$2"
        params="$params --secret-access-key $2"
        has_secret_access_key=true
        shift 2
        ;;
      --validate-s3)
        params="$params --validate-s3"
        shift
        ;;
      *)
        echo "Error: Unknown option: $1"
        show_help
        exit 1
        ;;
    esac
  done

  # Validate required parameters
  if [ "$has_endpoint_url" = false ]; then
    error "Missing required parameter: --endpoint-url"
    show_help
    exit 1
  fi
  
  if [ "$has_access_key_id" = false ]; then
    error "Missing required parameter: --access-key-id"
    show_help
    exit 1
  fi
  
  if [ "$has_secret_access_key" = false ]; then
    error "Missing required parameter: --secret-access-key"
    show_help
    exit 1
  fi

  # Return parameters
  echo "$params"
}

# Parse uninstall parameters
parse_uninstall_parameters() {
  local params=""
  
  # Process options
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --delete-ns)
        params="$params --delete-ns"
        shift
        ;;
      --force)
        params="$params --force"
        shift
        ;;
      *)
        echo "Error: Unknown option: $1"
        show_help
        exit 1
        ;;
    esac
  done
  
  # Return parameters
  echo "$params"
}

# Parse test parameters
parse_test_parameters() {
  local params=""
  
  # Process options
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --skip-go-tests)
        params="$params --skip-go-tests"
        shift
        ;;
      --go-test-args)
        params="$params --go-test-args $2"
        shift 2
        ;;
      --focus)
        params="$params --focus $2"
        shift 2
        ;;
      --skip)
        params="$params --skip $2"
        shift 2
        ;;
      --tags)
        params="$params --tags $2"
        shift 2
        ;;
      --namespace)
        params="$params --namespace $2"
        shift 2
        ;;
      *)
        echo "Error: Unknown option: $1"
        show_help
        exit 1
        ;;
    esac
  done
  
  # Return parameters
  echo "$params"
}

# Main execution
main() {
  # Process command line arguments
  COMMAND=${1:-install}
  shift || true # Remove the command from the arguments
  
  case $COMMAND in
    install)
      source "${MODULES_DIR}/install.sh"
      # Pass all command-line parameters to install module
      exec_cmd do_install "$@"
      ;;
    test)
      source "${MODULES_DIR}/test.sh"
      # Parse test parameters
      local test_parameters=$(parse_test_parameters "$@")
      # Pass processed parameters to test module
      exec_cmd do_test $test_parameters
      ;;
    go-test)
      # This command runs only the Go tests without verification
      source "${MODULES_DIR}/test.sh"
      # Parse test parameters
      local test_parameters=$(parse_test_parameters "$@")
      # Pass processed parameters to run_go_tests function directly
      test_parameters="$test_parameters --skip-verification"
      exec_cmd do_test $test_parameters
      ;;
    all)
      log "Starting Scality CSI driver installation and tests..."
      
      source "${MODULES_DIR}/install.sh"
      
      # Pass all command-line parameters to install module
      exec_cmd do_install "$@"
      
      source "${MODULES_DIR}/test.sh"
      
      # Run tests
      exec_cmd do_test
      
      log "Scality CSI driver setup and tests completed successfully."
      ;;
    uninstall)
      source "${MODULES_DIR}/uninstall.sh"
      # Parse uninstall parameters
      local uninstall_parameters=$(parse_uninstall_parameters "$@")
      # Pass processed parameters to uninstall module
      exec_cmd do_uninstall $uninstall_parameters
      ;;
    help)
      show_help
      ;;
    *)
      error "Unknown command: $COMMAND"
      show_help
      exit 1
      ;;
  esac
}

# Execute main function with all arguments
main "$@"
