// package s3client

// import (
// 	"context"
// 	"fmt"
// 	"strings"
// 	"time"

// 	"github.com/aws/aws-sdk-go-v2/aws"
// 	"github.com/aws/aws-sdk-go-v2/config"
// 	"github.com/aws/aws-sdk-go-v2/credentials"
// 	"github.com/aws/aws-sdk-go-v2/service/s3"
// 	"github.com/aws/aws-sdk-go-v2/service/s3/types"
// 	"github.com/google/uuid"
// )

// // Config represents the configuration for the S3 client
// type Config struct {
// 	EndpointURL     string
// 	AccessKeyID     string
// 	SecretAccessKey string
// 	BucketPrefix    string
// }

// // Client manages operations with the S3 server
// type Client struct {
// 	s3Client       *s3.Client
// 	config         *Config
// 	createdBuckets []string
// }

// // NewClient creates a new S3 client
// func NewClient(cfg *Config) (*Client, error) {
// 	// Validate configuration
// 	if cfg.EndpointURL == "" {
// 		return nil, fmt.Errorf("S3 endpoint URL cannot be empty")
// 	}
// 	if cfg.AccessKeyID == "" {
// 		return nil, fmt.Errorf("access key ID cannot be empty")
// 	}
// 	if cfg.SecretAccessKey == "" {
// 		return nil, fmt.Errorf("secret access key cannot be empty")
// 	}
// 	if cfg.BucketPrefix == "" {
// 		cfg.BucketPrefix = "e2e-test"
// 	}

// 	// Create custom resolver to use the provided endpoint
// 	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
// 		return aws.Endpoint{
// 			URL:               cfg.EndpointURL,
// 			HostnameImmutable: true,
// 			SigningRegion:     "us-east-1", // Default region
// 		}, nil
// 	})

// 	// Create AWS credentials provider
// 	credProvider := credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")

// 	// Create AWS configuration
// 	awsCfg, err := config.LoadDefaultConfig(context.Background(),
// 		config.WithEndpointResolverWithOptions(customResolver),
// 		config.WithCredentialsProvider(credProvider),
// 		config.WithRegion("us-east-1"),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to load AWS config: %v", err)
// 	}

// 	// Create S3 client
// 	s3Client := s3.NewFromConfig(awsCfg)

// 	return &Client{
// 		s3Client:       s3Client,
// 		config:         cfg,
// 		createdBuckets: []string{},
// 	}, nil
// }

// // CreateBucket creates a new S3 bucket with a unique name
// func (c *Client) CreateBucket(ctx context.Context) (string, error) {
// 	// Generate a unique bucket name
// 	uniqueID := uuid.New().String()[:8]
// 	bucketName := fmt.Sprintf("%s-%s", c.config.BucketPrefix, uniqueID)
// 	bucketName = strings.ToLower(bucketName)

// 	// Create the bucket
// 	_, err := c.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
// 		Bucket: aws.String(bucketName),
// 	})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create bucket %s: %v", bucketName, err)
// 	}

// 	// Wait for the bucket to be created
// 	waiter := s3.NewBucketExistsWaiter(c.s3Client)
// 	err = waiter.Wait(ctx, &s3.HeadBucketInput{
// 		Bucket: aws.String(bucketName),
// 	}, 30*time.Second)
// 	if err != nil {
// 		return "", fmt.Errorf("failed waiting for bucket %s to be created: %v", bucketName, err)
// 	}

// 	// Track created buckets
// 	c.createdBuckets = append(c.createdBuckets, bucketName)

// 	return bucketName, nil
// }

// // DeleteBucket deletes an S3 bucket
// func (c *Client) DeleteBucket(ctx context.Context, bucketName string) error {
// 	// Delete all objects in the bucket
// 	if err := c.emptyBucket(ctx, bucketName); err != nil {
// 		return err
// 	}

// 	// Delete the bucket
// 	_, err := c.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
// 		Bucket: aws.String(bucketName),
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to delete bucket %s: %v", bucketName, err)
// 	}

// 	return nil
// }

// // emptyBucket deletes all objects in a bucket
// func (c *Client) emptyBucket(ctx context.Context, bucketName string) error {
// 	// List all objects in the bucket
// 	listResp, err := c.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
// 		Bucket: aws.String(bucketName),
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to list objects in bucket %s: %v", bucketName, err)
// 	}

// 	// If there are no objects, return
// 	if len(listResp.Contents) == 0 {
// 		return nil
// 	}

// 	// Create a list of objects to delete
// 	objIds := make([]types.ObjectIdentifier, len(listResp.Contents))
// 	for i, obj := range listResp.Contents {
// 		objIds[i] = types.ObjectIdentifier{Key: obj.Key}
// 	}

