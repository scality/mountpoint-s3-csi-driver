/*
Copyright 2023 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	custom_testsuites "github.com/awslabs/aws-s3-csi-driver/tests/e2e-kubernetes/testsuites"
)

type s3CSIFileOperationsTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

func InitS3CSIFileOperationsTestSuite() storageframework.TestSuite {
	return &s3CSIFileOperationsTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "fileoperations",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsPreprovisionedPV,
			},
		},
	}
}

func (t *s3CSIFileOperationsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

func (t *s3CSIFileOperationsTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, pattern storageframework.TestPattern) {
	if pattern.VolType != storageframework.PreprovisionedPV {
		framework.Skipf("Suite %q does not support %v", t.tsInfo.Name, pattern.VolType)
	}
}

func (t *s3CSIFileOperationsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var (
		l local
	)

	f := framework.NewFrameworkWithCustomTimeouts(custom_testsuites.NamespacePrefix+"fileoperations", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelBaseline

	cleanup := func(ctx context.Context) {
		var errs []error
		for _, resource := range l.resources {
			errs = append(errs, resource.CleanupResource(ctx))
		}
		framework.ExpectNoError(errors.NewAggregate(errs), "while cleanup resource")
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		l = local{}
		l.config = driver.PrepareTest(ctx, f)
		ginkgo.DeferCleanup(cleanup)
	})

	// Helper functions
	checkFileExists := func(pod *v1.Pod, path string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -f %s", path))
	}

	checkDirExists := func(pod *v1.Pod, path string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("test -d %s", path))
	}

	createFileWithContent := func(pod *v1.Pod, path, content string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo '%s' > %s", content, path))
	}

	appendToFile := func(pod *v1.Pod, path, content string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("echo '%s' >> %s", content, path))
	}

	verifyFileContent := func(pod *v1.Pod, path, expectedContent string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("grep -q '%s' %s", expectedContent, path))
	}

	verifyFileSize := func(pod *v1.Pod, path string, expectedSize int) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%s' %s | grep '%d'", path, expectedSize))
	}

	verifyFilePermissions := func(pod *v1.Pod, path string, expectedPermissions string) {
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%a' %s | grep '%s'", path, expectedPermissions))
	}

	listDirectory := func(pod *v1.Pod, path string) string {
		stdout, _, err := e2evolume.PodExec(f, pod, fmt.Sprintf("ls -1 %s", path))
		framework.ExpectNoError(err)
		return stdout
	}

	checkListingPathWithEntries := func(pod *v1.Pod, path string, entries []string) {
		cmd := fmt.Sprintf("ls -1 %s", path)
		stdout, stderr, err := e2evolume.PodExec(f, pod, cmd)
		framework.ExpectNoError(err,
			"%q should succeed, but failed with error message %q\nstdout: %s\nstderr: %s",
			cmd, err, stdout, stderr)

		// Split output by newlines and remove empty strings
		fileList := strings.Split(strings.TrimSpace(stdout), "\n")
		gomega.Expect(fileList).To(gomega.ConsistOf(entries))
	}

	// Main test functions
	testBasicFileOperations := func(ctx context.Context) {
		resource := custom_testsuites.CreateVolumeResourceWithMountOptions(ctx, l.config, pattern, []string{"allow-delete"})
		l.resources = append(l.resources, resource)

		ginkgo.By("Creating pod with a volume")
		pod := e2epod.MakePod(f.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelBaseline, "")
		var err error
		pod, err = custom_testsuites.CreatePod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		basePath := "/mnt/volume1"

		// Test file creation
		ginkgo.By("Creating files of different sizes")
		emptyFile := filepath.Join(basePath, "empty.txt")
		smallFile := filepath.Join(basePath, "small.txt")
		mediumFile := filepath.Join(basePath, "medium.txt")

		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("touch %s", emptyFile))
		createFileWithContent(pod, smallFile, "This is a small file")

		// Create a 100KB file
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=/dev/urandom of=%s bs=1024 count=100", mediumFile))

		// Verify files exist
		checkFileExists(pod, emptyFile)
		checkFileExists(pod, smallFile)
		checkFileExists(pod, mediumFile)

		// Verify file sizes
		verifyFileSize(pod, emptyFile, 0)
		verifyFileContent(pod, smallFile, "This is a small file")

		// Test file with special characters
		specialCharFile := filepath.Join(basePath, "special_#$%.txt")
		createFileWithContent(pod, specialCharFile, "File with special chars in name")
		checkFileExists(pod, specialCharFile)

		// Test long filename (255 chars is max in many filesystems)
		longName := strings.Repeat("a", 200) + ".txt"
		longNameFile := filepath.Join(basePath, longName)
		createFileWithContent(pod, longNameFile, "File with very long name")
		checkFileExists(pod, longNameFile)

		// Test file updates
		ginkgo.By("Updating files")
		// Overwrite
		createFileWithContent(pod, smallFile, "This file has been overwritten")
		verifyFileContent(pod, smallFile, "This file has been overwritten")

		// Append
		appendToFile(pod, smallFile, "This text is appended")
		verifyFileContent(pod, smallFile, "This text is appended")

		// Test file deletion
		ginkgo.By("Deleting files")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("rm %s", emptyFile))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("rm %s", smallFile))

		// Verify files are deleted
		e2evolume.VerifyExecInPodFail(f, pod, fmt.Sprintf("test -f %s", emptyFile), 1)
		e2evolume.VerifyExecInPodFail(f, pod, fmt.Sprintf("test -f %s", smallFile), 1)

		// Try to delete non-existent file
		e2evolume.VerifyExecInPodFail(f, pod, fmt.Sprintf("rm %s", filepath.Join(basePath, "nonexistent.txt")), 1)
	}

	testDirectoryOperations := func(ctx context.Context) {
		resource := custom_testsuites.CreateVolumeResourceWithMountOptions(ctx, l.config, pattern, []string{"allow-delete"})
		l.resources = append(l.resources, resource)

		ginkgo.By("Creating pod with a volume")
		pod := e2epod.MakePod(f.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelBaseline, "")
		var err error
		pod, err = custom_testsuites.CreatePod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		basePath := "/mnt/volume1"

		// Test directory creation
		ginkgo.By("Creating directories")
		emptyDir := filepath.Join(basePath, "empty-dir")
		nestedPath := filepath.Join(basePath, "dir1/dir2/dir3")
		specialCharDir := filepath.Join(basePath, "special_$dir#")

		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir %s", emptyDir))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir -p %s", nestedPath))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir %s", specialCharDir))

		// Verify directories exist
		checkDirExists(pod, emptyDir)
		checkDirExists(pod, nestedPath)
		checkDirExists(pod, specialCharDir)

		// Test directory listing
		ginkgo.By("Listing directories")
		// List base directory with the created dirs
		checkListingPathWithEntries(pod, basePath, []string{"empty-dir", "dir1", "special_$dir#"})

		// Create files in directories
		createFileWithContent(pod, filepath.Join(emptyDir, "file1.txt"), "File 1")
		createFileWithContent(pod, filepath.Join(emptyDir, "file2.txt"), "File 2")
		createFileWithContent(pod, filepath.Join(nestedPath, "nested-file.txt"), "Nested file")

		// List directory with files
		checkListingPathWithEntries(pod, emptyDir, []string{"file1.txt", "file2.txt"})

		// Test directory deletion
		ginkgo.By("Deleting directories")
		// Delete empty dir first (after removing its files)
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("rm %s/file1.txt %s/file2.txt", emptyDir, emptyDir))
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("rmdir %s", emptyDir))

		// Delete non-empty directory with recursive flag
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("rm -rf %s", filepath.Join(basePath, "dir1")))

		// Verify directories are deleted
		e2evolume.VerifyExecInPodFail(f, pod, fmt.Sprintf("test -d %s", emptyDir), 1)
		e2evolume.VerifyExecInPodFail(f, pod, fmt.Sprintf("test -d %s", filepath.Join(basePath, "dir1")), 1)
	}

	testMetadataAndPermissions := func(ctx context.Context) {
		resource := custom_testsuites.CreateVolumeResourceWithMountOptions(ctx, l.config, pattern, []string{
			"allow-delete",
			"allow-other",
			fmt.Sprintf("uid=%d", custom_testsuites.DefaultNonRootUser),
			fmt.Sprintf("gid=%d", custom_testsuites.DefaultNonRootGroup),
		})
		l.resources = append(l.resources, resource)

		ginkgo.By("Creating pod with a volume")
		pod := e2epod.MakePod(f.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelBaseline, "")
		custom_testsuites.PodModifierNonRoot(pod)
		var err error
		pod, err = custom_testsuites.CreatePod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		basePath := "/mnt/volume1"
		testFile := filepath.Join(basePath, "permissions-test.txt")
		testDir := filepath.Join(basePath, "permissions-test-dir")

		// Create file and directory
		createFileWithContent(pod, testFile, "Testing permissions")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir %s", testDir))

		// Test file metadata
		ginkgo.By("Testing file metadata")
		// Check file size
		verifyFileSize(pod, testFile, 19) // "Testing permissions" = 19 bytes

		// Check file permissions
		ginkgo.By("Testing file permissions")
		verifyFilePermissions(pod, testFile, "644") // Default file permissions
		verifyFilePermissions(pod, testDir, "755")  // Default directory permissions

		// Check ownership
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -c '%%u:%%g' %s | grep '%d:%d'",
			testFile, custom_testsuites.DefaultNonRootUser, custom_testsuites.DefaultNonRootGroup))

		// Change permissions and verify
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("chmod 600 %s", testFile))
		verifyFilePermissions(pod, testFile, "600")
	}

	testConcurrentAccess := func(ctx context.Context) {
		resource := custom_testsuites.CreateVolumeResourceWithMountOptions(ctx, l.config, pattern, []string{"allow-delete"})
		l.resources = append(l.resources, resource)

		ginkgo.By("Creating multiple pods to access the same volume")
		const numPods = 3
		var pods []*v1.Pod

		// Create pods
		for i := 0; i < numPods; i++ {
			pod := e2epod.MakePod(f.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelBaseline, "")
			pod.Name = fmt.Sprintf("%s-concurrent-%d", f.Namespace.Name, i)
			var err error
			pod, err = custom_testsuites.CreatePod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err)
			defer func(p *v1.Pod) {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, p))
			}(pod)
			pods = append(pods, pod)
		}

		// Test multiple readers
		ginkgo.By("Testing multiple readers")
		sharedFile := "/mnt/volume1/shared-file.txt"

		// First pod creates the file
		content := "This is a shared file for multiple pods to read"
		createFileWithContent(pods[0], sharedFile, content)

		// All pods read the file
		for i, pod := range pods {
			ginkgo.By(fmt.Sprintf("Pod %d reading shared file", i))
			verifyFileContent(pod, sharedFile, content)
		}

		// Test multiple writers to different files
		ginkgo.By("Testing multiple writers to different files")

		// Each pod writes to its own file
		for i, pod := range pods {
			fileName := fmt.Sprintf("/mnt/volume1/pod-%d-file.txt", i)
			fileContent := fmt.Sprintf("This file was written by pod %d", i)
			createFileWithContent(pod, fileName, fileContent)
		}

		// Each pod verifies all files exist and have correct content
		for _, pod := range pods {
			for i := 0; i < numPods; i++ {
				fileName := fmt.Sprintf("/mnt/volume1/pod-%d-file.txt", i)
				fileContent := fmt.Sprintf("This file was written by pod %d", i)
				verifyFileContent(pod, fileName, fileContent)
			}
		}
	}

	testEdgeCases := func(ctx context.Context) {
		resource := custom_testsuites.CreateVolumeResourceWithMountOptions(ctx, l.config, pattern, []string{"allow-delete"})
		l.resources = append(l.resources, resource)

		ginkgo.By("Creating pod with a volume")
		pod := e2epod.MakePod(f.Namespace.Name, nil, []*v1.PersistentVolumeClaim{resource.Pvc}, admissionapi.LevelBaseline, "")
		var err error
		pod, err = custom_testsuites.CreatePod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		basePath := "/mnt/volume1"

		// Test path handling
		ginkgo.By("Testing path handling")
		// Create nested directory structure
		nestedDir := filepath.Join(basePath, "path/test/dir")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("mkdir -p %s", nestedDir))

		// Test relative paths
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("cd %s && echo 'relative path test' > ./rel-file.txt", basePath))
		checkFileExists(pod, filepath.Join(basePath, "rel-file.txt"))

		// Test path traversal
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("cd %s && echo 'path traversal test' > path/test/../traversal-file.txt", basePath))
		checkFileExists(pod, filepath.Join(basePath, "path/traversal-file.txt"))

		// Test special files
		ginkgo.By("Testing special files")
		// Zero-byte file
		zeroByteFile := filepath.Join(basePath, "zero-byte.txt")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("touch %s", zeroByteFile))
		verifyFileSize(pod, zeroByteFile, 0)

		// Large file (1MB)
		largeFile := filepath.Join(basePath, "large-file.bin")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("dd if=/dev/urandom of=%s bs=1M count=1", largeFile))
		checkFileExists(pod, largeFile)

		// File with unusual characters
		unusualCharsFile := filepath.Join(basePath, "unusual_チars_файл_αρχείο.txt")
		createFileWithContent(pod, unusualCharsFile, "File with Unicode characters in the name")
		checkFileExists(pod, unusualCharsFile)
	}

	// Define the tests
	ginkgo.It("should support basic file operations", func(ctx context.Context) {
		testBasicFileOperations(ctx)
	})

	ginkgo.It("should support directory operations", func(ctx context.Context) {
		testDirectoryOperations(ctx)
	})

	ginkgo.It("should handle file metadata and permissions", func(ctx context.Context) {
		testMetadataAndPermissions(ctx)
	})

	ginkgo.It("should support concurrent access from multiple pods", func(ctx context.Context) {
		testConcurrentAccess(ctx)
	})

	ginkgo.It("should handle edge cases", func(ctx context.Context) {
		testEdgeCases(ctx)
	})
}
