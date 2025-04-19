# E2E Testing Framework for Scality Mountpoint S3 CSI Driver

## Goals
- Implement end-to-end tests for the Scality Mountpoint S3 CSI Driver
- Utilize the Kubernetes e2e framework for testing
- Focus on core functionality validation
- Ensure tests work with Helm-deployed clusters
- Support both local development and CI environments

## Functional Requirements
- Tests must validate core CSI driver functionality
- Tests should run against a Helm-deployed Kubernetes cluster
- Must support Scality S3 endpoint configuration
- Should reuse existing script structure in tests/e2e-tests
- Test results should be easily interpretable and actionable

## Task Dashboard

| Phase | Task | Description | Status | Depends On |
|-------|------|-------------|--------|------------|
| 1 | 1 | Initialize Go Module Setup | ⬜ To Do | |
| 1 | 1.1 | Create Go module in tests/e2e-tests | ⬜ To Do | 1 |
| 1 | 1.2 | Add required dependencies | ⬜ To Do | 1.1 |
| 1 | 1.3 | Set up test framework structure | ⬜ To Do | 1.2 |
| 2 | 2 | Implement S3 Client | ⬜ To Do | 1 |
| 2 | 2.1 | Create s3client directory and base file | ⬜ To Do | 1.3 |
| 2 | 2.2 | Implement client for Scality S3 endpoint | ⬜ To Do | 2.1 |
| 2 | 2.3 | Implement bucket management functions | ⬜ To Do | 2.2 |
| 3 | 3 | Implement Test Driver | ⬜ To Do | 2 |
| 3 | 3.1 | Create testdriver.go with CSI driver interface | ⬜ To Do | 2.3 |
| 3 | 3.2 | Implement volume creation/deletion | ⬜ To Do | 3.1 |
| 3 | 3.3 | Configure driver capabilities | ⬜ To Do | 3.1 |
| 4 | 4 | Create Main Test File | ⬜ To Do | 3 |
| 4 | 4.1 | Implement e2e_test.go with framework setup | ⬜ To Do | 3.3 |
| 4 | 4.2 | Add S3 configuration flags | ⬜ To Do | 4.1 |
| 4 | 4.3 | Set up test suite execution | ⬜ To Do | 4.2 |
| 5 | 5 | Add Core Test Suites | ⬜ To Do | 4 |
| 5 | 5.1 | Create basic volume test | ⬜ To Do | 4.3 |
| 5 | 5.2 | Implement multi-volume test | ⬜ To Do | 5.1 |
| 5 | 5.3 | Add mount options test | ⬜ To Do | 5.1 |
| 6 | 6 | Integrate with Test Scripts | ⬜ To Do | 5 |
| 6 | 6.1 | Update test.sh to run Go tests | ⬜ To Do | 5.3 |
| 6 | 6.2 | Add parameter passing to tests | ⬜ To Do | 6.1 |
| 6 | 6.3 | Update script documentation | ⬜ To Do | 6.2 |
| 7 | 7 | Update Makefile | ⬜ To Do | 6 |
| 7 | 7.1 | Add e2e-scality-go target | ⬜ To Do | 6.3 |
| 7 | 7.2 | Update e2e-scality-all target | ⬜ To Do | 7.1 |
| 7 | 7.3 | Document Makefile changes | ⬜ To Do | 7.2 |
| 8 | 8 | Testing and Validation | ⬜ To Do | 7 |
| 8 | 8.1 | Test with local Helm cluster | ⬜ To Do | 7.3 |
| 8 | 8.2 | Verify all test cases pass | ⬜ To Do | 8.1 |
| 8 | 8.3 | Document test execution process | ⬜ To Do | 8.2 |

---
## Plan Context (Jira: S3CSI-E2E)

The goal is to create end-to-end tests for the Scality Mountpoint S3 CSI Driver using the Kubernetes e2e framework, similar to the existing tests in the e2e-kubernetes folder but built from scratch. We will implement these tests in the tests/e2e-tests directory.

