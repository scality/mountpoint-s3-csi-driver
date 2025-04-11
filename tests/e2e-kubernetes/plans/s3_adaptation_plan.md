# S3 Adaptation Plan for E2E Tests

This document outlines the plan for adapting the S3 CSI Driver E2E tests to work with S3-compatible storage implementations using static credentials.

## 1. Disable S3 Express Directory Bucket Tests

### 1.1 Modify Directory Bucket Creation in Test Driver
- In `testdriver.go`, modify the `CreateVolume` method to always use standard buckets:

```go
func (d *s3Driver) CreateVolume(ctx context.Context, config *framework.PerTestConfig, volumeType framework.TestVolType) framework.TestVolume {
    if volumeType != framework.PreprovisionedPV {
        f.Failf("Unsupported volType: %v is specified", volumeType)
    }

    // Always use standard buckets, even for S3 Express test identifier
    bucketName, deleteBucket := d.client.CreateStandardBucket(ctx)

    return &s3Volume{
        bucketName:           bucketName,
        deleteBucket:         deleteBucket,
        authenticationSource: custom_testsuites.AuthenticationSourceFromContext(ctx),
    }
}
```

### 1.2 Add Skip Logic for S3 Express-Specific Test Blocks
- Add skip statements to S3 Express test suites in `testsuites/cache.go`:

```go
Describe("Express", Serial, func() {
    BeforeEach(func() {
        Skip("S3 Express tests are disabled")
    })
    // ...existing test code...
})

Describe("Multi-Level", Serial, func() {
    BeforeEach(func() {
        Skip("Multi-level cache tests using S3 Express are disabled")
    })
    // ...existing test code...
})
```

### 1.3 Skip Individual S3 Express Tests in Other Suites
- Add skip statements to individual tests with S3 Express in their name:

```go
// In testsuites/mountoptions.go
ginkgo.It("S3 express -- should not be able to access volume as a non-root user", func(ctx context.Context) {
    Skip("S3 Express tests are disabled")
    // ...existing test code...
})
```

## 2. Ensure Consistent AWS Configuration Across All Tests

### 2.1 Modify `awsConfig` in `testsuites/util.go`
- Update to match existing S3 client configuration:

```go
func awsConfig(ctx context.Context) aws.Config {
    // Match the existing S3 client configuration
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion(DefaultRegion),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
            s3client.DefaultAccessKey,
            s3client.DefaultSecretKey,
            "",
        )),
        config.WithRetryer(func() aws.Retryer {
            return retry.NewStandard(func(opts *retry.StandardOptions) {
                opts.MaxAttempts = 5
                opts.MaxBackoff = 2 * time.Minute
            })
        }),
    )
    framework.ExpectNoError(err)
    
    return cfg
}
```

### 2.2 Add Helper Function for S3 Client Creation
- Add a helper function to ensure all S3 clients use path style and custom endpoint:

```go
// Helper function to create properly configured S3 clients
func newS3ClientFromConfig(cfg aws.Config) *s3.Client {
    return s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.UsePathStyle = true
        o.BaseEndpoint = aws.String(s3client.DefaultEndpoint)
    })
}
```

### 2.3 Update Direct S3 Client Usage
- Update any code that creates S3 clients directly:

```go
// Find and replace instances like:
client := s3.NewFromConfig(awsConfig(ctx))

// With:
client := newS3ClientFromConfig(awsConfig(ctx))
```

## 3. Skip or Adapt AWS IAM/STS Tests

### 3.1 Skip IAM Role Tests
- In `testsuites/credentials.go`, add skip logic for IRSA tests:

```go
Context("IAM Roles for Service Accounts (IRSA)", Ordered, func() {
    BeforeEach(func(ctx context.Context) {
        Skip("These tests rely on AWS IAM/STS services - using static credentials instead")
    })
    // ...existing test code...
})
```

### 3.2 Skip IMDS-Dependent Tests
- Skip tests that require Instance Metadata Service:

```go
It("should automatically detect the STS region if IMDS is available", func(ctx context.Context) {
    Skip("This test requires AWS IMDS - using configured region instead")
    // ...existing code...
})
```

## 4. Keep All Standard S3 Operation Tests Active

- Maintain all tests that only use basic S3 operations:
  - Basic volume mounting tests
  - File read/write operations
  - Permission verification
  - Multi-volume tests 
  - Mount options tests (except Express-specific ones)

## 5. Implementation Notes

### 5.1 Environment Variables
The tests already read the following environment variables:
- `S3_ENDPOINT_URL` - Custom S3 endpoint
- `AWS_ACCESS_KEY_ID` - Access key ID
- `AWS_SECRET_ACCESS_KEY` - Secret access key

These should be set in the CI environment before running tests.

### 5.2 Test Execution
The tests will be run using the existing GitHub Actions workflow, which already sets up the environment with:
```yaml
env:
  AWS_ACCESS_KEY_ID: "accessKey1"
  AWS_SECRET_ACCESS_KEY: "verySecretKey1"
  S3_ENDPOINT_URL: <endpoint_url>
```

### 5.3 Expected Outcomes
After these changes:
- All standard S3 functionality tests should pass
- S3 Express tests will be skipped
- AWS IAM/STS integration tests will be skipped
- Any S3 client will use the provided endpoint, region, and credentials

This adaptation maintains maximum test coverage for the standard S3 functionality while accommodating S3-compatible storage systems. 