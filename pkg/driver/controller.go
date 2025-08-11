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

	params, err := storageclass.ParseAndValidate(req.GetParameters())
	if err != nil {
		klog.Errorf("CreateVolume: failed to parse StorageClass parameters: %v", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to parse StorageClass parameters: %v", err))
	}

	volumeID := generateVolumeID()
	klog.V(4).Infof("Generated volume ID: %s", volumeID)

	// Controller Credential Resolution for Bucket Operations
	//
	// The controller handles bucket creation/deletion operations and uses a separate
	// credential resolution strategy from the node operations. This allows proper
	// separation of concerns where the controller can have administrative permissions
	// for bucket management while nodes have limited permissions for mounting.
	//
	// Controller Credential Resolution Order:
	// 1. provisioner-secret: If specified in StorageClass, controller uses this secret
	//    for bucket operations (e.g., CreateBucket, DeleteBucket)
	// 2. driver credentials: If no provisioner-secret, controller uses driver-level
	//    credentials for bucket operations
	//
	// Note: This is independent of node-publish-secret, which only affects what
	// credentials the node uses for mounting operations. The two-stage approach enables:
	// - Admin credentials for bucket management (controller)
	// - Read/write credentials for data access (node)
	// - Proper security boundaries between management and access operations

	awsConfig, err := d.controllerCredProvider.ProvideForCreateVolume(ctx, params)
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
	// This implements a credential hierarchy that determines what credentials
	// the node should use for mounting operations. The controller may use different
	// credentials (provisioner-secret) for bucket creation than what the node uses
	// for mounting.
	//
	// Credential Resolution Order:
	// 1. node-publish-secret: If specified in StorageClass, the node uses this secret
	//    for mount operations (highest priority)
	// 2. provisioner-secret: If no node-publish-secret but provisioner-secret exists,
	//    the node falls back to using the provisioner-secret for mount operations
	// 3. driver credentials: If no secrets are specified, both controller and node
	//    use the driver-level credentials
	//
	// This design allows flexible credential management:
	// - Separate credentials for management (controller) vs access (node)
	// - Single secret can serve both roles when only one is provided
	// - Seamless fallback to driver credentials when no secrets are configured
	//
	// The authenticationSource field tells the node's credential provider which
	// credentials to use, ensuring proper authentication during volume mount.

	if params.HasNodePublishSecret() {
		// Priority 1: node-publish-secret
		volumeContext[volumecontext.AuthenticationSource] = credentialprovider.AuthenticationSourceSecret
		volumeContext[constants.VolumeContextNodePublishSecretNameKey] = params.NodePublishSecretName
		volumeContext[constants.VolumeContextNodePublishSecretNamespaceKey] = params.NodePublishSecretNamespace
	} else if params.HasProvisionerSecret() {
		// Priority 2: provisioner-secret as fallback
		volumeContext[volumecontext.AuthenticationSource] = credentialprovider.AuthenticationSourceSecret
		volumeContext[constants.VolumeContextNodePublishSecretNameKey] = params.ProvisionerSecretName
		volumeContext[constants.VolumeContextNodePublishSecretNamespaceKey] = params.ProvisionerSecretNamespace
	} else {
		// Priority 3: driver credentials
		volumeContext[volumecontext.AuthenticationSource] = credentialprovider.AuthenticationSourceDriver
	}

	// Always store provisioner secret info if present (for reference/debugging)
	if params.HasProvisionerSecret() {
		volumeContext[constants.VolumeContextProvisionerSecretNameKey] = params.ProvisionerSecretName
		volumeContext[constants.VolumeContextProvisionerSecretNamespaceKey] = params.ProvisionerSecretNamespace
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

func generateVolumeID() string {
	return fmt.Sprintf("csi-s3-%s", uuid.NewString())
}
