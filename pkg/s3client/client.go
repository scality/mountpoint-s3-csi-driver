package s3client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"k8s.io/klog/v2"
)

type Client interface {
	CreateBucket(ctx context.Context, bucket string) error
	DeleteBucket(ctx context.Context, bucket string) error
}

type Config struct {
	Region      string
	EndpointURL string
	Credentials aws.CredentialsProvider
}

// S3API interface for dependency injection and testing
type S3API interface {
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
}

type client struct {
	s3 S3API
}

func New(ctx context.Context, cfg Config) (Client, error) {
	if cfg.Credentials == nil {
		return nil, fmt.Errorf("credentials are required")
	}

	configOpts := []func(*config.LoadOptions) error{
		config.WithCredentialsProvider(cfg.Credentials),
	}

	if cfg.Region != "" {
		configOpts = append(configOpts, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &client{
		s3: s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.UsePathStyle = true
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}),
	}, nil
}

func (c *client) CreateBucket(ctx context.Context, bucket string) error {
	klog.V(4).Infof("Creating S3 bucket: %s", bucket)
	_, err := c.s3.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var bucketAlreadyExists *types.BucketAlreadyExists
		var bucketAlreadyOwnedByYou *types.BucketAlreadyOwnedByYou
		if errors.As(err, &bucketAlreadyExists) || errors.As(err, &bucketAlreadyOwnedByYou) {
			klog.V(4).Infof("Bucket %s already exists, continuing", bucket)
			return nil
		}
		klog.Errorf("Failed to create bucket %s: %v", bucket, err)
		return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
	}
	klog.V(4).Infof("Successfully created bucket: %s", bucket)
	return nil
}

func (c *client) DeleteBucket(ctx context.Context, bucket string) error {
	klog.V(4).Infof("Deleting S3 bucket: %s", bucket)
	_, err := c.s3.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		var noSuchBucketErr *types.NoSuchBucket
		if errors.As(err, &noSuchBucketErr) {
			klog.V(4).Infof("Bucket %s does not exist, continuing", bucket)
			return nil
		}

		// Check for bucket not empty error (409 Conflict)
		errStr := err.Error()
		if strings.Contains(errStr, "BucketNotEmpty") || strings.Contains(errStr, "bucket is not empty") {
			klog.V(4).Infof("Bucket %s is not empty, skipping deletion for safety", bucket)
			return nil
		}

		klog.Errorf("Failed to delete bucket %s: %v", bucket, err)
		return fmt.Errorf("failed to delete bucket %s: %w", bucket, err)
	}
	klog.V(4).Infof("Successfully deleted bucket: %s", bucket)
	return nil
}
