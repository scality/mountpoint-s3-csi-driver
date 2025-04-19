# Scality S3 CSI Driver E2E Test Plan - SCALE-123

## Goals
- Implement comprehensive end-to-end tests for the Scality S3 CSI Driver
- Verify driver functionality in a Kubernetes environment
- Create a robust, maintainable test framework that follows Kubernetes E2E patterns
- Support both direct testing and full driver installation testing
- Implement custom tests for Scality-specific features

## Functional Requirements
1. Test Framework
   - Use Kubernetes E2E testing framework
   - Implement required interfaces for the test driver
   - Support standard test patterns
   - Use only simple access key and secret key authentication

2. Test Execution
   - Support both direct Go test commands and Make commands
   - Require kubectl path to be explicitly provided
   - Allow for selective test execution with Ginkgo focus
   - Provide cleanup options for debugging

3. Test Suites
   - Implement basic volume tests using Kubernetes standard suites
   - Create custom mount options tests for Scality
   - Add multi-volume tests for various bucket scenarios
   - Include performance and scalability tests

4. Verification
   - Verify Kubernetes resources during tests
   - Implement proper cleanup of test resources
   - Generate test reports for CI

## Task Dashboard

| Phase | Task | Description | Status | Depends On |
|-------|------|-------------|--------|------------|
| 1 | 1 | Framework Setup | 🟡 In Progress | |
| 1 | 1.1 | Remove existing files in e2e-scality directory | 🟡 In Progress | |
| 1 | 1.2 | Create proper directory structure | ⬜ To Do | 1.1 |
| 1 | 1.3 | Configure project dependencies | 🟡 In Progress | 1.2 |
| 1 | 1.4 | Implement test driver interfaces | 🟡 In Progress | 1.3 |
| 1 | 1.5 | Create S3 client package | ✅ Done | 1.3 |
| 1 | 1.6 | Set up test configuration | 🟡 In Progress | 1.3 |
| 1 | 1.7 | Modify run.sh to require kubectl path | ✅ Done | 1.2 |
| 1 | 1.8 | Document and verify framework setup | ⬜ To Do | 1.4, 1.5, 1.6, 1.7 |
| 2 | 2 | Basic Test Implementation | ⬜ To Do | 1 |
| 2 | 2.1 | Resolve Go dependencies | ⬜ To Do | 1.3 |
| 2 | 2.2 | Implement basic volume tests | ⬜ To Do | 2.1 |
| 2 | 2.3 | Add test verification methods | ✅ Done | 2.2 |
| 2 | 2.4 | Create simple Scality test suite | ⬜ To Do | 2.2 |
| 2 | 2.5 | Document and verify basic tests | ⬜ To Do | 2.3, 2.4 |
| 3 | 3 | Custom Test Suites | ⬜ To Do | 2 |
| 3 | 3.1 | Implement mount options test suite | ⬜ To Do | 2.4 |
| 3 | 3.2 | Implement multi-volume test suite | ⬜ To Do | 2.4 |
| 3 | 3.3 | Create Kubernetes resource verification helpers | ✅ Done | 3.1, 3.2 |
| 3 | 3.4 | Document and verify custom test suites | ⬜ To Do | 3.3 |
| 4 | 4 | Performance and Scalability Tests | ⬜ To Do | 3 |
| 4 | 4.1 | Create performance test suite | ⬜ To Do | 3.4 |
| 4 | 4.2 | Implement FIO tests | ⬜ To Do | 4.1 |
| 4 | 4.3 | Implement scalability tests | ⬜ To Do | 4.1 |
| 4 | 4.4 | Document and verify performance tests | ⬜ To Do | 4.2, 4.3 |
| 5 | 5 | CI Integration | ⬜ To Do | 4 |
| 5 | 5.1 | Update GitHub Actions workflow | ⬜ To Do | 4.4 |
| 5 | 5.2 | Create smoke tests for CI | ⬜ To Do | 5.1 |
| 5 | 5.3 | Document CI integration | ⬜ To Do | 5.2 |
| 5 | 5.4 | Final documentation and verification | ⬜ To Do | 5.3 |

---
## Plan Context (Jira: SCALE-123)

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
- **Ensure all tests can be run via direct Go commands, Make/run.sh commands, and CI workflows**
- **Use kubectl to verify Kubernetes resources during tests**
- **Require kubectl path to be explicitly provided in command-line arguments**
- **Focus exclusively on standard S3 functionality (no S3 Express Zone)**
- **Use only simple access key and secret key authentication**
- **Organize code according to the Kubernetes E2E framework patterns**

