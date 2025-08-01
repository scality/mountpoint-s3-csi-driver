package mounter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter"
	mock_driver "github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter/mocks"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/system"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
	"k8s.io/mount-utils"
)

type mounterTestEnv struct {
	ctx        context.Context
	mockCtl    *gomock.Controller
	mockRunner *mock_driver.MockServiceRunner
	mounter    *mounter.SystemdMounter
}

func initMounterTestEnv(t *testing.T) *mounterTestEnv {
	// Set test environment variables for static credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "TESTKEY123456789")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "TESTSECRET123456789ABCDEFGHIJKLMNOPQRSTUV")

	ctx := context.Background()
	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()
	mockRunner := mock_driver.NewMockServiceRunner(mockCtl)
	mountpointVersion := "TEST_MP_VERSION-v1.1"

	// Create a basic SystemdMounter without setting the unexported credProvider field
	// Instead, we'll mock the service runner responses directly in our tests
	sysMount := &mounter.SystemdMounter{
		Runner:      mockRunner,
		Mounter:     mount.NewFakeMounter(nil),
		MpVersion:   mountpointVersion,
		MountS3Path: mounter.MountS3Path(),
	}

	return &mounterTestEnv{
		ctx:        ctx,
		mockCtl:    mockCtl,
		mockRunner: mockRunner,
		mounter:    sysMount,
	}
}

