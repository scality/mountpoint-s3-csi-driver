/*
Copyright 2022 The Kubernetes Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	// StorageClass parameters
	parameterBucketNaming  = "bucketNaming"
	parameterS3Region      = "s3Region"
	parameterBucketPrefix  = "bucketPrefix"
	parameterMountOptions  = "mountOptions"
	parameterReclaimPolicy = "reclaimPolicy"

	// Bucket naming strategies
	bucketNamingDedicated = "dedicated" // Create dedicated bucket per volume
	bucketNamingShared    = "shared"    // Use shared bucket with prefix per volume

	// Default values
	defaultBucketNaming  = bucketNamingDedicated
	defaultS3Region      = "us-east-1"
	defaultReclaimPolicy = "Delete"

	// Volume attributes
	volumeAttributeBucketName = "bucketName"
	volumeAttributePrefix     = "prefix"
	volumeAttributeRegion     = "region"

	// Bucket naming constraints
	maxBucketNameLength = 63
	minBucketNameLength = 3
	bucketNamePrefix    = "s3-csi-"
)

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("CreateVolume: called with args %s", protosanitizer.StripSecrets(req))

	// Validate request
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name is required")
	}

	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if capacityBytes < 0 {
		return nil, status.Error(codes.OutOfRange, "Required bytes must not be negative")
	}

	// Parse parameters
	params := req.GetParameters()
	if params == nil {
		params = make(map[string]string)
	}

	// Set default parameters
	bucketNaming := getParameterWithDefault(params, parameterBucketNaming, defaultBucketNaming)
	s3Region := getParameterWithDefault(params, parameterS3Region, defaultS3Region)
	bucketPrefix := params[parameterBucketPrefix]
	reclaimPolicy := getParameterWithDefault(params, parameterReclaimPolicy, defaultReclaimPolicy)

	// Validate parameters
	if bucketNaming != bucketNamingDedicated && bucketNaming != bucketNamingShared {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid bucketNaming: %s. Supported values: %s, %s",
			bucketNaming, bucketNamingDedicated, bucketNamingShared)
	}

	if bucketNaming == bucketNamingShared && bucketPrefix == "" {
		return nil, status.Error(codes.InvalidArgument, "bucketPrefix is required when bucketNaming is 'shared'")
	}

	// Create S3 client
	s3Client, err := d.createS3Client(ctx, s3Region)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create S3 client: %v", err)
	}

	// Prepare volume attributes
	volumeAttributes := make(map[string]string)
	volumeAttributes[volumeAttributeRegion] = s3Region

	var bucketName, volumePrefix string
	volumeID := req.GetName()

	switch bucketNaming {
	case bucketNamingDedicated:
		// Create a dedicated bucket for this volume
		bucketName = generateBucketName(volumeID)

		// Validate bucket name
		if err := validateBucketName(bucketName); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid bucket name: %v", err)
		}

		// Check if bucket already exists
		if exists, err := d.bucketExists(ctx, s3Client, bucketName); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to check bucket existence: %v", err)
		} else if exists {
			klog.V(4).Infof("CreateVolume: bucket %s already exists, using existing bucket", bucketName)
		} else {
			// Create the bucket
			if err := d.createBucket(ctx, s3Client, bucketName, s3Region); err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to create bucket: %v", err)
			}
			klog.V(4).Infof("CreateVolume: created bucket %s", bucketName)
		}

		volumeAttributes[volumeAttributeBucketName] = bucketName

	case bucketNamingShared:
		// Use shared bucket with volume-specific prefix
		bucketName = bucketPrefix
		volumePrefix = generateVolumePrefix(volumeID)

		// Check if shared bucket exists
		if exists, err := d.bucketExists(ctx, s3Client, bucketName); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to check shared bucket existence: %v", err)
		} else if !exists {
			return nil, status.Errorf(codes.Internal, "Shared bucket %s does not exist. Please create it manually.", bucketName)
		}

		volumeAttributes[volumeAttributeBucketName] = bucketName
		volumeAttributes[volumeAttributePrefix] = volumePrefix
	}

	// Create volume response
	volume := &csi.Volume{
		VolumeId:      volumeID,
		CapacityBytes: capacityBytes,
		VolumeContext: volumeAttributes,
	}

	return &csi.CreateVolumeResponse{Volume: volume}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("DeleteVolume: called with args: %s", protosanitizer.StripSecrets(req))

	volumeID := req.GetVolumeId()
	if volumeID == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	// For dynamic provisioning, we need to determine if this was a dedicated bucket
	// Since we don't have access to the original parameters here, we use naming convention
	bucketName := generateBucketName(volumeID)

	// Create S3 client with default region first, we'll get the actual region from bucket location
	s3Client, err := d.createS3Client(ctx, defaultS3Region)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create S3 client: %v", err)
	}

	// Check if bucket exists
	exists, err := d.bucketExists(ctx, s3Client, bucketName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to check bucket existence: %v", err)
	}

	if !exists {
		klog.V(4).Infof("DeleteVolume: bucket %s does not exist, volume already deleted", bucketName)
		return &csi.DeleteVolumeResponse{}, nil
	}

	// Get bucket region to create client with correct region
	bucketRegion, err := d.getBucketRegion(ctx, s3Client, bucketName)
	if err != nil {
		klog.Warningf("DeleteVolume: failed to get bucket region, using default: %v", err)
		bucketRegion = defaultS3Region
	}

	// Create client with correct region if different
	if bucketRegion != defaultS3Region {
		s3Client, err = d.createS3Client(ctx, bucketRegion)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to create S3 client for region %s: %v", bucketRegion, err)
		}
	}

	// Only delete if it matches our naming convention (safety check)
	if strings.HasPrefix(bucketName, bucketNamePrefix) {
		// Delete all objects in the bucket first
		if err := d.deleteBucketContents(ctx, s3Client, bucketName); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to delete bucket contents: %v", err)
		}

		// Delete the bucket
		if err := d.deleteBucket(ctx, s3Client, bucketName); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to delete bucket: %v", err)
		}

		klog.V(4).Infof("DeleteVolume: deleted bucket %s", bucketName)
	} else {
		klog.V(4).Infof("DeleteVolume: bucket %s does not match CSI naming convention, skipping deletion", bucketName)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	klog.V(4).Infof("ControllerGetCapabilities: called with args %s", protosanitizer.StripSecrets(req))

	caps := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	}

	var capsResponse []*csi.ControllerServiceCapability
	for _, cap := range caps {
		c := &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
		capsResponse = append(capsResponse, c)
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: capsResponse}, nil
}

func (d *Driver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	klog.V(4).Infof("GetCapacity: called with args %s", protosanitizer.StripSecrets(req))
	// S3 has virtually unlimited capacity
	return &csi.GetCapacityResponse{
		AvailableCapacity: 1024 * 1024 * 1024 * 1024 * 1024, // 1 PB
	}, nil
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("ListVolumes: called with args %s", protosanitizer.StripSecrets(req))
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %s", protosanitizer.StripSecrets(req))

	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID is required")
	}

	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities are required")
	}

	// Check each capability
	for _, cap := range req.GetVolumeCapabilities() {
		// Check access mode
		switch cap.GetAccessMode().GetMode() {
		case csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY:
			// Supported
		default:
			return &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: nil,
				Message:   fmt.Sprintf("Unsupported access mode: %v", cap.GetAccessMode().GetMode()),
			}, nil
		}

		// Check volume type
		if cap.GetMount() == nil {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: nil,
				Message:   "Only mount volumes are supported",
			}, nil
		}
	}

	// All capabilities are supported
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
			VolumeContext:      req.GetVolumeContext(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (d *Driver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ControllerModifyVolume(context.Context, *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// Helper functions

func (d *Driver) createS3Client(ctx context.Context, region string) (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Get endpoint URL from environment variable
	endpointURL := getEnvOrDefault("AWS_ENDPOINT_URL", "")

	clientOptions := func(o *s3.Options) {
		o.UsePathStyle = true
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	}

	return s3.NewFromConfig(cfg, clientOptions), nil
}

func (d *Driver) bucketExists(ctx context.Context, client *s3.Client, bucketName string) (bool, error) {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		var nfe *types.NotFound
		if errors.As(err, &nfe) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *Driver) createBucket(ctx context.Context, client *s3.Client, bucketName, region string) error {
	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}

	// Only set CreateBucketConfiguration for regions other than us-east-1
	if region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}

	_, err := client.CreateBucket(ctx, input)
	return err
}

func (d *Driver) getBucketRegion(ctx context.Context, client *s3.Client, bucketName string) (string, error) {
	result, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "", err
	}

	region := string(result.LocationConstraint)
	if region == "" {
		// Empty location constraint means us-east-1
		region = "us-east-1"
	}

	return region, nil
}

func (d *Driver) deleteBucketContents(ctx context.Context, client *s3.Client, bucketName string) error {
	// List all objects in the bucket
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects: %v", err)
		}

		if len(page.Contents) == 0 {
			continue
		}

		// Prepare objects for deletion
		var objectIds []types.ObjectIdentifier
		for _, obj := range page.Contents {
			objectIds = append(objectIds, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		// Delete objects in batch
		_, err = client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucketName),
			Delete: &types.Delete{
				Objects: objectIds,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete objects: %v", err)
		}
	}

	return nil
}

func (d *Driver) deleteBucket(ctx context.Context, client *s3.Client, bucketName string) error {
	_, err := client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	return err
}

func generateBucketName(volumeID string) string {
	// Sanitize volume ID for bucket naming
	sanitized := strings.ToLower(volumeID)
	sanitized = strings.ReplaceAll(sanitized, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")

	bucketName := bucketNamePrefix + sanitized

	// Ensure bucket name is within limits
	if len(bucketName) > maxBucketNameLength {
		bucketName = bucketName[:maxBucketNameLength]
	}

	// Ensure bucket name doesn't end with hyphen
	bucketName = strings.TrimRight(bucketName, "-")

	return bucketName
}

func generateVolumePrefix(volumeID string) string {
	// Generate a prefix for the volume within a shared bucket
	sanitized := strings.ReplaceAll(volumeID, "_", "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	return fmt.Sprintf("volumes/%s/", sanitized)
}

func validateBucketName(name string) error {
	if len(name) < minBucketNameLength || len(name) > maxBucketNameLength {
		return fmt.Errorf("bucket name must be between %d and %d characters", minBucketNameLength, maxBucketNameLength)
	}

	if !strings.HasPrefix(name, bucketNamePrefix) {
		return fmt.Errorf("bucket name must start with %s", bucketNamePrefix)
	}

	return nil
}

func getParameterWithDefault(params map[string]string, key, defaultValue string) string {
	if value, exists := params[key]; exists && value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
