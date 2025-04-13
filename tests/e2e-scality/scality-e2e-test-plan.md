# Scality S3 CSI Driver E2E Test Plan

## Background
This document outlines a comprehensive plan for implementing end-to-end (E2E) tests for the Scality S3 CSI Driver. These tests will verify that the driver functions correctly in a Kubernetes environment, interacting properly with Scality S3 storage.

## Approach
- Start completely from scratch (no copying existing code)
- **Use the Kubernetes E2E testing framework** as done in e2e-kubernetes folder
- Follow the same patterns and interfaces as the AWS implementation
- Implement required Kubernetes E2E framework interfaces
- Leverage standard Kubernetes storage test suites
- Create custom test suites for Scality-specific features
- Focus on understanding the testing framework first
- Implement one simple test initially
- Build a robust foundation for future test expansion
- Ensure proper integration with Makefile and run.sh scripts
- Test locally before committing any code
- Add option to disable cleanup for debugging purposes
- Document and verify each step before proceeding

## Initial Cleanup
Before beginning implementation, we will:
1. Remove all existing files in the `tests/e2e-scality/e2e-tests/` directory
2. Keep only the test plan and create new directories as needed
3. Use the e2e-kubernetes folder as reference (not copying code)

## Documentation and Verification Process
After each phase or major step:

1. **Documentation Updates**
   - Update README.md with new functionality
   - Add code comments for new functions and types
   - Document any configuration changes
   - Update troubleshooting guide if needed
   - Add examples of new commands or features

2. **Verification Steps**
   - Execute end-to-end tests for the CSI driver
   - Focus only on testing driver functionality in a Kubernetes environment
   - Verify with different configuration combinations
   - Test error handling and edge cases
   - Document test results

3. **Commit Process**
   - Create detailed commit message
   - Include test results in commit description
   - Reference related issues or documentation
   - Push changes to feature branch
   - Create pull request if ready for review

## Phase 1: Kubernetes E2E Framework Integration

1. **Configure Project Dependencies**
   - Add Kubernetes E2E framework dependencies to go.mod
   - Import required packages from k8s.io/kubernetes/test/e2e/framework
   - Import standard test suites from k8s.io/kubernetes/test/e2e/storage/testsuites
   - Set up proper module versioning

2. **Implement Test Driver Interfaces**
   ```go
   // In testdriver.go
   type ScalityDriver struct {
       client     *s3client.Client
       driverInfo framework.DriverInfo
   }
   
   // Implement required interfaces
   var _ framework.TestDriver = &ScalityDriver{}
   var _ framework.PreprovisionedVolumeTestDriver = &ScalityDriver{}
   var _ framework.PreprovisionedPVTestDriver = &ScalityDriver{}
   
   // Implement required methods
   func (d *ScalityDriver) GetDriverInfo() *framework.DriverInfo
   func (d *ScalityDriver) SkipUnsupportedTest(pattern framework.TestPattern)
   func (d *ScalityDriver) PrepareTest(ctx context.Context, f *f.Framework) *framework.PerTestConfig
   func (d *ScalityDriver) CreateVolume(ctx context.Context, config *framework.PerTestConfig, volumeType framework.TestVolType) framework.TestVolume
   func (d *ScalityDriver) GetPersistentVolumeSource(readOnly bool, fsType string, volume framework.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity)
   ```

3. **Create S3 Client Package**
   ```go
   // In pkg/s3client/client.go
   type Client struct {
       client     *s3.Client
       bucketName string
       prefix     string
   }
   
   // Create methods similar to AWS implementation
   func (c *Client) CreateStandardBucket(ctx context.Context) (string, DeleteBucketFunc)
   func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error
   ```

