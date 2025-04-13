#!/bin/bash
# common.sh - Shared functions for e2e-scality scripts

# Define colors for better readability
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Print with timestamp
log() {
  echo -e "${GREEN}[$(date '+%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
  echo -e "${YELLOW}[$(date '+%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
  echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}" >&2
  return 1
}

# Fatal error - logs and exits
fatal() {
  echo -e "${RED}[$(date '+%Y-%m-%d %H:%M:%S')] FATAL: $1${NC}" >&2
  exit 1
}

# Execute a command
exec_cmd() {
  # Execute the command
  "$@"
  
  # Return the exit code from the command
  return $?
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
  
  if ! command -v aws &> /dev/null; then
    warn "AWS CLI is not installed. This is optional but recommended for better S3 validation."
    warn "Alternative validation methods will be used, but they may be less reliable."
    
    # Check for curl as a fallback
    if ! command -v curl &> /dev/null; then
      warn "curl is not installed. This is needed for alternate S3 validation methods."
      warn "Limited validation capabilities will be available."
    fi
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