1. **Go Module Setup**
   - We'll set up a Go module in tests/e2e-tests with the path github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests
   - Add necessary dependencies for the Kubernetes e2e framework
   - Configure the module to work with Ginkgo and Gomega testing frameworks

2. **S3 Client Implementation**
   - Create an S3 client that works with Scality S3 endpoints
   - Implement bucket creation and deletion functionality
   - Handle authentication with the provided credentials
   - Set up proper error handling and logging

3. **Test Driver Implementation**
   - Create a driver that implements the Kubernetes CSI testing interfaces
   - Configure the driver to work with the S3 client
   - Implement volume provisioning mechanisms
   - Set up driver capabilities and supported features

4. **Main Test File**
   - Set up the main test entry point with e2e_test.go
   - Configure command line flags for S3 endpoint, credentials, and other options
   - Initialize the testing framework and register test suites

5. **Core Test Suites**
   - Implement basic volume mounting tests
   - Create multi-volume tests to verify concurrent volume operations
   - Add tests for mount options and configurations

6. **Script Integration**
   - Update the test.sh script to detect and run Go tests
   - Add parameter passing from script to tests
   - Ensure tests can be run individually or as part of the full suite

7. **Makefile Updates**
   - Add targets for running Go-based e2e tests
   - Update the all-in-one target to include Go tests
   - Document usage in Makefile comments

8. **Testing and Validation**
   - Test with local Helm cluster
   - Verify all test cases pass
   - Document the test execution process

We will focus on using the standard Kubernetes e2e framework rather than creating custom test suites. The tests will validate core CSI driver functionality including volume provisioning, mounting, and basic operations. The implementation will be done in a way that allows for both local development testing and CI integration.

---
## Implementation Details

Below are the detailed commands and code snippets to implement each task:

### Phase 1: Initialize Go Module Setup

#### Task 1.1: Create Go module in tests/e2e-tests

```bash
# Create directory structure if it doesn't exist
mkdir -p tests/e2e-tests/testsuites
mkdir -p tests/e2e-tests/s3client

# Initialize Go module
cd tests/e2e-tests
go mod init github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests
```

#### Task 1.2: Add required dependencies

```bash
# Add required dependencies
cd tests/e2e-tests
go get github.com/onsi/ginkgo/v2@v2.9.5
go get github.com/onsi/gomega@v1.27.7
go get k8s.io/kubernetes@v1.26.5
go get k8s.io/api@v0.26.5
go get k8s.io/apimachinery@v0.26.5
go get github.com/aws/aws-sdk-go-v2@v1.18.0
go get github.com/aws/aws-sdk-go-v2/config@v1.18.25
go get github.com/aws/aws-sdk-go-v2/credentials@v1.13.24
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.33.1
go get k8s.io/kubernetes/test/e2e/storage/framework@v1.26.5

# Tidy the module
go mod tidy
```

#### Task 1.3: Set up test framework structure

```bash
# Create necessary directories if they don't exist yet
mkdir -p tests/e2e-tests/testsuites
mkdir -p tests/e2e-tests/s3client
touch tests/e2e-tests/.gitignore
```

Create a basic .gitignore file:
```
# Add to tests/e2e-tests/.gitignore
*.test
/csi-*
/junit*
```

### Phase 2: Implement S3 Client

#### Task 2.1: Create s3client directory and base file

Create the file `tests/e2e-tests/s3client/client.go`:

```go
// Package s3client provides a client for interacting with Scality S3 endpoints
package s3client

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/kubernetes/test/e2e/framework"
)

// DeleteBucketFunc is a function to delete a bucket
type DeleteBucketFunc func(context.Context) error

// Client represents an S3 client
type Client struct {
	endpointURL    string
	accessKeyID    string
	secretAccessKey string
	client         *s3.Client
}

const s3BucketNameMaxLength = 63
const s3BucketNamePrefix = "s3-csi-e2e-"

// New creates a new S3 client
func New(endpointURL, accessKeyID, secretAccessKey string) *Client {
	// Create a custom endpoint resolver for our S3 endpoint
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:               endpointURL,
			SigningRegion:     "us-east-1", // Default region, doesn't matter for Scality
			HostnameImmutable: true,
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithRegion("us-east-1"), // Default region, doesn't matter for Scality
	)
	framework.ExpectNoError(err, "Failed to create S3 client")

	return &Client{
		endpointURL:    endpointURL,
		accessKeyID:    accessKeyID,
		secretAccessKey: secretAccessKey,
		client:         s3.NewFromConfig(cfg),
	}
}
```