## Initial Cleanup
Before beginning implementation, we will:
1. Remove all existing files in the `tests/e2e-tests/e2e-scality/` directory
2. Keep only the test plan and create new directories as needed
3. Use the e2e-kubernetes folder as reference (not copying code)

## Directory Structure
Create a proper directory structure to organize the tests according to Kubernetes E2E framework patterns:

```
tests/e2e-tests/
├── kubernetes/        # Standard Kubernetes E2E tests
├── scality/           # Scality-specific tests
├── pkg/               # Common packages
│   ├── s3client/      # S3 client implementation
│   └── testutil/      # Test utilities
├── testsuites/        # Test suite implementations
├── e2e_test.go        # Main test file
├── testdriver.go      # Driver implementation
├── go.mod             # Go module file
└── scripts/           # Already exists
    ├── run.sh         # Script to run tests
    └── modules/       # Script modules
```

## Prerequisites
1. **Kubernetes Cluster Access**
   - **CI Environment**: kind cluster installed via GitHub Actions
   - **Local Environment**: Minikube cluster installed locally
   - Valid kubeconfig file (always required as input parameter)
   
2. **Required Tools**
   - **CI Environment**: kubectl installed via GitHub Actions
   - **Local Environment**: kubectl installed locally
   - **kubectl path must be provided explicitly as an input parameter**
   - Helm is needed for chart installation but will be used directly by scripts
   
3. **S3 Storage Access**
   - S3 endpoint URL (for Scality S3 server)
   - Access key ID
   - Secret access key
   
4. **Development Tools**
   - Go installed (version 1.18+)
   - GNU Make

## Command-line Arguments for Tests
The tests should accept the following command-line arguments:

```bash
# Required parameters
--kubectl-path=<path>           # Path to kubectl binary
--kubeconfig=<path>             # Path to kubeconfig file
--s3-endpoint-url=<url>         # S3 endpoint URL
--access-key-id=<id>            # S3 access key ID
--secret-access-key=<key>       # S3 secret access key

# Optional parameters
--commit-id=<id>                # Commit ID for bucket naming (default: local)
--bucket-prefix=<prefix>        # Prefix for temporary buckets (default: e2e-test-)
--skip-cleanup                  # Skip resource cleanup after tests
--ginkgo.focus=<expr>           # Focus on specific test cases
```

## Documentation and Verification Process
After each phase or major step:

1. **Documentation Updates**
   - Update README.md with new functionality and command examples
   - Add code comments for new functions and types
   - Document any configuration changes
   - Update troubleshooting guide if needed
   - Add examples of all testing commands (Go commands, Makefile commands)
   - **Ensure README includes commands for running tests with kubectl verification**
   - **Document kubectl commands needed to verify test results**
   - **Document that kubectl path must be explicitly provided**

2. **Verification Steps**
   - Execute end-to-end tests for the CSI driver
   - Focus only on testing driver functionality in a Kubernetes environment
   - Verify with different configuration combinations
   - Test error handling and edge cases
   - Document test results
   - **For each testable phase, verify tests can be run using two methods:**
     - Direct Go test commands for quick component testing
     - Make commands with `e2e-test-all` which installs CSI driver
   - **Use kubectl to verify Kubernetes resources created during tests**
   - **Verify tests work with kubectl path provided explicitly**

3. **Commit Process**
   - Create detailed commit message
   - Include test results in commit description
   - Reference related issues or documentation
   - Stage changes for commit
   - Commit changes locally

## Phase 1: Kubernetes E2E Framework Integration

1. **Configure Project Dependencies**
   - Add Kubernetes E2E framework dependencies to go.mod
   - Import required packages from k8s.io/kubernetes/test/e2e/framework
   - Import standard test suites from k8s.io/kubernetes/test/e2e/storage/testsuites
   - Set up proper module versioning
   - Create standard directory structure according to Kubernetes E2E patterns

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