4. **Set Up Configuration**
   ```go
   // In e2e_test.go
   func init() {
       testing.Init()
       f.RegisterClusterFlags(flag.CommandLine) // configures --kubeconfig flag
       f.RegisterCommonFlags(flag.CommandLine)  // configures --kubectl flag
       f.AfterReadingAllFlags(&f.TestContext)
   
       // Add custom flags
       flag.StringVar(&CommitId, "commit-id", "local", "commit id will be used to name buckets")
       flag.StringVar(&BucketRegion, "region", "us-east-1", "region where temporary buckets will be created")
       flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test-", "prefix for temporary buckets")
       flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "S3 endpoint URL")
       flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
       flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
       flag.BoolVar(&SkipCleanup, "skip-cleanup", false, "Skip resource cleanup after tests")
       flag.Parse()
   }
   ```

### Phase 1 Documentation and Verification
After implementing Phase 1:

1. **Documentation**
   ```bash
   # Update documentation
   cd tests/e2e-scality
   # Document framework integration
   # Document test driver interfaces
   # Document S3 client usage
   ```

2. **Verification**
   ```bash
   # No tests yet, verify compilation only
   go build ./...
   
   # Check framework integration
   go test -v ./... -run=TestNothing
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 1: Kubernetes E2E Framework Integration
   
   - Added Kubernetes E2E framework dependencies
   - Implemented test driver interfaces
   - Created S3 client package
   - Set up configuration flags
   - Framework successfully integrated"
   ```

## Phase 2: Standard Test Suite Integration

1. **Configure Standard Test Suites**
   ```go
   // In e2e_test.go
   var StandardTestSuites = []func() framework.TestSuite{
       testsuites.InitVolumesTestSuite,         // Basic volume tests
       testsuites.InitVolumeIOTestSuite,        // IO operations on volumes
       testsuites.InitVolumeModeTestSuite,      // Different volume modes
       testsuites.InitSubPathTestSuite,         // Subpath tests
       testsuites.InitProvisioningTestSuite,    // Provisioning tests
       testsuites.InitMultiVolumeTestSuite,     // Multiple volumes tests
   }
   
   // This executes testSuites for csi volumes.
   var _ = utils.SIGDescribe("Scality CSI Volumes", func() {
       curDriver := initScalityDriver()
   
       args := framework.GetDriverNameWithFeatureTags(curDriver)
       args = append(args, func() {
           framework.DefineTestSuites(curDriver, StandardTestSuites)
       })
       f.Context(args...)
   })
   ```

2. **Create Test Volume Management**
   ```go
   // In testdriver.go
   type ScalityVolume struct {
       bucketName   string
       deleteBucket s3client.DeleteBucketFunc
   }
   
   func (v *ScalityVolume) DeleteVolume(ctx context.Context) {
       err := v.deleteBucket(ctx)
       f.ExpectNoError(err, "Failed to delete S3 Bucket: %s", v.bucketName)
   }
   ```

3. **Set Up Run Script Integration**
   - Update scripts/run.sh to properly pass framework flags
   - Add support for Kubernetes E2E framework parameters
   - Configure proper parameter passing to Go tests

### Phase 2 Documentation and Verification
After implementing Phase 2:

1. **Documentation**
   ```bash
   # Update framework documentation
   cd tests/e2e-scality
   # Document standard test suites
   # Document run script usage
   ```

2. **Verification**
   ```bash
   # Test with minimal standard test suites
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --kubeconfig="$HOME/.kube/config"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 2: Standard Test Suite Integration
   
   - Configured standard Kubernetes test suites
   - Implemented test volume management
   - Set up run script integration
   - Tested with minimal standard test suites"
   ```

## Phase 3: Custom Test Suites for Scality

1. **Create Custom Test Suites**
   ```go
   // In testsuites/mountoptions.go
   func InitScalityMountOptionsTestSuite() framework.TestSuite {
       return &scalityMountOptionsTestSuite{}
   }
   
   type scalityMountOptionsTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   
   // In e2e_test.go
   var CustomTestSuites = []func() framework.TestSuite{
       testsuites.InitScalityMountOptionsTestSuite,
       testsuites.InitScalityMultiVolumeTestSuite,
       testsuites.InitScalityCredentialsTestSuite,
   }
   ```