#### Task 2.2: Implement client for Scality S3 endpoint

Add this to `s3client/client.go`:

```go
// CreateBucket creates a new bucket with a random name
func (c *Client) CreateBucket(ctx context.Context) (string, DeleteBucketFunc) {
	bucketName := c.randomBucketName()

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	_, err := c.client.CreateBucket(ctx, input)
	framework.ExpectNoError(err, "Failed to create S3 bucket")
	framework.Logf("S3 Bucket %s created", bucketName)

	return bucketName, func(ctx context.Context) error {
		return c.DeleteBucket(ctx, bucketName)
	}
}

// randomBucketName generates a random bucket name
func (c *Client) randomBucketName() string {
	prefixLen := len(s3BucketNamePrefix)
	rand := utilrand.String(s3BucketNameMaxLength - prefixLen)
	return s3BucketNamePrefix + rand
}
```

#### Task 2.3: Implement bucket management functions

Add this to `s3client/client.go`:

```go
// DeleteBucket deletes a bucket and all objects in it
func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error {
	framework.Logf("Deleting S3 Bucket %s...", bucketName)

	// First, delete all objects in the bucket
	err := c.WipeoutBucket(ctx, bucketName)
	if err != nil {
		return err
	}

	// Then, delete the bucket
	_, err = c.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %v", bucketName, err)
	}

	framework.Logf("S3 Bucket %s deleted", bucketName)
	return nil
}

// WipeoutBucket removes all objects from a bucket
func (c *Client) WipeoutBucket(ctx context.Context, bucketName string) error {
	// List all objects
	objects, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to list objects in bucket %s: %v", bucketName, err)
	}

	var objectIds []types.ObjectIdentifier
	// Get all object keys in the bucket
	for _, obj := range objects.Contents {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: obj.Key})
	}

	// Delete all objects from the bucket
	if len(objectIds) > 0 {
		_, err = c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &types.Delete{Objects: objectIds},
		})
		if err != nil {
			return fmt.Errorf("failed to delete objects from bucket %s: %v", bucketName, err)
		}
	}

	return nil
}

// ValidateCredentials verifies if the S3 credentials work
func (c *Client) ValidateCredentials(ctx context.Context) error {
	// Try listing buckets as a simple test
	_, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to validate S3 credentials: %v", err)
	}
	return nil
}
```

### Phase 3: Implement Test Driver

#### Task 3.1: Create testdriver.go with CSI driver interface

Create the file `tests/e2e-tests/testdriver.go`:

```go
package e2e

import (
	"context"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/s3client"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	f "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	"k8s.io/kubernetes/test/e2e/storage/framework"
)

var (
	EndpointURL    string
	AccessKeyID    string
	SecretAccessKey string
)

type s3Driver struct {
	client     *s3client.Client
	driverInfo framework.DriverInfo
}

type s3Volume struct {
	bucketName  string
	deleteBucket s3client.DeleteBucketFunc
}

var _ framework.TestDriver = &s3Driver{}
var _ framework.PreprovisionedVolumeTestDriver = &s3Driver{}
var _ framework.PreprovisionedPVTestDriver = &s3Driver{}

func initS3Driver() *s3Driver {
	return &s3Driver{
		client: s3client.New(EndpointURL, AccessKeyID, SecretAccessKey),
		driverInfo: framework.DriverInfo{
			Name:        "s3.csi.scality.com",
			MaxFileSize: framework.FileSizeLarge,
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
			Capabilities: map[framework.Capability]bool{
				framework.CapPersistence: true,
			},
			RequiredAccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteMany,
				v1.ReadOnlyMany,
			},
		},
	}
}

func (d *s3Driver) GetDriverInfo() *framework.DriverInfo {
	return &d.driverInfo
}

func (d *s3Driver) SkipUnsupportedTest(pattern framework.TestPattern) {
	if pattern.VolType != framework.PreprovisionedPV {
		e2eskipper.Skipf("S3 Driver only supports static provisioning -- skipping")
	}
}
```