func TestS3MounterMount(t *testing.T) {
	const testBucketName = "test-bucket"

	// Use a temp directory for the target path
	testTargetPath := filepath.Join(t.TempDir(), "mount")

	testProvideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
		PodID:                "test-pod",
		VolumeID:             "test-volume",
		// We'll specify the actual WritePath and EnvPath in each test case
	}

	testCases := []struct {
		name        string
		bucketName  string
		targetPath  string
		provideCtx  credentialprovider.ProvideContext
		options     []string
		expectedErr bool
		before      func(*testing.T, *mounterTestEnv)
	}{
		{
			name:       "success: mounts with empty options",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: testProvideCtx,
			options:    []string{},
		},
		{
			name:       "success: mounts with nil credentials",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).Return("success", nil)
			},
		},
		{
			name:       "success: replaces user agent prefix",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--user-agent-prefix=mycustomuseragent"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					for _, a := range config.Args {
						if strings.Contains(a, "mycustomuseragent") {
							t.Fatal("Bad user agent")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "success: aws max attempts",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--aws-max-attempts=10"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					if slices.Contains(config.Env, "AWS_MAX_ATTEMPTS=10") {
						return "success", nil
					}
					t.Fatal("Bad env")
					return "", nil
				})
			},
		},
		{
			name:       "success: driver environment s3 endpoint url",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--aws-max-attempts=10"},
			before: func(t *testing.T, env *mounterTestEnv) {
				// Set AWS_ENDPOINT_URL in the environment
				t.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com:8000")

				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify that the environment variable is passed to mountpoint-s3
					endpointPassed := false
					for _, envVar := range config.Env {
						if envVar == "AWS_ENDPOINT_URL=https://s3.example.com:8000" {
							endpointPassed = true
							break
						}
					}

					if !endpointPassed {
						t.Fatal("Driver level AWS_ENDPOINT_URL should be passed to mountpoint-s3")
					}

					return "success", nil
				})
			},
		},
		{
			name:       "success: always removes endpoint-url from options for security",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--endpoint-url=https://malicious-endpoint.example.com"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the endpoint URL is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--endpoint-url") {
							t.Fatal("endpoint-url should be removed from mount options for security")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:        "failure: fails on mount failure",
			bucketName:  testBucketName,
			targetPath:  testTargetPath,
			provideCtx:  credentialprovider.ProvideContext{},
			options:     []string{},
			expectedErr: true,
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).Return("fail", errors.New("test failure"))
			},
		},
		{
			name:        "failure: won't mount empty bucket name",
			targetPath:  testTargetPath,
			provideCtx:  testProvideCtx,
			options:     []string{},
			expectedErr: true,
		},
		{
			name:        "failure: won't mount empty target",
			bucketName:  testBucketName,
			provideCtx:  testProvideCtx,
			options:     []string{},
			expectedErr: true,
		},
		{
			name:       "security: both driver and mount options endpoint URLs - driver takes precedence",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--endpoint-url=https://malicious-endpoint.example.com"},
			before: func(t *testing.T, env *mounterTestEnv) {
				// Set AWS_ENDPOINT_URL in the environment
				t.Setenv("AWS_ENDPOINT_URL", "https://s3.example.com:8000")

				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the endpoint URL is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--endpoint-url") {
							t.Fatal("endpoint-url should be removed from mount options for security")
						}
					}

					// Verify the environment variable is passed through
					endpointPassed := false
					trustedEndpoint := false
					for _, envVar := range config.Env {
						if strings.HasPrefix(envVar, "AWS_ENDPOINT_URL=") {
							endpointPassed = true
							if envVar == "AWS_ENDPOINT_URL=https://s3.example.com:8000" {
								trustedEndpoint = true
							}
						}
					}

					if !endpointPassed {
						t.Fatal("Driver level AWS_ENDPOINT_URL should be passed to mountpoint-s3")
					}

					if !trustedEndpoint {
						t.Fatal("Driver level AWS_ENDPOINT_URL should take precedence over PV-level endpoint")
					}

					return "success", nil
				})
			},
		},
		{
			name:       "security: endpoint URL with space separator is removed",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			// Using space separator instead of equals
			options: []string{"--endpoint-url https://malicious-endpoint.example.com"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the endpoint URL is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--endpoint-url") {
							t.Fatal("endpoint-url should be removed from mount options for security regardless of format")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "security: endpoint URL without -- prefix is removed",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			// Without -- prefix
			options: []string{"endpoint-url=https://malicious-endpoint.example.com"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the endpoint URL is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--endpoint-url") || strings.Contains(arg, "endpoint-url") {
							t.Fatal("endpoint-url should be removed from mount options for security regardless of format")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "Mount arg policy: strips --cache-xz flag",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--cache-xz"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the cache-xz option is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--cache-xz") {
							t.Fatal("cache-xz should be removed from mount options due to policy")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "Mount arg policy: strips --incremental-upload flag",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--incremental-upload"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the incremental-upload option is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--incremental-upload") {
							t.Fatal("incremental-upload should be removed from mount options due to policy")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "Mount arg policy: strips --storage-class flag",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--storage-class=REDUCED_REDUNDANCY"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the storage-class option is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--storage-class") {
							t.Fatal("storage-class should be removed from mount options due to policy")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "Mount arg policy: strips --profile flag",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--profile=my-aws-profile"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify the profile option is not in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--profile") {
							t.Fatal("profile should be removed from mount options due to policy")
						}
					}
					return "success", nil
				})
			},
		},
		{
			name:       "Mount arg policy: strips multiple disallowed flags",
			bucketName: testBucketName,
			targetPath: testTargetPath,
			provideCtx: credentialprovider.ProvideContext{},
			options:    []string{"--cache-xz", "--incremental-upload", "--storage-class=STANDARD", "-o"},
			before: func(t *testing.T, env *mounterTestEnv) {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, config *system.ExecConfig) (string, error) {
					// Verify none of the policy-disallowed options are in the command-line arguments
					for _, arg := range config.Args {
						if strings.Contains(arg, "--cache-xz") ||
							strings.Contains(arg, "--incremental-upload") ||
							strings.Contains(arg, "--storage-class") ||
							strings.Contains(arg, "--profile") ||
							strings.Contains(arg, "-o") {
							t.Fatal("policy-disallowed options should be removed from mount options")
						}
					}
					return "success", nil
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := initMounterTestEnv(t)

			// Create a temp directory for credentials
			credentialsDir := t.TempDir()

			// Make a copy of the provide context with the proper paths
			provideCtx := tc.provideCtx
			provideCtx.WritePath = credentialsDir
			provideCtx.EnvPath = credentialsDir

			// Mock the credential provider behavior by directly setting up expectations on mockRunner
			// For test cases expected to succeed, set up an expectation on StartService
			if !tc.expectedErr && tc.bucketName != "" && tc.targetPath != "" && tc.before == nil {
				env.mockRunner.EXPECT().StartService(gomock.Any(), gomock.Any()).Return("success", nil)
			}

			if tc.before != nil {
				tc.before(t, env)
			}

			err := env.mounter.Mount(env.ctx, tc.bucketName, tc.targetPath, provideCtx, mountpoint.ParseArgs(tc.options))

			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

// TestNewSystemdMounter tests the SystemdMounter constructor error handling
// This test focuses on verifying proper error handling when SystemD is unavailable
func TestNewSystemdMounter(t *testing.T) {
	credProvider := &credentialprovider.Provider{}
	mpVersion := "1.0.0"
	kubernetesVersion := "1.28.0"

	mounter, err := mounter.NewSystemdMounter(credProvider, mpVersion, kubernetesVersion)

	if err != nil {
		// On systems without SystemD, verify we get a proper error
		t.Logf("SystemdMounter creation failed (expected on non-SystemD systems): %v", err)

		// Verify error message indicates SystemD runner failure (not generic error)
		expectedText := "failed to start systemd runner"
		if !strings.Contains(err.Error(), expectedText) {
			t.Errorf("Expected error to contain %q, got: %v", expectedText, err)
		}

		// Verify mounter is nil on error
		if mounter != nil {
			t.Errorf("Expected nil mounter on error, got: %v", mounter)
		}
	} else {
		// On systems with SystemD, verify proper initialization
		t.Log("SystemdMounter created successfully - SystemD is available")

		if mounter == nil {
			t.Errorf("Expected non-nil mounter")
			return
		}

		// Verify proper field initialization
		if mounter.MpVersion != mpVersion {
			t.Errorf("Expected MpVersion %s, got %s", mpVersion, mounter.MpVersion)
		}

		if mounter.Runner == nil {
			t.Errorf("Expected non-nil Runner")
		}

		if mounter.Mounter == nil {
			t.Errorf("Expected non-nil Mounter")
		}
	}
}

func TestIsMountPoint(t *testing.T) {
	testDir := t.TempDir()
	mountpointS3MountPath := filepath.Join(testDir, "/var/lib/kubelet/pods/46efe8aa-75d9-4b12-8fdd-0ce0c2cabd99/volumes/kubernetes.io~csi/s3-mp-csi-pv/mount")
	tmpFsMountPath := filepath.Join(testDir, "/var/lib/kubelet/pods/3af4cdb5-6131-4d4b-bed3-4b7a74d357e4/volumes/kubernetes.io~projected/kube-api-access-tmxk4")
	testProcMountsContent := []mount.MountPoint{
		{
			Device: "proc",
			Path:   "/proc",
			Type:   "proc",
			Opts:   []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
			Freq:   0,
			Pass:   0,
		},
		{
			Device: "sysfs",
			Path:   "/sys",
			Type:   "sysfs",
			Opts:   []string{"rw", "seclabel", "nosuid", "nodev", "noexec", "relatime"},
			Freq:   0,
			Pass:   0,
		},
		{
			Device: "tmpfs",
			Path:   tmpFsMountPath,
			Type:   "tmpfs",
			Opts:   []string{"rw", "seclabel", "relatime", "size=3364584k"},
			Freq:   0,
			Pass:   0,
		},
		{
			Device: "mountpoint-s3",
			Path:   mountpointS3MountPath,
			Type:   "fuse",
			Opts:   []string{"rw", "nosuid", "nodev", "noatime", "user_id=0", "group_id=0", "default_permissions"},
			Freq:   0,
			Pass:   0,
		},
	}

	_ = os.MkdirAll(tmpFsMountPath, 0o755)
	_ = os.MkdirAll(mountpointS3MountPath, 0o755)

	tests := map[string]struct {
		procMountsContent []mount.MountPoint
		target            string
		isMountPoint      bool
		expectErr         bool
	}{
		"mountpoint-s3 mount": {
			procMountsContent: testProcMountsContent,
			target:            mountpointS3MountPath,
			isMountPoint:      true,
			expectErr:         false,
		},
		"tmpfs mount": {
			procMountsContent: testProcMountsContent,
			target:            tmpFsMountPath,
			isMountPoint:      false,
			expectErr:         false,
		},
		"non existing mount on /proc/mounts": {
			procMountsContent: testProcMountsContent[:2],
			target:            mountpointS3MountPath,
			isMountPoint:      false,
			expectErr:         false,
		},
		"non existing mount on filesystem": {
			procMountsContent: testProcMountsContent,
			target:            "/var/lib/kubelet/pods/46efe8aa-75d9-4b12-8fdd-0ce0c2cabd99/volumes/kubernetes.io~csi/s3-mp-csi-pv/mount",
			isMountPoint:      false,
			expectErr:         true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mounter := &mounter.SystemdMounter{Mounter: mount.NewFakeMounter(test.procMountsContent)}
			isMountPoint, err := mounter.IsMountPoint(test.target)
			assert.Equals(t, test.isMountPoint, isMountPoint)
			assert.Equals(t, test.expectErr, err != nil)
		})
	}
}