2. **Implement Mount Options Tests**
   - Create tests for mount options specific to Scality
   - Test read/write permissions
   - Test cache options

3. **Implement Multi-Volume Tests**
   - Test multiple volumes from the same bucket
   - Test multiple volumes from different buckets
   - Test concurrent access

4. **Implement Credentials Tests**
   - Test various authentication methods
   - Test credential rotation
   - Test access control

### Phase 3 Documentation and Verification
After implementing Phase 3:

1. **Documentation**
   ```bash
   # Update custom tests documentation
   cd tests/e2e-scality
   # Document custom test suites
   # Document specific Scality features tested
   ```

2. **Verification**
   ```bash
   # Run custom test suites
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --focus="Scality"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 3: Custom Test Suites for Scality
   
   - Implemented custom test suites for Scality features
   - Added mount options tests
   - Added multi-volume tests
   - Added credentials tests
   - All custom tests passing"
   ```

## Phase 4: Performance and Scalability Tests

1. **Create Performance Test Suite**
   ```go
   // In testsuites/performance.go
   func InitScalityPerformanceTestSuite() framework.TestSuite {
       return &scalityPerformanceTestSuite{}
   }
   
   type scalityPerformanceTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   ```

2. **Implement FIO Tests**
   - Create tests using FIO for throughput measurement
   - Test read performance
   - Test write performance
   - Test mixed workloads

3. **Implement Scalability Tests**
   - Test with large numbers of volumes
   - Test with large file sizes
   - Test with many concurrent operations

### Phase 4 Documentation and Verification
After implementing Phase 4:

1. **Documentation**
   ```bash
   # Update performance test documentation
   cd tests/e2e-scality
   # Document performance tests
   # Document how to interpret results
   ```

2. **Verification**
   ```bash
   # Run performance tests
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --focus="Performance"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 4: Performance and Scalability Tests
   
   - Implemented performance test suite
   - Added FIO tests for read/write performance
   - Added scalability tests
   - Documented performance results"
   ```

## Phase 5: CI Integration

1. **Update GitHub Actions Workflow**
   - Configure workflow to run E2E tests in CI
   - Set up matrix testing with different Kubernetes versions
   - Add JUnit report generation
   - Configure test results publishing

2. **Create Integration Tests for CI**
   - Create smoke tests for quick validation
   - Configure test focus for CI environment
   - Set up proper cleanup for CI

3. **Create Documentation for CI**
   - Document how to run tests in CI
   - Document how to interpret test results
   - Create troubleshooting guide for CI

### Phase 5 Documentation and Verification
After implementing Phase 5:

1. **Documentation**
   ```bash
   # Update CI documentation
   cd tests/e2e-scality
   # Document CI workflow
   # Document CI troubleshooting
   ```

2. **Verification**
   ```bash
   # Test CI workflow locally
   act -j e2e-tests
   
   # Verify full test suite with CI parameters
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     ADDITIONAL_ARGS="--focus=\"Smoke\" --junit-report=./test-results.xml"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 5: CI Integration
   
   - Updated GitHub Actions workflow
   - Created smoke tests for CI
   - Added JUnit reporting
   - Documented CI process"
   ```

## Parameter Handling Flow

1. **Makefile Parameters:**
   ```bash
   # Basic test with namespace and bucket isolation via commit ID
   make e2e-scality-go S3_ENDPOINT_URL=https://s3.example.com ACCESS_KEY_ID=key SECRET_ACCESS_KEY=secret COMMIT_ID=abc123 BUCKET_REGION=us-east-1 BUCKET_PREFIX=test CSI_NAMESPACE=mount-s3
   
   # Advanced test with categories and debug options
   make e2e-scality-go S3_ENDPOINT_URL=https://s3.example.com ACCESS_KEY_ID=key SECRET_ACCESS_KEY=secret COMMIT_ID=abc123 BUCKET_REGION=us-east-1 BUCKET_PREFIX=test CSI_NAMESPACE=mount-s3 ADDITIONAL_ARGS="--skip-cleanup --debug-level debug --categories smoke,functional --parallel 2"
   ```

