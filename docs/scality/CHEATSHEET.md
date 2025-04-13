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

### Run Only Go-Based End-to-End Tests
```bash
make e2e-scality-go
```

### Run Go Tests with Filters
```bash
# Run tests matching a specific pattern
make e2e-scality-go FOCUS="Basic Functionality"

# Skip tests matching a specific pattern
make e2e-scality-go SKIP="Volume Operations"

# Run tests in a specific namespace
make e2e-scality-go NAMESPACE="custom-namespace"

# Combine multiple options
make e2e-scality-go FOCUS="Basic" SKIP="Volume" NAMESPACE="mount-s3"
```

### Install and Test in One Step
```bash
make e2e-scality-all \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret
```

### Run End-to-End Tests with Filtering (Install and Test)
```bash
make e2e-scality-all \
  S3_ENDPOINT_URL=https://s3.example.com \
  ACCESS_KEY_ID=your_key \
  SECRET_ACCESS_KEY=your_secret \
  FOCUS="Basic Functionality" \
  SKIP="Volume Operations"
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