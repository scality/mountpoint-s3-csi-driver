# Comprehensive Plan for Implementing Scality S3 CSI Driver E2E Tests

## Background
This document outlines a comprehensive plan for implementing end-to-end (E2E) tests for the Scality S3 CSI Driver. These tests will verify that the driver functions correctly in a Kubernetes environment, interacting properly with Scality S3 storage.

## Approach
- Start completely from scratch (no copying existing code)
- **IMPORTANT: All existing files in the e2e-tests directory under e2e-scality will be completely removed**
- The existing implementation was only for demo/CI testing purposes and is no longer needed
- Reference the e2e-kubernetes implementation (which uses AWS) for ideas and patterns only
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
3. Use the e2e-kubernetes folder as reference only (not copying code)

## Documentation and Verification Process
After each phase or major step:

1. **Documentation Updates**
   - Update README.md with new functionality
   - Add code comments for new functions and types
   - Document any configuration changes
   - Update troubleshooting guide if needed
   - Add examples of new commands or features

2. **Verification Steps**
   - Run unit tests for new code
   - Execute integration tests if applicable
   - Verify with different configuration combinations
   - Test error handling and edge cases
   - Document test results

3. **Commit Process**
   - Create detailed commit message
   - Include test results in commit description
   - Reference related issues or documentation
   - Push changes to feature branch
   - Create pull request if ready for review

## Phase 1: Configuration and Integration

### Step 1: Define Configuration Parameters
Create a configuration system that handles:
- S3 endpoint URL (default: "http://localhost:8000")
- Access key and secret key
- Namespace configuration with unique suffix from commit ID
- Commit ID (for bucket naming purposes)
- Bucket region (default: "us-east-1") 
- Bucket prefix with commit ID for test bucket identification
- Skip cleanup flag (for debugging resources)
- Debug logging levels (normal, debug, trace)

```go
// Config parameters with defaults
var (
    s3EndpointURL    string = "http://localhost:8000"
    accessKeyID      string
    secretAccessKey  string
    namespace        string = "mount-s3"
    namespaceSuffix  string
    commitID         string = "local"  // Default for local development
    bucketRegion     string = "us-east-1"
    bucketPrefix     string = "scality-e2e-test"
    skipCleanup      bool   = false    // Default to performing cleanup
    debugLevel       string = "normal" // normal, debug, trace
)

// Initialize namespace suffix based on commitID for test isolation
func init() {
    // Get commitID from flags or env
    // Use shortened commitID as namespace suffix for test isolation
    if commitID != "local" {
        namespaceSuffix = commitID[0:8]
        namespace = namespace + "-" + namespaceSuffix
        bucketPrefix = bucketPrefix + "-" + namespaceSuffix
    }
}
```

### Step 2: Create S3Client Package
Create a simple S3 client for Scality that will:
- Connect to Scality S3 server using provided credentials
- Create and delete test buckets using proper bucket naming convention with commit ID
- Handle authentication
- Provide verification utilities
- Support conditional cleanup based on skipCleanup flag
- Implement detailed error reporting

### Step 3: Create Test Driver
Implement a Scality-specific test driver with proper integration for:
- Driver capabilities definition
- S3 volume (bucket) creation and management
- Proper bucket naming with prefix and commit ID
- Persistence capabilities
- Authentication handling
- Conditional cleanup based on skipCleanup flag
- Robust error handling with context

### Phase 1 Documentation and Verification
After implementing Phase 1:

1. **Documentation**
   ```bash
   # Update documentation
   cd tests/e2e-scality
   # Add configuration parameters to README
   # Document S3 client usage
   # Add test driver capabilities documentation
   ```

2. **Verification**
   ```bash
   # Verify configuration loading
   go test -v ./... -run TestConfigLoading
   
   # Test S3 client connection
   go test -v ./... -run TestS3ClientConnection \
     -endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test"
   
   # Verify test driver
   go test -v ./... -run TestDriverCapabilities
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 1: Configuration and Integration
   
   - Added configuration parameter handling
   - Implemented S3 client package
   - Created test driver with capabilities
   - Added documentation and examples
   - All tests passing"
   ```