2. **Makefile to run.sh:**
   ```bash
   INSTALL_ARGS="--endpoint-url ${S3_ENDPOINT_URL} --access-key-id ${ACCESS_KEY_ID} --secret-access-key ${SECRET_ACCESS_KEY} --commit-id ${COMMIT_ID} --bucket-region ${BUCKET_REGION} --bucket-prefix ${BUCKET_PREFIX} --namespace ${CSI_NAMESPACE}"
   
   # Add any additional args
   if [ ! -z "$(ADDITIONAL_ARGS)" ]; then
     INSTALL_ARGS="$$INSTALL_ARGS $(ADDITIONAL_ARGS)"
   fi
   
   ./tests/e2e-scality/scripts/run.sh go-test ${INSTALL_ARGS}
   ```

3. **run.sh to Go Test:**
   ```bash
   # Set environment variables for Go tests
   export S3_ENDPOINT_URL="${ENDPOINT_URL}"
   export ACCESS_KEY_ID="${ACCESS_KEY_ID}"
   export SECRET_ACCESS_KEY="${SECRET_ACCESS_KEY}"
   export NAMESPACE="${NAMESPACE}" 
   export COMMIT_ID="${COMMIT_ID}"
   export BUCKET_REGION="${BUCKET_REGION}"
   export BUCKET_PREFIX="${BUCKET_PREFIX}"
   export SKIP_CLEANUP="${SKIP_CLEANUP:-false}"
   export DEBUG_LEVEL="${DEBUG_LEVEL:-normal}"
   export TEST_CATEGORIES="${TEST_CATEGORIES:-all}"
   export PARALLEL="${PARALLEL:-1}"
   
   # Run Go tests with proper parameters
   cd ${TEST_DIR} && go test -v -tags=e2e \
       -endpoint-url="${ENDPOINT_URL}" \
       -access-key-id="${ACCESS_KEY_ID}" \
       -secret-access-key="${SECRET_ACCESS_KEY}" \
       -namespace="${NAMESPACE}" \
       -commit-id="${COMMIT_ID}" \
       -bucket-region="${BUCKET_REGION}" \
       -bucket-prefix="${BUCKET_PREFIX}" \
       -skip-cleanup="${SKIP_CLEANUP}" \
       -debug-level="${DEBUG_LEVEL}" \
       -categories="${TEST_CATEGORIES}" \
       -parallel="${PARALLEL}" \
       -ginkgo.focus="${FOCUS}" \
       -ginkgo.skip="${SKIP}"
   ```

## Directory Structure
```
tests/e2e-scality/e2e-tests/
├── go.mod
├── go.sum
├── main.go                     # Main test setup with flag/env handling
├── scality/
│   ├── s3client.go             # Scality S3 client
│   ├── resource.go             # Resource management
│   └── logging.go              # Debug logging system
├── testdriver.go               # Test driver implementation
├── testsuites/
│   ├── util.go                 # Common utilities
│   ├── volumes.go              # Volume tests
│   ├── errors.go               # Error handling
│   └── parallelism.go          # Parallel test helpers
└── README.md                   # Documentation
```

## Next Steps

Once the first test is implemented and working:
1. Verify the test pipeline works end-to-end with parameters from Makefile/run.sh
2. Test with --skip-cleanup to verify resources remain for debugging
3. Test with different debug levels to verify logging works as expected
4. Test with categories to verify filtering works properly
5. Test with parallelism to verify concurrent tests work correctly
6. Add more complex test scenarios (multivolume tests, permission tests, etc.)
7. Expand test coverage by adding new test suites
8. Improve test utilities for more robust testing
9. Add additional configuration parameters as needed
10. Run comprehensive CI testing
11. Update documentation with examples and troubleshooting guide

This comprehensive approach ensures a robust, maintainable test framework that integrates well with the existing build system, provides strong debugging capabilities, and offers flexible test execution options. 