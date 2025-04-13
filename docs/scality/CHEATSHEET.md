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

### Test Already Installed Driver
```bash
make e2e-scality
```

### Install and Test in One Step
```bash
make e2e-scality-all \
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