## Phase 2: Framework Setup

### Step 4: Update Main Test Framework
Modify the existing main.go to use the Kubernetes e2e framework properly:
- Set up proper Ginkgo/Gomega framework integration
- Configure all flags matching run.sh parameters
- Read environment variables set by the scripts
- Configure Ginkgo parallelism options
- Handle default values properly
- Add namespace isolation based on commit ID
- Add skip-cleanup flag
- Implement debug logging levels
- Add test categorization flags

```go
// Example flag setup
func init() {
    flag.StringVar(&s3EndpointURL, "endpoint-url", getEnvOrDefault("S3_ENDPOINT_URL", "http://localhost:8000"), "S3 endpoint URL")
    flag.StringVar(&accessKeyID, "access-key-id", getEnvOrDefault("ACCESS_KEY_ID", ""), "S3 access key ID")
    flag.StringVar(&secretAccessKey, "secret-access-key", getEnvOrDefault("SECRET_ACCESS_KEY", ""), "S3 secret access key")
    flag.StringVar(&namespace, "namespace", getEnvOrDefault("NAMESPACE", "mount-s3"), "Kubernetes namespace for testing")
    flag.StringVar(&commitID, "commit-id", getEnvOrDefault("COMMIT_ID", "local"), "Commit ID for bucket and namespace naming")
    flag.StringVar(&bucketRegion, "bucket-region", getEnvOrDefault("BUCKET_REGION", "us-east-1"), "S3 bucket region")
    flag.StringVar(&bucketPrefix, "bucket-prefix", getEnvOrDefault("BUCKET_PREFIX", "scality-e2e-test"), "Prefix for test buckets")
    flag.BoolVar(&skipCleanup, "skip-cleanup", getBoolEnvOrDefault("SKIP_CLEANUP", false), "Skip resource cleanup for debugging")
    flag.StringVar(&debugLevel, "debug-level", getEnvOrDefault("DEBUG_LEVEL", "normal"), "Debug level: normal, debug, trace")
    flag.StringVar(&testCategories, "categories", getEnvOrDefault("TEST_CATEGORIES", "all"), "Test categories to run (comma-separated): smoke,functional,integration,stress")
    flag.IntVar(&parallel, "parallel", getIntEnvOrDefault("PARALLEL", 1), "Number of parallel test processes to run")
    
    // Handle test isolation by adding commit ID suffix to namespace and bucket prefix
    if commitID != "local" {
        namespaceSuffix = commitID[0:8]
        namespace = namespace + "-" + namespaceSuffix
        bucketPrefix = bucketPrefix + "-" + namespaceSuffix
    }
    
    // Ensure required parameters are set
    flag.Parse()
    validateRequiredParams()
}
```

### Step 5: Update Script Integration
Modify the run.sh script to pass all parameters to Go tests properly:
- Pass environment variables for all needed configuration
- Make sure all parameters from Makefile are propagated
- Use the proper format for JUnit reports 
- Validate credentials before running tests
- Add support for --skip-cleanup flag
- Add support for debugging levels
- Add support for test categorization
- Configure Ginkgo parallelism

```bash
# Add to parse_test_parameters in run.sh
case "$1" in
  --skip-cleanup)
    params="$params --skip-cleanup"
    shift
    ;;
  --debug-level)
    params="$params --debug-level $2"
    shift 2
    ;;
  --categories)
    params="$params --categories $2"
    shift 2
    ;;
  --parallel)
    params="$params --parallel $2"
    shift 2
    ;;
  # existing cases...
esac
```

### Phase 2 Documentation and Verification
After implementing Phase 2:

1. **Documentation**
   ```bash
   # Update framework documentation
   cd tests/e2e-scality
   # Document framework setup
   # Add script usage examples
   # Update parameter documentation
   ```

2. **Verification**
   ```bash
   # Test framework initialization
   go test -v ./... -run TestFrameworkInit
   
   # Verify parameter passing
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --debug-level="debug"
   
   # Test parallelism setup
   go test -v ./... -run TestParallelExecution -parallel=2
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 2: Framework Setup
   
   - Updated main test framework
   - Integrated run.sh script
   - Added parameter handling
   - Verified parallel execution
   - Documentation updated"
   ```

