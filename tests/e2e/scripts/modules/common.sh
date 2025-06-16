#!/bin/bash
# common.sh - Shared functions for e2e scripts

# Basic error handling - exit on error, undefined variables, pipe failures
set -euo pipefail

# Print with timestamp
log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

warn() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1"
}

error() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1" >&2
  return 1
}

# Fatal error - logs and exits
fatal() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] FATAL: $1" >&2
  exit 1
}

# Execute a command with logging
exec_cmd() {
  log "Executing: $*"
  "$@"
}

# Check for required tools
check_dependencies() {
  log "Checking dependencies..."

  local missing_deps=0

  if ! command -v kubectl &> /dev/null; then
    error "kubectl is not installed. Please install it first."
    missing_deps=1
  fi

  if ! command -v helm &> /dev/null; then
    error "Helm is not installed. Please install it first."
    missing_deps=1
  fi

  if ! command -v curl &> /dev/null; then
    error "curl is not installed. It's required for basic endpoint validation."
    missing_deps=1
  fi

  if ! command -v aws &> /dev/null; then
    warn "AWS CLI is not installed. Only endpoint connectivity will be validated."
    warn "Credentials (access key and secret key) cannot be validated without AWS CLI."
  fi

  if [ $missing_deps -ne 0 ]; then
    fatal "Missing dependencies. Please install required tools before proceeding."
  fi

  log "All critical dependencies are installed."
}

# Get the path to the project root
get_project_root() {
  # Navigate to the root of the project (four levels up from modules dir)
  echo "$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../" && pwd)"
}
