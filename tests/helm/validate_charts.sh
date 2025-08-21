#!/bin/bash
# validate_charts.sh - A script to validate Helm chart requirements and configurations

set -eo pipefail

# Define color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script root directory (the directory this script is in)
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
# Project root directory
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"
# Charts directory
CHARTS_DIR="$PROJECT_ROOT/charts"

# Print script usage information
usage() {
    echo -e "${BLUE}Usage:${NC} $0 [validation_name]"
    echo ""
    echo "When run without arguments, all validations will be executed."
    echo "To run a specific validation, provide its function name as an argument."
    echo ""
    echo -e "${BLUE}Available validations:${NC}"
    echo "  validate_custom_endpoint - Verify ability to set custom S3 endpoint URL"
    echo "  validate_s3_region     - Verify ability to set S3 region"
    echo "  validate_backward_compatibility - Verify legacy node.s3* values still work"
    echo "  validate_legacy_precedence - Verify legacy node.s3* values take precedence over global"
    echo "  validate_global_only_config - Verify global s3.* values work without legacy values"
    echo "  validate_region_precedence_scenarios - Verify all region precedence scenarios"
    echo "  validate_endpoint_precedence_scenarios - Verify all endpoint URL precedence scenarios"
    echo ""
    echo -e "${BLUE}Examples:${NC}"
    echo "  $0                           # Run all validations"
    echo "  $0 validate_custom_endpoint  # Run only the custom S3 endpoint validation"
    echo ""
}

# Function to run a validation test and report results
run_validation() {
  local test_name="$1"
  local test_func="$2"

  echo -e "\n${YELLOW}Running validation: ${test_name}${NC}"

  if $test_func; then
    echo -e "${GREEN}✅ PASSED: ${test_name}${NC}"
    return 0
  else
    echo -e "${RED}❌ FAILED: ${test_name}${NC}"
    return 1
  fi
}

# Check if helm is installed
check_helm_installed() {
  if ! command -v helm &> /dev/null; then
    echo -e "${RED}Error: helm is not installed. Please install helm before running this script.${NC}"
    exit 1
  fi
}

# Validation test for custom S3 endpoint URL (using new global config)
validate_custom_endpoint() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"
  local custom_endpoint="https://custom-s3.example.com:8443"

  echo "Testing ability to set custom S3 endpoint URL using global s3.endpointUrl..."

  # Run helm template with custom endpoint using new global config
  echo "Rendering template with custom endpoint: $custom_endpoint"
  local node_result=$(helm template "$chart_dir" --set s3.endpointUrl="$custom_endpoint" --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" --set s3.endpointUrl="$custom_endpoint" --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  # Check if rendering succeeded
  if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm template failed with custom endpoint URL:${NC}"
    echo "$result"
    return 1
  fi

  # Check if our custom endpoint appears in both node and controller templates
  if echo "$result" | grep -q "value: $custom_endpoint"; then
    local node_count=$(echo "$result" | grep -c "value: $custom_endpoint")
    if [ "$node_count" -ge 2 ]; then
      echo -e "${GREEN}✓ Custom endpoint URL successfully applied in both node and controller templates${NC}"
      return 0
    else
      echo -e "${RED}✗ Custom endpoint URL not found in both templates (found $node_count times)${NC}"
      return 1
    fi
  else
    echo -e "${RED}✗ Custom endpoint URL not found in rendered templates${NC}"
    return 1
  fi
}

# Validation test for S3 region configuration (using new global config)
validate_s3_region() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"
  local custom_region="us-west-2"

  echo "Testing ability to set S3 region using global s3.region..."

  # First check default value
  echo "Checking default region is set to us-east-1"
  local node_result=$(helm template "$chart_dir" --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  if ! echo "$result" | grep -Eq "^[[:space:]]*value: us-east-1"; then
    echo -e "${RED}✗ Default S3 region not properly set to us-east-1${NC}"
    return 1
  else
    echo -e "${GREEN}✓ Default S3 region correctly set to us-east-1${NC}"
  fi

  # Then check custom value using new global config
  echo "Rendering template with custom region: $custom_region"
  node_result=$(helm template "$chart_dir" --set s3.region="$custom_region" --show-only templates/node.yaml 2>&1)
  controller_result=$(helm template "$chart_dir" --set s3.region="$custom_region" --show-only templates/controller.yaml 2>&1)
  result="$node_result"$'\n'"$controller_result"

  if echo "$result" | grep -Eq "^[[:space:]]*value: $custom_region"; then
    local region_count=$(echo "$result" | grep -c "value: $custom_region")
    if [ "$region_count" -ge 2 ]; then
      echo -e "${GREEN}✓ Custom S3 region successfully applied in both node and controller templates${NC}"
      return 0
    else
      echo -e "${RED}✗ Custom S3 region not found in both templates (found $region_count times)${NC}"
      return 1
    fi
  else
    echo -e "${RED}✗ Custom S3 region not found in rendered templates${NC}"
    return 1
  fi
}