#### Task 3.2: Implement volume creation/deletion

Add these methods to `testdriver.go`:

```go
func (d *s3Driver) PrepareTest(ctx context.Context, f *f.Framework) *framework.PerTestConfig {
	config := &framework.PerTestConfig{
		Driver:    d,
		Prefix:    "s3",
		Framework: f,
	}

	return config
}

func (d *s3Driver) CreateVolume(ctx context.Context, config *framework.PerTestConfig, volumeType framework.TestVolType) framework.TestVolume {
	if volumeType != framework.PreprovisionedPV {
		f.Failf("Unsupported volType: %v is specified", volumeType)
	}

	bucketName, deleteBucket := d.client.CreateBucket(ctx)

	return &s3Volume{
		bucketName:  bucketName,
		deleteBucket: deleteBucket,
	}
}

func (v *s3Volume) DeleteVolume(ctx context.Context) {
	err := v.deleteBucket(ctx)
	f.ExpectNoError(err, "Failed to delete S3 Bucket: %s", v.bucketName)
}
```

#### Task 3.3: Configure driver capabilities

Add the method to complete `testdriver.go`:

```go
func (d *s3Driver) GetPersistentVolumeSource(readOnly bool, fsType string, testVolume framework.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
	volume, _ := testVolume.(*s3Volume)

	return &v1.PersistentVolumeSource{
		CSI: &v1.CSIPersistentVolumeSource{
			Driver: d.driverInfo.Name,
			VolumeAttributes: map[string]string{
				"bucketName":    volume.bucketName,
				"s3Endpoint":    EndpointURL,
				"accessKeyID":   AccessKeyID,
				"secretAccessKey": SecretAccessKey,
			},
			VolumeHandle: volume.bucketName,
		},
	}, nil
}
```

### Phase 4: Create Main Test File

#### Task 4.1: Implement e2e_test.go with framework setup

Create the file `tests/e2e-tests/e2e_test.go`:

```go
package e2e

import (
	"flag"
	"testing"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	f "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
)

func init() {
	testing.Init()
	f.RegisterClusterFlags(flag.CommandLine)
	f.RegisterCommonFlags(flag.CommandLine)
	f.AfterReadingAllFlags(&f.TestContext)
}
```

#### Task 4.2: Add S3 configuration flags

Add this to `e2e_test.go` in the init function:

```go
func init() {
	testing.Init()
	f.RegisterClusterFlags(flag.CommandLine)
	f.RegisterCommonFlags(flag.CommandLine)
	f.AfterReadingAllFlags(&f.TestContext)

	// Add S3-specific flags
	flag.StringVar(&EndpointURL, "endpoint-url", "", "S3 endpoint URL")
	flag.StringVar(&AccessKeyID, "access-key-id", "", "S3 access key ID")
	flag.StringVar(&SecretAccessKey, "secret-access-key", "", "S3 secret access key")
	flag.Parse()
}

func TestE2E(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Scality S3 CSI E2E Suite")
}
```

#### Task 4.3: Set up test suite execution

Add this to `e2e_test.go`:

```go
var CSITestSuites = []func() framework.TestSuite{
	testsuites.InitVolumesTestSuite, // Basic volume tests
}

// This executes testSuites for CSI volumes.
var _ = utils.SIGDescribe("CSI Volumes", func() {
	curDriver := initS3Driver()

	args := framework.GetDriverNameWithFeatureTags(curDriver)
	args = append(args, func() {
		framework.DefineTestSuites(curDriver, CSITestSuites)
	})
	f.Context(args...)
})
```

