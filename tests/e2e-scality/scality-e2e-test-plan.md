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
- **Automatically detect kubectl path in run.sh**
- **Focus exclusively on standard S3 functionality (no S3 Express Zone)**
- **Use only simple access key and secret key authentication**
- **Organize code according to the Kubernetes E2E framework patterns**

## Initial Cleanup
Before beginning implementation, we will:
1. Remove all existing files in the `tests/e2e-scality/e2e-tests/` directory
2. Keep only the test plan and create new directories as needed
3. Use the e2e-kubernetes folder as reference (not copying code)

## Directory Structure
Create a proper directory structure to organize the tests according to Kubernetes E2E framework patterns:

```
tests/e2e-scality/
├── e2e-tests/
│   ├── kubernetes/        # Standard Kubernetes E2E tests
│   ├── scality/           # Scality-specific tests
│   ├── pkg/               # Common packages
│   │   ├── s3client/      # S3 client implementation
│   │   └── testutil/      # Test utilities
│   ├── testsuites/        # Test suite implementations
│   ├── e2e_test.go        # Main test file
│   ├── testdriver.go      # Driver implementation
│   └── go.mod             # Go module file
├── scripts/
│   ├── run.sh             # Script to run tests
│   └── modules/           # Script modules
└── scality-e2e-test-plan.md
```

## Prerequisites
1. **Kubernetes Cluster Access**
   - **CI Environment**: kind cluster installed via GitHub Actions
   - **Local Environment**: Minikube cluster installed locally
   - Valid kubeconfig file (always required as input parameter)
   
2. **Required Tools**
   - **CI Environment**: kubectl installed via GitHub Actions
   - **Local Environment**: kubectl installed locally
   - kubectl path will be auto-detected by run.sh (not required as input parameter)
   - Helm is needed for chart installation but will be used directly by scripts
   
3. **S3 Storage Access**
   - S3 endpoint URL (for Scality S3 server)
   - Access key ID
   - Secret access key
   
4. **Development Tools**
   - Go installed (version 1.18+)
   - GNU Make

## Kubectl Auto-detection in run.sh
The run.sh script will include logic to detect kubectl:

