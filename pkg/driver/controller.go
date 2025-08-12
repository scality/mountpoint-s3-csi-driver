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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/volumecontext"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/storageclass"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/s3client"
)

const defaultVolumeCapacityBytes int64 = 1 << 30 // 1 GiB

func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	klog.V(4).Infof("CreateVolume: called with args %s", protosanitizer.StripSecrets(req))

	if err := validateCreateVolumeRequest(req); err != nil {
		klog.Errorf("CreateVolume: invalid request: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	klog.V(4).Infof("CreateVolume: received parameters: %+v", req.GetParameters())
	klog.V(4).Infof("CreateVolume: received secrets count: %d", len(req.GetSecrets()))
	params, err := storageclass.ParseAndValidate(req.GetParameters())
	if err != nil {
		klog.Errorf("CreateVolume: failed to parse StorageClass parameters: %v", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse StorageClass parameters: %v", err))
	}
	klog.V(4).Infof("CreateVolume: parsed parameters - HasProvisionerSecret: %v, HasNodePublishSecret: %v", params.HasProvisionerSecret(), params.HasNodePublishSecret())

	volumeID := generateVolumeID()
	klog.V(4).Infof("Generated volume ID: %s", volumeID)

	// Controller Credential Resolution for Bucket Operations
	//
	// CSI Credential Resolution:
	// According to the CSI specification, when StorageClass contains secret parameters
	// like csi.storage.k8s.io/provisioner-secret-name, the CSI provisioner sidecar
	// is responsible for resolving the secret and passing the actual credential values
	// in the 'secrets' field of the CreateVolumeRequest. The secret names/namespaces
	// are NOT passed in the 'parameters' field.
	//
	// Credential Resolution Order:
	// 1. CSI secrets: If provisioner resolved secrets, use credentials from req.GetSecrets()
	// 2. Driver credentials: If no secrets provided, use driver-level credentials
	//
	// This approach properly separates:
	// - CSI provisioner handles secret resolution from StorageClass
	// - Controller handles credential usage for S3 operations
	// - Node handles mounting with appropriate credentials passed via volume context

	awsConfig, err := d.resolveControllerCredentials(ctx, req, params)
	if err != nil {
		klog.Errorf("CreateVolume: failed to resolve credentials for volume %s: %v", volumeID, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve credentials: %v", err))
	}

	klog.V(4).Infof("Resolved credentials for volume %s using authentication tier: %s", volumeID, params.AuthTier)

	s3Client, err := d.createS3Client(ctx, &awsConfig)
	if err != nil {
		klog.Errorf("CreateVolume: failed to create S3 client for volume %s: %v", volumeID, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create S3 client: %v", err))
	}

	if err := s3Client.CreateBucket(ctx, volumeID); err != nil {
		klog.Errorf("CreateVolume: bucket creation failed for volume %s: %v", volumeID, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("bucket creation failed: %v", err))
	}

	volumeContext := map[string]string{
		"dynamicProvisioning": "true",
		"bucketName":          volumeID,
	}

	// Authentication Source Configuration for Dynamic Provisioning
	//
	// CSI Credential Resolution:
	// The CSI provisioner resolves StorageClass secret parameters and passes credential values
	// in the 'secrets' field of CreateVolumeRequest. The authentication source is determined
	// by whether the CSI provisioner provided secrets or not.
	//
	// Authentication Source Logic:
	// 1. If CSI secrets provided: authenticationSource = "secret" (node should use secrets)
	// 2. If no CSI secrets: authenticationSource = "driver" (node should use driver credentials)
	//
	// Note: With CSI secret resolution, we don't get the original StorageClass parameter names,
	// so we can't distinguish between provisioner-secret vs node-publish-secret in the controller.
	// The node will need to determine the appropriate credential source during NodePublishVolume.

	csiSecrets := req.GetSecrets()
	if len(csiSecrets) > 0 {
		// CSI provisioner resolved secrets from StorageClass, node should use secret authentication
		volumeContext[volumecontext.AuthenticationSource] = credentialprovider.AuthenticationSourceSecret
		klog.V(4).Infof("Set authenticationSource=secret for volume %s (CSI secrets provided)", volumeID)
	} else {
		// No CSI secrets provided, node should use driver credentials
		volumeContext[volumecontext.AuthenticationSource] = credentialprovider.AuthenticationSourceDriver
		klog.V(4).Infof("Set authenticationSource=driver for volume %s (no CSI secrets)", volumeID)
	}

	capacity := req.GetCapacityRange().GetRequiredBytes()
	if capacity == 0 {
		capacity = defaultVolumeCapacityBytes
	}

	klog.V(4).Infof("CreateVolume: successfully created volume %s", volumeID)
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumeID,
			CapacityBytes: capacity,
			VolumeContext: volumeContext,
		},
	}, nil
}

func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	klog.V(4).Infof("DeleteVolume: called with args: %s", protosanitizer.StripSecrets(req))

	if err := validateDeleteVolumeRequest(req); err != nil {
		klog.Errorf("DeleteVolume: invalid request: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volumeID := req.GetVolumeId()
	klog.V(4).Infof("DeleteVolume: processing volume %s", volumeID)

	// For dynamic provisioning, we need to resolve credentials to delete the bucket
	// Since we don't have volume context in DeleteVolumeRequest, we'll try both credential strategies

	// Try to get credentials - first attempt with driver credentials
	awsConfig, err := d.controllerCredProvider.ProvideForDeleteVolume(ctx, map[string]string{})
	if err != nil {
		klog.Errorf("DeleteVolume: failed to resolve credentials for volume %s: %v", volumeID, err)
		// Don't fail - CSI DeleteVolume should be idempotent
		klog.V(4).Infof("DeleteVolume: treating as successful due to credential resolution failure for volume %s", volumeID)
		return &csi.DeleteVolumeResponse{}, nil
	}

	// Create S3 client
	s3Client, err := d.createS3Client(ctx, &awsConfig)
	if err != nil {
		klog.Errorf("DeleteVolume: failed to create S3 client for volume %s: %v", volumeID, err)
		// Don't fail - CSI DeleteVolume should be idempotent
		klog.V(4).Infof("DeleteVolume: treating as successful due to S3 client creation failure for volume %s", volumeID)
		return &csi.DeleteVolumeResponse{}, nil
	}

	// Delete the bucket - S3 client handles all S3-specific logic
	if err := s3Client.DeleteBucket(ctx, volumeID); err != nil {
		klog.Errorf("DeleteVolume: bucket deletion failed for volume %s: %v", volumeID, err)
		// CSI DeleteVolume must be idempotent - always succeed even if underlying storage operation fails
		klog.V(4).Infof("DeleteVolume: treating as successful (CSI idempotency requirement)")
	}

	// CSI DeleteVolume is idempotent - always successful
	klog.V(4).Infof("DeleteVolume: successfully completed for volume %s", volumeID)

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
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	klog.V(4).Infof("ListVolumes: called with args %s", protosanitizer.StripSecrets(req))
	return nil, status.Error(codes.Unimplemented, "")
}

func (d *Driver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	klog.V(4).Infof("ValidateVolumeCapabilities: called with args %s", protosanitizer.StripSecrets(req))
	return &csi.ValidateVolumeCapabilitiesResponse{}, nil
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

func validateCreateVolumeRequest(req *csi.CreateVolumeRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.GetName() == "" {
		return fmt.Errorf("volume name is required")
	}

	// CSI spec requires volume capabilities to be provided
	if len(req.GetVolumeCapabilities()) == 0 {
		return fmt.Errorf("volume capabilities are required")
	}

	for _, cap := range req.GetVolumeCapabilities() {
		if cap.GetAccessMode() == nil {
			return fmt.Errorf("volume capability access mode is required")
		}
		mode := cap.GetAccessMode().GetMode()
		// S3 only supports multi-node access modes since it's object storage
		// Single-node modes don't make sense for S3
		if mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER ||
			mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY ||
			mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER ||
			mode == csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER {
			return fmt.Errorf("S3 volumes only support multi-node access modes, got %v", mode)
		}
	}
	return nil
}

func validateDeleteVolumeRequest(req *csi.DeleteVolumeRequest) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	if req.GetVolumeId() == "" {
		return fmt.Errorf("volume ID is required")
	}

	return nil
}

func (d *Driver) createS3Client(ctx context.Context, awsConfig *aws.Config) (s3client.Client, error) {
	// Check if there's a test factory function (for dependency injection in tests)
	if d.testS3ClientFactory != nil {
		return d.testS3ClientFactory(ctx, awsConfig)
	}

	// Get environment configuration for region and endpoint URL
	env := envprovider.Default()

	// Get endpoint URL from environment (from Helm chart configuration)
	endpointURL := env[envprovider.EnvEndpointURL]

	// Use region from the driver/credential provider configuration
	region := awsConfig.Region

	klog.V(4).Infof("Creating S3 client with region: %s, endpointURL: %s", region, endpointURL)

	s3Config := s3client.Config{
		Region:      region,
		EndpointURL: endpointURL,
		Credentials: awsConfig.Credentials,
	}

	return s3client.New(ctx, s3Config)
}

// resolveControllerCredentials resolves AWS credentials for controller operations from CSI request
// This handles the CSI specification requirement where the CSI provisioner resolves secrets
// and passes credential values in the secrets field of CreateVolumeRequest
func (d *Driver) resolveControllerCredentials(ctx context.Context, req *csi.CreateVolumeRequest, params *storageclass.Parameters) (aws.Config, error) {
	secrets := req.GetSecrets()

	// If CSI provisioner provided secrets (from provisioner-secret), use those
	if len(secrets) > 0 {
		klog.V(4).Infof("Using CSI provisioner secrets for CreateVolume (secret count: %d)", len(secrets))
		return d.createAWSConfigFromCSISecrets(ctx, secrets)
	}

	// Fallback to driver credentials if no CSI secrets provided
	klog.V(4).Infof("Using driver credentials for CreateVolume (no CSI secrets provided)")
	// Use empty params to trigger driver credential fallback in the credential provider
	emptyParams := &storageclass.Parameters{}
	return d.controllerCredProvider.ProvideForCreateVolume(ctx, emptyParams)
}

// createAWSConfigFromCSISecrets creates AWS config from CSI secrets passed by the provisioner
// The CSI provisioner resolves StorageClass secret parameters and passes the actual credential
// values in the secrets field of the CreateVolumeRequest
func (d *Driver) createAWSConfigFromCSISecrets(ctx context.Context, secrets map[string]string) (aws.Config, error) {
	// Extract standard AWS credential fields from CSI secrets
	accessKeyID, hasAccessKey := secrets[constants.AccessKeyIDField]
	secretAccessKey, hasSecretKey := secrets[constants.SecretAccessKeyField]

	if !hasAccessKey || !hasSecretKey {
		return aws.Config{}, fmt.Errorf("CSI secrets missing required AWS credentials: access_key_id=%v, secret_access_key=%v", hasAccessKey, hasSecretKey)
	}

	if accessKeyID == "" || secretAccessKey == "" {
		return aws.Config{}, fmt.Errorf("CSI secrets contain empty AWS credentials")
	}

	// Optional session token for temporary credentials
	sessionToken := secrets[constants.SessionTokenField] // empty if not present

	// Optional region override
	region := secrets[constants.RegionField] // empty if not present

	// Create static credential provider from CSI secrets
	credsProvider := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)

	// Load base config with static credentials
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create AWS config from CSI secrets: %w", err)
	}

	// Override region if provided in secrets
	if region != "" {
		cfg.Region = region
	}

	klog.V(4).Infof("Created AWS config from CSI secrets: region=%s, hasSessionToken=%v", cfg.Region, sessionToken != "")
	return cfg, nil
}

func generateVolumeID() string {
	return fmt.Sprintf("csi-s3-%s", uuid.NewString())
}
