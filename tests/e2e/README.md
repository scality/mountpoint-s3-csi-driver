# E2E Tests

End-to-end tests for the Scality CSI Driver for S3 that validate functionality in real Kubernetes environments.

## Quick Start

```bash
# Full workflow: load credentials, install driver, run tests
S3_ENDPOINT_URL=https://your-s3-endpoint.com mage e2e:all
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
- **Credentials**: Loaded automatically from `integration_config.json` by `mage e2e:all`

### Optional

- **KUBECONFIG**: Path to kubeconfig (default: `~/.kube/config`)
- **CSI_NAMESPACE**: Kubernetes namespace (default: `kube-system`)

## Examples

```bash
# Full workflow (loads credentials from integration_config.json automatically)
S3_ENDPOINT_URL=http://10.200.4.125:8000 mage e2e:all

# With custom kubeconfig
KUBECONFIG=/path/to/config S3_ENDPOINT_URL=https://s3.example.com mage e2e:all

# Run only Go tests
S3_ENDPOINT_URL=https://s3.example.com mage e2e:goTest
```

## Credential Configuration

Credentials are loaded automatically from `tests/e2e/integration_config.json` by `mage e2e:all`.
The Mage target reads the JSON file and sets `ACCOUNT1_ACCESS_KEY` and `ACCOUNT1_SECRET_KEY`
environment variables.

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

1. A running Kubernetes cluster with the CSI driver installed
2. S3 credentials set as environment variables (or use `mage e2e:all` which loads them automatically)
3. `S3_ENDPOINT_URL` environment variable set
4. `KUBECONFIG` pointing to your cluster

**Basic setup:**

```bash
# Set environment variables
export KUBECONFIG=~/.kube/config
export S3_ENDPOINT_URL="http://s3.example.com:8000"

# Navigate to e2e directory
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
