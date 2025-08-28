//go:build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
)

var Default = Up

// Up builds and installs the CSI driver from local source
func Up() {
	mg.SerialDeps(
		LoadCredentials,
		BuildImage,
		LoadImageToCluster,
		CreateSecret,
		InstallCSI,
	)
}

// Down removes the CSI driver and associated resources
func Down() {
	mg.SerialDeps(
		UninstallCSI,
		RemoveSecret,
		RemoveDNS,
	)
}

// ConfigureS3DNS configures DNS mapping for s3.example.com
func ConfigureS3DNS() {
	mg.Deps(ConfigureDNS)
}

// RemoveS3DNS removes DNS mapping for s3.example.com
func RemoveS3DNS() {
	mg.Deps(RemoveDNS)
}

// ShowS3DNS shows the current S3 DNS configuration and tests it
func ShowS3DNSStatus() {
	mg.Deps(ShowS3DNS)
}

// Install installs a specific CSI version from OCI registry (requires SCALITY_CSI_VERSION)
func Install() error {
	// Check version first before doing anything
	if GetCSIChartVersion() == "" {
		return fmt.Errorf("SCALITY_CSI_VERSION environment variable is required for 'mage install'\n\n" +
			"Usage:\n" +
			"  SCALITY_CSI_VERSION=<version> mage install\n\n" +
			"Examples:\n" +
			"  SCALITY_CSI_VERSION=1.2.0 mage install    # Install version 1.2.0\n" +
			"  SCALITY_CSI_VERSION=1.1.0 mage install    # Install version 1.1.0\n\n" +
			"To see available versions:\n" +
			"  helm search repo oci://ghcr.io/scality/mountpoint-s3-csi-driver/helm-charts --versions\n\n" +
			"For local development builds, use:\n" +
			"  mage up")
	}

	mg.SerialDeps(
		LoadCredentials,
		CreateSecret,
		InstallCSIWithVersion,
	)
	return nil
}
