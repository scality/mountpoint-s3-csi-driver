package util

import "os"

func UsePodMounter() bool {
	return os.Getenv("MOUNTER_KIND") == "pod"
}

// SupportLegacySystemdMounts returns true if the driver should support
// existing systemd mounts during upgrade from v1.x to v2.x.
// When enabled, existing systemd mounts will be preserved and only
// credentials will be refreshed. New mounts will use pod mounter.
func SupportLegacySystemdMounts() bool {
	return os.Getenv("SUPPORT_LEGACY_SYSTEMD_MOUNTS") == "true"
}