## Phase 3: Implement Resource Management and Logging

### Step 6: Create Resource Management System
Implement a robust resource management system:
- Track all resources created during tests
- Create a cleanup registry for easy tracking
- Implement conditional cleanup functionality
- Add resource tagging with commit ID
- Log cleanup operations for debugging
- Add proper warnings when skipCleanup is enabled
- Add resource state dumping for debugging failed tests

```go
// Example resource manager
type ResourceManager struct {
    skipCleanup bool
    debugLevel  string
    commitID    string
    resources   []Resource
}

type Resource interface {
    Name() string
    Type() string
    Tags() map[string]string
    Cleanup(ctx context.Context) error
    DumpState(w io.Writer) error
}

func NewResourceManager(skipCleanup bool, debugLevel string, commitID string) *ResourceManager {
    if skipCleanup {
        log.Printf("WARNING: Resource cleanup is disabled. Resources will not be automatically removed after tests.")
    }
    return &ResourceManager{
        skipCleanup: skipCleanup,
        debugLevel:  debugLevel,
        commitID:    commitID,
        resources:   []Resource{},
    }
}

func (r *ResourceManager) RegisterResource(resource Resource) {
    // Add standard tags like commitID to all resources
    tags := resource.Tags()
    tags["commitID"] = r.commitID
    tags["testID"] = uuid.New().String()
    
    r.resources = append(r.resources, resource)
    if r.debugLevel == "debug" || r.debugLevel == "trace" {
        log.Printf("Registered resource: %s (type: %s)", resource.Name(), resource.Type())
    }
}

func (r *ResourceManager) Cleanup(ctx context.Context) error {
    if r.skipCleanup {
        log.Printf("Skipping cleanup of %d resources as requested", len(r.resources))
        return nil
    }
    
    // Perform cleanup in reverse order (LIFO)
    var errs []error
    for i := len(r.resources) - 1; i >= 0; i-- {
        resource := r.resources[i]
        if r.debugLevel != "normal" {
            log.Printf("Cleaning up resource: %s (type: %s)", resource.Name(), resource.Type())
        }
        
        if err := resource.Cleanup(ctx); err != nil {
            log.Printf("Error cleaning up resource %s: %v", resource.Name(), err)
            errs = append(errs, fmt.Errorf("failed to clean up %s (type: %s): %w", 
                          resource.Name(), resource.Type(), err))
        }
    }
    
    if len(errs) > 0 {
        return errors.NewAggregate(errs)
    }
    return nil
}

func (r *ResourceManager) DumpResourceState() {
    if len(r.resources) == 0 {
        log.Printf("No resources to dump state for")
        return
    }
    
    log.Printf("Dumping state for %d resources:", len(r.resources))
    for _, resource := range r.resources {
        var buf bytes.Buffer
        if err := resource.DumpState(&buf); err != nil {
            log.Printf("Error dumping state for %s: %v", resource.Name(), err)
            continue
        }
        log.Printf("Resource %s (type: %s) state:\n%s", resource.Name(), resource.Type(), buf.String())
    }
}
```

### Step 7: Create Logging System
Implement a structured logging system with debug levels:
- Normal - only test progress and results
- Debug - detailed test steps and resource operations
- Trace - everything including API calls and detailed data

```go
// Logger with debug levels
type Logger struct {
    debugLevel string
    testName   string
}

func NewLogger(debugLevel, testName string) *Logger {
    return &Logger{
        debugLevel: debugLevel,
        testName:   testName,
    }
}

func (l *Logger) Normal(format string, args ...interface{}) {
    log.Printf("[%s] %s", l.testName, fmt.Sprintf(format, args...))
}

func (l *Logger) Debug(format string, args ...interface{}) {
    if l.debugLevel == "debug" || l.debugLevel == "trace" {
        log.Printf("[DEBUG:%s] %s", l.testName, fmt.Sprintf(format, args...))
    }
}

func (l *Logger) Trace(format string, args ...interface{}) {
    if l.debugLevel == "trace" {
        log.Printf("[TRACE:%s] %s", l.testName, fmt.Sprintf(format, args...))
    }
}
```

