package util

import "os"

func UsePodMounter() bool {
	return os.Getenv("MOUNTER_KIND") == "pod"
}

// SupportLegacySystemdMounts returns true if the driver should support
// existing systemd mounts during upgrade from v1.x to v2.x.
//
// NOTE: This is hardcoded to true for backward compatibility.
// Disabling this is not supported and will cause upgrade failures.
// This function and related systemd mount handling will be removed in a future version.
func SupportLegacySystemdMounts() bool {
	// Always enabled to ensure smooth upgrades from v1.x
	// TODO: S3C-10414 - Remove this method and all legacy systemd mount handling in a future major version
	// We are keeping this method as it will be needed for transition, first deprecation and then removal.
	return true
}
