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

### Installation with Custom Namespace
```bash
make csi-install \
  S3_ENDPOINT_URL=http://localhost:8000 \
  ACCESS_KEY_ID=accessKey1 \
  SECRET_ACCESS_KEY=verySecretKey1 \
  CSI_NAMESPACE=custom-namespace
```

### Installation with All Options
```bash
make csi-install \
  CSI_IMAGE_TAG=v1.14.0 \
  CSI_IMAGE_REPOSITORY=my-registry/mountpoint-s3-csi-driver \
  CSI_NAMESPACE=custom-namespace \
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

### Testing with Custom Namespace
```bash
make e2e-scality CSI_NAMESPACE=custom-namespace
```

### Run Only Basic Verification Tests (Skip Go Tests)
```bash
make e2e-scality-verify CSI_NAMESPACE=custom-namespace
```
This command only checks if:
- The CSI driver pods are running correctly in the specified namespace (or in any namespace as a fallback)
- The CSI driver is properly registered in the cluster
It skips running the Go-based tests.

### Run Only Go-Based End-to-End Tests
```bash
make e2e-scality-go CSI_NAMESPACE=custom-namespace
```

### Advanced Testing with Go Test (for filtering tests)
```bash
# Go to the tests directory
cd tests/e2e-scality/e2e-tests

# Run tests with focus on specific test patterns (runs only matching tests)
go test -v -tags=e2e -ginkgo.focus="Basic Functionality" -args -namespace=custom-namespace

# Skip specific test patterns
go test -v -tags=e2e -ginkgo.skip="Volume Operations" -args -namespace=custom-namespace

# Combine multiple filters
go test -v -tags=e2e -ginkgo.focus="Basic" -ginkgo.skip="Volume" -args -namespace=custom-namespace
```

### Install and Test in One Step
```bash
make e2e-scality-all \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret
```

### Install with Custom Namespace and Test
```bash
make e2e-scality-all \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret \
  CSI_NAMESPACE=custom-namespace
```

## Uninstallation

### Interactive Uninstall (will prompt)
```bash
make csi-uninstall
```

### Uninstall from a Custom Namespace
```bash
make csi-uninstall CSI_NAMESPACE=custom-namespace
```

### Auto Uninstall (no prompts)
```bash
make csi-uninstall-clean CSI_NAMESPACE=custom-namespace
```

### Force Uninstall (for stuck resources)
```bash
make csi-uninstall-force CSI_NAMESPACE=custom-namespace
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
  SECRET_ACCESS_KEY=ringsecret \
  CSI_NAMESPACE=scality-ring
```

### Scality Artesca
```bash
make csi-install \
  S3_ENDPOINT_URL=https://s3.artesca.example.com \
  ACCESS_KEY_ID=artescakey \
  SECRET_ACCESS_KEY=artescasecret \
  CSI_NAMESPACE=scality-artesca
``` 