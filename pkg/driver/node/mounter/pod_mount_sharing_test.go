package mounter_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/mounter/mountertest"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/mountpoint"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/podmounter/mppod"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

func TestPodMountSharing(t *testing.T) {
	t.Run("Source/Bind Mount Pattern", func(t *testing.T) {
		t.Run("Mounts to source directory first, then bind mounts to target", func(t *testing.T) {
			testCtx := setup(t)

			// Track mount operations
			var mountOperations []string
			var bindMountCalled bool

			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				mountOperations = append(mountOperations, "mount:"+target)
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			// Create a custom PodMounter with bind mount tracking
			bindMountSyscall := func(source, target string) error {
				bindMountCalled = true
				mountOperations = append(mountOperations, "bind:"+source+"->"+target)
				return testCtx.mount.Mount(source, target, "", []string{"bind"})
			}

			podMounter, err := mounter.NewPodMounter(
				testCtx.podMounter.GetPodWatcher(),
				testCtx.podMounter.GetCredentialProvider(),
				testCtx.mount,
				testCtx.mountSyscall,
				bindMountSyscall,
				testCtx.kubeletPath,
				nil,
			)
			assert.NoError(t, err)

			mpPodName := mppod.MountpointPodNameFor(testCtx.podUID, testCtx.pvName)
			expectedSource := filepath.Join(mounter.SourceMountDir(testCtx.kubeletPath), mpPodName)

			go func() {
				mpPod := createMountpointPod(testCtx)
				mpPod.run()
				mpPod.receiveMountOptions(testCtx.ctx)
			}()

			err = podMounter.Mount(testCtx.ctx, testCtx.bucketName, testCtx.targetPath, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			}, mountpoint.ParseArgs(nil), "")
			assert.NoError(t, err)

			// Verify mount operations occurred in correct order
			assert.Equals(t, 2, len(mountOperations))
			assert.Equals(t, "mount:"+expectedSource, mountOperations[0])
			assert.Equals(t, "bind:"+expectedSource+"->"+testCtx.targetPath, mountOperations[1])
			assert.Equals(t, true, bindMountCalled)

			// Verify both source and target are mount points
			isSourceMount, err := testCtx.mount.IsMountPoint(expectedSource)
			assert.NoError(t, err)
			assert.Equals(t, true, isSourceMount)

			isTargetMount, err := testCtx.mount.IsMountPoint(testCtx.targetPath)
			assert.NoError(t, err)
			assert.Equals(t, true, isTargetMount)
		})

		t.Run("Multiple containers share same source mount", func(t *testing.T) {
			testCtx := setup(t)

			mountCount := 0
			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				mountCount++
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			// Setup multiple target paths for same volume
			target1 := testCtx.targetPath
			podUID2 := uuid.New().String()
			target2 := filepath.Join(
				testCtx.kubeletPath,
				"pods",
				podUID2,
				"volumes/kubernetes.io~csi",
				testCtx.pvName,
				"mount",
			)
			err := os.MkdirAll(filepath.Dir(target2), 0o750)
			assert.NoError(t, err)

			go func() {
				mpPod := createMountpointPod(testCtx)
				mpPod.run()
				// Receive mount options twice
				mpPod.receiveMountOptions(testCtx.ctx)
				mpPod.receiveMountOptions(testCtx.ctx)
			}()

			// First mount
			err = testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, target1, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			}, mountpoint.ParseArgs(nil), "")
			assert.NoError(t, err)

			// Second mount to different target (same source)
			err = testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, target2, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    podUID2,
			}, mountpoint.ParseArgs(nil), "")
			assert.NoError(t, err)

			// Should only mount to source once
			assert.Equals(t, 1, mountCount)

			// Both targets should be mount points
			isTarget1Mount, err := testCtx.mount.IsMountPoint(target1)
			assert.NoError(t, err)
			assert.Equals(t, true, isTarget1Mount)

			isTarget2Mount, err := testCtx.mount.IsMountPoint(target2)
			assert.NoError(t, err)
			assert.Equals(t, true, isTarget2Mount)

			// Verify source is mounted only once
			mpPodName := mppod.MountpointPodNameFor(testCtx.podUID, testCtx.pvName)
			source := filepath.Join(mounter.SourceMountDir(testCtx.kubeletPath), mpPodName)
			isSourceMount, err := testCtx.mount.IsMountPoint(source)
			assert.NoError(t, err)
			assert.Equals(t, true, isSourceMount)
		})

		t.Run("Unmount only removes bind mount, not source", func(t *testing.T) {
			testCtx := setup(t)

			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			mpPodName := mppod.MountpointPodNameFor(testCtx.podUID, testCtx.pvName)
			source := filepath.Join(mounter.SourceMountDir(testCtx.kubeletPath), mpPodName)

			go func() {
				mpPod := createMountpointPod(testCtx)
				mpPod.run()
				mpPod.receiveMountOptions(testCtx.ctx)
			}()

			// Mount
			err := testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, testCtx.targetPath, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			}, mountpoint.ParseArgs(nil), "")
			assert.NoError(t, err)

			// Verify both are mounted
			isSourceMount, err := testCtx.mount.IsMountPoint(source)
			assert.NoError(t, err)
			assert.Equals(t, true, isSourceMount)

			isTargetMount, err := testCtx.mount.IsMountPoint(testCtx.targetPath)
			assert.NoError(t, err)
			assert.Equals(t, true, isTargetMount)

			// Unmount target
			err = testCtx.podMounter.Unmount(testCtx.ctx, testCtx.targetPath, credentialprovider.CleanupContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			})
			assert.NoError(t, err)

			// Target should be unmounted
			isTargetMount, err = testCtx.mount.IsMountPoint(testCtx.targetPath)
			assert.NoError(t, err)
			assert.Equals(t, false, isTargetMount)

			// Source should still be mounted (for other potential containers)
			isSourceMount, err = testCtx.mount.IsMountPoint(source)
			assert.NoError(t, err)
			assert.Equals(t, true, isSourceMount)
		})

		t.Run("FSGroup filtering for pod sharing", func(t *testing.T) {
			testCtx := setup(t)

			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			fsGroup1 := "1000"
			fsGroup2 := "2000"

			// Create two different targets with different FSGroups
			target1 := testCtx.targetPath
			podUID2 := uuid.New().String()
			target2 := filepath.Join(
				testCtx.kubeletPath,
				"pods",
				podUID2,
				"volumes/kubernetes.io~csi",
				testCtx.pvName,
				"mount",
			)
			err := os.MkdirAll(filepath.Dir(target2), 0o750)
			assert.NoError(t, err)

			mountOptions1 := make(chan mountpoint.Args, 1)
			mountOptions2 := make(chan mountpoint.Args, 1)

			go func() {
				// First mount with FSGroup 1000
				mpPod1 := createMountpointPod(testCtx)
				mpPod1.run()
				opts1 := mpPod1.receiveMountOptions(testCtx.ctx)
				// Extract mount args from options
				args1 := mountpoint.ParseArgs(opts1.Args)
				mountOptions1 <- args1

				// Second mount with FSGroup 2000 (different pod)
				testCtx.podUID = podUID2
				mpPod2 := createMountpointPod(testCtx)
				mpPod2.run()
				opts2 := mpPod2.receiveMountOptions(testCtx.ctx)
				args2 := mountpoint.ParseArgs(opts2.Args)
				mountOptions2 <- args2
			}()

			// First mount with FSGroup 1000
			err = testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, target1, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			}, mountpoint.ParseArgs([]string{"--gid=1000"}), fsGroup1)
			assert.NoError(t, err)

			// Second mount with FSGroup 2000
			err = testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, target2, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    podUID2,
			}, mountpoint.ParseArgs([]string{"--gid=2000"}), fsGroup2)
			assert.NoError(t, err)

			// Verify different FSGroups resulted in different mount arguments
			args1 := <-mountOptions1
			args2 := <-mountOptions2

			gid1, ok1 := args1.Value("--gid")
			assert.Equals(t, true, ok1)
			assert.Equals(t, "1000", gid1)

			gid2, ok2 := args2.Value("--gid")
			assert.Equals(t, true, ok2)
			assert.Equals(t, "2000", gid2)
		})

		t.Run("Source mount reused if already mounted", func(t *testing.T) {
			testCtx := setup(t)

			mountCalls := 0
			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				mountCalls++
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			mpPodName := mppod.MountpointPodNameFor(testCtx.podUID, testCtx.pvName)
			source := filepath.Join(mounter.SourceMountDir(testCtx.kubeletPath), mpPodName)

			// Pre-mount the source
			err := testCtx.mount.Mount("mountpoint-s3", source, "fuse", nil)
			assert.NoError(t, err)

			go func() {
				mpPod := createMountpointPod(testCtx)
				mpPod.run()
				// Note: receiveMountOptions is NOT called since source is already mounted
				// The implementation should detect this and skip the mount
			}()

			// Mount should detect source is already mounted and skip to bind mount
			err = testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, testCtx.targetPath, credentialprovider.ProvideContext{
				VolumeID: testCtx.volumeID,
				PodID:    testCtx.podUID,
			}, mountpoint.ParseArgs(nil), "")
			assert.NoError(t, err)

			// Should not have called mountSyscall since source was pre-mounted
			assert.Equals(t, 0, mountCalls)

			// Both source and target should be mounted
			isSourceMount, err := testCtx.mount.IsMountPoint(source)
			assert.NoError(t, err)
			assert.Equals(t, true, isSourceMount)

			isTargetMount, err := testCtx.mount.IsMountPoint(testCtx.targetPath)
			assert.NoError(t, err)
			assert.Equals(t, true, isTargetMount)
		})

		t.Run("Concurrent mount operations are serialized", func(t *testing.T) {
			testCtx := setup(t)

			mountStarted := make(chan string, 10)
			mountCompleted := make(chan string, 10)

			testCtx.mountSyscall = func(target string, args mountpoint.Args) (fd int, err error) {
				mountStarted <- target
				time.Sleep(50 * time.Millisecond) // Simulate slow mount
				_ = testCtx.mount.Mount("mountpoint-s3", target, "fuse", nil)
				mountCompleted <- target
				return int(mountertest.OpenDevNull(t).Fd()), nil
			}

			// Create multiple targets
			numTargets := 3
			targets := make([]string, numTargets)
			podUIDs := make([]string, numTargets)
			for i := 0; i < numTargets; i++ {
				podUIDs[i] = uuid.New().String()
				targets[i] = filepath.Join(
					testCtx.kubeletPath,
					"pods",
					podUIDs[i],
					"volumes/kubernetes.io~csi",
					testCtx.pvName,
					"mount",
				)
				err := os.MkdirAll(filepath.Dir(targets[i]), 0o750)
				assert.NoError(t, err)
			}

			go func() {
				mpPod := createMountpointPod(testCtx)
				mpPod.run()
				// Receive mount options for all targets
				for i := 0; i < numTargets; i++ {
					mpPod.receiveMountOptions(testCtx.ctx)
				}
			}()

			// Start concurrent mounts
			errChan := make(chan error, numTargets)
			for i := 0; i < numTargets; i++ {
				go func(idx int) {
					err := testCtx.podMounter.Mount(testCtx.ctx, testCtx.bucketName, targets[idx], credentialprovider.ProvideContext{
						VolumeID: testCtx.volumeID,
						PodID:    podUIDs[idx],
					}, mountpoint.ParseArgs(nil), "")
					errChan <- err
				}(i)
			}

			// Wait for all mounts to complete
			for i := 0; i < numTargets; i++ {
				err := <-errChan
				assert.NoError(t, err)
			}

			// Verify mount operations were properly serialized
			// Only one mount to source should have occurred
			mountedTargets := make(map[string]bool)
			select {
			case target := <-mountStarted:
				mountedTargets[target] = true
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Expected at least one mount to start")
			}

			// Drain any remaining mount operations
			done := false
			for !done {
				select {
				case target := <-mountStarted:
					mountedTargets[target] = true
				case <-time.After(10 * time.Millisecond):
					done = true
				}
			}

			// Should only have mounted to the source once
			mpPodName := mppod.MountpointPodNameFor(testCtx.podUID, testCtx.pvName)
			expectedSource := filepath.Join(mounter.SourceMountDir(testCtx.kubeletPath), mpPodName)
			assert.Equals(t, 1, len(mountedTargets))
			assert.Equals(t, true, mountedTargets[expectedSource])

			// All targets should be mount points
			for i := 0; i < numTargets; i++ {
				isMounted, err := testCtx.mount.IsMountPoint(targets[i])
				assert.NoError(t, err)
				assert.Equals(t, true, isMounted)
			}
		})
	})
}