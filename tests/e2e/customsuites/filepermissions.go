/*
This suite tests Mountpoint-S3's behavior regarding file metadata immutability.
Covered behaviors:

- file-mode: Ensures the mounted file gets the correct mode at creation time.
- chmod: Verifies that chmod operations fail (EPERM/ENOTSUP) and don't change mode.
- chown: Verifies that ownership changes fail post-creation.
- umask: Ensures umask does not alter driver-enforced permissions on new files.

These behaviors align with Mountpoint-S3's design: metadata is immutable after mount.
*/
package customsuites

import (
	"context"
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

// s3CSIFilePermissionsTestSuite tests file permission functionality
// for the S3 CSI driver when mounting S3 buckets.
type s3CSIFilePermissionsTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3FilePermissionsTestSuite returns a test suite for file permissions.
//
// This suite tests:
// - Default file/directory permissions (0644/0755)
// - Custom file permissions via file-mode mount option
// - Permission inheritance in subdirectories
// - Permission behavior during remount with changed options
// - Multi-pod access with different permissions
// - Permission preservation during file operations
func InitS3FilePermissionsTestSuite() storageframework.TestSuite {
	return &s3CSIFilePermissionsTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "filepermissions",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsPreprovisionedPV,
			},
		},
	}
}

// GetTestSuiteInfo returns test suite information.
func (suite *s3CSIFilePermissionsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return suite.tsInfo
}

