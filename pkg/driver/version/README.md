# Version Package

This package handles version information for the S3 CSI Driver.

## Overview

The version package provides a standardized way to access build-time and runtime version information about the driver. This is useful for:

1. Logging version information during startup
2. Reporting version via metrics
3. Debugging version-specific issues
4. Displaying version information in CLI tools

## Files in This Package

### version.go

This file defines the `VersionInfo` structure and provides methods to access version information:

- `GetVersion()`: Returns a `VersionInfo` structure with all version details
- `GetVersionJSON()`: Returns a JSON string representation of the version information

The version information includes:
- `driverVersion`: The semantic version of the driver
- `gitCommit`: The Git commit hash of the build
- `buildDate`: The timestamp when the driver was built
- `goVersion`: The Go version used to compile the driver
- `compiler`: The compiler used
- `platform`: The OS/architecture platform

These values are populated at build time via `-ldflags` in the Makefile.
TODO [before-release]: Answer question on how we can pass new version for releases

## Usage

The version information is typically accessed at driver startup to log the version details:

```go
version := version.GetVersion()
klog.Infof("Driver version: %v, Git commit: %v, build date: %v",
    version.DriverVersion, version.GitCommit, version.BuildDate)
```

This helps with debugging and ensuring the correct version is deployed.