### Phase 5: Add Core Test Suites

#### Task 5.1: Create basic volume test

For now, we're using the built-in Kubernetes test suites. We'll start with just the basic volume tests.

The volume tests are already set up in the previous task with:
```go
var CSITestSuites = []func() framework.TestSuite{
	testsuites.InitVolumesTestSuite, // Basic volume tests
}
```

#### Task 5.2: Implement multi-volume test

Update `CSITestSuites` in `e2e_test.go` to add multi-volume tests:

```go
var CSITestSuites = []func() framework.TestSuite{
	testsuites.InitVolumesTestSuite,     // Basic volume tests
	testsuites.InitMultiVolumeTestSuite, // Multi-volume tests
}
```

#### Task 5.3: Add mount options test

Update `CSITestSuites` in `e2e_test.go` again:

```go
var CSITestSuites = []func() framework.TestSuite{
	testsuites.InitVolumesTestSuite,     // Basic volume tests
	testsuites.InitMultiVolumeTestSuite, // Multi-volume tests
	// Add more test suites as needed
}
```

For mount options, we'll need a custom test later since the standard Kubernetes test suites don't cover S3-specific mount options.

### Phase 6: Integrate with Test Scripts

#### Task 6.1: Update test.sh to run Go tests

Edit the file `tests/e2e-tests/scripts/modules/test.sh` to add a function for running Go tests:

```bash
# Add this function to the test.sh file
run_go_tests() {
  local namespace="$1"
  local endpoint_url="$2"
  local access_key_id="$3"
  local secret_access_key="$4"
  local junit_report="$5"

  info "Running Go-based e2e tests"
  info "Using S3 endpoint: $endpoint_url"

  # Change to the test directory
  cd "${PROJECT_ROOT}/tests/e2e-tests"

  # Build test arguments
  local test_args="--endpoint-url=${endpoint_url} --access-key-id=${access_key_id} --secret-access-key=${secret_access_key}"
  
  # Add junit report if specified
  if [ -n "$junit_report" ]; then
    test_args="$test_args --ginkgo.junit-report=${junit_report}"
  fi

  # Run the tests
  go test -v -tags=e2e -args $test_args
  local result=$?

  if [ $result -ne 0 ]; then
    error "Go-based e2e tests failed"
    return 1
  fi

  info "Go-based e2e tests passed"
  return 0
}
```

#### Task 6.2: Add parameter passing to tests

Update the `run_tests` function in `test.sh` to call the Go tests:

```bash
# Modify the run_tests function in test.sh
run_tests() {
  # ... existing code ...

  # Check if we should skip Go tests
  if [ "$skip_go_tests" != "true" ]; then
    # Run Go-based e2e tests
    if ! run_go_tests "$namespace" "$ENDPOINT_URL" "$ACCESS_KEY_ID" "$SECRET_ACCESS_KEY" "$junit_report"; then
      error "Go tests failed"
      tests_passed=false
    fi
  else
    info "Skipping Go-based e2e tests"
  fi

  # ... existing code ...
}
```

#### Task 6.3: Update script documentation

Add documentation to the README.md file in the scripts directory:

```markdown
# E2E Tests for Scality CSI Driver

This directory contains scripts for end-to-end testing of the Scality CSI Driver.

## Features

- Basic volume mount tests
- Multi-volume tests
- Kubernetes e2e framework integration

## Running Tests

To run the tests, use one of the following methods:

### Using the Makefile (Recommended)

```bash
# Run all tests including installation and verification
make e2e-scality-all S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1

# Run only Go-based e2e tests
make e2e-scality-go
```

### Using Scripts Directly

```bash
# Run all tests
./tests/e2e-tests/scripts/run.sh all --endpoint-url http://localhost:8000 --access-key-id accessKey1 --secret-access-key verySecretKey1

