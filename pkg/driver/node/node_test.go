package node_test

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/mock/gomock"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter"
	mock_driver "github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter/mocks"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
)

type nodeServerTestEnv struct {
	mockCtl     *gomock.Controller
	mockMounter *mock_driver.MockMounter
	server      *node.S3NodeServer
}

func initNodeServerTestEnv(t *testing.T) *nodeServerTestEnv {
	mockCtl := gomock.NewController(t)
	mockMounter := mock_driver.NewMockMounter(mockCtl)
	server := node.NewS3NodeServer("test-nodeID", mockMounter)
	return &nodeServerTestEnv{
		mockCtl:     mockCtl,
		mockMounter: mockMounter,
		server:      server,
	}
}

func TestNodePublishVolume(t *testing.T) {
	var (
		volumeId   = "test-volume-id"
		bucketName = "test-bucket-name"
		stdVolCap  = &csi.VolumeCapability{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
			},
		}
		targetPath = "/target/path"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: normal mount",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId:         volumeId,
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
					VolumeContext:    map[string]string{"bucketName": bucketName},
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Any())
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: reader only volume access type",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY,
						},
					},
					TargetPath:    targetPath,
					VolumeContext: map[string]string{"bucketName": bucketName},
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--read-only", "--force-path-style"})))
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: mount with mount options and read only",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags: []string{"foo", "bar", "--test 123"},
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					TargetPath:    targetPath,
					VolumeContext: map[string]string{"bucketName": bucketName},
					Readonly:      true,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--bar", "--foo", "--read-only", "--test=123", "--force-path-style"})))
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: foreground option is removed",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags: []string{"--foreground", "-f", "--test 123"},
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
					Readonly:      true,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--read-only", "--test=123", "--force-path-style"}))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "fail: missing volume id",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeCapability: stdVolCap,
					TargetPath:       targetPath,
					VolumeContext:    map[string]string{"bucketName": bucketName},
				}

				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}
				nodeTestEnv.mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodePublishVolumeForPodMounter(t *testing.T) {
	t.Setenv("MOUNTER_KIND", "pod")
	var (
		volumeId   = "test-volume-id"
		bucketName = "test-bucket-name"
		targetPath = "/target/path"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: sets gid, allow-other, dir-mode, file-mode flags if fsGroup is provided",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags:       []string{},
								VolumeMountGroup: "123",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--gid=123", "--allow-other", "--dir-mode=770", "--file-mode=660", "--force-path-style"}))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: sets gid, allow-other, dir-mode, file-mode flags if fsGroup is provided and allow-other flag is provided in mountOptions",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags:       []string{"--allow-other"},
								VolumeMountGroup: "123",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--gid=123", "--allow-other", "--dir-mode=770", "--file-mode=660", "--force-path-style"}))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: sets only allow-root flag if fsGroup is empty string and allow-other flag is not provided in mountOptions",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags:       []string{},
								VolumeMountGroup: "",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--allow-root", "--force-path-style"}))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: does not set allow-root flag if fsGroup is empty string and allow-other flag is provided in mountOptions",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags:       []string{"--allow-other"},
								VolumeMountGroup: "",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs([]string{"--allow-other", "--force-path-style"}))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: uses gid, allow-other, dir-mode, file-mode from mountOptions if fsGroup is set and these flags are provided in mountOptions",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				mountFlags := []string{"--gid 456", "--allow-other", "--dir-mode=555", "--file-mode=444", "--force-path-style"}
				req := &csi.NodePublishVolumeRequest{
					VolumeId: volumeId,
					VolumeCapability: &csi.VolumeCapability{
						AccessType: &csi.VolumeCapability_Mount{
							Mount: &csi.VolumeCapability_MountVolume{
								MountFlags:       mountFlags,
								VolumeMountGroup: "123",
							},
						},
						AccessMode: &csi.VolumeCapability_AccessMode{
							Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
						},
					},
					VolumeContext: map[string]string{"bucketName": bucketName},
					TargetPath:    targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().Mount(
					gomock.Eq(context.Background()),
					gomock.Eq(bucketName),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.ProvideContext{
						VolumeID: volumeId,
					}),
					gomock.Eq(mountpoint.ParseArgs(mountFlags))).Return(nil)
				_, err := nodeTestEnv.server.NodePublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume is failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeUnpublishVolume(t *testing.T) {
	var (
		volumeId   = "test-volume-id"
		targetPath = "/target/path"
	)
	testCases := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "success: happy path",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().IsMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				nodeTestEnv.mockMounter.EXPECT().Unmount(gomock.Eq(ctx), gomock.Eq(targetPath), gomock.Any())
				_, err := nodeTestEnv.server.NodeUnpublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: not mounted",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().IsMountPoint(gomock.Eq(targetPath)).Return(false, nil)
				_, err := nodeTestEnv.server.NodeUnpublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "failure: unmount failure is error",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				nodeTestEnv.mockMounter.EXPECT().IsMountPoint(gomock.Eq(targetPath)).Return(true, nil)
				nodeTestEnv.mockMounter.EXPECT().Unmount(
					gomock.Eq(ctx),
					gomock.Eq(targetPath),
					gomock.Eq(credentialprovider.CleanupContext{
						VolumeID: volumeId,
					}),
				).Return(errors.New(""))
				_, err := nodeTestEnv.server.NodeUnpublishVolume(ctx, req)
				if err == nil {
					t.Fatalf("NodePublishVolume must fail")
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
		{
			name: "success: inexistent dir",
			testFunc: func(t *testing.T) {
				nodeTestEnv := initNodeServerTestEnv(t)
				ctx := context.Background()
				req := &csi.NodeUnpublishVolumeRequest{
					VolumeId:   volumeId,
					TargetPath: targetPath,
				}

				expectedError := fs.ErrNotExist
				nodeTestEnv.mockMounter.EXPECT().IsMountPoint(gomock.Eq(targetPath)).Return(false, expectedError)
				_, err := nodeTestEnv.server.NodeUnpublishVolume(ctx, req)
				if err != nil {
					t.Fatalf("NodePublishVolume failed: %v", err)
				}

				nodeTestEnv.mockCtl.Finish()
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, tc.testFunc)
	}
}

func TestNodeGetCapabilitiesForSystemd(t *testing.T) {
	nodeTestEnv := initNodeServerTestEnv(t)
	ctx := context.Background()
	req := &csi.NodeGetCapabilitiesRequest{}

	resp, err := nodeTestEnv.server.NodeGetCapabilities(ctx, req)
	if err != nil {
		t.Fatalf("NodeGetCapabilities failed: %v", err)
	}

	capabilities := resp.GetCapabilities()
	if len(capabilities) != 0 {
		t.Fatalf("NodeGetCapabilities failed: capabilities not empty")
	}

	nodeTestEnv.mockCtl.Finish()
}

func TestNodeGetCapabilitiesForPodMounter(t *testing.T) {
	t.Setenv("MOUNTER_KIND", "pod")
	nodeTestEnv := initNodeServerTestEnv(t)
	ctx := context.Background()
	req := &csi.NodeGetCapabilitiesRequest{}

	resp, err := nodeTestEnv.server.NodeGetCapabilities(ctx, req)
	if err != nil {
		t.Fatalf("NodeGetCapabilities failed: %v", err)
	}

	assert.Equals(t, []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_VOLUME_MOUNT_GROUP,
				},
			},
		},
	}, resp.GetCapabilities())

	nodeTestEnv.mockCtl.Finish()
}

var _ mounter.Mounter = &dummyMounter{}

type dummyMounter struct{}

func (d *dummyMounter) Mount(ctx context.Context, bucketName string, target string, provideCtx credentialprovider.ProvideContext, args mountpoint.Args) error {
	return nil
}

func (d *dummyMounter) Unmount(ctx context.Context, target string, cleanupCtx credentialprovider.CleanupContext) error {
	return nil
}

func (d *dummyMounter) IsMountPoint(target string) (bool, error) {
	return true, nil
}
