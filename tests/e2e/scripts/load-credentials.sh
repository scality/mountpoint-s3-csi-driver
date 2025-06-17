#!/bin/bash
# load-credentials.sh - Load credentials from JSON configuration file

set -e  # Exit on any error

# Get the directory where this script is located
if [[ -n "${BASH_SOURCE[0]}" ]]; then
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
else
    # Fallback for when BASH_SOURCE[0] is empty (when sourced in some contexts)
    if [[ -f "./load-credentials.sh" ]]; then
        SCRIPT_DIR="$(pwd)"
    else
        SCRIPT_DIR="$(pwd)/tests/e2e/scripts"
    fi
fi

# Default config file path
CONFIG_FILE="${CREDENTIALS_CONFIG_FILE:-${SCRIPT_DIR}/../integration_config.json}"

# Simple logging
log() { echo "[$(date '+%H:%M:%S')] $1"; }
error() { echo "[$(date '+%H:%M:%S')] ERROR: $1" >&2; }

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --config-file)
            CONFIG_FILE="$2"
            shift 2
            ;;
        --help)
            cat << EOF
Usage: $0 [OPTIONS]

Load credentials from JSON configuration file and export as environment variables.

Options:
  --config-file PATH    Specify custom path to credentials JSON file
  --help               Show this help message

Environment Variables:
  CREDENTIALS_CONFIG_FILE    Path to credentials JSON file

Examples:
  source $0                                    # Load with default config
  source $0 --config-file /path/to/creds.json # Load with custom config

Required JSON structure:
  {
    "credentials": {
      "account": {
        "account1": { "accessKey": "...", "secretKey": "...", "canonicalId": "..." },
        "account2": { "accessKey": "...", "secretKey": "...", "canonicalId": "..." }
      }
    }
  }
EOF
            return 0 2>/dev/null || exit 0
            ;;
        *)
            error "Unknown option: $1"
            return 1 2>/dev/null || exit 1
            ;;
    esac
done

# Check if jq is available
if ! command -v jq &> /dev/null; then
    error "jq is required but not installed. Please install jq to use this script."
    return 1 2>/dev/null || exit 1
fi

# Check if config file exists
if [[ ! -f "$CONFIG_FILE" ]]; then
    error "Credentials file not found: $CONFIG_FILE"
    return 1 2>/dev/null || exit 1
fi

log "Loading credentials from: $CONFIG_FILE"

# Parse JSON and extract credentials
ACCOUNT1_ACCESS_KEY=$(jq -r '.credentials.account.account1.accessKey' "$CONFIG_FILE")
ACCOUNT1_SECRET_KEY=$(jq -r '.credentials.account.account1.secretKey' "$CONFIG_FILE")
ACCOUNT1_CANONICAL_ID=$(jq -r '.credentials.account.account1.canonicalId' "$CONFIG_FILE")

ACCOUNT2_ACCESS_KEY=$(jq -r '.credentials.account.account2.accessKey' "$CONFIG_FILE")
ACCOUNT2_SECRET_KEY=$(jq -r '.credentials.account.account2.secretKey' "$CONFIG_FILE")
ACCOUNT2_CANONICAL_ID=$(jq -r '.credentials.account.account2.canonicalId' "$CONFIG_FILE")

# Basic validation
if [[ "$ACCOUNT1_ACCESS_KEY" == "null" || -z "$ACCOUNT1_ACCESS_KEY" ]]; then
    error "Failed to parse account1 accessKey from JSON"
    return 1 2>/dev/null || exit 1
fi

if [[ "$ACCOUNT1_SECRET_KEY" == "null" || -z "$ACCOUNT1_SECRET_KEY" ]]; then
    error "Failed to parse account1 secretKey from JSON"
    return 1 2>/dev/null || exit 1
fi

if [[ "$ACCOUNT1_CANONICAL_ID" == "null" || -z "$ACCOUNT1_CANONICAL_ID" ]]; then
    error "Failed to parse account1 canonicalId from JSON"
    return 1 2>/dev/null || exit 1
fi

# Export environment variables
export ACCOUNT1_ACCESS_KEY
export ACCOUNT1_SECRET_KEY
export ACCOUNT1_CANONICAL_ID

# Export account2 credentials if they exist
if [[ "$ACCOUNT2_ACCESS_KEY" != "null" && -n "$ACCOUNT2_ACCESS_KEY" ]]; then
    export ACCOUNT2_ACCESS_KEY
    export ACCOUNT2_SECRET_KEY
    export ACCOUNT2_CANONICAL_ID
fi

log "Credentials loaded and exported successfully"
log "Account1 credentials: ACCOUNT1_ACCESS_KEY=${ACCOUNT1_ACCESS_KEY:0:8}... (truncated)"

# Success message
echo "✅ Credentials loaded from $CONFIG_FILE"
