# E2E Tests

End-to-end tests for the Scality CSI Driver for S3 that validate functionality in real Kubernetes environments.

## Quick Start

```bash
# 1. Load credentials (from integration_config.json)
source tests/e2e/scripts/load-credentials.sh

# 2. Run tests
make e2e-all S3_ENDPOINT_URL=https://your-s3-endpoint.com
```

## Available Commands

| Command | Description |
|---------|-------------|
| `make e2e-all` | Install driver + run all tests |
| `make e2e` | Run tests on existing driver |
| `make e2e-go` | Run only Go-based tests |

## Configuration

### Required

- **S3_ENDPOINT_URL**: Your S3 endpoint URL
- **Credentials**: Load from `integration_config.json` using the load-credentials script

### Optional

- **KUBECONFIG**: Path to kubeconfig (default: `~/.kube/config`)
- **CSI_NAMESPACE**: Kubernetes namespace (default: `kube-system`)

## Examples

```bash
# Basic usage (loads from default integration_config.json)
source tests/e2e/scripts/load-credentials.sh
make e2e-all S3_ENDPOINT_URL=http://10.200.4.125:8000

# Using custom credentials file
source tests/e2e/scripts/load-credentials.sh --config-file /path/to/my-credentials.json
make e2e-all S3_ENDPOINT_URL=https://s3.example.com

# With custom kubeconfig
make e2e-all S3_ENDPOINT_URL=https://s3.example.com KUBECONFIG=/path/to/config

# Run only Go tests
make e2e-go S3_ENDPOINT_URL=https://s3.example.com
```

## Credential Configuration

The `load-credentials.sh` script reads S3 credentials from a JSON configuration file and exports them as environment variables.

### Default Configuration

By default, credentials are loaded from `tests/e2e/integration_config.json`:

### Using Custom Configuration File

```bash
# Load from custom file
source tests/e2e/scripts/load-credentials.sh --config-file /path/to/my-config.json

# Or set environment variable
export CREDENTIALS_CONFIG_FILE="/path/to/my-config.json"
source tests/e2e/scripts/load-credentials.sh
```

## Running Specific Test Suites

You can run specific test suites using ginkgo's `--focus` flag. Below are all available test suites:

### Available Test Suites (--focus options)

| Focus Pattern | Description | Prerequisites |
|---------------|-------------|---------------|
| `"credentials"` | Authentication scenarios and credential handling | Multi-account credentials loaded |
| `"mountoptions"` | Mount option validation and behavior | Standard credentials |
| `"multivolume"` | Multiple volume mounting scenarios | Standard credentials |
| `"cache"` | Local cache functionality testing | Standard credentials |
| `"filepermissions"` | File permission handling on mounted volumes | Standard credentials |
| `"directorypermissions"` | Directory permission handling | Standard credentials |
| `"CSI Volumes"` | **All test suites** (includes standard CSI compliance + custom tests) | Standard credentials |

### Running Specific Test Suites

**Prerequisites (for all test suites):**

1. Load credentials from `integration_config.json`
2. Set KUBECONFIG to point to your cluster
3. Have a running Kubernetes cluster with the CSI driver installed
4. Set S3_ENDPOINT_URL environment variable

**Basic setup:**

```bash
# 1. Load credentials
source tests/e2e/scripts/load-credentials.sh

# 2. Set environment variables
export KUBECONFIG=~/.kube/config
export S3_ENDPOINT_URL="http://s3.example.com:8000"

# 3. Navigate to e2e directory
cd tests/e2e
```

**Examples:**

```bash
# Run credentials tests
ginkgo run -v --focus "credentials" .

# Run mount options tests
ginkgo run -v --focus "mountoptions" .

# Run ALL test suites (includes standard CSI compliance + custom tests)
ginkgo run -v --focus "CSI Volumes" .

# Run multiple test suites (using regex pattern)
ginkgo run -v --focus "credentials|mountoptions" .

# Alternative: Run with S3 endpoint as flag
ginkgo run -v --focus "credentials" . -- --s3-endpoint-url=http://s3.example.com:8000
```

## Troubleshooting

- **Authentication errors**: Check credentials in `integration_config.json`
- **Network issues**: Verify S3 endpoint is accessible
- **KUBECONFIG errors**: Ensure kubectl can connect to your cluster
