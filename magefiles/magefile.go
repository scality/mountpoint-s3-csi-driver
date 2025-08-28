//go:build mage

package main

import (
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
