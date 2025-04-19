# S3 Client Package

This package provides a Go client for interacting with S3-compatible storage services, specifically designed for use with the Scality S3 CSI Driver E2E tests.

## Features

- Connection to S3-compatible storage services with custom endpoints
- Creation and deletion of buckets with unique names
- Management of objects within buckets
- Automatic tracking and cleanup of created resources
- Simple authentication with access key and secret key

## Usage

```go
import (
    "context"
    "log"
    
    "github.com/scality/mountpoint-s3-csi-driver/tests/e2e-tests/pkg/s3client"
)

func main() {
    // Create client configuration
    cfg := &s3client.Config{
        EndpointURL:     "http://localhost:8000",
        AccessKeyID:     "accessKey1",
        SecretAccessKey: "verySecretKey1",
        BucketPrefix:    "e2e-test",
    }
    
    // Initialize client
    client, err := s3client.NewClient(cfg)
    if err != nil {
        log.Fatalf("Failed to create S3 client: %v", err)
    }
    
    // Create a bucket
    ctx := context.Background()
    bucketName, err := client.CreateBucket(ctx)
    if err != nil {
        log.Fatalf("Failed to create bucket: %v", err)
    }
    log.Printf("Created bucket: %s", bucketName)
    
    // Check if a bucket exists
    exists, err := client.BucketExists(ctx, bucketName)
    if err != nil {
        log.Fatalf("Failed to check if bucket exists: %v", err)
    }
    log.Printf("Bucket %s exists: %v", bucketName, exists)
    
    // Cleanup all created buckets when done
    defer func() {
        if err := client.CleanupAllBuckets(ctx); err != nil {
            log.Printf("Warning: failed to cleanup all buckets: %v", err)
        }
    }()
}
```

## Functions

- `NewClient(cfg *Config) (*Client, error)`: Creates a new S3 client with the provided configuration
- `CreateBucket(ctx context.Context) (string, error)`: Creates a new S3 bucket with a unique name
- `DeleteBucket(ctx context.Context, bucketName string) error`: Deletes an S3 bucket
- `CleanupAllBuckets(ctx context.Context) error`: Deletes all buckets created by this client
- `BucketExists(ctx context.Context, bucketName string) (bool, error)`: Checks if a bucket exists 