```bash
# Auto-detect kubectl path
detect_kubectl_path() {
  # Check if kubectl is in PATH
  if command -v kubectl &> /dev/null; then
    KUBECTL_PATH=$(command -v kubectl)
    echo "Using kubectl at: $KUBECTL_PATH"
    return 0
  fi
  
  # Check common locations for CI environments
  for path in "/usr/local/bin/kubectl" "/usr/bin/kubectl" "/bin/kubectl"; do
    if [ -f "$path" ]; then
      KUBECTL_PATH="$path"
      echo "Using kubectl at: $KUBECTL_PATH"
      return 0
    fi
  done
  
  echo "Error: kubectl not found. Please install kubectl."
  return 1
}

# Detect environment (CI vs local)
detect_environment() {
  # Check for GitHub Actions environment
  if [ -n "$GITHUB_ACTIONS" ]; then
    ENVIRONMENT="github-actions"
    echo "Detected GitHub Actions environment"
    return 0
  fi
  
  # Check for Minikube
  if command -v minikube &> /dev/null && minikube status &> /dev/null; then
    ENVIRONMENT="minikube"
    echo "Detected Minikube environment"
    return 0
  fi
  
  # Default to generic environment
  ENVIRONMENT="generic"
  echo "Using generic environment"
  return 0
}
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
   - **Document that kubectl path is auto-detected by run.sh**

2. **Verification Steps**
   - Execute end-to-end tests for the CSI driver
   - Focus only on testing driver functionality in a Kubernetes environment
   - Verify with different configuration combinations
   - Test error handling and edge cases
   - Document test results
   - **For each testable phase, verify tests can be run using two methods:**
     - Direct Go test commands for quick component testing
     - Make commands with `e2e-scality-all` which installs CSI driver
   - **Use kubectl to verify Kubernetes resources created during tests**
   - **Verify auto-detection of kubectl works in both CI and local environments**

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
   func (c *Client) CreateBucket(ctx context.Context) (string, DeleteBucketFunc)
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

5. **Modify run.sh to Auto-detect kubectl**
   ```bash
   # In scripts/run.sh
   
   # Auto-detect kubectl path
   detect_kubectl_path
   detect_environment
   
   # Only require the user to provide kubeconfig
   if [ -z "$KUBECONFIG" ]; then
     echo "Error: KUBECONFIG environment variable is not set"
     exit 1
   fi
   
   # Use the detected kubectl path when calling go test
   GO_TEST_ARGS="-kubectl-path=$KUBECTL_PATH -kubeconfig=$KUBECONFIG"
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
   # Document kubectl auto-detection in run.sh
   ```

2. **Verification**
   ```bash
   # Verify compilation
   cd tests/e2e-scality
   go build ./...
   
   # Check for any build errors
   go vet ./...
   
   # Verify basic setup with a simple test that does nothing
   go test -v ./... -run=TestFrameworkCompiles
   
   # Verify kubectl auto-detection works
   ./scripts/run.sh check-kubectl
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 1: Kubernetes E2E Framework Integration
   
   - Added Kubernetes E2E framework dependencies
   - Implemented test driver interfaces 
   - Created S3 client package for standard S3 operations
   - Set up configuration for simple access key/secret key authentication
   - Added auto-detection of kubectl in run.sh
   - Established proper directory structure aligned with Kubernetes E2E patterns
   - Framework successfully compiles"
   ```

## Phase 2: Standard Test Suite Integration

1. **Configure Standard Kubernetes Test Suites**
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

3. **Update Run Script Integration**
   - Enhance scripts/run.sh to use auto-detected kubectl path
   - Add support for Kubernetes E2E framework parameters
   - Configure proper parameter passing to Go tests

4. **Implement CSI Driver Installation Script with Simple Authentication**
   ```bash
   # In scripts/install_csi_driver.sh
   
   install_csi_driver() {
     # Use helm directly (not via Go tests)
     echo "Installing CSI driver using Helm"
     
     # Check if release already exists
     if helm list -n kube-system | grep -q "csi-driver-s3"; then
       echo "CSI driver already installed, upgrading..."
       helm upgrade csi-driver-s3 ./charts/csi-driver-s3 \
         --namespace kube-system \
         --set s3.endpoint=$S3_ENDPOINT_URL \
         --set s3.accessKeyID=$ACCESS_KEY_ID \
         --set s3.secretAccessKey=$SECRET_ACCESS_KEY
     else
       echo "Installing CSI driver..."
       helm install csi-driver-s3 ./charts/csi-driver-s3 \
         --namespace kube-system \
         --create-namespace \
         --set s3.endpoint=$S3_ENDPOINT_URL \
         --set s3.accessKeyID=$ACCESS_KEY_ID \
         --set s3.secretAccessKey=$SECRET_ACCESS_KEY
     fi
     
     # Wait for CSI driver to be ready using auto-detected kubectl
     echo "Waiting for CSI driver pods to be running..."
     $KUBECTL_PATH wait --for=condition=Ready pods -l app=csi-driver-s3 -n kube-system --timeout=120s
   }
   ```

### Phase 2 Documentation and Verification
After implementing Phase 2:

1. **Documentation**
   ```bash
   # Update framework documentation
   cd tests/e2e-scality
   # Document standard test suites
   # Document run script usage with examples
   # Document how test suite interacts with Kubernetes resources
   # Document kubectl auto-detection
   ```

2. **Verification**
   ```bash
   # Test with minimal setup - just verify S3 client works
   go test -v ./pkg/s3client -run=TestS3ClientBasic \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test"
   
   # Test driver initialization without running actual tests
   go test -v ./... -run=TestDriverInit \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Test run script auto-detection and parameter passing
   KUBECONFIG="$HOME/.kube/config" ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --run="TestDriverInit"
   
   # Verify Kubernetes connection using auto-detected kubectl
   ./scripts/run.sh check-kubernetes
   ```

3. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 2: Standard Test Suite Integration
   
   - Configured standard Kubernetes test suites
   - Implemented test volume management
   - Enhanced run.sh with kubectl auto-detection
   - Added CSI driver installation script with simple authentication
   - Verified S3 client and driver initialization with Kubernetes framework"
   ```

## Phase 3: Basic Volume Test Implementation

1. **Implement Simple Volume Test Using Kubernetes E2E Framework**
   ```go
   // In e2e_test.go - add a simple test that runs outside the framework
   func TestBasicVolumeCreation(t *testing.T) {
       // Initialize driver and create a test volume
       driver := initScalityDriver()
       ctx := context.Background()
       
       // Create a test config
       config := &framework.PerTestConfig{
           Framework: &framework.Framework{},
       }
       
       // Create a volume
       volume := driver.CreateVolume(ctx, config, framework.StandardVolumeType)
       defer volume.DeleteVolume(ctx)
       
       // Verify volume is accessible
       // ... add verification steps
   }
   ```

2. **Test Ginkgo Integration**
   - Add a minimal Ginkgo test to verify framework integration
   - Configure proper parameter passing to Ginkgo tests

3. **Implement Skip Patterns**
   - Identify which standard tests should be skipped for Scality
   - Implement proper skip logic in driver

4. **Implement Kubernetes Resource Verification**
   - Add helper functions to verify PVs and PVCs are created correctly
   - Add helper functions to check pod mounts
   - Use kubectl (via framework or directly) to verify resources

### Phase 3 Documentation and Verification
After implementing Phase 3:

1. **Documentation**
   ```bash
   # Update test documentation in README.md
   cd tests/e2e-scality
   # Document basic volume tests
   # Document test patterns and skip logic
   # Add examples of running tests with make commands
   # Document kubectl commands for verifying test results
   # Document that only kubeconfig needs to be specified
   ```

