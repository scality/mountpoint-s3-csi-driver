#!/bin/bash
# install.sh - Installation functions for e2e-scality scripts

# Source common functions
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

# Validate S3 configuration by testing connectivity and credentials
validate_s3_configuration() {
  local endpoint_url="$1"
  local access_key_id="$2"
  local secret_access_key="$3"
  
  log "Validating S3 configuration..."
  log "Attempting to connect to S3 endpoint: $endpoint_url"
  log "Using access key ID: $access_key_id"
  log "Secret key length: ${#secret_access_key} characters"
  
  # Create a temporary file for capturing output
  local temp_output=$(mktemp)
  
  # For local development/test endpoints, accept more relaxed validation
  local is_local=false
  if [[ "$endpoint_url" == *"localhost"* ]] || [[ "$endpoint_url" == *"127.0.0.1"* ]]; then
    is_local=true
    log "Detected local endpoint, will use relaxed validation requirements."
  fi
  
  # Method 1: Try using AWS CLI if available
  if command -v aws &> /dev/null; then
    log "AWS CLI found, using it for validation..."
    
    # Use environment variables method for AWS credentials (more reliable with Scality S3)
    if AWS_ACCESS_KEY_ID="$access_key_id" AWS_SECRET_ACCESS_KEY="$secret_access_key" exec_cmd aws --endpoint-url "$endpoint_url" s3 ls > "$temp_output" 2>&1; then
      log "Successfully connected to S3 endpoint using AWS CLI."
      log "Available buckets:"
      cat "$temp_output"
      
      # Clean up temporary file
      rm -f "$temp_output"
      return 0
    else
      log "AWS CLI validation failed. Error details:"
      cat "$temp_output"
      log "Trying alternative validation methods..."
    fi
  else
    log "AWS CLI not found, using alternative validation methods..."
  fi
  
  # Method 2: Try using curl to access a specific bucket or list endpoint
  log "Attempting alternative endpoint tests with curl..."
  
  # For local/test endpoints, try accessing common test endpoints
  local endpoints_to_try=(
    "${endpoint_url}/"
    "${endpoint_url}/s3"
    "${endpoint_url}/s3/health"
    "${endpoint_url}/health"
  )
  
  for test_url in "${endpoints_to_try[@]}"; do
    log "Testing endpoint: $test_url"
    local curl_result=$(curl -s -o "$temp_output" -w "%{http_code}" "$test_url" 2>/dev/null)
    
    if [[ "$curl_result" == 2* ]] || [[ "$curl_result" == 3* ]]; then
      log "Successfully connected to endpoint: $test_url (HTTP $curl_result)"
      
      if [ "$is_local" = true ]; then
        log "Local endpoint connectivity confirmed. Assuming credentials are valid for development environment."
        rm -f "$temp_output"
        return 0
      fi
    else
      log "Connection to $test_url failed with HTTP code $curl_result"
    fi
  done
  
  # Method 3: Try a simplified S3 list access
  if [ "$is_local" = true ]; then
    log "Attempting simplified S3 access for local endpoint..."
    local query_params="?AWSAccessKeyId=${access_key_id}&Signature=not_checked_for_local&Timestamp=$(date -u +"%Y-%m-%dT%H%%3A%M%%3A%SZ")"
    local test_url="${endpoint_url}/${query_params}"
    
    local curl_result=$(curl -s -o "$temp_output" -w "%{http_code}" "$test_url" 2>/dev/null)
    
    # Many test S3 servers will at least respond with XML, even if authentication fails
    if grep -q "<?xml" "$temp_output"; then
      log "Local S3 endpoint responding with XML content, suggesting the endpoint is functional."
      log "For local development endpoints, this is considered sufficient validation."
      # Clean up temporary file
      rm -f "$temp_output"
      return 0
    fi
  fi
  
  # Final method: Basic connection test (last resort)
  log "Attempting basic connection test to endpoint..."
  if curl -s -o /dev/null -w "%{http_code}" "$endpoint_url" 2>/dev/null | grep -q -E '^[23]'; then
    if [ "$is_local" = true ]; then
      log "Basic connection to local S3 endpoint succeeded."
      log "For local development, treating this as successful validation."
      # Clean up temporary file
      rm -f "$temp_output"
      return 0
    else
      log "Basic connection to S3 endpoint succeeded, but credentials could not be validated."
      log "Warning: This only confirms the endpoint is reachable, not that the credentials are valid."
    fi
  fi
  
  # All validation methods failed
  if [ "$is_local" = true ]; then
    error "Failed to validate local S3 endpoint. Check if your S3 server is running."
  else
    error "Failed to validate S3 endpoint and credentials using multiple methods."
  fi
  
  log "Please verify your endpoint URL and credentials manually before proceeding."
  
  # Clean up temporary file
  rm -f "$temp_output"
  return 1
}