# Validation test for backward compatibility with legacy node.s3* values
validate_backward_compatibility() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"
  local legacy_endpoint="https://legacy-s3.example.com:9000"
  local legacy_region="eu-west-1"

  echo "Testing backward compatibility with legacy node.s3* configuration..."

  # Test with legacy node.s3EndpointUrl and node.s3Region
  echo "Rendering template with legacy node.s3EndpointUrl and node.s3Region"
  local node_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --set node.s3Region="$legacy_region" \
    --set s3.endpointUrl="" \
    --set s3.region="" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --set node.s3Region="$legacy_region" \
    --set s3.endpointUrl="" \
    --set s3.region="" \
    --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  # Check if rendering succeeded
  if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm template failed with legacy configuration:${NC}"
    echo "$result"
    return 1
  fi

  # Check if legacy values appear in both templates
  local endpoint_count=$(echo "$result" | grep -c "value: $legacy_endpoint")
  local region_count=$(echo "$result" | grep -c "value: $legacy_region")

  if [ "$endpoint_count" -ge 2 ] && [ "$region_count" -ge 2 ]; then
    echo -e "${GREEN}✓ Legacy node.s3* values successfully applied in both node and controller templates${NC}"
    return 0
  else
    echo -e "${RED}✗ Legacy values not found in both templates (endpoint: $endpoint_count, region: $region_count)${NC}"
    return 1
  fi
}

# Validation test for legacy node.s3* config precedence over global values
validate_legacy_precedence() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"
  local global_endpoint="https://global-s3.example.com:8080"
  local global_region="ap-southeast-1"
  local legacy_endpoint="https://legacy-s3.example.com:9000"
  local legacy_region="eu-west-1"

  echo "Testing that legacy node.s3* values take precedence over global s3.* values..."

  # Test with both global and legacy values set (legacy should win)
  echo "Rendering template with both global and legacy values (legacy should take precedence)"
  local node_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --set s3.region="$global_region" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --set node.s3Region="$legacy_region" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --set s3.region="$global_region" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --set node.s3Region="$legacy_region" \
    --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  # Check if rendering succeeded
  if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm template failed with mixed configuration:${NC}"
    echo "$result"
    return 1
  fi

  # Check if legacy values appear in templates (not global values)
  local legacy_endpoint_count=$(echo "$result" | grep -c "value: $legacy_endpoint")
  local legacy_region_count=$(echo "$result" | grep -c "value: $legacy_region")
  local global_endpoint_count=$(echo "$result" | grep -c "value: $global_endpoint")
  local global_region_count=$(echo "$result" | grep -c "value: $global_region")

  if [ "$legacy_endpoint_count" -ge 2 ] && [ "$legacy_region_count" -ge 2 ] && [ "$global_endpoint_count" -eq 0 ] && [ "$global_region_count" -eq 0 ]; then
    echo -e "${GREEN}✓ Legacy node.s3* values correctly take precedence over global s3.* values${NC}"
    return 0
  else
    echo -e "${RED}✗ Legacy values did not take precedence (legacy endpoint: $legacy_endpoint_count, legacy region: $legacy_region_count, global endpoint: $global_endpoint_count, global region: $global_region_count)${NC}"
    return 1
  fi
}