2. **Verification**
   ```bash
   # Run the basic volume test with direct Go command
   go test -v ./... -run=TestBasicVolumeCreation \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run a minimal Ginkgo test with direct Go command
   go test -v ./... -ginkgo.focus="Basic volume test" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Use run.sh script with auto-detection
   KUBECONFIG="$HOME/.kube/config" ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
     --ginkgo.focus="Basic volume test"
     
   # Verify Kubernetes resources created by tests using auto-detected kubectl
   ./scripts/run.sh kubectl get pv
   ./scripts/run.sh kubectl get pvc
   ./scripts/run.sh kubectl get pods
   ```

3. **Ensuring Both Testing Methods Work**
   
   a. **Direct Go Commands**:
   ```bash
   # Direct go test command for basic volume test
   go test -v ./... -run=TestBasicVolumeCreation \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Direct go test command for Ginkgo test
   go test -v ./... -ginkgo.focus="Basic volume test" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands with CSI Driver Installation (Primary Method)**:
   ```bash
   # Primary method - installs CSI driver and runs tests
   # Only need to specify kubeconfig, kubectl path is auto-detected
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBECONFIG="$HOME/.kube/config" \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
   ```

4. **Commit Changes**
   ```bash
   git add .
   git commit -m "Phase 3: Basic Volume Test Implementation
   
   - Implemented basic volume creation test with Kubernetes E2E framework
   - Added Ginkgo integration
   - Implemented test skip patterns
   - Verified volume creation and deletion
   - Added kubectl verification of Kubernetes resources
   - Verified auto-detection of kubectl works"
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
   cd tests/e2e-scality
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
     -kubeconfig="$HOME/.kube/config"
   
   # Run multi-volume tests
   go test -v ./... -ginkgo.focus="Multi volume" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run all custom Scality tests
   ./scripts/run.sh go-test \
     --endpoint-url="http://localhost:8000" \
     --access-key-id="test" \
     --secret-access-key="test" \
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
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands with CSI Driver Installation (Primary Method)**:
   ```bash
   # Primary method - installs CSI driver and runs tests
   # Only need to specify kubeconfig, kubectl path is auto-detected
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
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
   - All custom tests passing with make e2e-scality-all"
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
   cd tests/e2e-scality
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
     -kubeconfig="$HOME/.kube/config"
   
   # Run write performance tests
   go test -v ./... -ginkgo.focus="Performance write" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
     -kubeconfig="$HOME/.kube/config"
   
   # Run scalability tests
   go test -v ./... -ginkgo.focus="Scalability" \
     -s3-endpoint-url="http://localhost:8000" \
     -access-key-id="test" \
     -secret-access-key="test" \
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
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands with CSI Driver Installation (Primary Method)**:
   ```bash
   # Primary method - installs CSI driver and runs tests
   # Only need to specify kubeconfig, kubectl path is auto-detected
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
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
   - Documented performance results with make e2e-scality-all"
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
   cd tests/e2e-scality
   # Document CI workflow
   # Document CI troubleshooting
   ```

2. **Verification**
   ```bash
   # Verify full test suite with smoke tests
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Smoke\" --junit-report=./test-results.xml"
   
   # Verify with matrix test parameters
   make e2e-scality-all \
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
     -kubeconfig="$HOME/.kube/config"
   ```
   
   b. **Make Commands for Full Testing (Primary Method)**:
   ```bash
   # Primary testing method - installs CSI driver and runs all tests
   # Only need to specify kubeconfig, kubectl path is auto-detected
   make e2e-scality-all \
     S3_ENDPOINT_URL=http://localhost:8000 \
     ACCESS_KEY_ID=test \
     SECRET_ACCESS_KEY=test \
     KUBECONFIG="$HOME/.kube/config" \
     ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
   
   # For running specific test groups
   make e2e-scality-all \
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
   - Documented make e2e-scality-all usage in README.md
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
  -kubeconfig="$HOME/.kube/config"

# Example: Run tests matching a pattern
go test -v ./... -ginkgo.focus="Mount options" \
  -s3-endpoint-url="http://localhost:8000" \
  -access-key-id="test" \
  -secret-access-key="test" \
  -kubeconfig="$HOME/.kube/config"
```

### 2. Make Commands with CSI Driver Installation (Primary Method)
- For running full tests with CSI driver installation
- Provides real-world testing in a Kubernetes environment
- Matches how tests will run in CI
```bash
# Primary method - installs CSI driver and runs tests
# Only need to specify kubeconfig, kubectl path is auto-detected
make e2e-scality-all \
  S3_ENDPOINT_URL=http://localhost:8000 \
  ACCESS_KEY_ID=test \
  SECRET_ACCESS_KEY=test \
  KUBECONFIG="$HOME/.kube/config" \
  ADDITIONAL_ARGS="--ginkgo.focus=\"Basic\""
```

The README.md will document both approaches, with emphasis on the make e2e-scality-all command as the primary testing method that most closely matches the CI environment. It will also include instructions for using kubectl to verify the test results. 