### Step 8: Create Test Utilities
Implement helper functions for:
- Pod creation and management
- File operations (read/write/verify)
- Assertions
- Resource cleanup with skipCleanup awareness
- Error handling with context
- Test result reporting
- Resource state dumping for failure analysis

### Phase 3 Documentation and Verification
After implementing Phase 3:

1. **Documentation**
   ```bash
   # Update resource management docs
   cd tests/e2e-scality
   # Document resource types
   # Add logging level examples
   # Update troubleshooting guide
   ```

2. **Verification**
   ```bash
   # Test resource management
   go test -v ./... -run TestResourceManager
   
   # Verify cleanup functionality
   go test -v ./... -run TestCleanup --skip-cleanup=true
   
   # Test logging levels
   go test -v ./... -run TestLogging -debug-level=trace
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 3: Resource Management and Logging
   
   - Implemented resource tracking
   - Added cleanup functionality
   - Created logging system
   - Added debug levels
   - Updated documentation"
   ```

## Phase 4: Test Suite Implementation

### Step 9: Create Test Categories
Define different test categories for better organization:
- Smoke tests: Quick tests to verify basic functionality
- Functional tests: Comprehensive tests for specific features
- Integration tests: Tests for interoperability with other components
- Stress tests: Tests for performance under load

```go
// Test categories
const (
    CategorySmoke       = "smoke"
    CategoryFunctional  = "functional"
    CategoryIntegration = "integration"
    CategoryStress      = "stress"
)

// Test categories filter
func shouldRunCategory(testCategory string) bool {
    if testCategories == "all" {
        return true
    }
    
    categories := strings.Split(testCategories, ",")
    for _, cat := range categories {
        if cat == testCategory {
            return true
        }
    }
    return false
}
```

### Step 10: Create Volume Tests
Implement volume test suite with different categories:
- Basic volume tests (smoke)
- Multi-volume tests (functional)
- Mount options tests (functional)
- Concurrent access tests (integration)
- Error recovery tests (integration)
- Performance tests (stress)

### Step 11: Create Failure Handling System
Implement robust failure handling:
- Capture detailed error context
- Resource state dumping on failure
- Cleanup recovery for partially completed tests
- Detailed failure reporting

```go
// Example failure handler
func HandleTestFailure(ctx context.Context, rm *ResourceManager, err error, description string) {
    log.Printf("TEST FAILURE: %s - %v", description, err)
    
    // Dump the state of all resources for debugging
    rm.DumpResourceState()
    
    // Take additional debug actions based on error type
    var podErr *PodError
    if errors.As(err, &podErr) {
        log.Printf("Pod error details: pod=%s, namespace=%s", podErr.PodName, podErr.Namespace)
        DumpPodLogs(ctx, podErr.PodName, podErr.Namespace)
    }
    
    // You can add more error type handling here
}
```

### Phase 4 Documentation and Verification
After implementing Phase 4:

1. **Documentation**
   ```bash
   # Update test suite documentation
   cd tests/e2e-scality
   # Document test categories
   # Add example test cases
   # Update failure handling guide
   ```

2. **Verification**
   ```bash
   # Run smoke tests
   go test -v ./... -categories=smoke
   
   # Test failure handling
   go test -v ./... -run TestFailureHandling
   
   # Verify resource cleanup on failure
   go test -v ./... -run TestFailureCleanup
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 4: Test Suite Implementation
   
   - Added test categories
   - Implemented volume tests
   - Created failure handling
   - Added test examples
   - Updated documentation"
   ```

## Phase 5: Parallelism and Advanced Features

### Step 12: Implement Ginkgo Parallelism
Configure Ginkgo for parallel test execution:
- Set up proper test node isolation
- Configure test timeouts
- Implement parallel-safe resource naming
- Add locks for shared resources

