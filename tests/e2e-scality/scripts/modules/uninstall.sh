#!/bin/bash
# uninstall.sh - Uninstallation functions for e2e-scality scripts

# Source common functions
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"

# Define error codes
readonly ERROR_HELM_UNINSTALL=10
readonly ERROR_NS_DELETE=11
readonly ERROR_CSI_DELETE=12

# Uninstall the CSI driver
uninstall_csi_driver() {
  log "Uninstalling Scality CSI driver..."
  
  # Process arguments
  local DELETE_NS=false
  local FORCE=false
  
  # Parse arguments
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --delete-ns)
        DELETE_NS=true
        shift
        ;;
      --force)
        DELETE_NS=true
        FORCE=true
        shift
        ;;
      *)
        warn "Unknown parameter for uninstall: $1"
        shift
        ;;
    esac
  done
  
  # Check if the namespace exists
  if ! exec_cmd kubectl get namespace mount-s3 &> /dev/null; then
    warn "Namespace mount-s3 does not exist. Nothing to uninstall."
    return 0
  fi
  
  # Check if the Helm release exists
  if ! exec_cmd helm status scality-s3-csi -n mount-s3 &> /dev/null; then
    warn "Helm release scality-s3-csi not found in namespace mount-s3."
  else
    # Uninstall the Helm release
    log "Uninstalling Helm release scality-s3-csi..."
    if ! exec_cmd helm uninstall scality-s3-csi -n mount-s3; then
      error "Failed to uninstall Helm release. Error code: $ERROR_HELM_UNINSTALL"
      if [ "$FORCE" = true ]; then
        warn "Force mode enabled. Continuing with namespace deletion despite Helm uninstall failure."
      else
        return $ERROR_HELM_UNINSTALL
      fi
    else
      log "Helm release uninstalled successfully."
    fi
  fi
  
  # Delete the namespace if requested or ask interactively
  if [ "$FORCE" = true ]; then
    log "Force mode enabled. Deleting namespace mount-s3..."
    if ! exec_cmd kubectl delete namespace mount-s3 --timeout=60s; then
      error "Failed to delete namespace mount-s3. Error code: $ERROR_NS_DELETE"
      warn "You may need to manually delete stuck resources in the namespace."
      if [ "$FORCE" = true ]; then
        warn "Continuing with uninstallation despite namespace deletion failure."
      else
        return $ERROR_NS_DELETE
      fi
    else
      log "Namespace deleted successfully."
    fi
  elif [ "$DELETE_NS" = true ]; then
    log "Deleting namespace mount-s3..."
    if ! exec_cmd kubectl delete namespace mount-s3 --timeout=60s; then
      error "Failed to delete namespace mount-s3. Error code: $ERROR_NS_DELETE"
      return $ERROR_NS_DELETE
    else
      log "Namespace deleted successfully."
    fi
  else
    # Interactive mode
    read -p "Do you want to delete the mount-s3 namespace and all its resources? (y/N): " DELETE_NAMESPACE
    if [[ "$DELETE_NAMESPACE" =~ ^[Yy]$ ]]; then
      log "Deleting namespace mount-s3..."
      if ! exec_cmd kubectl delete namespace mount-s3 --timeout=60s; then
        error "Failed to delete namespace mount-s3. Error code: $ERROR_NS_DELETE"
        return $ERROR_NS_DELETE
      else
        log "Namespace deleted successfully."
      fi
    else
      log "Keeping namespace mount-s3."
    fi
  fi
  
  # Check if CSI driver is still registered
  if exec_cmd kubectl get csidrivers | grep -q "s3.csi.aws.com"; then
    warn "CSI driver s3.csi.aws.com is still registered. You may need to delete it manually:"
    warn "kubectl delete csidriver s3.csi.aws.com"
    
    # In force mode, automatically delete the CSI driver
    if [ "$FORCE" = true ]; then
      log "Force mode enabled. Deleting CSI driver s3.csi.aws.com..."
      if ! exec_cmd kubectl delete csidriver s3.csi.aws.com; then
        error "Failed to delete CSI driver. Error code: $ERROR_CSI_DELETE"
        warn "You may need to manually delete the CSI driver registration."
        return $ERROR_CSI_DELETE
      else
        log "CSI driver deleted successfully."
      fi
    fi
  else
    log "CSI driver is no longer registered."
  fi
  
  log "Uninstallation complete."
  return 0
}

# Main uninstall function that will be called from run.sh
do_uninstall() {
  # Call uninstall_csi_driver with all arguments
  uninstall_csi_driver "$@"
  local result=$?
  
  if [ $result -ne 0 ]; then
    case $result in
      $ERROR_HELM_UNINSTALL)
        fatal "Uninstallation failed during Helm release uninstall. Please check the logs for details."
        ;;
      $ERROR_NS_DELETE)
        fatal "Uninstallation failed during namespace deletion. Please check the logs for details."
        ;;
      $ERROR_CSI_DELETE)
        fatal "Uninstallation failed during CSI driver deletion. Please check the logs for details."
        ;;
      *)
        fatal "Uninstallation failed with unknown error code: $result"
        ;;
    esac
  fi
}