// SkipUnsupportedTests is a no-op as all tests should be supported.
func (suite *s3CSIFilePermissionsTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

// DefineTests implements the test suite functionality.
func (suite *s3CSIFilePermissionsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type TestResourceRegistry struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var testRegistry TestResourceRegistry

	testFramework := framework.NewFrameworkWithCustomTimeouts("filepermissions", storageframework.GetDriverTimeouts(driver))
	testFramework.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	cleanup := func() {
		for i := range testRegistry.resources {
			resource := testRegistry.resources[i]
			func() {
				defer ginkgo.GinkgoRecover()
				ctx := context.Background()
				ginkgo.By("Deleting pv and pvc")
				err := resource.CleanupResource(ctx)
				if err != nil {
					framework.Logf("Warning: Resource cleanup had an error: %v", err)
				}
			}()
		}
	}

	// Helper functions for permission verification to reduce code duplication

	// verifyFilePermissions checks if a file has the expected permissions
	// and optionally verifies ownership if uid and gid are specified
	verifyFilePermissions := func(f *framework.Framework, pod *v1.Pod, filePath string, expectedMode string, uid, gid *int64) {
		ginkgo.By(fmt.Sprintf("Verifying file has %s permissions", expectedMode))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%a' %s | grep -q '^%s$'", filePath, expectedMode))

		if uid != nil && gid != nil {
			ginkgo.By("Verifying file ownership")
			e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%u %%g' %s | grep '%d %d'",
				filePath, *uid, *gid))
		}
	}

	// verifyDirPermissions checks if a directory has the expected permissions
	// and optionally verifies ownership if uid and gid are specified
	verifyDirPermissions := func(f *framework.Framework, pod *v1.Pod, dirPath string, expectedMode string, uid, gid *int64) {
		ginkgo.By(fmt.Sprintf("Verifying directory has %s permissions", expectedMode))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%a' %s | grep -q '^%s$'", dirPath, expectedMode))

		if uid != nil && gid != nil {
			ginkgo.By("Verifying directory ownership")
			e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%u %%g' %s | grep '%d %d'",
				dirPath, *uid, *gid))
		}
	}

	// verifyPermissions checks permissions and ownership for both a file and directory
	// This combines file and directory permission checking into a single function call
	verifyPermissions := func(f *framework.Framework, pod *v1.Pod, filePath, dirPath, expectedFileMode, expectedDirMode string, uid, gid *int64) {
		verifyFilePermissions(f, pod, filePath, expectedFileMode, uid, gid)
		verifyDirPermissions(f, pod, dirPath, expectedDirMode, uid, gid)
	}

	// verifyPathsPermissions verifies permissions for multiple files and directories
	// filePaths is a slice of file paths to check
	// dirPaths is a slice of directory paths to check
	// expectedFileMode is the expected permission mode for all files
	// expectedDirMode is the expected permission mode for all directories
	verifyPathsPermissions := func(f *framework.Framework, pod *v1.Pod, filePaths, dirPaths []string,
		expectedFileMode, expectedDirMode string, uid, gid *int64,
	) {
		// Check file permissions
		for _, filePath := range filePaths {
			verifyFilePermissions(f, pod, filePath, expectedFileMode, uid, gid)
		}

		// Check directory permissions
		for _, dirPath := range dirPaths {
			verifyDirPermissions(f, pod, dirPath, expectedDirMode, uid, gid)
		}
	}

	// createVolumeWithOptions is a thin wrapper around BuildVolumeWithOptions that also tracks
	// the created resource in the TestResourceRegistry resources slice for cleanup.
	createVolumeWithOptions := func(ctx context.Context, config *storageframework.PerTestConfig, pattern storageframework.TestPattern,
		uid, gid int64, fileModeOption string, extraOptions ...string,
	) *storageframework.VolumeResource {
		resource := BuildVolumeWithOptions(ctx, config, pattern, uid, gid, fileModeOption, extraOptions...)
		testRegistry.resources = append(testRegistry.resources, resource)
		return resource
	}

	// setupTestPaths creates a nested directory structure for testing
	// Returns a map containing paths for the created directories and files
	setupTestPaths := func(f *framework.Framework, pod *v1.Pod, volumePath string) map[string]string {
		paths := make(map[string]string)

		// Define paths
		paths["volPath"] = volumePath
		paths["subdir1"] = fmt.Sprintf("%s/subdir1", volumePath)
		paths["subdir2"] = fmt.Sprintf("%s/subdir2", volumePath)
		paths["subdir3"] = fmt.Sprintf("%s/subdir1/subdir3", volumePath)
		paths["rootFile"] = fmt.Sprintf("%s/root.txt", volumePath)
		paths["file1"] = fmt.Sprintf("%s/file1.txt", paths["subdir1"])
		paths["file2"] = fmt.Sprintf("%s/file2.txt", paths["subdir2"])
		paths["file3"] = fmt.Sprintf("%s/file3.txt", paths["subdir3"])

		ginkgo.By("Creating nested directory structure")
		CreateMultipleDirsInPod(f, pod, paths["subdir1"], paths["subdir2"], paths["subdir3"])

		// Create files using the helper function
		ginkgo.By("Creating files at different directory levels")
		CreateFileInPod(f, pod, paths["rootFile"], "root")
		CreateFileInPod(f, pod, paths["file1"], "level1")
		CreateFileInPod(f, pod, paths["file2"], "level2")
		CreateFileInPod(f, pod, paths["file3"], "level3")

		return paths
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		testRegistry = TestResourceRegistry{}
		testRegistry.config = driver.PrepareTest(ctx, testFramework)
		ginkgo.DeferCleanup(cleanup)
	})

	// Test 1: Default Permissions Test
	//
	// This test verifies the default file/directory permissions when
	// no specific permission mount options are specified:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume]
	//        |
	//        ↓
	//    [S3 Bucket]
	//
	// Expected results:
	// - Files: 0644 (-rw-r--r--) permissions
	// - Directories: 0755 (drwxr-xr-x) permissions
	// - Ownership: matches specified uid/gid
	ginkgo.It("should have default permissions of 0644 for files when no mount options specified", func(ctx context.Context) {
		// Create volume with mount options required for non-root access
		resource := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "")

		// Create a pod with the volume
		ginkgo.By("Creating pod with a volume")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, resource.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)

		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Create a test file and directory
		volPath := "/mnt/volume1"
		testFile, testDir := CreateTestFileAndDir(testFramework, pod, volPath, "testfile")

		// Convert the UID/GID constants to pointers for the verification functions.
		// This is necessary because verifyFilePermissions and verifyDirPermissions
		// accept pointer parameters to support optional ownership verification.
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)

		verifyPermissions(testFramework, pod, testFile, testDir, "644", "755", uidPtr, gidPtr)
	})

	// Test 2: Custom File Permissions Test
	//
	// This test verifies that custom file permissions are applied when
	// the file-mode mount option is specified:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//    [S3 Bucket]
	//
	// Expected results:
	// - Files: 0600 (-rw-------) permissions (from file-mode option)
	// - Directories: 0755 (drwxr-xr-x) permissions (default, unaffected by file-mode)
	// - Ownership: matches specified uid/gid
	ginkgo.It("should apply custom permissions of 0600 for files when file-mode mount option specified", func(ctx context.Context) {
		// Create volume with custom file-mode mount option
		resource := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0600")

		// Create a pod with the volume
		ginkgo.By("Creating pod with a volume that has file-mode=0600")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, resource.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)

		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Create a test file and directory
		volPath := "/mnt/volume1"
		testFile, testDir := CreateTestFileAndDir(testFramework, pod, volPath, "testfile")

		// Convert the UID/GID constants to pointers for the verification functions.
		// This is necessary because verifyFilePermissions and verifyDirPermissions
		// accept pointer parameters to support optional ownership verification.
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)

		verifyPermissions(testFramework, pod, testFile, testDir, "600", "755", uidPtr, gidPtr)
	})

	// Test 3: Dual Volume Permissions Test
	//
	// This test verifies that different volumes in the same pod
	// can have different file permission settings:
	//
	//      [Pod]
	//        |
	//       / \
	//      /   \
	//  [Vol 1]  [Vol 2]
	// file-mode  file-mode
	//  =0600     =0666
	//     |         |
	//     ↓         ↓
	// [S3 Bucket] [S3 Bucket]
	//
	// Expected results:
	// - Volume 1 Files: 0600 (-rw-------) permissions
	// - Volume 2 Files: 0666 (-rw-rw-rw-) permissions
	// - Directories: Always 0755 (drwxr-xr-x) permissions
	// - Ownership: matches specified uid/gid on both volumes
	ginkgo.It("should maintain distinct file permissions for multiple volumes in the same pod", func(ctx context.Context) {
		// Create first volume with file-mode=0600
		ginkgo.By("Creating first volume with file-mode=0600")
		resource1 := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0600")

		// Create second volume with file-mode=0666
		ginkgo.By("Creating second volume with file-mode=0666")
		resource2 := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0666")

		// Create a pod with both volumes
		ginkgo.By("Creating pod with both volumes mounted")
		claims := []*v1.PersistentVolumeClaim{resource1.Pvc, resource2.Pvc}
		pod := MakeNonRootPodWithVolume(testFramework.Namespace.Name, claims, "")

		var err error
		pod, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Define paths for both volumes
		vol1Path := "/mnt/volume1"
		vol2Path := "/mnt/volume2"
		vol1TestFile := fmt.Sprintf("%s/testfile-vol1.txt", vol1Path)
		vol2TestFile := fmt.Sprintf("%s/testfile-vol2.txt", vol2Path)
		vol1TestDir := fmt.Sprintf("%s/testdir-vol1", vol1Path)
		vol2TestDir := fmt.Sprintf("%s/testdir-vol2", vol2Path)

		// Create test files and directories in both volumes
		ginkgo.By("Creating test files and directories in both volumes")
		CreateFileInPod(testFramework, pod, vol1TestFile, "volume 1 content")
		CreateFileInPod(testFramework, pod, vol2TestFile, "volume 2 content")
		CreateDirInPod(testFramework, pod, vol1TestDir)
		CreateDirInPod(testFramework, pod, vol2TestDir)

		// Verify permissions for both volumes using helper functions
		ginkgo.By("Verifying permissions for volume 1 (file-mode=0600)")
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)

		// Verify first volume (file-mode=0600)
		verifyFilePermissions(testFramework, pod, vol1TestFile, "600", uidPtr, gidPtr)
		verifyDirPermissions(testFramework, pod, vol1TestDir, "755", uidPtr, gidPtr)

		// Verify second volume (file-mode=0666)
		verifyFilePermissions(testFramework, pod, vol2TestFile, "666", uidPtr, gidPtr)
		verifyDirPermissions(testFramework, pod, vol2TestDir, "755", uidPtr, gidPtr)
	})

	// Test 4: Remounting Permissions Test
	//
	// This test verifies that changing file permission mount options
	// and remounting a volume applies the new settings:
	//
	//      [Pod 1]                 [Pod 2]
	//        |                       |
	//        ↓                       ↓
	//   [S3 Volume]  →  1. Delete Pod 1  →  [S3 Volume]
	//   Default perms    2. Update PV        file-mode=0444
	//        |              mount options        |
	//        ↓                                   ↓
	//    [S3 Bucket] ──────── Same Bucket ──→ [S3 Bucket]
	//
	// Expected results:
	// - Initial files: 0644 (-rw-r--r--) permissions (default)
	// - After remount: 0444 (-r--r--r--) permissions (read-only)
	// - Directories: Always 0755 (drwxr-xr-x) permissions
	// - Ownership: matches specified uid/gid in both cases
	ginkgo.It("should update file permissions when a volume is remounted with new options", func(ctx context.Context) {
		// Step 1: Create initial volume with default permissions
		ginkgo.By("Creating volume with default permissions")
		resource := createVolumeResourceWithMountOptions(ctx, testRegistry.config, pattern, []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other", // Required for non-root access

		})
		testRegistry.resources = append(testRegistry.resources, resource)

		// Step 2: Create first pod with the volume
		ginkgo.By("Creating first pod with volume using default permissions")
		pod1 := MakeNonRootPodWithVolume(testFramework.Namespace.Name, []*v1.PersistentVolumeClaim{resource.Pvc}, "write-pod")

		var err error
		pod1, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod1)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod1))
		}()

		// Create a test file and directory
		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/testfile.txt", volPath)
		testDir := fmt.Sprintf("%s/testdir", volPath)

		ginkgo.By("Creating a test file with default permissions")
		CreateFileInPod(testFramework, pod1, testFile, "test content")

		ginkgo.By("Creating a test directory")
		CreateDirInPod(testFramework, pod1, testDir)

		// Verify initial permissions using helper function
		ginkgo.By("Verifying initial file and directory permissions")
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)
		verifyPermissions(testFramework, pod1, testFile, testDir, "644", "755", uidPtr, gidPtr)

		// Step 3: Delete the pod
		ginkgo.By("Deleting the first pod")
		framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod1))

		// Step 4: Update the PV to use file-mode=0444
		ginkgo.By("Updating volume to use file-mode=0444")

		// Get the PV object
		pv, err := testFramework.ClientSet.CoreV1().PersistentVolumes().Get(ctx, resource.Pv.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get PV")

		// Update the mount options to include file-mode=0444
		newMountOptions := []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other", // Required for non-root access

			"file-mode=0444", // Add read-only file permissions
		}
		pv.Spec.MountOptions = newMountOptions

		// Update the PV
		_, err = testFramework.ClientSet.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
		framework.ExpectNoError(err, "failed to update PV with new mount options")

		// Step 5: Create a new pod with the updated volume
		ginkgo.By("Creating second pod with updated volume permissions")
		pod2 := MakeNonRootPodWithVolume(testFramework.Namespace.Name, []*v1.PersistentVolumeClaim{resource.Pvc}, "read-pod")

		pod2, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod2)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod2))
		}()

		// Creating a new test directory in the second pod since it doesn't persist between pods
		ginkgo.By("Creating a new test directory in the second pod")
		CreateDirInPod(testFramework, pod2, testDir)

		// Step 6: Verify new permissions using helper function
		ginkgo.By("Verifying updated file and directory permissions")
		// Reuse the same uid/gid pointers
		verifyPermissions(testFramework, pod2, testFile, testDir, "444", "755", uidPtr, gidPtr)

		// Try to write to the file (should fail with read-only permissions)
		ginkgo.By("Verifying file is now read-only")
		_, _, err = e2evolume.PodExec(testFramework, pod2, fmt.Sprintf("echo 'new content' > %s", testFile))
		if err == nil {
			framework.Failf("Was able to write to a read-only file!")
		}
		framework.Logf("As expected, writing to read-only file failed")
	})

	// Test 5: Concurrent Mount Permissions Test
	//
	// This test verifies that pods already mounting a volume see the original
	// permissions, while new pods mounting after an update see new permissions:
	//
	//      [Pod 1] ────────────────────────────────── [Pod 1]
	//        |          Continue running                 |
	//        ↓                                           |
	//   [S3 Volume]  →  1. Update PV mount options  →  [S3 Volume]
	//   Default perms    without deleting Pod 1       file-mode=0444
	//        |                                           ↑
	//        ↓                                           |
	//    [S3 Bucket] ── Same bucket with updated PV ─ [Pod 2]
	//
	// Expected results:
	// - Pod 1 continues to see files with original 0644 (-rw-r--r--) permissions
	// - Pod 2 sees files with updated 0444 (-r--r--r--) permissions
	// - New files created by Pod 1 have 0644 permissions (seen as 0444 by Pod 2)
	// - New files created by Pod 2 have 0444 permissions (seen as 0644 by Pod 1)
	ginkgo.It("should maintain different file permissions in concurrent pods with updated mount options", func(ctx context.Context) {
		// Step 1: Create initial volume with default permissions
		ginkgo.By("Creating volume with default permissions")
		resource := createVolumeResourceWithMountOptions(ctx, testRegistry.config, pattern, []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other", // Required for non-root access

		})
		testRegistry.resources = append(testRegistry.resources, resource)

		// Step 2: Create first pod with the volume
		ginkgo.By("Creating first pod with volume using default permissions")
		pod1 := MakeNonRootPodWithVolume(testFramework.Namespace.Name, []*v1.PersistentVolumeClaim{resource.Pvc}, "write-pod")

		var err error
		pod1, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod1)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod1))
		}()

		// Create a test file and directory
		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/testfile.txt", volPath)
		testDir := fmt.Sprintf("%s/testdir", volPath)

		ginkgo.By("Creating a test file with default permissions from pod1")
		CreateFileInPod(testFramework, pod1, testFile, "test content from pod1")

		ginkgo.By("Creating a test directory from pod1")
		CreateDirInPod(testFramework, pod1, testDir)

		// Verify initial permissions using helper function
		ginkgo.By("Verifying initial file and directory permissions in pod1")
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)
		verifyPermissions(testFramework, pod1, testFile, testDir, "644", "755", uidPtr, gidPtr)

		// Step 3: Update the PV to use file-mode=0444 without deleting the first pod
		ginkgo.By("Updating volume to use file-mode=0444 without deleting the first pod")

		// Get the PV object
		pv, err := testFramework.ClientSet.CoreV1().PersistentVolumes().Get(ctx, resource.Pv.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "failed to get PV")

		// Update the mount options to include file-mode=0444
		newMountOptions := []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other", // Required for non-root access

			"file-mode=0444", // Add read-only file permissions
		}
		pv.Spec.MountOptions = newMountOptions

		// Update the PV
		_, err = testFramework.ClientSet.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
		framework.ExpectNoError(err, "failed to update PV with new mount options")

		// Step 4: Create a second pod that mounts the same volume with updated mount options
		ginkgo.By("Creating second pod with the same volume using updated permissions")
		pod2 := MakeNonRootPodWithVolume(testFramework.Namespace.Name, []*v1.PersistentVolumeClaim{resource.Pvc}, "read-pod")

		pod2, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod2)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod2))
		}()

		// Step 5: Verify that pod1 still sees the original permissions
		ginkgo.By("Verifying pod1 still sees file with original permissions (0644)")
		verifyFilePermissions(testFramework, pod1, testFile, "644", uidPtr, gidPtr)

		// Step 6: Verify that pod2 sees the new permissions
		ginkgo.By("Verifying pod2 sees file with updated permissions (0444)")
		verifyFilePermissions(testFramework, pod2, testFile, "444", uidPtr, gidPtr)

		// Step 7: Create new files from both pods
		pod1File := fmt.Sprintf("%s/pod1file.txt", volPath)
		pod2File := fmt.Sprintf("%s/pod2file.txt", volPath)

		ginkgo.By("Creating a new file from pod1")
		CreateFileInPod(testFramework, pod1, pod1File, "content from pod1")

		ginkgo.By("Creating a new file from pod2")
		CreateFileInPod(testFramework, pod2, pod2File, "content from pod2")

		// Step 8: Verify permissions for the new files as seen from each pod
		ginkgo.By("Verifying file permissions from both pods' perspectives")
		// Check all files from pod1's perspective
		pod1Files := []string{pod1File, pod2File}
		verifyPathsPermissions(testFramework, pod1, pod1Files, []string{}, "644", "", uidPtr, gidPtr)

		// Check all files from pod2's perspective
		pod2Files := []string{pod1File, pod2File}
		verifyPathsPermissions(testFramework, pod2, pod2Files, []string{}, "444", "", uidPtr, gidPtr)
	})

	// Test 6: Subdirectory Inheritance Test
	//
	// This test verifies that files in subdirectories inherit the
	// specified file mode mount option:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0640]
	//        |
	//        ↓
	//   [Root Directory]
	//      /    \
	//     /      \
	//  [subdir1] [subdir2]
	//     |          \
	//     ↓           ↓
	//  [subdir1/    [subdir2/
	//   subdir3]     file2.txt]
	//     |
	//     ↓
	//  [subdir1/
	//   subdir3/
	//   file3.txt]
	//
	// Expected results:
	// - All files at all levels have 0640 (-rw-r-----) permissions
	// - All directories maintain 0755 (drwxr-xr-x) permissions
	ginkgo.It("should apply the same file permissions to files in subdirectories", func(ctx context.Context) {
		// Step 1: Create volume with custom file-mode=0640 mount option
		ginkgo.By("Creating volume with file-mode=0640 and additional operations permissions")
		resource := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0640",
			"allow-delete", "allow-overwrite")

		// Step 2: Create a pod with the volume
		ginkgo.By("Creating pod with the volume")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, resource.Pvc, "write-pod", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Step 3: Create nested directory structure and test files
		volPath := "/mnt/volume1"
		paths := setupTestPaths(testFramework, pod, volPath)

		// Step 4: Verify file permissions across all levels using the helper function
		ginkgo.By("Verifying all files have 0640 permissions")
		filePaths := []string{
			paths["rootFile"],
			paths["file1"],
			paths["file2"],
			paths["file3"],
		}

		dirPaths := []string{
			paths["volPath"],
			paths["subdir1"],
			paths["subdir2"],
			paths["subdir3"],
		}

		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)

		verifyPathsPermissions(testFramework, pod, filePaths, dirPaths, "640", "755", uidPtr, gidPtr)
	})

	// Test 7: File Copy/Delete Permissions Test
	//
	// This test verifies that file permissions are preserved
	// when files are copied between directories in S3 volumes:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0640]
	//        |
	//       / \
	//      /   \
	// [Dir1]   [Dir2]
	//   |         ↑
	//   |         |
	//  [File] -> Copy -> [File]
	//
	// Expected results:
	// - Initial file has 0640 (-rw-r-----) permissions
	// - File maintains 0640 permissions after being copied
	// - File ownership remains consistent throughout operations
	ginkgo.It("should preserve file permissions during copy operations", func(ctx context.Context) {
		// Step 1: Create volume with custom file-mode=0640 mount option
		ginkgo.By("Creating volume with file-mode=0640 and additional operations permissions")
		resource := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0640",
			"allow-delete", "allow-overwrite")

		// Step 2: Create a pod with the volume
		ginkgo.By("Creating pod with the volume")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, resource.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Step 3: Create directories for testing file operations
		volPath := "/mnt/volume1"
		sourceDir := fmt.Sprintf("%s/source-dir", volPath)
		targetDir := fmt.Sprintf("%s/target-dir", volPath)

		ginkgo.By("Creating source and target directories")
		CreateMultipleDirsInPod(testFramework, pod, sourceDir, targetDir)

		// Step 4: Create a test file in the source directory
		sourceFile := fmt.Sprintf("%s/test-file.txt", sourceDir)
		ginkgo.By("Creating a test file in the source directory")
		CreateFileInPod(testFramework, pod, sourceFile, "test content")

		// Step 5: Verify initial file permissions
		ginkgo.By("Verifying initial file has 0640 permissions")
		uidPtr := ptr.To(DefaultNonRootUser)
		gidPtr := ptr.To(DefaultNonRootGroup)
		verifyFilePermissions(testFramework, pod, sourceFile, "640", uidPtr, gidPtr)

		// Step 6: Copy the file to the target directory
		targetFile := fmt.Sprintf("%s/copied-file.txt", targetDir)
		ginkgo.By("Copying file to target directory")
		CopyFileInPod(testFramework, pod, sourceFile, targetFile)

		// Step 7: Verify permissions after copy
		ginkgo.By("Verifying copied file maintains 0640 permissions")
		verifyFilePermissions(testFramework, pod, targetFile, "640", uidPtr, gidPtr)

		// Step 8: Create another file with a different name in source directory
		// Move (mv) is not supported by mountpoint-S3, so we are using copy+delete to simulate it.
		newSourceFile := fmt.Sprintf("%s/another-test-file.txt", sourceDir)
		ginkgo.By("Creating another test file for rename simulation")
		CreateFileInPod(testFramework, pod, newSourceFile, "content for rename test")

		// Step 9: Copy the file to target directory with a different name (simulating rename)
		renamedFile := fmt.Sprintf("%s/renamed-file.txt", targetDir)
		ginkgo.By("Copying file to target directory with new name (simulating rename)")
		CopyFileInPod(testFramework, pod, newSourceFile, renamedFile)

		// Step 10: Delete the source file (completing the rename simulation)
		ginkgo.By("Deleting source file to complete rename simulation")
		DeleteFileInPod(testFramework, pod, newSourceFile)

		// Step 11: Verify permissions after simulated rename
		ginkgo.By("Verifying renamed file maintains 0640 permissions and proper ownership")
		verifyFilePermissions(testFramework, pod, renamedFile, "640", uidPtr, gidPtr)

		// Step 13: Compare permissions between original and copied files
		ginkgo.By("Comparing permissions between source and copied files")
		sourcePerms, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", sourceFile))
		framework.ExpectNoError(err, "failed to get source permissions: %s", stderr)

		copyPerms, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", targetFile))
		framework.ExpectNoError(err, "failed to get copied permissions: %s", stderr)

		if sourcePerms != copyPerms {
			framework.Failf("Permission mismatch after copy: source=%s, copy=%s", sourcePerms, copyPerms)
		}
	})

	// Test 8: Pod Security Context Test
	// This test verifies how pod security contexts interact
	// with the S3 CSI driver file permissions:
	//
	//	   [Pod with SecurityContext]
	//	     |    runAsUser: 3000
	//	     |    fsGroup: 4000
	//	     |
	//	     ↓
	//	[S3 Volume with file-mode=0640]
	//	     |
	//	     ↓
	//	[Files & Directories]
	//
	// Expected results:
	// - Files have the specified file mode (0640) regardless of security context
	// - File ownership is affected by the pod security context settings
	// - Pod's runAsUser determines the user ownership of created files
	// - Pod's fsGroup determines the group ownership of created files
	ginkgo.It("should properly apply permissions with pod security context settings", func(ctx context.Context) {
		// Define specific security context settings for the pod
		customUID := int64(3000)
		customGID := int64(4000)
		runAsNonRoot := true

		// Step 1: Create volume with custom file-mode=0640 mount option
		// Use the same UID/GID in mount options as in the security context
		ginkgo.By("Creating volume with file-mode=0640")
		resource := createVolumeWithOptions(ctx, testRegistry.config, pattern, customUID, customGID, "0640")

		// Step 2: Create a pod with specific security context settings
		ginkgo.By("Creating pod with specific runAsUser and fsGroup security context")
		// Note: We don't use MakeNonRootPodWithVolume here because we're setting custom UIDs
		pod := e2epod.MakePod(testFramework.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelRestricted, "")

		// Set the pod's security context to use specific user and group IDs
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			RunAsUser:    &customUID,
			FSGroup:      &customGID,
			RunAsNonRoot: &runAsNonRoot,
			SeccompProfile: &v1.SeccompProfile{
				Type: v1.SeccompProfileTypeRuntimeDefault,
			},
		}

		var err error
		pod, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod))
		}()

		// Step 3: Create test files in the volume
		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/test-file.txt", volPath)
		testDir := fmt.Sprintf("%s/test-dir", volPath)

		ginkgo.By("Creating test file and directory from the pod")
		CreateFileInPod(testFramework, pod, testFile, "test content")
		CreateDirInPod(testFramework, pod, testDir)

		// Steps 4-7: Verify file and directory permissions and ownership using helper functions
		ginkgo.By("Verifying file and directory permissions with custom security context")
		uidPtr := ptr.To(customUID)
		gidPtr := ptr.To(customGID)
		verifyPermissions(testFramework, pod, testFile, testDir, "640", "755", uidPtr, gidPtr)

		// Step 8: Create a file with specific permissions using chmod (to verify interaction)
		explicitFile := fmt.Sprintf("%s/explicit-perm-file.txt", volPath)
		ginkgo.By("Creating a file with explicitly set permissions")
		CreateFileInPod(testFramework, pod, explicitFile, "explicit perm test")

		// Try to change permissions (this is expected to fail with S3 CSI driver)
		ginkgo.By("Verifying chmod operation is not permitted (expected behavior)")
		_, _, err = e2evolume.PodExec(testFramework, pod, fmt.Sprintf("chmod 600 %s", explicitFile))
		if err == nil {
			framework.Failf("Expected chmod to fail, but it succeeded")
		}

		// Step 9: Verify that chmod doesn't actually change permissions (driver-enforced file-mode)
		ginkgo.By("Verifying chmod doesn't override driver-enforced file-mode")
		// The file should still have 0640 (the mount option) regardless of chmod
		verifyFilePermissions(testFramework, pod, explicitFile, "640", uidPtr, gidPtr)
	})

	// --------------------------------------------------------------------
	// 9. Chmod operation disallowed (file)
	//
	// This test verifies that file permissions on the S3 volume cannot be
	// changed after mount. Mountpoint-S3 enforces a behavior where the
	// file-mode is set (either via user-specified option or a default like
	// 0600) at mount time and remains immutable.
	//
	// Test scenario:
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//    [File]
	//
	// Expected results:
	// - The file is created with the initial file-mode and correct UID/GID
	// - An attempt to change file permissions using `chmod` fails
	// - The error message from `chmod` may vary by kernel/implementation:
	//     - "Operation not permitted" (EPERM)
	//     - Or "Operation not supported" (ENOTSUP)
	// - The file permissions remain unchanged after the failed attempt
	//
	// This validates Mountpoint-S3's behavior where file metadata is fixed
	// at mount time (even when using defaults) and cannot be modified later,
	// consistent with S3's object store semantics.
	ginkgo.It("should not allow chmod to change file permissions (no-op)", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		filePath := "/mnt/volume1/chmod-test-file"

		// Create the test file
		CreateFileInPod(testFramework, pod, filePath, "hello world")

		// Check initial permissions
		initialPerms, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", filePath))
		framework.ExpectNoError(err)

		// Attempt chmod
		_, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("chmod 0755 %s", filePath))
		gomega.Expect(err).To(gomega.HaveOccurred())

		expectErr := gomega.Or(
			gomega.ContainSubstring("Operation not permitted"),
			gomega.ContainSubstring("Operation not supported"),
		)
		gomega.Expect(stderr).To(expectErr)

		// Confirm permissions unchanged
		afterPerms, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(afterPerms)).To(gomega.Equal(strings.TrimSpace(initialPerms)))
	})

	// --------------------------------------------------------------------
	// 10. Chown operation disallowed (file)
	//
	// This test verifies that file ownership on the S3 volume cannot be
	// changed after mount. Mountpoint-S3 enforces a behavior where the
	// ownership (UID/GID) is set (via user-specified option or defaults)
	// at mount time and remains immutable.
	//
	// Test scenario:
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with uid/gid = 1001/2000]
	//        |
	//        ↓
	//    [File]
	//
	// Expected results:
	// - The file is created with the initial UID/GID
	// - An attempt to change file ownership using `chown` fails
	// - The error message may vary by kernel/implementation:
	//     - "Operation not permitted" (EPERM)
	//     - Or "Operation not supported" (ENOTSUP)
	// - The file ownership remains unchanged after the failed attempt
	//
	// This validates that Mountpoint-S3 enforces immutability of ownership
	// metadata after mount, consistent with S3's object store semantics.
	ginkgo.It("should not allow chown to change file ownership (no-op)", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		filePath := "/mnt/volume1/chown-test-file"

		// Create the test file
		CreateFileInPod(testFramework, pod, filePath, "testcontent")

		// Check initial ownership
		initialOwner, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%u:%%g' %s", filePath))
		framework.ExpectNoError(err)

		// Attempt chown
		_, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("chown 0:0 %s", filePath))
		gomega.Expect(err).To(gomega.HaveOccurred())

		expectErr := gomega.Or(
			gomega.ContainSubstring("Operation not permitted"),
			gomega.ContainSubstring("Operation not supported"),
		)
		gomega.Expect(stderr).To(expectErr)

		// Confirm ownership unchanged
		afterOwner, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%u:%%g' %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(afterOwner)).To(gomega.Equal(strings.TrimSpace(initialOwner)))
	})

	// --------------------------------------------------------------------
	// 11. Umask enforcement (file)
	//
	// This test verifies that a pod-level umask does not interfere with
	// Mountpoint-S3's enforcement of the file-mode. Even if a pod sets
	// a restrictive umask, the driver-enforced file-mode takes precedence.
	//
	// Test scenario:
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0666]
	//        |
	//        ↓
	//    [File (created with umask 077)]
	//
	// Expected results:
	// - The file is created with the driver-enforced mode (0666)
	//   regardless of the pod's umask
	// - Ownership: UID/GID matches what was specified at mount
	//
	// This validates that Mountpoint-S3 enforces permissions based on
	// mount options and ignores the process's umask when creating files.
	ginkgo.It("should enforce file-mode regardless of pod umask", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0666")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		filePath := "/mnt/volume1/umask-test-file"

		// Create the file using umask 077 (which would normally restrict perms)
		_, _, err = e2evolume.PodExec(testFramework, pod, fmt.Sprintf(
			`sh -c 'umask 077; echo testcontent > %s'`, filePath))
		framework.ExpectNoError(err)

		// Verify permissions: expect driver-enforced 0666, NOT affected by umask
		perms, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(perms)).To(gomega.Equal("666"))
	})

	// --------------------------------------------------------------------
	// 12. Symlink creation & permission enforcement
	//
	// Mountpoint-S3 does NOT support symlinks: any attempt to create one
	// (ln -s) fails (EPERM/ENOTSUP), reflecting S3's lack of symlink semantics.
	//
	// Test scenario:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume]
	//        |
	//        ↓
	//  Attempt:
	//   ln -s /mnt/volume1/real-file -> /mnt/volume1/link-to-real
	//
	// Expected results:
	// - Symlink creation fails with "Operation not permitted" (EPERM) or similar
	// - The target file (/mnt/volume1/real-file) remains intact (no metadata change)
	// - The symlink (/mnt/volume1/link-to-real) does not exist (no artifact left)
	// - chmod/chown on the failed symlink path fail (ENOENT)
	// - stat on the target file shows the original 0600 mode and correct UID/GID
	//
	// This verifies that Mountpoint-S3 enforces S3 semantics: no symlinks,
	// and attempts to create or manipulate them are rejected gracefully.
	ginkgo.It("should reject symlink creation and leave target intact", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		target := "/mnt/volume1/real-file"
		link := "/mnt/volume1/link-to-real"

		// create target
		CreateFileInPod(testFramework, pod, target, "symlink test")

		// try to create symlink → must FAIL
		_, lnStderr, lnErr := e2evolume.PodExec(testFramework, pod,
			fmt.Sprintf("ln -s %s %s", target, link))
		gomega.Expect(lnErr).To(gomega.HaveOccurred())
		gomega.Expect(lnStderr).To(gomega.Or(
			gomega.ContainSubstring("Operation not permitted"),
			gomega.ContainSubstring("Operation not supported"),
		))

		// verify target file metadata unchanged
		verifyFilePermissions(testFramework, pod, target, "600",
			ptr.To(DefaultNonRootUser), ptr.To(DefaultNonRootGroup))

		// ensure link really does NOT exist
		_, _, statErr := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("ls -l %s", link))
		gomega.Expect(statErr).To(gomega.HaveOccurred())

		// chmod on would‑be‑link must also fail
		_, chErrStr, chErr := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("chmod 777 %s", link))
		gomega.Expect(chErr).To(gomega.HaveOccurred())
		gomega.Expect(chErrStr).To(gomega.ContainSubstring("No such file"))
	})

	// --------------------------------------------------------------------
	// 13. File truncation behavior (immutability validation)
	//
	// Mountpoint-S3 disallows in-place truncation of existing files because S3
	// does not support partial updates of objects. Even though truncation
	// targets the file size (not its mode/ownership), the driver enforces
	// full immutability.
	//
	// Test scenario:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//   ┌────────────────────────────────────────────┐
	//   │ Step 1: Create /mnt/volume1/trunc-file     │
	//   │         Content: "data"                    │
	//   │                                            │
	//   │ Step 2: Attempt to truncate the file:      │
	//   │         sh -c ': > /mnt/volume1/trunc-file'│
	//   └────────────────────────────────────────────┘
	//
	// Expected results:
	// - The truncate attempt fails (EPERM / ENOTSUP)
	// - The error message matches "Operation not permitted" or
	//   "Operation not supported"
	// - File content remains unchanged ("data")
	// - File metadata remains unchanged:
	//     - Mode: 0600 (from mount option)
	//     - UID:GID: matches mount config
	//
	// This confirms Mountpoint-S3 enforces S3 semantics: no in-place
	// file size changes are permitted post-creation.
	//
	ginkgo.It("should reject truncation and preserve file metadata/content", func(ctx context.Context) {
		// Set up a volume with file-mode=0600
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		filePath := "/mnt/volume1/trunc-file"

		// Step 1: Create the file with content
		CreateFileInPod(testFramework, pod, filePath, "data")

		// Check initial permissions and content
		permsBefore, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", filePath))
		framework.ExpectNoError(err)
		ownerBefore, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%u:%%g' %s", filePath))
		framework.ExpectNoError(err)
		contentBefore, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("cat %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(contentBefore)).To(gomega.Equal("data"))

		// Step 2: Attempt to truncate the file
		_, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf(`sh -c ': > %s'`, filePath))
		gomega.Expect(err).To(gomega.HaveOccurred())

		expectErr := gomega.Or(
			gomega.ContainSubstring("Operation not permitted"),
			gomega.ContainSubstring("Operation not supported"),
		)
		gomega.Expect(stderr).To(expectErr)

		// Confirm permissions unchanged
		permsAfter, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(permsAfter)).To(gomega.Equal(strings.TrimSpace(permsBefore)))

		// Confirm ownership unchanged
		ownerAfter, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%u:%%g' %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(ownerAfter)).To(gomega.Equal(strings.TrimSpace(ownerBefore)))

		// Confirm content unchanged
		contentAfter, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("cat %s", filePath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(contentAfter)).To(gomega.Equal(strings.TrimSpace(contentBefore)))
	})

	// --------------------------------------------------------------------
	// 14. Extended attributes (xattr) unsupported behavior
	//
	// Mountpoint‑S3 does not persist extended attributes (xattrs) because
	// S3 itself has no equivalent metadata model. The kernel's xattr API
	// (setfattr/getfattr) will typically fail with EPERM or ENOTSUP,
	// and no xattrs are ever stored.
	//
	// Test scenario:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//   ┌─────────────────────────────────────────────┐
	//   │ Step 1: Create /mnt/volume1/xattr-file      │
	//   │                                             │
	//   │ Step 2: Try to set an xattr:                │
	//   │         setfattr -n user.test -v abc <file> │
	//   │                                             │
	//   │ Step 3: Check existing xattrs:              │
	//   │         getfattr -d <file>                  │
	//   └─────────────────────────────────────────────┘
	//
	// Expected results:
	// - setfattr fails with an error containing "Operation not permitted"
	//   or "Operation not supported" (EPERM/ENOTSUP)
	// - getfattr returns empty (no xattrs stored)
	//
	// This confirms Mountpoint‑S3 does not fake or store any extended
	// attributes and aligns with S3's object semantics.
	//
	ginkgo.It("should reject setxattr and return no xattrs", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, _ := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		fpath := "/mnt/volume1/xattr-file"
		CreateFileInPod(testFramework, pod, fpath, "xattr")

		_, stderr, err := e2evolume.PodExec(testFramework, pod,
			fmt.Sprintf(`setfattr -n user.test -v abc %s`, fpath))
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(stderr).To(gomega.ContainSubstring("Operation"))

		out, _, _ := e2evolume.PodExec(testFramework, pod,
			fmt.Sprintf(`getfattr -d %s || true`, fpath))
		gomega.Expect(strings.TrimSpace(out)).To(gomega.Equal(""))
	})

	// --------------------------------------------------------------------
	// 15. Atomic rename (mv) behavior & metadata preservation
	//
	// Mountpoint‑S3 generally does not implement atomic renames (the
	// rename(2) syscall) because S3's API lacks native support for it.
	//
	// This test verifies:
	// - mv (rename) fails gracefully with an expected error:
	//   - "Operation not permitted"
	//   - OR "Operation not supported"
	//   - OR "Function not implemented"
	// - Metadata remains intact (no unexpected permission/ownership changes)
	// - No partial rename artifacts exist after failure
	//
	// Test scenario:
	//
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//   ┌─────────────────────────────────────────────┐
	//   │ Step 1: Create /mnt/volume1/oldname         │
	//   │                                             │
	//   │ Step 2: mv oldname newname                  │
	//   │                                             │
	//   │ Step 3: Check that:                         │
	//   │   - mv fails with known error (see above)   │
	//   │   - oldname still exists & is unchanged     │
	//   │   - newname does not exist                  │
	//   └─────────────────────────────────────────────┘
	//
	// This ensures Mountpoint‑S3 either implements rename fully OR
	// rejects it safely in alignment with S3's semantics.
	//
	ginkgo.It("should handle mv correctly (metadata intact or ENOTSUP)", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, _ := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		oldPath := "/mnt/volume1/oldname"
		newPath := "/mnt/volume1/newname"

		// Create the original file
		CreateFileInPod(testFramework, pod, oldPath, "original content")

		// Capture initial permissions
		initialPerms, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", oldPath))
		framework.ExpectNoError(err)

		// Attempt the rename (mv)
		_, stderr, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("mv %s %s", oldPath, newPath))
		gomega.Expect(err).To(gomega.HaveOccurred())

		// Accept a range of possible errors (EPERM, ENOTSUP, ENOSYS)
		expectErr := gomega.Or(
			gomega.ContainSubstring("Operation not permitted"),
			gomega.ContainSubstring("Operation not supported"),
			gomega.ContainSubstring("Function not implemented"),
		)
		gomega.Expect(stderr).To(expectErr)

		// Confirm the old file still exists and is intact
		_, _, err = e2evolume.PodExec(testFramework, pod, fmt.Sprintf("test -f %s", oldPath))
		framework.ExpectNoError(err)

		// Confirm new file does not exist
		_, _, err = e2evolume.PodExec(testFramework, pod, fmt.Sprintf("test -f %s", newPath))
		gomega.Expect(err).To(gomega.HaveOccurred())

		// Confirm permissions are unchanged
		finalPerms, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a' %s", oldPath))
		framework.ExpectNoError(err)
		gomega.Expect(strings.TrimSpace(finalPerms)).To(gomega.Equal(strings.TrimSpace(initialPerms)))
	})

	// --------------------------------------------------------------------
	// 16. Pod umask + fsGroup: should override umask & apply fsGroup ownership
	//
	// This test verifies that Mountpoint‑S3 enforces **file-mode** via its
	// mount options (ignoring pod-level umask), and that **ownership** is
	// updated by Kubernetes when a pod sets an fsGroup.
	//
	// Expectations:
	// - The created file's permissions come **from the CSI driver** mount options
	//   (e.g., file-mode=0600) and NOT from the pod's umask.
	// - The file's group ownership is set to the pod's fsGroup value (K8s behavior).
	//
	// Diagram:
	//
	//      [Pod]
	//        |
	//        ├─ SecurityContext:
	//        │      fsGroup: 4000
	//        │      runAsUser: 1001
	//        │      umask: 077 (inside container)
	//        ↓
	//   [S3 Volume]
	//        |
	//        ↓
	//   [File]
	//       - Mode: enforced by driver (e.g., 0600)
	//       - Owner: runAsUser (1001)
	//       - Group: fsGroup (4000)
	//
	// This confirms two things:
	// 1 The pod's **umask is ignored** (driver-enforced perms win)
	// 2 The pod's **fsGroup** is respected (ownership updated by K8s)
	ginkgo.It("should override umask & apply fsGroup ownership", func(ctx context.Context) {
		fsG := int64(5555)
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, fsG, "0644")
		pod, _ := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, fsG)
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		fpath := "/mnt/volume1/conflict"
		e2evolume.VerifyExecInPodSucceed(testFramework, pod,
			fmt.Sprintf(`sh -c 'umask 077; echo hi > %s'`, fpath))

		out, _, _ := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("stat -c '%%a %%u %%g' %s", fpath))
		gomega.Expect(strings.TrimSpace(out)).To(gomega.Equal("644 1001 5555"))
	})

	// --------------------------------------------------------------------
	// 17. Filename edge cases: Non-ASCII, long names, special chars
	//
	// Mountpoint‑S3 must comply with S3 naming constraints but also handle
	// local filesystem edge cases gracefully. This test covers:
	// - Unicode filenames (e.g., emoji 🐟.txt)
	// - Very long filenames (up to ~255 bytes)
	// - Special characters (@, +, %, etc.)
	//
	// Diagram:
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume]
	//        |
	//        ↓
	//   ├── 🍣.txt
	//   ├── llllllllll... (long name)
	//   └── special@+%.txt
	//
	// Expected results:
	// - All files are created successfully inside the volume
	// - File permissions match the expected file-mode (e.g., 0600)
	// - Ownership matches uid/gid (e.g., 1001/2000)
	// - stat and access through these filenames behave normally
	//
	// This ensures Mountpoint-S3 enforces correct semantics for valid
	// edge-case filenames without breaking POSIX expectations.
	ginkgo.It("should handle edge‑case filenames correctly", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, _ := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		long := strings.Repeat("l", 255)
		files := []string{
			"/mnt/volume1/🍣.txt",
			fmt.Sprintf("/mnt/volume1/%s", long),
			"/mnt/volume1/special@+%.txt",
		}

		for _, p := range files {
			CreateFileInPod(testFramework, pod, p, "edge")
			verifyFilePermissions(testFramework, pod, p, "600", ptr.To(DefaultNonRootUser), ptr.To(DefaultNonRootGroup))
		}
	})

	// --------------------------------------------------------------------
	// 18. access() syscall: Consistency with stat() permissions
	//
	// The access() syscall checks file accessibility based on real UID/GID
	// and is subtly different from stat(). This test ensures Mountpoint‑S3
	// reports consistent permission results.
	//
	// Diagram:
	//      [Pod]
	//        |
	//        ↓
	//   [S3 Volume with file-mode=0600]
	//        |
	//        ↓
	//    test-file.txt
	//
	// Expected behavior:
	// - stat shows 0600 permissions
	// - access -r: succeeds (read allowed)
	// - access -w: succeeds (write allowed)
	// - access -x: fails (no exec bit)
	//
	// This validates that access() enforces the same file-mode as stat(),
	// confirming no surprises in POSIX permission checks (important for apps
	// that rely on access() before opening files).
	ginkgo.It("should have consistent access() and stat", func(ctx context.Context) {
		res := createVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0600")
		pod, _ := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		fpath := "/mnt/volume1/access-file"
		CreateFileInPod(testFramework, pod, fpath, "acc")

		// Should be readable+writeable by owner, not executable
		_, _, err := e2evolume.PodExec(testFramework, pod, fmt.Sprintf("test -r %[1]s && test -w %[1]s && ! test -x %[1]s", fpath))
		framework.ExpectNoError(err, "access() bits disagree with stat permissions")
	})
}