3. **Create S3 Client Package for Standard S3 Operations**
   ```go
   // In pkg/s3client/client.go
   type Client struct {
       client     *s3.Client
       bucketName string
       prefix     string
       endpoint   string
       accessKey  string
       secretKey  string
   }
   
   // DeleteBucketFunc is returned from CreateBucket to be used for cleanup
   type DeleteBucketFunc func(context.Context) error
   
   // Create methods for standard S3 bucket operations
   func New(endpoint, accessKey, secretKey string) *Client
   func (c *Client) CreateBucket(ctx context.Context) (string, DeleteBucketFunc, error)
   func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error
   func (c *Client) WipeoutBucket(ctx context.Context, bucketName string) error
   ```

4. **Set Up Configuration for Simple Authentication**
   ```go
   // In e2e_test.go
   func init() {
       testing.Init()
       f.RegisterClusterFlags(flag.CommandLine) // configures --kubeconfig flag
       f.RegisterCommonFlags(flag.CommandLine)  // configures --kubectl flag (path to kubectl binary)
       f.AfterReadingAllFlags(&f.TestContext)
   
       // Add custom flags
       flag.StringVar(&CommitId, "commit-id", "local", "commit id will be used to name buckets")
       flag.StringVar(&BucketPrefix, "bucket-prefix", "e2e-test-", "prefix for temporary buckets")
       flag.StringVar(&S3EndpointURL, "s3-endpoint-url", "", "S3 endpoint URL for Scality S3 server")
       flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
       flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
       flag.BoolVar(&SkipCleanup, "skip-cleanup", false, "Skip resource cleanup after tests")
       flag.Parse()
   }
   ```

5. **Modify run.sh to Require kubectl Path**
   ```bash
   # In scripts/run.sh
   
   # Require the user to provide kubectl path
   if [ -z "$KUBECTL_PATH" ]; then
     echo "Error: KUBECTL_PATH environment variable or --kubectl-path parameter is not set"
     echo "Please provide the path to kubectl binary"
     exit 1
   fi
   
   # Use the provided kubectl path when calling go test
   GO_TEST_ARGS="--kubectl-path=$KUBECTL_PATH --kubeconfig=$KUBECONFIG"
   ```

### Phase 1 Documentation and Verification
After implementing Phase 1:

1. **Documentation**
   ```bash
   # Update documentation
   cd tests/e2e-tests
   # Document framework integration
   # Document test driver interfaces
   # Document S3 client usage
   # Document kubectl path requirement
   ```

2. **Verification**
   ```bash
   # Verify compilation
   cd tests/e2e-tests
   go build ./...
   
   # Check for any build errors
   go vet ./...
   
   # Verify basic setup with a simple test that does nothing
   go test -v ./... -run=TestFrameworkCompiles
   
   # Verify kubectl path is required
   ./scripts/run.sh check-kubectl
   # Should fail with error about missing kubectl path
   
   # Verify with explicit kubectl path
   ./scripts/run.sh check-kubectl --kubectl-path=/usr/local/bin/kubectl
   # Should succeed
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 1: Kubernetes E2E Framework Integration
   
   - Added Kubernetes E2E framework dependencies
   - Implemented test driver interfaces 
   - Created S3 client package for standard S3 operations
   - Set up configuration for simple access key/secret key authentication
   - Required explicit kubectl path to be provided
   - Established proper directory structure aligned with Kubernetes E2E patterns
   - Framework successfully compiles"
   ```

## Phase 2: Go Test Implementation with Existing Infrastructure

1. **Resolve Go Dependencies**
   ```go
   // Fix module dependencies to ensure proper versions
   // In go.mod
   require (
       // Match the versions used in e2e-kubernetes
       github.com/aws/aws-sdk-go-v2 v1.30.3
       github.com/onsi/ginkgo/v2 v2.19.0
       github.com/onsi/gomega v1.33.1
       k8s.io/api v0.29.8
       k8s.io/apimachinery v0.29.8
       k8s.io/kubernetes v1.29.14
       // Other required dependencies
   )
   ```

2. **Implement Basic Volume Tests**
   ```go
   // In e2e_test.go
   var StandardTestSuites = []func() framework.TestSuite{
       testsuites.InitVolumesTestSuite, // Basic volume tests, verified to work with S3
   }
   
   // This executes testSuites for csi volumes
   var _ = utils.SIGDescribe("Scality CSI Volumes", func() {
       curDriver := initScalityDriver()
       
       args := framework.GetDriverNameWithFeatureTags(curDriver)
       args = append(args, func() {
           framework.DefineTestSuites(curDriver, StandardTestSuites)
       })
       f.Context(args...)
   })
   ```

