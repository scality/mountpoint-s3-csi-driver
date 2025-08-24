#!/bin/bash

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_phase() {
    echo ""
    echo "========================================"
    echo "PHASE: $1"
    echo "========================================"
}

log_info() {
    echo -e "[INFO] $1"
}

log_error() {
    echo -e "${RED}[ERROR] $1${NC}" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS] $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}[WARNING] $1${NC}"
}

log_title() {
    echo ""
    echo "########################################"
    echo "# $1"
    echo "########################################"
    echo ""
}