// 	// Delete all objects
// 	_, err = c.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
// 		Bucket: aws.String(bucketName),
// 		Delete: &types.Delete{
// 			Objects: objIds,
// 			Quiet:   aws.Bool(true),
// 		},
// 	})
// 	if err != nil {
// 		return fmt.Errorf("failed to delete objects in bucket %s: %v", bucketName, err)
// 	}

// 	return nil
// }

// // CleanupAllBuckets deletes all buckets created by this client
// func (c *Client) CleanupAllBuckets(ctx context.Context) error {
// 	var errors []string

// 	for _, bucketName := range c.createdBuckets {
// 		if err := c.DeleteBucket(ctx, bucketName); err != nil {
// 			errors = append(errors, err.Error())
// 		}
// 	}

// 	if len(errors) > 0 {
// 		return fmt.Errorf("failed to cleanup all buckets: %s", strings.Join(errors, "; "))
// 	}

// 	// Clear the list of created buckets
// 	c.createdBuckets = []string{}

// 	return nil
// }

// // BucketExists checks if a bucket exists
// func (c *Client) BucketExists(ctx context.Context, bucketName string) (bool, error) {
// 	_, err := c.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
// 		Bucket: aws.String(bucketName),
// 	})
// 	if err != nil {
// 		// Check if the error is a "not found" error
// 		if strings.Contains(err.Error(), "NotFound") {
// 			return false, nil
// 		}
// 		return false, fmt.Errorf("failed to check if bucket %s exists: %v", bucketName, err)
// 	}
// 	return true, nil
// }

package s3client

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// DefaultRegion is the default AWS region to use if unspecified.
// It is public in order to be modified from test binary which receives region to use as a flag.
var DefaultRegion string

const s3BucketNameMaxLength = 63
const s3BucketNamePrefix = "s3-csi-k8s-e2e-"

// DeleteBucketFunc is a cleanup function thats returned as a result of "Create*Bucket" calls.
// It clears the content of the bucket if not empty, and then deletes the bucket.
type DeleteBucketFunc func(context.Context) error

type Client struct {
	region string
	client *s3.Client
}

// New returns a new client with "DefaultRegion".
func New() *Client {
	return NewWithRegion(DefaultRegion)
}

// NewWithRegion returns a new client with the given `region`.
// TODO: path style and attach endpoint
func NewWithRegion(region string) *Client {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithRetryer(func() aws.Retryer {
			return retry.NewStandard(func(opts *retry.StandardOptions) {
				opts.MaxAttempts = 5
				opts.MaxBackoff = 2 * time.Minute
			})
		}),
	)
	framework.ExpectNoError(err)
	return &Client{region: region, client: s3.NewFromConfig(cfg)} // TODO: Add endpoint here
}

// CreateStandardBucket creates a new standard S3 bucket with a random name,
// and returns the bucket name and a clean up function.
func (c *Client) CreateStandardBucket(ctx context.Context) (string, DeleteBucketFunc) {
	bucketName := c.randomBucketName("")

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	if c.region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(c.region),
		}
	}

	return bucketName, c.create(ctx, input)
}


// randomBucketName generates a random bucket name by using prefix (`s3BucketNamePrefix`) and `suffix`
// and generating random string for the remaining space according to S3's limit (63 as of today).
func (c *Client) randomBucketName(suffix string) string {
	prefixLen := len(s3BucketNamePrefix)
	suffixLen := len(suffix)
	rand := utilrand.String(s3BucketNameMaxLength - prefixLen - suffixLen)  // TODO: Add util
	return s3BucketNamePrefix + rand + suffix
}

// WipeoutBucket removes all objects from given `bucketName`.
func (c *Client) WipeoutBucket(ctx context.Context, bucketName string) error {
	objects, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}

	var objectIds []types.ObjectIdentifier
	// get all object keys in the s3 bucket
	for _, obj := range objects.Contents {
		objectIds = append(objectIds, types.ObjectIdentifier{Key: obj.Key})
	}

	// delete all objects from the bucket
	if len(objectIds) > 0 {
		_, err = c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &types.Delete{Objects: objectIds},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) create(ctx context.Context, input *s3.CreateBucketInput) DeleteBucketFunc {
	bucketName := *input.Bucket

	_, err := c.client.CreateBucket(ctx, input)
	framework.ExpectNoError(err, "Failed to create S3 bucket")
	if err == nil {
		framework.Logf("S3 Bucket %s created", bucketName)
	}

	return func(ctx context.Context) error {
		return c.delete(ctx, bucketName)
	}
}

func (c *Client) delete(ctx context.Context, bucketName string) error {
	framework.Logf("Deleting S3 Bucket %s...", bucketName)

	err := c.WipeoutBucket(ctx, bucketName)
	if err != nil {
		return err
	}

	// finally delete the bucket
	_, err = c.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return err
	}

	framework.Logf("S3 Bucket %s deleted", bucketName)

	return nil
}