3. **Add Test Verification Methods**
   ```go
   // In pkg/testutil/verification.go
   // Implement helpers for verifying test results
   
   // VerifyPVCreated checks if a PV with the given name exists
   func VerifyPVCreated(ctx context.Context, c clientset.Interface, pvName string) error {
       pv, err := c.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
       if err != nil {
           return fmt.Errorf("failed to get PV %s: %v", pvName, err)
       }
       
       if pv.Status.Phase != v1.VolumeBound && pv.Status.Phase != v1.VolumeAvailable {
           return fmt.Errorf("PV %s is not in bound or available state: %s", pvName, pv.Status.Phase)
       }
       
       return nil
   }
   
   // Similar verification helpers for PVCs, Pods, etc.
   ```

4. **Create Simple Scality-Specific Test Suite**
   ```go
   // In testsuites/s3_basic.go
   func InitScalityBasicTestSuite() framework.TestSuite {
       return &scalityBasicTestSuite{}
   }
   
   type scalityBasicTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   
   // Implement framework.TestSuite interface methods
   func (t *scalityBasicTestSuite) GetTestSuiteInfo() framework.TestSuiteInfo
   func (t *scalityBasicTestSuite) SkipUnsupportedTests(driver framework.TestDriver, pattern framework.TestPattern)
   func (t *scalityBasicTestSuite) DefineTests(driver framework.TestDriver, pattern framework.TestPattern)
   ```

### Phase 2 Documentation and Verification
After implementing Phase 2:

1. **Documentation**
   ```bash
   # Update test documentation
   cd tests/e2e-tests
   # Document test integration with existing infrastructure
   # Document how to run tests with existing SCALE-T make commands
   # Document required kubectl path parameter
   ```

2. **Verification**
   ```bash
   # Test with existing infrastructure
   # Run using SCALE-T specific make commands that install the CSI driver
   
   # Then verify that the tests work with the installed driver
   go test -v ./... -ginkgo.focus="Basic" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 2: Test Implementation with Existing Infrastructure
   
   - Resolved Go dependencies
   - Implemented basic volume tests
   - Added verification helpers
   - Created Scality-specific test suite
   - Required explicit kubectl path
   - Integrated with existing SCALE-T infrastructure"
   ```

## Phase 3: Extended Test Suite Implementation

1. **Implement Scality-Specific Mount Options Test Suite**
   ```go
   // In testsuites/mount_options.go
   func InitScalityMountOptionsTestSuite() framework.TestSuite {
       return &scalityMountOptionsTestSuite{}
   }
   
   type scalityMountOptionsTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   
   // Implement framework.TestSuite interface methods
   func (t *scalityMountOptionsTestSuite) GetTestSuiteInfo() framework.TestSuiteInfo
   func (t *scalityMountOptionsTestSuite) SkipUnsupportedTests(driver framework.TestDriver, pattern framework.TestPattern)
   func (t *scalityMountOptionsTestSuite) DefineTests(driver framework.TestDriver, pattern framework.TestPattern)
   ```

## Phase 4: Custom Test Suites for Scality

1. **Create Custom Test Suites Following Kubernetes E2E Patterns**
   ```go
   // In testsuites/mountoptions.go
   func InitScalityMountOptionsTestSuite() framework.TestSuite {
       return &scalityMountOptionsTestSuite{}
   }
   
   type scalityMountOptionsTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   
   // Implement framework.TestSuite interface methods
   func (t *scalityMountOptionsTestSuite) GetTestSuiteInfo() framework.TestSuiteInfo
   func (t *scalityMountOptionsTestSuite) SkipUnsupportedTests(driver framework.TestDriver, pattern framework.TestPattern)
   func (t *scalityMountOptionsTestSuite) DefineTests(driver framework.TestDriver, pattern framework.TestPattern)
   
   // In e2e_test.go
   var CustomTestSuites = []func() framework.TestSuite{
       testsuites.InitScalityMountOptionsTestSuite,
       testsuites.InitScalityMultiVolumeTestSuite,
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

### Kubernetes Resource Verification Helpers

As part of the implementation, we will create helper functions to verify Kubernetes resources:

```go
// In pkg/testutil/resource_verification.go
package testutil

import (
    "context"
    "fmt"
    
    v1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    clientset "k8s.io/client-go/kubernetes"
)