# Run only Go-based tests
./tests/e2e-tests/scripts/run.sh go-test --endpoint-url http://localhost:8000 --access-key-id accessKey1 --secret-access-key verySecretKey1
```

## Test Output

Test results will be displayed in the console. If you specify a JUnit report path, a detailed report will be written to that location.
```

### Phase 7: Update Makefile

#### Task 7.1: Add e2e-scality-go target

Add the following to the Makefile:

```makefile
# Run only the Go-based e2e tests (requires S3 endpoint and credentials)
# 
# Required parameters:
#   S3_ENDPOINT_URL - Your Scality S3 endpoint 
#   ACCESS_KEY_ID - Your S3 access key
#   SECRET_ACCESS_KEY - Your S3 secret key
#
# Usage: make e2e-scality-go S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1
.PHONY: e2e-scality-go
e2e-scality-go:
	@if [ -z "$(S3_ENDPOINT_URL)" ]; then \
		echo "Error: S3_ENDPOINT_URL is required. Please provide it with 'make S3_ENDPOINT_URL=http://your-s3-endpoint.com e2e-scality-go'"; \
		exit 1; \
	fi; \
	if [ -z "$(ACCESS_KEY_ID)" ]; then \
		echo "Error: ACCESS_KEY_ID is required. Please provide it with 'make ACCESS_KEY_ID=your_access_key e2e-scality-go'"; \
		exit 1; \
	fi; \
	if [ -z "$(SECRET_ACCESS_KEY)" ]; then \
		echo "Error: SECRET_ACCESS_KEY is required. Please provide it with 'make SECRET_ACCESS_KEY=your_secret_key e2e-scality-go'"; \
		exit 1; \
	fi; \
	cd tests/e2e-tests && go test -v -tags=e2e -args --endpoint-url=$(S3_ENDPOINT_URL) --access-key-id=$(ACCESS_KEY_ID) --secret-access-key=$(SECRET_ACCESS_KEY)
```

#### Task 7.2: Update e2e-scality-all target

Update the Makefile's e2e-scality-all target:

```makefile
# Install CSI driver and run all tests in one command
# 
# Required parameters:
#   S3_ENDPOINT_URL - Your Scality S3 endpoint 
#   ACCESS_KEY_ID - Your S3 access key
#   SECRET_ACCESS_KEY - Your S3 secret key
#
# Optional parameters:
#   CSI_IMAGE_TAG - Specific version of the driver
#   CSI_IMAGE_REPOSITORY - Custom image repository for the driver
#   CSI_NAMESPACE - Namespace to deploy the CSI driver in (defaults to kube-system)
#   VALIDATE_S3 - Set to "true" to verify S3 credentials
#
# Example: make e2e-scality-all S3_ENDPOINT_URL=https://s3.example.com ACCESS_KEY_ID=key SECRET_ACCESS_KEY=secret
.PHONY: e2e-scality-all
e2e-scality-all:
	@if [ -z "$(S3_ENDPOINT_URL)" ]; then \
		echo "Error: S3_ENDPOINT_URL is required. Please provide it with 'make S3_ENDPOINT_URL=https://your-s3-endpoint.com e2e-scality-all'"; \
		exit 1; \
	fi; \
	if [ -z "$(ACCESS_KEY_ID)" ]; then \
		echo "Error: ACCESS_KEY_ID is required. Please provide it with 'make ACCESS_KEY_ID=your_access_key e2e-scality-all'"; \
		exit 1; \
	fi; \
	if [ -z "$(SECRET_ACCESS_KEY)" ]; then \
		echo "Error: SECRET_ACCESS_KEY is required. Please provide it with 'make SECRET_ACCESS_KEY=your_secret_key e2e-scality-all'"; \
		exit 1; \
	fi; \
	INSTALL_ARGS=""; \
	if [ ! -z "$(CSI_IMAGE_TAG)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-tag $(CSI_IMAGE_TAG)"; \
	fi; \
	if [ ! -z "$(CSI_IMAGE_REPOSITORY)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --image-repository $(CSI_IMAGE_REPOSITORY)"; \
	fi; \
	if [ ! -z "$(CSI_NAMESPACE)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --namespace $(CSI_NAMESPACE)"; \
	fi; \
	INSTALL_ARGS="$$INSTALL_ARGS --endpoint-url $(S3_ENDPOINT_URL)"; \
	INSTALL_ARGS="$$INSTALL_ARGS --access-key-id $(ACCESS_KEY_ID)"; \
	INSTALL_ARGS="$$INSTALL_ARGS --secret-access-key $(SECRET_ACCESS_KEY)"; \
	if [ "$(VALIDATE_S3)" = "true" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS --validate-s3"; \
	fi; \
	if [ ! -z "$(ADDITIONAL_ARGS)" ]; then \
		INSTALL_ARGS="$$INSTALL_ARGS $(ADDITIONAL_ARGS)"; \
	fi; \
	./tests/e2e-tests/scripts/run.sh all $$INSTALL_ARGS
```

