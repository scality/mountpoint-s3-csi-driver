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
  log "Checking endpoint connectivity: $endpoint_url"
  
  # Create a temporary file for capturing output
  local temp_output=$(mktemp)
  
  # Step 1: Basic endpoint connectivity check with curl
  local http_code=$(curl -s -o "$temp_output" -w "%{http_code}" "$endpoint_url" 2>/dev/null)
  
  if [[ "$http_code" == 2* ]] || [[ "$http_code" == 3* ]]; then
    log "S3 endpoint is reachable (HTTP $http_code)"
    
    # If we get a 403 Forbidden/Access Denied, that's good because it means the endpoint exists and is an S3 service
    if [[ "$http_code" == "403" ]] || grep -q "AccessDenied\|InvalidAccessKeyId" "$temp_output"; then
      log "Endpoint is confirmed as an S3 service (received access denied, which is expected without credentials)"
      
      # Step 2: Check credentials with AWS CLI if it's installed
      if command -v aws &> /dev/null; then
        log "AWS CLI found, validating access key and secret key..."
        
        # Use environment variables method for AWS credentials
        if AWS_ACCESS_KEY_ID="$access_key_id" AWS_SECRET_ACCESS_KEY="$secret_access_key" exec_cmd aws --endpoint-url "$endpoint_url" s3 ls > "$temp_output" 2>&1; then
          log "SUCCESS: AWS access key and secret key validated successfully!"
          log "Available buckets:"
          cat "$temp_output"
        else
          error "Failed to validate AWS credentials. Error details:"
          cat "$temp_output"
          log "Please check your access key and secret key."
          rm -f "$temp_output"
          return 1
        fi
      else
        log "AWS CLI not installed - cannot validate access key and secret key."
        log "Only basic endpoint connectivity was confirmed."
        log "Proceeding with installation, but credential issues might occur later."
      fi
      
      # Clean up temporary file
      rm -f "$temp_output"
      return 0
    else
      log "Endpoint is reachable but response doesn't seem to be from an S3 service."
      log "Response received:"
      cat "$temp_output"
    fi
  else
    error "Failed to connect to S3 endpoint (HTTP code: $http_code)"
    log "Please check if the endpoint URL is correct and the S3 service is running."
    cat "$temp_output"
    rm -f "$temp_output"
    return 1
  fi
  
  # If we get here, the endpoint is reachable but we couldn't confirm it's an S3 service
  log "Proceeding with installation, but S3 configuration issues might occur."
  rm -f "$temp_output"
  return 0
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