// VerifyPVCreated checks if a PV with the given name exists
func VerifyPVCreated(ctx context.Context, c clientset.Interface, pvName string) error {
    pv, err := c.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get PV %s: %v", pvName, err)
    }
    
    if pv.Status.Phase != v1.VolumeBound && pv.Status.Phase != v1.VolumeAvailable {
        return fmt.Errorf("PV %s is not in bound or available state: %s", pvName, pv.Status.Phase)
    }
    
    return nil
}

// VerifyPVCBound checks if a PVC is bound to a PV
func VerifyPVCBound(ctx context.Context, c clientset.Interface, namespace, pvcName string) error {
    pvc, err := c.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get PVC %s: %v", pvcName, err)
    }
    
    if pvc.Status.Phase != v1.ClaimBound {
        return fmt.Errorf("PVC %s is not bound: %s", pvcName, pvc.Status.Phase)
    }
    
    return nil
}

// VerifyPodHasVolumeMounted checks if a Pod has the volume mounted
func VerifyPodHasVolumeMounted(ctx context.Context, c clientset.Interface, namespace, podName, volumeName string) error {
    pod, err := c.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
    if err != nil {
        return fmt.Errorf("failed to get Pod %s: %v", podName, err)
    }
    
    for _, volume := range pod.Spec.Volumes {
        if volume.Name == volumeName {
            return nil
        }
    }
    
    return fmt.Errorf("Pod %s does not have volume %s mounted", podName, volumeName)
}
```

### Phase 4 Documentation and Verification
After implementing Phase 4:

1. **Documentation**
   ```bash
   # Update custom tests documentation in README.md
   cd tests/e2e-tests
   # Document custom test suites
   # Document specific Scality features tested
   # Add examples of running custom tests
   ```

2. **Verification**
   ```bash
   # Run mount options tests
   go test -v ./... -ginkgo.focus="Mount options" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run multi-volume tests
   go test -v ./... -ginkgo.focus="Multi volume" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run all custom Scality tests
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --kubectl-path="/usr/local/bin/kubectl" \
     --kubeconfig="$HOME/.kube/config" \
     --ginkgo.focus="Scality"
   ```

3. **Ensuring Both Testing Methods Work**
   
   a. **Direct Go Commands**:
   ```bash
   # For mount options tests
   go test -v ./... -ginkgo.focus="Mount options" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands with CSI Driver Installation (Primary Method)**:
   ```bash
   # Primary method - installs CSI driver and runs tests
   # Required parameters for kubectl path and kubeconfig
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBECTL_PATH=/usr/local/bin/kubectl \
     KUBECONFIG="$HOME/.kube/config" \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 4: Custom Test Suites for Scality
   
   - Implemented custom test suites following Kubernetes E2E framework patterns
   - Added mount options tests specific to Scality
   - Added multi-volume tests for Scality S3 storage
   - Created proper Kubernetes resource verification utilities
   - All custom tests passing with make e2e-test-all"
   ```

## Phase 5: Performance and Scalability Tests

1. **Create Performance Test Suite Following Kubernetes E2E Patterns**
   ```go
   // In testsuites/performance.go
   func InitScalityPerformanceTestSuite() framework.TestSuite {
       return &scalityPerformanceTestSuite{}
   }
   
   type scalityPerformanceTestSuite struct {
       tsInfo framework.TestSuiteInfo
   }
   
   // Implement framework.TestSuite interface methods
   func (t *scalityPerformanceTestSuite) GetTestSuiteInfo() framework.TestSuiteInfo
   func (t *scalityPerformanceTestSuite) SkipUnsupportedTests(driver framework.TestDriver, pattern framework.TestPattern)
   func (t *scalityPerformanceTestSuite) DefineTests(driver framework.TestDriver, pattern framework.TestPattern)
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

### Phase 5 Documentation and Verification
After implementing Phase 5:

1. **Documentation**
   ```bash
   # Update performance test documentation in README.md
   cd tests/e2e-tests
   # Document performance tests
   # Document how to interpret results
   # Add examples of running performance tests
   ```

2. **Verification**
   ```bash
   # Run read performance tests
   go test -v ./... -ginkgo.focus="Performance read" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run write performance tests
   go test -v ./... -ginkgo.focus="Performance write" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run scalability tests
   go test -v ./... -ginkgo.focus="Scalability" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   ```

3. **Ensuring Both Testing Methods Work**
   
   a. **Direct Go Commands**:
   ```bash
   # For performance tests
   go test -v ./... -ginkgo.focus="Performance" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands with CSI Driver Installation (Primary Method)**:
   ```bash
   # Primary method - installs CSI driver and runs tests
   # Required parameters for kubectl path and kubeconfig
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBECTL_PATH=/usr/local/bin/kubectl \
     KUBECONFIG="$HOME/.kube/config" \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Performance|Scalability\""
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 5: Performance and Scalability Tests
   
   - Implemented performance test suite following Kubernetes E2E patterns
   - Added FIO tests for read/write performance
   - Added scalability tests
   - Documented performance results with make e2e-test-all"
   ```

