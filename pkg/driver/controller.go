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

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/constants"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/storageclass"
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

	creds, err := d.controllerCredProvider.ProvideForCreateVolume(ctx, params)
	if err != nil {
		klog.Errorf("CreateVolume: failed to resolve credentials for volume %s: %v", volumeID, err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve credentials: %v", err))
	}

	klog.V(4).Infof("Resolved credentials for volume %s using authentication tier: %s", volumeID, params.AuthTier)
	_ = creds // Credentials resolved but not used right now, we will use them once we implement the actual bucket creation

	volumeContext := map[string]string{
		"dynamicProvisioning": "true",
		"bucketName":          volumeID,
	}

	if params.HasProvisionerSecret() {
		volumeContext[constants.VolumeContextProvisionerSecretNameKey] = params.ProvisionerSecretName
		volumeContext[constants.VolumeContextProvisionerSecretNamespaceKey] = params.ProvisionerSecretNamespace
	}

	if params.HasNodePublishSecret() {
		volumeContext["node-publish-secret-name"] = params.NodePublishSecretName
		volumeContext["node-publish-secret-namespace"] = params.NodePublishSecretNamespace
	}

	capacity := req.GetCapacityRange().GetRequiredBytes()
	if capacity == 0 {
		capacity = defaultVolumeCapacityBytes
	}

	klog.V(4).Infof("CreateVolume: successfully created volume %s with metadata only (no bucket created)", volumeID)
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

	// TODO: S3CSI-142 - Implement real S3 volume deletion
	// This will include:
	// 1. Validate volume ID and parse bucket information
	// 2. Safely delete S3 bucket only if empty
	// 3. Clean up bucket policies and access credentials
	// 4. Handle idempotent deletion of non-existent volumes
	return nil, status.Error(codes.Unimplemented, "DeleteVolume will be implemented in S3CSI-142")
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

	if len(req.GetVolumeCapabilities()) > 0 {
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
	}

	return nil
}

func generateVolumeID() string {
	return fmt.Sprintf("csi-s3-%s", uuid.NewString())
}