# Validation test for global-only configuration
validate_global_only_config() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"
  local global_endpoint="https://global-only-s3.example.com:8080"
  local global_region="ap-northeast-1"

  echo "Testing global s3.* configuration without legacy values..."

  # Test with only global values set (no legacy values)
  echo "Rendering template with only global s3.* values"
  local node_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --set s3.region="$global_region" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --set s3.region="$global_region" \
    --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  # Check if rendering succeeded
  if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Helm template failed with global-only configuration:${NC}"
    echo "$result"
    return 1
  fi

  # Check if global values appear in both templates
  local global_endpoint_count=$(echo "$result" | grep -c "value: $global_endpoint")
  local global_region_count=$(echo "$result" | grep -c "value: $global_region")

  if [ "$global_endpoint_count" -ge 2 ] && [ "$global_region_count" -ge 2 ]; then
    echo -e "${GREEN}✓ Global s3.* values successfully applied when no legacy values present${NC}"
    return 0
  else
    echo -e "${RED}✗ Global values not found in both templates (endpoint: $global_endpoint_count, region: $global_region_count)${NC}"
    return 1
  fi
}

# Validation test specifically for region precedence scenarios
validate_region_precedence_scenarios() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"

  echo "Testing all region precedence scenarios..."

  # Scenario 1: Only legacy node.s3Region set
  echo "Scenario 1: Testing legacy node.s3Region only"
  local legacy_region="eu-central-1"
  local node_result=$(helm template "$chart_dir" \
    --set node.s3Region="$legacy_region" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set node.s3Region="$legacy_region" \
    --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  local legacy_count=$(echo "$result" | grep -c "value: $legacy_region")
  if [ "$legacy_count" -lt 2 ]; then
    echo -e "${RED}✗ Legacy node.s3Region not applied to both templates (found $legacy_count times)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Legacy node.s3Region successfully applied when only legacy value present${NC}"

  # Scenario 2: Only global s3.region set
  echo "Scenario 2: Testing global s3.region only"
  local global_region="ap-south-1"
  local node_result=$(helm template "$chart_dir" \
    --set s3.region="$global_region" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set s3.region="$global_region" \
    --show-only templates/controller.yaml 2>&1)
  result="$node_result"$'\n'"$controller_result"

  local global_count=$(echo "$result" | grep -c "value: $global_region")
  if [ "$global_count" -lt 2 ]; then
    echo -e "${RED}✗ Global s3.region not applied to both templates (found $global_count times)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Global s3.region successfully applied when only global value present${NC}"

  # Scenario 3: Both set - legacy should win
  echo "Scenario 3: Testing both legacy and global region (legacy should win)"
  local legacy_region_mixed="us-west-1"
  local global_region_mixed="eu-north-1"
  local node_result=$(helm template "$chart_dir" \
    --set node.s3Region="$legacy_region_mixed" \
    --set s3.region="$global_region_mixed" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set node.s3Region="$legacy_region_mixed" \
    --set s3.region="$global_region_mixed" \
    --show-only templates/controller.yaml 2>&1)
  result="$node_result"$'\n'"$controller_result"

  local legacy_mixed_count=$(echo "$result" | grep -c "value: $legacy_region_mixed")
  local global_mixed_count=$(echo "$result" | grep -c "value: $global_region_mixed")

  if [ "$legacy_mixed_count" -lt 2 ] || [ "$global_mixed_count" -ne 0 ]; then
    echo -e "${RED}✗ Legacy region precedence failed (legacy: $legacy_mixed_count, global: $global_mixed_count)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Legacy node.s3Region correctly takes precedence over global s3.region${NC}"

  return 0
}

