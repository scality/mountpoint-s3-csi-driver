// This file implements a dynamic provisioning test suite for the S3 CSI driver,
// which validates automatic bucket creation, StorageClass parameter handling,
// and volume lifecycle management according to CSI specifications.
package customsuites

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
)

// s3CSIDynamicProvisioningTestSuite implements a test suite for dynamic provisioning
// functionality with the S3 CSI driver, validating StorageClass-based volume creation,
// automatic bucket management, and CSI specification compliance.
type s3CSIDynamicProvisioningTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3DynamicProvisioningTestSuite initializes and returns a test suite that validates
// dynamic provisioning functionality for the S3 CSI driver.
//
// This suite specifically tests:
// - Dedicated bucket creation per PVC
// - Shared bucket provisioning with unique prefixes
// - StorageClass parameter validation and handling
// - Volume lifecycle management (create, mount, delete)
// - Reclaim policy enforcement (Delete vs Retain)
// - Error scenarios and edge cases
//
// The test suite ensures compliance with CSI dynamic provisioning specifications
// and validates that the driver correctly manages S3 bucket resources.
func InitS3DynamicProvisioningTestSuite() storageframework.TestSuite {
	return &s3CSIDynamicProvisioningTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "dynamicprovisioning",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPV,
			},
		},
	}
}

