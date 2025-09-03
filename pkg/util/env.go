package util

import "os"

// UsePodMounter returns true if pod-based mounting should be used.
// Defaults to true (pod mounter) for SELinux and Red Hat OpenShift support.
// Set MOUNTER_KIND=systemd to use the legacy systemd mounter.
func UsePodMounter() bool {
	mounterKind := os.Getenv("MOUNTER_KIND")
	if mounterKind == "" {
		// Default to pod mounter for SELinux support
		return true
	}
	return mounterKind == "pod"
}
