# E2E Test Scripts

## Mage Targets (Primary)

E2E testing is orchestrated via Mage targets defined in `magefiles/e2e.go`. Run `mage -l` to see all available targets.

```bash
# Full workflow: load credentials, install driver, run tests
S3_ENDPOINT_URL=http://s3.example.com:8000 mage e2e:all

# Install only
S3_ENDPOINT_URL=http://s3.example.com:8000 mage e2e:install

# Run tests on already-installed driver
S3_ENDPOINT_URL=http://s3.example.com:8000 mage e2e:test

# Run only Go tests (skip verification)
S3_ENDPOINT_URL=http://s3.example.com:8000 mage e2e:goTest

# Verify driver health
mage e2e:verify

# Uninstall
mage e2e:uninstall
mage e2e:uninstallClean   # also delete custom namespace
mage e2e:uninstallForce   # force delete including CSI driver registration

# CI diagnostics: capture Kubernetes events and logs
mage e2e:startCapture     # start background capture
mage e2e:stopCapture      # stop capture, compress, collect S3 logs
```

### Environment Variables

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `S3_ENDPOINT_URL` | Yes (install/test) | — | S3 endpoint URL |
| `CSI_IMAGE_TAG` | No | chart default | Image tag for Helm install |
| `CSI_IMAGE_REPOSITORY` | No | chart default | Image repo for Helm install |
| `CSI_NAMESPACE` | No | `kube-system` | Target namespace |
| `JUNIT_REPORT` | No | — | JUnit XML output path |
| `KUBECONFIG` | No | `~/.kube/config` | Kubernetes config |

### Makefile Compatibility

The Makefile targets (`make csi-install`, `make e2e-all`, etc.) delegate to Mage internally.

## Remaining Scripts

- `capture-events-and-logs.sh` — Background Kubernetes event and log capture for CI diagnostics. Wrapped by `mage e2e:startCapture` and `mage e2e:stopCapture`.