# Install the Scality CSI driver using Helm
install_csi_driver() {
  # Process arguments
  local IMAGE_TAG=""
  local ENDPOINT_URL=""
  local ACCESS_KEY_ID=""
  local SECRET_ACCESS_KEY=""
  local VALIDATE_S3="false"
  
  # Parse arguments
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --image-tag)
        IMAGE_TAG="$2"
        shift 2
        ;;
      --endpoint-url)
        ENDPOINT_URL="$2"
        shift 2
        ;;
      --access-key-id)
        ACCESS_KEY_ID="$2"
        shift 2
        ;;
      --secret-access-key)
        SECRET_ACCESS_KEY="$2"
        shift 2
        ;;
      --validate-s3)
        VALIDATE_S3="true"
        shift
        ;;
      *)
        warn "Unknown parameter: $1"
        shift
        ;;
    esac
  done

  # Validate required parameters
  if [ -z "$ENDPOINT_URL" ]; then
    error "Missing required parameter: --endpoint-url"
    exit 1
  fi
  
  if [ -z "$ACCESS_KEY_ID" ]; then
    error "Missing required parameter: --access-key-id"
    exit 1
  fi
  
  if [ -z "$SECRET_ACCESS_KEY" ]; then
    error "Missing required parameter: --secret-access-key"
    exit 1
  fi

  # Validate S3 configuration if validation is enabled
  if [ "$VALIDATE_S3" = "true" ]; then
    if ! validate_s3_configuration "$ENDPOINT_URL" "$ACCESS_KEY_ID" "$SECRET_ACCESS_KEY"; then
      error "S3 configuration validation failed. Fix your S3 endpoint URL and credentials."
      exit 1
    fi
  fi

  log "Installing Scality CSI driver using Helm..."
  
  # Get project root from common function
  PROJECT_ROOT=$(get_project_root)
  
  # Create S3 credentials secret if it doesn't exist
  log "Creating S3 credentials secret..."
  exec_cmd kubectl create namespace mount-s3 --dry-run=client -o yaml | kubectl apply -f -
  
  # Create or update the secret with provided values
  exec_cmd kubectl create secret generic aws-secret \
    --from-literal=key_id="$ACCESS_KEY_ID" \
    --from-literal=access_key="$SECRET_ACCESS_KEY" \
    -n mount-s3 \
    --dry-run=client -o yaml | kubectl apply -f -
    
  log "S3 credentials secret created/updated."
  
  # Prepare helm command parameters
  local HELM_PARAMS=(
    "$PROJECT_ROOT/charts/scality-mountpoint-s3-csi-driver"
    --namespace mount-s3
    --create-namespace
    --set "node.s3EndpointUrl=$ENDPOINT_URL"
    --wait
  )
  
  # Add image tag if specified
  if [ -n "$IMAGE_TAG" ]; then
    log "Using custom image tag: $IMAGE_TAG"
    HELM_PARAMS+=(--set "image.tag=$IMAGE_TAG")
  fi
  
  # Install/upgrade the Helm chart
  log "Running Helm upgrade with parameters: ${HELM_PARAMS[*]}"
  
  exec_cmd helm upgrade --install scality-s3-csi "${HELM_PARAMS[@]}"
  
  log "CSI driver installation complete."
}

# Verify the installation
verify_installation() {
  log "Verifying CSI driver installation..."
  
  # Wait for the pods to be running
  log "Waiting for CSI driver pods to be in Running state..."
  
  # Maximum wait time in seconds (5 minutes)
  MAX_WAIT_TIME=300
  WAIT_INTERVAL=10
  ELAPSED_TIME=0
  
  while [ $ELAPSED_TIME -lt $MAX_WAIT_TIME ]; do
    if exec_cmd kubectl get pods -n mount-s3 | grep -q "Running"; then
      log "CSI driver pods are now running."
      
      exec_cmd kubectl get pods -n mount-s3
      break
    else
      log "Pods not yet in Running state. Waiting ${WAIT_INTERVAL} seconds... (${ELAPSED_TIME}/${MAX_WAIT_TIME}s)"
      sleep $WAIT_INTERVAL
      ELAPSED_TIME=$((ELAPSED_TIME + WAIT_INTERVAL))
    fi
  done
  
  # Check if we timed out
  if [ $ELAPSED_TIME -ge $MAX_WAIT_TIME ]; then
    log "Timed out waiting for pods to be in Running state. Current pod status:"
    exec_cmd kubectl get pods -n mount-s3
    error "CSI driver pods did not reach Running state within ${MAX_WAIT_TIME} seconds."
  fi
  
  # Check if CSI driver is registered
  log "Checking if CSI driver is registered..."
  
  if exec_cmd kubectl get csidrivers | grep -q "s3.csi.aws.com"; then
    log "CSI driver is registered successfully."
  else
    error "CSI driver is not registered properly."
  fi
}

# Main installation function that will be called from run.sh
do_install() {
  log "Starting Scality CSI driver installation..."
  
  check_dependencies
  
  # Pass all arguments to install_csi_driver
  install_csi_driver "$@"
  
  verify_installation
  log "Scality CSI driver setup completed successfully."
}
