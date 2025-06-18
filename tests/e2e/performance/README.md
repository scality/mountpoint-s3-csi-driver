# Performance Testing for S3 CSI Driver

This directory contains performance tests for the Scality S3 CSI driver using FIO (Flexible I/O Tester).

## Overview

The performance test suite measures I/O throughput characteristics of the S3 CSI driver under concurrent load conditions. It validates:

- Sequential read/write throughput
- Random read performance
- Performance with multiple concurrent pods
- Minimum throughput under load

## Test Modes

### Light Mode (CI-friendly)

- File sizes: 100MB
- Runtime: 10 seconds per test
- Suitable for CI environments with limited resources
- Total test time: ~5-10 minutes

### Full Mode (Production testing)

- File sizes: 10GB (reads), 100GB (writes)
- Runtime: 30 seconds per test
- Requires significant storage and time
- Total test time: ~20-30 minutes

## Running Tests Locally

### Prerequisites

1. Install the CSI driver:

    ```bash
    source tests/e2e/scripts/load-credentials.sh
    make csi-install S3_ENDPOINT_URL=https://s3.example.com
    ```

2. Run performance tests:

**Light mode (default):**

```bash
make perf-test S3_ENDPOINT_URL=https://s3.example.com
```

**Full mode:**

```bash
make perf-test-full S3_ENDPOINT_URL=https://s3.example.com
```

### Configuration Options

- `PERF_MODE`: `light` (default) or `full`
- `PERF_POD_COUNT`: Number of concurrent pods (default: 3)
- `PERF_FIO_IMAGE`: Custom FIO container image (optional)

Example with custom settings:

```bash
make perf-test \
  S3_ENDPOINT_URL=https://s3.example.com \
  PERF_MODE=light \
  PERF_POD_COUNT=2 \
  PERF_FIO_IMAGE=myregistry/fio:latest
```

## CI Integration

The performance tests are integrated into GitHub Actions via `.github/workflows/performance-tests.yaml`.

### CI Features

1. **Automatic lightweight configs**: CI creates smaller FIO configs automatically
2. **Custom FIO image support**: Avoids runtime installation of FIO
3. **Performance report**: Results are displayed in GitHub Actions summary
4. **Artifacts**: Test results and metrics are uploaded as artifacts

### Manual Trigger

You can manually trigger performance tests with full configuration:

1. Go to Actions → Performance Tests
2. Click "Run workflow"
3. Toggle "Run full performance tests" for production-level testing

## Building Custom FIO Image

To avoid runtime FIO installation:

```bash
make perf-image
# This creates: mountpoint-s3-csi-fio:latest
```

Push to your registry:

```bash
docker tag mountpoint-s3-csi-fio:latest myregistry/fio:latest
docker push myregistry/fio:latest
```

## Understanding Results

Results are saved to `test-results/output.json`:

```json
[
  {
    "name": "seq_write",
    "unit": "MiB/s",
    "value": "125.456000"
  },
  {
    "name": "seq_read",
    "unit": "MiB/s",
    "value": "89.123000"
  },
  {
    "name": "rand_read",
    "unit": "MiB/s",
    "value": "45.678000"
  }
]
```

The values represent the **minimum throughput** across all concurrent pods, ensuring the driver performs acceptably under contention.

## Troubleshooting

### Insufficient Resources

If tests fail due to resource constraints:

- Reduce `PERF_POD_COUNT`
- Use `light` mode
- Ensure nodes have sufficient CPU/memory

### Scheduling Issues

If pods can't schedule on the same node:

- Check node resources
- Consider relaxing the same-node requirement
- Use node labels/taints appropriately

### FIO Installation Timeout

If FIO installation times out:

- Use a custom FIO image
- Check network connectivity
- Increase timeout values

## FIO Configuration Files

### Production configs (`tests/e2e/fio/`)

- `seq_write.fio`: 100GB sequential writes
- `seq_read.fio`: 10GB sequential reads
- `rand_read.fio`: 10GB random reads

### CI configs (`tests/e2e/fio-ci/`)

- All tests use 100MB files
- Runtime reduced to 10 seconds
- Created automatically by CI or via `make perf-test`
