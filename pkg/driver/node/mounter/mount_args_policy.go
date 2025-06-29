package mounter

import (
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	"k8s.io/klog/v2"
)

// enforceCSIDriverMountArgPolicy strips Mountpoint args the CSI driver does not support.
// Reasons include platform limitations, unsupported backend features, and product scope choices.
func enforceCSIDriverMountArgPolicy(args *mountpoint.Args) {
	// The profile flag is not supported in our authentication model
	if _, ok := args.Remove(mountpoint.ArgProfile); ok {
		klog.Warningf("--profile ignored: only static keys are supported by the CSI driver")
	}

	// Volume-specific endpoint overrides are not supported
	if _, ok := args.Remove(mountpoint.ArgEndpointURL); ok {
		klog.Warningf("--endpoint-url ignored: driver does not support per-volume endpoint overrides")
	}

	// These features are not supported by our backend as they are specific to Express One Zone
	if _, ok := args.Remove(mountpoint.ArgExpressOneZoneCache); ok {
		klog.Warningf("--cache-xz ignored: S3 Express One Zone cache is not supported by backend")
	}
	if _, ok := args.Remove(mountpoint.ArgExpressOneZoneIncrementalUpload); ok {
		klog.Warningf("--incremental-upload ignored: S3 Express One Zone append not supported by backend")
	}

	// This driver only supports STANDARD storage class for now so we do not allow the user to override it
	if _, ok := args.Remove(mountpoint.ArgStorageClass); ok {
		klog.Warningf("--storage-class ignored: only STANDARD is supported by the CSI driver")
	}

	// This driver does not support fs-tab
	if _, ok := args.Remove(mountpoint.ArgFsTab); ok {
		klog.Warningf("-o ignored: driver does not support fs-tab")
	}
}
