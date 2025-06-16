# E2E Test Scripts

These scripts provide automation for installing, testing, and managing the Scality S3 CSI Driver in Kubernetes environments.

## Scripts

- `install`: Installs and verifies the CSI driver with the given parameters
- `test`: Runs end-to-end tests on an already installed CSI driver
- `uninstall`: Removes the CSI driver from the cluster
- `all`: Combines install, test, and uninstall operations
- `go-test`: Runs only the Go-based end-to-end tests
- `load-credentials.sh`: Loads S3 credentials from JSON configuration file and exports as environment variables

## Quick Examples

Start a documentation server:

```bash
# From project root
mkdocs serve
```

This will start a development server at <http://localhost:8000> with live reloading for documentation changes.

For complete usage instructions and examples, see the main project documentation and the individual script help output
(`./run.sh <command> --help`).

## Script Organization

### Core Automation Scripts

- `run.sh`: Main entry point that delegates to specific operation scripts
- `install.sh`: Handles CSI driver installation with validation
- `test.sh`: Executes test suites with configurable parameters
- `cleanup.sh`: Provides cleanup and uninstall functionality

### Module System

The scripts use a modular design with shared functionality in the `modules/` directory:

- `validation.sh`: Input validation and prerequisite checking
- `k8s.sh`: Kubernetes cluster interaction utilities  
- `s3.sh`: S3 endpoint validation and bucket operations
- `logging.sh`: Consistent logging and error reporting

### Configuration Files

- `config/`: Default configuration templates
- `templates/`: YAML templates for Kubernetes resources
- `../integration_config.json`: S3 credentials configuration file for testing

## Current Structure

The main entry point is `run.sh` which supports the following commands:

- `install`: Installs and verifies the CSI driver
- `test`: Runs end-to-end tests
- `go-test`: Runs only Go-based tests directly (skips verification checks)
- `all`: Installs the driver and runs tests
- `uninstall`: Uninstalls the CSI driver
- `help`: Shows usage information

## Required Parameters

For installation, the following parameters are required:

- `--endpoint-url`: S3 endpoint URL (e.g., <http://localhost:8000>)
- `--access-key-id`: S3 access key for authentication (used to create Kubernetes secret)
- `--secret-access-key`: S3 secret key for authentication (used to create Kubernetes secret)

For tests, only the endpoint URL is required:

- `--endpoint-url`: S3 endpoint URL (e.g., <http://localhost:8000>)

Tests use the credentials stored in the Kubernetes secret created during installation.

## Environment Variables

- `KUBECONFIG`: Path to the Kubernetes configuration file (required if not using the default ~/.kube/config)
- `CREDENTIALS_CONFIG_FILE`: Path to custom credentials JSON file (optional, defaults to `../integration_config.json`)

## Optional Parameters

- `--namespace`: Specify the namespace to use (default: kube-system)
- `--skip-go-tests`: Skip executing Go-based end-to-end tests (for test command)
- `--junit-report`: Generate JUnit XML report at specified path (for test command)

## Credentials Management

The `load-credentials.sh` script loads S3 credentials from JSON and exports them as `ACCOUNT1_*` and `ACCOUNT2_*` environment variables.

### Quick Start

```bash
# Load credentials (uses ../integration_config.json by default)
source ./load-credentials.sh

# Install driver with credentials (creates Kubernetes secret)
./run.sh install --endpoint-url http://localhost:8000

# Run tests (uses credentials from Kubernetes secret)
./run.sh test --endpoint-url http://localhost:8000

# Clean up when done
unset ACCOUNT1_ACCESS_KEY ACCOUNT1_SECRET_KEY ACCOUNT1_CANONICAL_ID ACCOUNT2_ACCESS_KEY ACCOUNT2_SECRET_KEY ACCOUNT2_CANONICAL_ID
```

### Custom Config File

```bash
# Use different config file
source ./load-credentials.sh --config-file /path/to/config.json

# Or with environment variable
CREDENTIALS_CONFIG_FILE=/path/to/config.json source ./load-credentials.sh
```

## Usage

Scripts in this directory can be called directly or from the Makefile targets.

### Direct script usage

```bash
# Load credentials first
source ./load-credentials.sh

# Install the driver (creates Kubernetes secret with credentials)
./run.sh install --endpoint-url http://localhost:8000

# Run tests (uses credentials from Kubernetes secret)
./run.sh test --endpoint-url http://localhost:8000

# Run only Go tests
./run.sh go-test --endpoint-url http://localhost:8000

# Install and test in one command
./run.sh all --endpoint-url http://localhost:8000
```

### Using Makefile targets

```bash
# Show all available CSI commands
make help-csi

# Load credentials (required)
source tests/e2e/scripts/load-credentials.sh

# Install the driver (only endpoint URL needed)
make csi-install S3_ENDPOINT_URL=http://localhost:8000

# Run tests (uses credentials from Kubernetes secret)
make e2e S3_ENDPOINT_URL=http://localhost:8000

# Run only Go tests
make e2e-go S3_ENDPOINT_URL=http://localhost:8000

# Install and test in one command
make e2e-all S3_ENDPOINT_URL=http://localhost:8000 CSI_IMAGE_TAG=<image-tag> CSI_IMAGE_REPOSITORY=ghcr.io/scality/mountpoint-s3-csi-driver
```