# Validation test specifically for endpoint URL precedence scenarios
validate_endpoint_precedence_scenarios() {
  local chart_dir="$CHARTS_DIR/scality-mountpoint-s3-csi-driver"

  echo "Testing all endpoint URL precedence scenarios..."

  # Scenario 1: Only legacy node.s3EndpointUrl set
  echo "Scenario 1: Testing legacy node.s3EndpointUrl only"
  local legacy_endpoint="https://legacy-only.s3.example.com:8000"
  local node_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint" \
    --show-only templates/controller.yaml 2>&1)
  local result="$node_result"$'\n'"$controller_result"

  local legacy_count=$(echo "$result" | grep -c "value: $legacy_endpoint")
  if [ "$legacy_count" -lt 2 ]; then
    echo -e "${RED}✗ Legacy node.s3EndpointUrl not applied to both templates (found $legacy_count times)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Legacy node.s3EndpointUrl successfully applied when only legacy value present${NC}"

  # Scenario 2: Only global s3.endpointUrl set
  echo "Scenario 2: Testing global s3.endpointUrl only"
  local global_endpoint="https://global-only.s3.example.com:9000"
  local node_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set s3.endpointUrl="$global_endpoint" \
    --show-only templates/controller.yaml 2>&1)
  result="$node_result"$'\n'"$controller_result"

  local global_count=$(echo "$result" | grep -c "value: $global_endpoint")
  if [ "$global_count" -lt 2 ]; then
    echo -e "${RED}✗ Global s3.endpointUrl not applied to both templates (found $global_count times)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Global s3.endpointUrl successfully applied when only global value present${NC}"

  # Scenario 3: Both set - legacy should win
  echo "Scenario 3: Testing both legacy and global endpoint (legacy should win)"
  local legacy_endpoint_mixed="https://legacy-wins.s3.example.com:8080"
  local global_endpoint_mixed="https://global-loses.s3.example.com:9090"
  local node_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint_mixed" \
    --set s3.endpointUrl="$global_endpoint_mixed" \
    --show-only templates/node.yaml 2>&1)
  local controller_result=$(helm template "$chart_dir" \
    --set node.s3EndpointUrl="$legacy_endpoint_mixed" \
    --set s3.endpointUrl="$global_endpoint_mixed" \
    --show-only templates/controller.yaml 2>&1)
  result="$node_result"$'\n'"$controller_result"

  local legacy_mixed_count=$(echo "$result" | grep -c "value: $legacy_endpoint_mixed")
  local global_mixed_count=$(echo "$result" | grep -c "value: $global_endpoint_mixed")

  if [ "$legacy_mixed_count" -lt 2 ] || [ "$global_mixed_count" -ne 0 ]; then
    echo -e "${RED}✗ Legacy endpoint precedence failed (legacy: $legacy_mixed_count, global: $global_mixed_count)${NC}"
    return 1
  fi
  echo -e "${GREEN}✓ Legacy node.s3EndpointUrl correctly takes precedence over global s3.endpointUrl${NC}"

  return 0
}

# Main function - runs all validations or a specific one
main() {
  # Display banner
  echo -e "${BLUE}===============================================${NC}"
  echo -e "${BLUE}   Scality CSI Driver for S3 Helm Validation Tool   ${NC}"
  echo -e "${BLUE}===============================================${NC}"

  # Check if helm is installed
  check_helm_installed

  # Check if a specific validation was requested
  if [ $# -eq 1 ]; then
    # Check if the function exists
    if declare -f "$1" > /dev/null; then
      # Run the specified validation
      run_validation "$1" "$1"
      exit $?
    else
      echo -e "${RED}Error: Validation function '$1' not found.${NC}"
      usage
      exit 1
    fi
  fi

  # If no specific validation was requested, run all validations
  local errors=0

  # Run all validations
  run_validation "Custom S3 endpoint URL can be specified" validate_custom_endpoint || ((errors++))
  run_validation "S3 region configuration" validate_s3_region || ((errors++))
  run_validation "Backward compatibility with legacy node.s3* values" validate_backward_compatibility || ((errors++))
  run_validation "Legacy node.s3* values take precedence over global values" validate_legacy_precedence || ((errors++))
  run_validation "Global s3.* values work without legacy values" validate_global_only_config || ((errors++))
  run_validation "All region precedence scenarios" validate_region_precedence_scenarios || ((errors++))
  run_validation "All endpoint URL precedence scenarios" validate_endpoint_precedence_scenarios || ((errors++))

  # Report final results
  if [ $errors -eq 0 ]; then
    echo -e "\n${GREEN}All validations passed!${NC}"
    return 0
  else
    echo -e "\n${RED}${errors} validation(s) failed!${NC}"
    return 1
  fi
}

# If the script is being sourced, don't run main
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  # Process command line arguments
  if [ "$1" == "-h" ] || [ "$1" == "--help" ]; then
    usage
    exit 0
  fi

  # Run main with all arguments
  main "$@"
fi