## Phase 6: CI Integration

1. **Create GitHub Actions Workflow File**
   - Create `.github/workflows/e2e-tests.yaml`
   - Configure workflow to run E2E tests in CI
   - Set up matrix testing with different Kubernetes versions
   - Add JUnit report generation
   - Configure test results publishing

2. **Create Smoke Tests for CI**
   - Create smoke tests for quick validation
   - Configure test focus for CI environment
   - Set up proper cleanup for CI

3. **Create Documentation for CI**
   - Document how CI works in README.md
   - Document how to interpret test results
   - Create troubleshooting guide for CI

### Phase 6 Documentation and Verification
After implementing Phase 6:

1. **Documentation**
   ```bash
   # Update CI documentation in README.md
   cd tests/e2e-tests
   # Document CI workflow
   # Document CI troubleshooting
   ```

2. **Verification**
   ```bash
   # Verify full test suite with smoke tests
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Smoke\" --junit-report=./test-results.xml"
   
   # Verify with matrix test parameters
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBERNETES_VERSION=1.25.0 \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Smoke\""
   ```

3. **Final README.md Documentation**
   
   a. **Direct Go Commands for Testing Components**:
   ```bash
   # Run smoke tests directly with Go (for quick component testing)
   go test -v ./... -ginkgo.focus="Smoke" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubectl-path="/usr/local/bin/kubectl" \
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands for Full Testing (Primary Method)**:
   ```bash
   # Primary testing method - installs CSI driver and runs all tests
   # Required parameters for kubectl path and kubeconfig
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBECTL_PATH=/usr/local/bin/kubectl \
     KUBECONFIG="$HOME/.kube/config" \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
   
   # For running specific test groups
   make e2e-test-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Smoke\""
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 6: CI Integration
   
   - Created GitHub Actions workflow file
   - Added smoke tests for CI
   - Added JUnit reporting
   - Documented make e2e-test-all usage in README.md
   - Updated documentation for running tests with kubectl"
   ```

## Testing Methods Summary

For any testable phase, we will ensure two testing methods work correctly:

### 1. Direct Go Test Commands
- For developers who want to test specific components quickly
- Provides most control over test parameters
- Useful for debugging specific test failures
```bash
# Example: Run a specific test
go test -v ./... -run=TestSpecificTest \
  -s3-endpoint-url="http://localhost:8000" \
  -access-key-id="test" \
  -secret-access-key="test" \
  -kubectl-path="/usr/local/bin/kubectl" \
  -kubeconfig="$HOME/.kube/config"

# Example: Run tests matching a pattern
go test -v ./... -ginkgo.focus="Mount options" \
  -s3-endpoint-url="http://localhost:8000" \
  -access-key-id="test" \
  -secret-access-key="test" \
  -kubectl-path="/usr/local/bin/kubectl" \
  -kubeconfig="$HOME/.kube/config"
```

### 2. Make Commands with CSI Driver Installation (Primary Method)
- For running full tests with CSI driver installation
- Provides real-world testing in a Kubernetes environment
- Matches how tests will run in CI
```bash
# Primary method - installs CSI driver and runs tests
# Required parameters for kubectl path and kubeconfig
make e2e-test-all \
  S3_ENDPOINT_URL=http://localhost:8000 \
  ACCESS_KEY_ID=test \
  SECRET_ACCESS_KEY=test \
  KUBECTL_PATH=/usr/local/bin/kubectl \
  KUBECONFIG="$HOME/.kube/config" \
  ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
```

The README.md will document both approaches, with emphasis on the make e2e-test-all command as the primary testing method that most closely matches the CI environment. It will also include instructions for using kubectl to verify the test results and emphasize that kubectl path must be provided explicitly.