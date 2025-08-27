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
	)
}