// GetTestSuiteInfo returns information about the test suite.
func (t *s3CSIDynamicProvisioningTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

// SkipUnsupportedTests allows test suites to skip certain tests based on driver capabilities.
// For S3 dynamic provisioning tests, all tests should be supported, so this is a no-op.
func (t *s3CSIDynamicProvisioningTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

// DefineTests defines all test cases for this test suite.
// The tests focus on validating dynamic provisioning behaviors according to CSI specifications.
func (t *s3CSIDynamicProvisioningTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		storageClasses []*storagev1.StorageClass
		pvcs           []*v1.PersistentVolumeClaim
		pvs            []*v1.PersistentVolume
		config         *storageframework.PerTestConfig
	}
	var l local

	f := framework.NewFrameworkWithCustomTimeouts("dynamicprovisioning", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	// cleanup function to be called after each test to ensure resources are properly deleted
	cleanup := func(ctx context.Context) {
		var errs []error

		// Clean up PVCs first (this should trigger PV deletion with Delete reclaim policy)
		for _, pvc := range l.pvcs {
			err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}

		// Clean up any remaining PVs
		for _, pv := range l.pvs {
			err := f.ClientSet.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}

		// Clean up StorageClasses
		for _, sc := range l.storageClasses {
			err := f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
			if err != nil {
				errs = append(errs, err)
			}
		}

		framework.ExpectNoError(errors.NewAggregate(errs), "while cleaning up resources")
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		l = local{}
		l.config = driver.PrepareTest(ctx, f)
		ginkgo.DeferCleanup(cleanup)
	})

	// Helper function to check if bucket exists
	bucketExists := func(ctx context.Context, bucketName string) (bool, error) {
		s3Client := s3client.New("", "", "")
		_, err := s3Client.ListObjects(ctx, bucketName)
		if err != nil {
			// If we get an error, assume bucket doesn't exist
			return false, nil
		}
		return true, nil
	}

	// Helper function to create a StorageClass for dynamic provisioning
	createStorageClass := func(ctx context.Context, name string, parameters map[string]string, reclaimPolicy *v1.PersistentVolumeReclaimPolicy) *storagev1.StorageClass {
		// Get the provisioner name from the driver
		provisioner := l.config.Driver.GetDriverInfo().Name

		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Provisioner:       provisioner,
			Parameters:        parameters,
			VolumeBindingMode: ptr.To(storagev1.VolumeBindingImmediate),
		}

		if reclaimPolicy != nil {
			sc.ReclaimPolicy = reclaimPolicy
		}

		var err error
		sc, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		l.storageClasses = append(l.storageClasses, sc)
		return sc
	}

	// Helper function to create a PVC that uses dynamic provisioning
	createDynamicPVC := func(ctx context.Context, name, storageClassName string, size string) *v1.PersistentVolumeClaim {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse(size),
					},
				},
				StorageClassName: ptr.To(storageClassName),
			},
		}

		var err error
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		l.pvcs = append(l.pvcs, pvc)
		return pvc
	}

	// Helper function to wait for PVC to be bound and get the corresponding PV
	waitForPVCBound := func(ctx context.Context, pvc *v1.PersistentVolumeClaim) *v1.PersistentVolume {
		// Wait for PVC to be bound
		err := e2epv.WaitForPersistentVolumeClaimPhase(ctx, v1.ClaimBound, f.ClientSet, f.Namespace.Name, pvc.Name, 2*time.Second, 2*time.Minute)
		framework.ExpectNoError(err)

		// Get the updated PVC to find the bound PV
		pvc, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err)

		// Get the PV
		pv, err := f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		framework.ExpectNoError(err)
		l.pvs = append(l.pvs, pv)
		return pv
	}

	// Test 1: Basic dedicated bucket dynamic provisioning
	ginkgo.It("should create dedicated bucket per PVC and allow file operations", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass for dedicated bucket provisioning")
		scName := "s3-csi-dedicated-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "dedicated",
			"s3Region":     "us-east-1",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating PVC to trigger dynamic provisioning")
		pvcName := "test-pvc-dedicated-" + uuid.New().String()[:8]
		pvc := createDynamicPVC(ctx, pvcName, sc.Name, "10Gi")

		ginkgo.By("Waiting for PVC to be bound")
		pv := waitForPVCBound(ctx, pvc)

		// Verify the PV has the correct CSI attributes
		ginkgo.By("Verifying PV has correct attributes")
		gomega.Expect(pv.Spec.CSI).NotTo(gomega.BeNil())
		gomega.Expect(pv.Spec.CSI.VolumeAttributes).To(gomega.HaveKey("bucketName"))
		bucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]
		gomega.Expect(bucketName).NotTo(gomega.BeEmpty())

		ginkgo.By("Verifying bucket was created in S3")
		exists, err := bucketExists(ctx, bucketName)
		framework.ExpectNoError(err)
		gomega.Expect(exists).To(gomega.BeTrue(), "Bucket should be created by dynamic provisioning")

		ginkgo.By("Creating pod with dynamically provisioned volume")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, f, pvc, "dynamic-test", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		ginkgo.By("Writing and reading files in the dynamically provisioned volume")
		testFile := "/mnt/volume1/dynamic-test.txt"
		testContent := "Dynamic provisioning test content"
		WriteAndVerifyFile(f, pod, testFile, testContent)

		ginkgo.By("Verifying file exists in S3 bucket")
		s3Client := s3client.New("", "", "")
		err = s3Client.VerifyObjectsExistInS3(ctx, bucketName, "", []string{"dynamic-test.txt"})
		framework.ExpectNoError(err)
	})

	// Test 2: Shared bucket dynamic provisioning
	ginkgo.It("should provision volumes with unique prefixes in shared bucket", func(ctx context.Context) {
		// First, create a shared bucket manually
		s3Client := s3client.New("", "", "")
		sharedBucketName, deleteBucket := s3Client.CreateBucket(ctx)
		defer func() { deleteBucket(ctx) }()

		ginkgo.By("Creating StorageClass for shared bucket provisioning")
		scName := "s3-csi-shared-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "shared",
			"bucketPrefix": sharedBucketName,
			"s3Region":     "us-east-1",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating first PVC to trigger dynamic provisioning")
		pvc1Name := "test-pvc-shared1-" + uuid.New().String()[:8]
		pvc1 := createDynamicPVC(ctx, pvc1Name, sc.Name, "5Gi")

		ginkgo.By("Creating second PVC to trigger dynamic provisioning")
		pvc2Name := "test-pvc-shared2-" + uuid.New().String()[:8]
		pvc2 := createDynamicPVC(ctx, pvc2Name, sc.Name, "5Gi")

		ginkgo.By("Waiting for both PVCs to be bound")
		pv1 := waitForPVCBound(ctx, pvc1)
		pv2 := waitForPVCBound(ctx, pvc2)

		// Verify both PVs reference the same bucket but different prefixes
		ginkgo.By("Verifying PVs have correct attributes")
		bucketName1 := pv1.Spec.CSI.VolumeAttributes["bucketName"]
		bucketName2 := pv2.Spec.CSI.VolumeAttributes["bucketName"]
		prefix1 := pv1.Spec.CSI.VolumeAttributes["prefix"]
		prefix2 := pv2.Spec.CSI.VolumeAttributes["prefix"]

		gomega.Expect(bucketName1).To(gomega.Equal(sharedBucketName))
		gomega.Expect(bucketName2).To(gomega.Equal(sharedBucketName))
		gomega.Expect(prefix1).NotTo(gomega.BeEmpty())
		gomega.Expect(prefix2).NotTo(gomega.BeEmpty())
		gomega.Expect(prefix1).NotTo(gomega.Equal(prefix2), "Each PVC should get a unique prefix")

		ginkgo.By("Creating pods with both volumes")
		pod1, err := CreatePodWithVolumeAndSecurity(ctx, f, pvc1, "shared-test1", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod1))
		}()

		pod2, err := CreatePodWithVolumeAndSecurity(ctx, f, pvc2, "shared-test2", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod2))
		}()

		ginkgo.By("Writing files in both volumes")
		WriteAndVerifyFile(f, pod1, "/mnt/volume1/file1.txt", "Content from volume 1")
		WriteAndVerifyFile(f, pod2, "/mnt/volume1/file2.txt", "Content from volume 2")

		ginkgo.By("Verifying files exist under correct prefixes in S3")
		s3ClientVerify := s3client.New("", "", "")
		err = s3ClientVerify.VerifyObjectsExistInS3(ctx, sharedBucketName, prefix1, []string{"file1.txt"})
		framework.ExpectNoError(err)
		err = s3ClientVerify.VerifyObjectsExistInS3(ctx, sharedBucketName, prefix2, []string{"file2.txt"})
		framework.ExpectNoError(err)

		ginkgo.By("Verifying files are isolated between volumes")
		// File from volume 1 should not be visible in volume 2
		_, _, err = e2evolume.PodExec(f, pod2, "test -f /mnt/volume1/file1.txt")
		gomega.Expect(err).To(gomega.HaveOccurred(), "Files should be isolated between volumes")

		// File from volume 2 should not be visible in volume 1
		_, _, err = e2evolume.PodExec(f, pod1, "test -f /mnt/volume1/file2.txt")
		gomega.Expect(err).To(gomega.HaveOccurred(), "Files should be isolated between volumes")
	})

	// Test 3: Volume deletion with Delete reclaim policy
	ginkgo.It("should delete bucket when PVC is deleted with Delete reclaim policy", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with Delete reclaim policy")
		scName := "s3-csi-delete-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "dedicated",
			"s3Region":     "us-east-1",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating PVC to trigger dynamic provisioning")
		pvcName := "test-pvc-delete-" + uuid.New().String()[:8]
		pvc := createDynamicPVC(ctx, pvcName, sc.Name, "5Gi")

		ginkgo.By("Waiting for PVC to be bound")
		pv := waitForPVCBound(ctx, pvc)
		bucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]

		ginkgo.By("Verifying bucket exists")
		exists, err := bucketExists(ctx, bucketName)
		framework.ExpectNoError(err)
		gomega.Expect(exists).To(gomega.BeTrue())

		ginkgo.By("Creating and using the volume to add some content")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, f, pvc, "delete-test", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		WriteAndVerifyFile(f, pod, "/mnt/volume1/test-file.txt", "test content")
		framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))

		ginkgo.By("Deleting PVC to trigger volume deletion")
		err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		framework.ExpectNoError(err)

		ginkgo.By("Waiting for PV to be deleted")
		err = e2epv.WaitForPersistentVolumeDeleted(ctx, f.ClientSet, pv.Name, 5*time.Second, 3*time.Minute)
		framework.ExpectNoError(err)

		ginkgo.By("Verifying bucket is deleted from S3")
		// Give some time for the deletion to propagate
		time.Sleep(10 * time.Second)
		exists, err = bucketExists(ctx, bucketName)
		framework.ExpectNoError(err)
		gomega.Expect(exists).To(gomega.BeFalse(), "Bucket should be deleted when PVC is deleted with Delete reclaim policy")
	})

	// Test 4: Multiple StorageClass configurations
	ginkgo.It("should support different StorageClass configurations", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with custom mount options")
		scName := "s3-csi-custom-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "dedicated",
			"s3Region":     "us-west-2",
			"mountOptions": "debug,allow-delete",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating PVC with custom configuration")
		pvcName := "test-pvc-custom-" + uuid.New().String()[:8]
		pvc := createDynamicPVC(ctx, pvcName, sc.Name, "8Gi")

		ginkgo.By("Waiting for PVC to be bound")
		pv := waitForPVCBound(ctx, pvc)

		ginkgo.By("Verifying PV has custom mount options")
		gomega.Expect(pv.Spec.MountOptions).To(gomega.ContainElement("debug"))
		gomega.Expect(pv.Spec.MountOptions).To(gomega.ContainElement("allow-delete"))

		ginkgo.By("Creating pod to test custom configuration")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, f, pvc, "custom-test", DefaultNonRootUser, DefaultNonRootGroup)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		ginkgo.By("Testing file operations with custom mount options")
		WriteAndVerifyFile(f, pod, "/mnt/volume1/custom-test.txt", "Custom configuration test")

		// Test delete functionality (enabled by allow-delete mount option)
		e2evolume.VerifyExecInPodSucceed(f, pod, "rm /mnt/volume1/custom-test.txt")
		_, _, err = e2evolume.PodExec(f, pod, "test -f /mnt/volume1/custom-test.txt")
		gomega.Expect(err).To(gomega.HaveOccurred(), "File should be deleted when allow-delete is enabled")
	})

	// Test 5: Error scenarios
	ginkgo.It("should handle invalid StorageClass parameters gracefully", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with invalid parameters")
		scName := "s3-csi-invalid-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "invalid-mode",
			"s3Region":     "invalid-region",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating PVC with invalid StorageClass")
		pvcName := "test-pvc-invalid-" + uuid.New().String()[:8]
		pvc := createDynamicPVC(ctx, pvcName, sc.Name, "5Gi")

		ginkgo.By("Waiting for PVC to remain pending due to provisioning failure")
		// PVC should not be bound due to invalid parameters
		time.Sleep(30 * time.Second) // Give time for provisioner to attempt and fail

		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err)
		gomega.Expect(pvc.Status.Phase).To(gomega.Equal(v1.ClaimPending), "PVC should remain pending due to provisioning failure")

		ginkgo.By("Checking for error events")
		events, err := f.ClientSet.CoreV1().Events(f.Namespace.Name).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=PersistentVolumeClaim", pvc.Name),
		})
		framework.ExpectNoError(err)

		foundError := false
		for _, event := range events.Items {
			if event.Type == "Warning" && strings.Contains(strings.ToLower(event.Message), "error") {
				framework.Logf("Found expected error event: %s", event.Message)
				foundError = true
				break
			}
		}
		gomega.Expect(foundError).To(gomega.BeTrue(), "Should find error event for invalid StorageClass parameters")
	})

	// Test 6: Concurrent volume creation
	ginkgo.It("should handle concurrent volume creation requests", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass for concurrent test")
		scName := "s3-csi-concurrent-" + uuid.New().String()[:8]
		sc := createStorageClass(ctx, scName, map[string]string{
			"bucketNaming": "dedicated",
			"s3Region":     "us-east-1",
		}, ptr.To(v1.PersistentVolumeReclaimDelete))

		ginkgo.By("Creating multiple PVCs concurrently")
		const numPVCs = 3
		pvcs := make([]*v1.PersistentVolumeClaim, numPVCs)

		for i := 0; i < numPVCs; i++ {
			pvcName := fmt.Sprintf("test-pvc-concurrent-%d-%s", i, uuid.New().String()[:8])
			pvcs[i] = createDynamicPVC(ctx, pvcName, sc.Name, "5Gi")
		}

		ginkgo.By("Waiting for all PVCs to be bound")
		pvs := make([]*v1.PersistentVolume, numPVCs)
		bucketNames := make([]string, numPVCs)

		for i := 0; i < numPVCs; i++ {
			pvs[i] = waitForPVCBound(ctx, pvcs[i])
			bucketNames[i] = pvs[i].Spec.CSI.VolumeAttributes["bucketName"]
			gomega.Expect(bucketNames[i]).NotTo(gomega.BeEmpty())
		}

		ginkgo.By("Verifying all buckets are unique")
		for i := 0; i < numPVCs; i++ {
			for j := i + 1; j < numPVCs; j++ {
				gomega.Expect(bucketNames[i]).NotTo(gomega.Equal(bucketNames[j]),
					"Each PVC should get a unique bucket")
			}
		}

		ginkgo.By("Verifying all buckets exist in S3")
		for i, bucketName := range bucketNames {
			exists, err := bucketExists(ctx, bucketName)
			framework.ExpectNoError(err)
			gomega.Expect(exists).To(gomega.BeTrue(), "Bucket %s for PVC %d should exist", bucketName, i)
		}

		ginkgo.By("Testing concurrent access to volumes")
		pods := make([]*v1.Pod, numPVCs)
		for i := 0; i < numPVCs; i++ {
			var err error
			pods[i], err = CreatePodWithVolumeAndSecurity(ctx, f, pvcs[i],
				fmt.Sprintf("concurrent-test-%d", i), DefaultNonRootUser, DefaultNonRootGroup)
			framework.ExpectNoError(err)
			defer func(pod *v1.Pod) {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}(pods[i])
		}

		ginkgo.By("Writing unique files in each volume")
		for i := 0; i < numPVCs; i++ {
			testContent := fmt.Sprintf("Concurrent test content for volume %d", i)
			WriteAndVerifyFile(f, pods[i], "/mnt/volume1/concurrent-test.txt", testContent)
		}
	})
}
