package s3client

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestNew(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		client, err := New(context.Background(), Config{
			Region:      "us-east-1",
			EndpointURL: "https://s3.example.com",
			Credentials: aws.AnonymousCredentials{},
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if client == nil {
			t.Fatal("Expected client but got nil")
		}
		// Verify interface compliance
		_ = Client(client)
	})

	t.Run("missing credentials", func(t *testing.T) {
		_, err := New(context.Background(), Config{
			Region:      "us-east-1",
			EndpointURL: "https://s3.example.com",
			Credentials: nil,
		})
		if err == nil {
			t.Fatal("Expected error but got none")
		}
	})
}

// Mock S3 API for testing
type mockS3API struct {
	createBucketFunc func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	deleteBucketFunc func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
}

func (m *mockS3API) CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	if m.createBucketFunc != nil {
		return m.createBucketFunc(ctx, params, optFns...)
	}
	return &s3.CreateBucketOutput{}, nil
}

func (m *mockS3API) DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
	if m.deleteBucketFunc != nil {
		return m.deleteBucketFunc(ctx, params, optFns...)
	}
	return &s3.DeleteBucketOutput{}, nil
}

func TestCreateBucket(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		mockFunc   func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
		wantErr    bool
	}{
		{
			name:       "successful creation",
			bucketName: "test-bucket",
			mockFunc: func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
				return &s3.CreateBucketOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:       "bucket already exists - should succeed",
			bucketName: "existing-bucket",
			mockFunc: func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
				return nil, &types.BucketAlreadyExists{
					Message: aws.String("The requested bucket name is not available"),
				}
			},
			wantErr: false,
		},
		{
			name:       "bucket already owned by you - should succeed",
			bucketName: "owned-bucket",
			mockFunc: func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
				return nil, &types.BucketAlreadyOwnedByYou{
					Message: aws.String("Your previous request to create the named bucket succeeded"),
				}
			},
			wantErr: false,
		},
		{
			name:       "other S3 error - should fail",
			bucketName: "error-bucket",
			mockFunc: func(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{
				createBucketFunc: tt.mockFunc,
			}
			client := &client{s3: mockAPI}

			err := client.CreateBucket(context.Background(), tt.bucketName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBucket() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeleteBucket(t *testing.T) {
	tests := []struct {
		name       string
		bucketName string
		mockFunc   func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
		wantErr    bool
	}{
		{
			name:       "successful deletion",
			bucketName: "test-bucket",
			mockFunc: func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
				return &s3.DeleteBucketOutput{}, nil
			},
			wantErr: false,
		},
		{
			name:       "bucket does not exist - should succeed",
			bucketName: "nonexistent-bucket",
			mockFunc: func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
				return nil, &types.NoSuchBucket{
					Message: aws.String("The specified bucket does not exist"),
				}
			},
			wantErr: false,
		},
		{
			name:       "bucket not empty - should succeed (safety)",
			bucketName: "nonempty-bucket",
			mockFunc: func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
				return nil, errors.New("BucketNotEmpty: The bucket you tried to delete is not empty")
			},
			wantErr: false,
		},
		{
			name:       "bucket not empty different format - should succeed",
			bucketName: "nonempty-bucket-2",
			mockFunc: func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
				return nil, errors.New("bucket is not empty")
			},
			wantErr: false,
		},
		{
			name:       "other S3 error - should fail",
			bucketName: "error-bucket",
			mockFunc: func(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error) {
				return nil, errors.New("access denied")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := &mockS3API{
				deleteBucketFunc: tt.mockFunc,
			}
			client := &client{s3: mockAPI}

			err := client.DeleteBucket(context.Background(), tt.bucketName)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteBucket() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