#### Task 7.3: Document Makefile changes

Add a comment section to the Makefile explaining the e2e test targets:

```makefile
################################################################
# Scality E2E Testing
################################################################
# The following targets provide end-to-end testing capabilities:
#
# e2e-scality-go: Run only the Go-based e2e tests
# e2e-scality-verify: Run only the verification tests
# e2e-scality-all: Install CSI driver and run all tests
#
# All targets require S3 credentials:
#   S3_ENDPOINT_URL - Your Scality S3 endpoint 
#   ACCESS_KEY_ID - Your S3 access key
#   SECRET_ACCESS_KEY - Your S3 secret key
#
# Example: make e2e-scality-all S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1
################################################################
```

### Phase 8: Testing and Validation

#### Task a.1: Test with local Helm cluster

Command to run the tests with a local cluster:

```bash
make e2e-scality-all S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1 VALIDATE_S3=true CSI_IMAGE_TAG=local CSI_IMAGE_REPOSITORY=ghcr.io/scality/mountpoint-s3-csi-driver
```

#### Task 8.2: Verify all test cases pass

Check the output to ensure all test cases pass. If there are failures, investigate and fix them.

#### Task 8.3: Document test execution process

Create a README.md file in the tests/e2e-tests directory with detailed instructions:

```markdown
# End-to-End Tests for Scality Mountpoint S3 CSI Driver

This directory contains end-to-end tests for the Scality Mountpoint S3 CSI Driver using the Kubernetes e2e framework.

## Prerequisites

- A running Kubernetes cluster
- Go 1.20 or later
- Access to a Scality S3 endpoint
- Valid S3 credentials (access key ID and secret access key)

## Test Structure

The tests are organized as follows:

- `e2e_test.go`: Main test entry point and framework setup
- `testdriver.go`: Implementation of the CSI driver for testing
- `s3client/`: S3 client implementation for bucket management
- `testsuites/`: (Future) Custom test suites specific to Scality S3 CSI driver

## Running the Tests

### Using the Makefile

The simplest way to run the tests is using the Makefile targets:

```bash
# Run all tests including Go tests
make e2e-scality-all S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1

# Run only Go tests
make e2e-scality-go S3_ENDPOINT_URL=http://localhost:8000 ACCESS_KEY_ID=accessKey1 SECRET_ACCESS_KEY=verySecretKey1
```

### Running Tests Directly

You can also run the tests directly:

```bash
cd tests/e2e-tests
go test -v -tags=e2e -args --endpoint-url=http://localhost:8000 --access-key-id=accessKey1 --secret-access-key=verySecretKey1
```

## Test Parameters

The following parameters are required:

- `--endpoint-url`: URL of the S3 endpoint
- `--access-key-id`: S3 access key ID
- `--secret-access-key`: S3 secret access key

## Adding New Tests

To add new tests:

1. Create a new test suite file in the testsuites directory
2. Implement the test suite interface
3. Add the test suite to the `CSITestSuites` array in `e2e_test.go`

## CI Integration

These tests are designed to run in CI environments. See the GitHub Actions workflows for details on how they are executed in CI.