```go
// In main.go test entry point
func TestE2E(t *testing.T) {
    RegisterFailHandler(Fail)
    
    // Configure parallel test execution
    suiteConfig, reporterConfig := GinkgoConfiguration()
    suiteConfig.ParallelTotal = parallel
    suiteConfig.ParallelProcess = 1 // This will be set by Ginkgo automatically when running in parallel
    suiteConfig.FailFast = false
    suiteConfig.EmitSpecProgress = true
    
    RunSpecs(t, "Scality S3 CSI Driver Suite", suiteConfig, reporterConfig)
}
```

### Step 13: Add Documentation
Create comprehensive documentation for the test framework:
- README with usage examples
- Test categories explanation
- Command examples for different scenarios
- Troubleshooting guide
- Resource cleanup procedures

### Phase 5 Documentation and Verification
After implementing Phase 5:

1. **Documentation**
   ```bash
   # Update parallelism documentation
   cd tests/e2e-scality
   # Document parallel execution
   # Add advanced feature guide
   # Update troubleshooting for parallel tests
   ```

2. **Verification**
   ```bash
   # Test parallel execution
   go test -v ./... -parallel=4
   
   # Verify resource isolation
   go test -v ./... -run TestParallelIsolation
   
   # Test with all features
   go test -v ./... -categories=all -parallel=4 -debug-level=debug
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 5: Parallelism and Advanced Features
   
   - Implemented parallel execution
   - Added resource isolation
   - Updated documentation
   - Verified parallel safety"
   ```

## Phase 6: Local Testing and CI Integration

### Step 14: Local Test Verification
Before committing any code:
- Run tests locally with different parameter combinations
- Test with and without the --skip-cleanup flag
- Test with different debug levels
- Test with different test categories
- Test with parallelism
- Verify proper connection to S3
- Ensure proper bucket creation and cleanup (when not skipped)
- Validate correct parameter handling
- Debug any issues before committing

Local testing examples:
```bash
# Basic smoke test with cleanup
make e2e-scality-go S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1 ADDITIONAL_ARGS="--categories smoke"

# Debug level with skip cleanup
make e2e-scality-go S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1 ADDITIONAL_ARGS="--skip-cleanup --debug-level debug"

# Parallel functional tests with commit ID for isolation
make e2e-scality-go S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1 COMMIT_ID=abc123 ADDITIONAL_ARGS="--categories functional --parallel 2"
```

### Step 15: Update CI Configuration
Update `.github/workflows/ci-and-e2e-tests.yaml` to:
- Include our new test suite
- Pass the right parameters for CI environment
- Use commit SHA for namespace and bucket naming
- Generate proper test reports for analysis
- Ensure proper setup of test environment
- Always perform cleanup in CI environment
- Configure appropriate parallelism for CI
- Run different test categories as appropriate

```yaml
- name: Run Scality Tests
  run: |
    mkdir -p test-results
    make e2e-scality-all \
      S3_ENDPOINT_URL=http://${{ steps.get_ip.outputs.host_ip }}:8000 \
      ACCESS_KEY_ID=accessKey1 \
      SECRET_ACCESS_KEY=verySecretKey1 \
      COMMIT_ID=${{ github.sha }} \
      BUCKET_REGION=us-east-1 \
      BUCKET_PREFIX=ci-test \
      CSI_IMAGE_TAG=${{ github.sha }} \
      CSI_IMAGE_REPOSITORY=ghcr.io/${{ github.repository }} \
      ADDITIONAL_ARGS="--categories smoke,functional --parallel 4 --junit-report=./test-results/scality-e2e-tests-results.xml"
```

### Phase 6 Documentation and Verification
After implementing Phase 6:

1. **Documentation**
   ```bash
   # Update CI documentation
   cd tests/e2e-scality
   # Document CI workflow
   # Add local testing guide
   # Update troubleshooting
   ```

2. **Verification**
   ```bash
   # Test CI workflow locally
   act -j e2e-tests
   
   # Verify full test suite
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     ADDITIONAL_ARGS="--categories all --parallel 4"
   
   # Test report generation
   make e2e-scality-all ADDITIONAL_ARGS="--junit-report=./test-results.xml"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 6: Local Testing and CI Integration
   
   - Added CI workflow
   - Implemented test reporting
   - Updated documentation
   - Verified full test suite"
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