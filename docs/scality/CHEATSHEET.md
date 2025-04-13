# Scality S3 CSI Driver Cheat Sheet

## Installation

### Basic Installation
```bash
make csi-install \
  S3_ENDPOINT_URL=http://localhost:8000 \
  ACCESS_KEY_ID=accessKey1 \
  SECRET_ACCESS_KEY=verySecretKey1 \
  VALIDATE_S3=true
```

### Installation with Options
```bash
make csi-install \
  CSI_IMAGE_TAG=v1.14.0 \
  CSI_IMAGE_REPOSITORY=my-registry/mountpoint-s3-csi-driver \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret \
  VALIDATE_S3=true
```

## Testing

### Basic Testing of Already Installed Driver
```bash
make e2e-scality
```

### Run Only Basic Verification Tests (Skip Go Tests)
```bash
make e2e-scality-verify
```
This command only checks if:
- The CSI driver pods are running correctly in the namespace
- The CSI driver is properly registered in the cluster
It skips running the Go-based tests.

### Run Only Go-Based End-to-End Tests
```bash
make e2e-scality-go
```

### Advanced Testing with Go Test (for filtering tests)
```bash
# Go to the tests directory
cd tests/e2e-scality/e2e-tests

# Run tests with focus on specific test patterns (runs only matching tests)
go test -v -tags=e2e -ginkgo.focus="Basic Functionality"

# Skip specific test patterns
go test -v -tags=e2e -ginkgo.skip="Volume Operations"

# Run tests in a specific namespace (default is "mount-s3")
go test -v -tags=e2e -namespace="custom-namespace"

# Combine multiple filters
go test -v -tags=e2e -ginkgo.focus="Basic" -ginkgo.skip="Volume" -namespace="mount-s3"
```

### Install and Test in One Step
```bash
make e2e-scality-all \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret
```

### Install with Custom Image and Test
```bash
make e2e-scality-all \
  CSI_IMAGE_TAG=v1.14.0 \
  CSI_IMAGE_REPOSITORY=my-registry/mountpoint-s3-csi-driver \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret
```

## Uninstallation

### Interactive Uninstall (will prompt)
```bash
make csi-uninstall
```

### Auto Uninstall (no prompts)
```bash
make csi-uninstall-clean
```

### Force Uninstall (for stuck resources)
```bash
make csi-uninstall-force
```

## Common Configurations

### Local Development
```bash
make csi-install \
  S3_ENDPOINT_URL=http://localhost:8000 \
  ACCESS_KEY_ID=localkey \
  SECRET_ACCESS_KEY=localsecret
```

### Scality Ring
```bash
make csi-install \
  S3_ENDPOINT_URL=https://s3.ring.example.com \
  ACCESS_KEY_ID=ringkey \
  SECRET_ACCESS_KEY=ringsecret
```

### Scality Artesca
```bash
make csi-install \
  S3_ENDPOINT_URL=https://s3.artesca.example.com \
  ACCESS_KEY_ID=artescakey \
  SECRET_ACCESS_KEY=artescasecret
``` 