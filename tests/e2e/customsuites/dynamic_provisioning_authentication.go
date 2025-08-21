package customsuites

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/utils/ptr"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"
)

// s3DynamicProvisioningAuthTestSuite implements TestSuite for testing credential hierarchy
// and authentication parameters in dynamic provisioning scenarios
type s3DynamicProvisioningAuthTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3DynamicProvisioningAuthTestSuite creates the test suite for dynamic provisioning authentication
func InitS3DynamicProvisioningAuthTestSuite() storageframework.TestSuite {
	return &s3DynamicProvisioningAuthTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "s3DynamicProvisioningAuth",
			TestPatterns: []storageframework.TestPattern{
				// Use DynamicPV pattern for dynamic provisioning tests
				{
					Name:    "Dynamic PV Auth Test",
					VolType: storageframework.DynamicPV,
				},
			},
		},
	}
}

// GetTestSuiteInfo returns the test suite information
func (t *s3DynamicProvisioningAuthTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

// SkipUnsupportedTests skips tests not applicable to this driver
func (t *s3DynamicProvisioningAuthTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	if pattern.VolType != storageframework.DynamicPV {
		ginkgo.Skip("S3 Dynamic Provisioning Auth test only supports DynamicPV")
	}
}

// DefineTests defines the authentication-specific tests for dynamic provisioning
func (t *s3DynamicProvisioningAuthTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var l local

	// Create a framework with custom timeouts based on the driver's requirements
	f := framework.NewFrameworkWithCustomTimeouts("s3-dynamic-auth", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityEnforceLevel = "privileged"

	// cleanup function to be called after each test to ensure resources are properly deleted
	cleanup := func(ctx context.Context) {
		for _, resource := range l.resources {
			if resource != nil {
				_ = resource.CleanupResource(ctx)
			}
		}
		l.resources = nil
	}

	// Test credential hierarchy: driver → provisioner → node-publish
	ginkgo.It("should use correct credential hierarchy for dynamic provisioning", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating StorageClass with full authentication parameters")

		// Create provisioner and node-publish secrets
		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		nodeSecretName, err := CreateNodePublishSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create node-publish secret")

		// Create StorageClass with both types of secrets
		scName := fmt.Sprintf("s3-auth-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":       provSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace":  f.Namespace.Name,
				"csi.storage.k8s.io/node-publish-secret-name":      nodeSecretName,
				"csi.storage.k8s.io/node-publish-secret-namespace": f.Namespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with authentication-enabled StorageClass")

		pvcName := fmt.Sprintf("auth-pvc-%s", uuid.NewString()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &sc.Name,
			},
		}

		_, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		ginkgo.By("Waiting for PVC to be bound with proper authentication")

		WaitForPVCToBeBoundWithTimeout(ctx, f, pvc.Name, f.Namespace.Name, 120*time.Second, 5*time.Second)

		ginkgo.By("Verifying PV contains correct volume attributes")

		boundPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get bound PVC")

		pv, err := f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, boundPVC.Spec.VolumeName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get PV")

		// Verify volume context contains authentication information
		VerifyVolumeContext(pv, map[string]string{
			"authenticationSource": "secret", // Should indicate secret-based auth
		})
	})

	// Test volume context contains correct authentication source
	ginkgo.It("should set volume context with correct authentication source", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating dynamic PV through standard test driver")

		// Use the test driver to create a dynamic StorageClass
		// Our driver always supports dynamic provisioning now
		dynamicDriver := driver.(storageframework.DynamicPVTestDriver)

		sc := dynamicDriver.GetDynamicProvisionStorageClass(ctx, l.config, "")
		_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC and verifying volume context")

		pvcName := fmt.Sprintf("context-pvc-%s", uuid.NewString()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &sc.Name,
			},
		}

		_, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		ginkgo.By("Waiting for binding and verifying volume attributes")

		var boundPV *v1.PersistentVolume
		gomega.Eventually(func(ctx context.Context) bool {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil || updatedPVC.Status.Phase != v1.ClaimBound {
				return false
			}

			boundPV, err = f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, updatedPVC.Spec.VolumeName, metav1.GetOptions{})
			return err == nil && boundPV != nil
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.BeTrue(), "PVC should be bound and PV should be available")

		// DEBUG: Print all volume context attributes
		ginkgo.By("DEBUG: Printing volume context for analysis")
		framework.Logf("Volume context attributes: %+v", boundPV.Spec.CSI.VolumeAttributes)

		// Also print the StorageClass parameters for debugging
		debugSC, err := f.ClientSet.StorageV1().StorageClasses().Get(ctx, sc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get StorageClass for debug")
		framework.Logf("StorageClass parameters: %+v", debugSC.Parameters)

		// Verify the volume context contains the correct authentication source
		VerifyVolumeContext(boundPV, map[string]string{
			"authenticationSource": "secret",
		})
	})

	// Test cross-namespace secret access
	ginkgo.It("should handle cross-namespace secret access correctly", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating secrets in different namespaces")

		// Create a separate namespace for secrets
		secretNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("secret-ns-%s", uuid.NewString()[:8]),
			},
		}
		_, err := f.ClientSet.CoreV1().Namespaces().Create(ctx, secretNamespace, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create secret namespace")

		defer func() {
			_ = f.ClientSet.CoreV1().Namespaces().Delete(ctx, secretNamespace.Name, metav1.DeleteOptions{})
		}()

		// Create secrets in the separate namespace
		secretFramework := &framework.Framework{
			BaseName:  f.BaseName,
			ClientSet: f.ClientSet,
			Namespace: secretNamespace,
		}

		provSecretName, err := CreateProvisionerSecret(ctx, secretFramework)
		framework.ExpectNoError(err, "Failed to create provisioner secret in separate namespace")

		nodeSecretName, err := CreateNodePublishSecret(ctx, secretFramework)
		framework.ExpectNoError(err, "Failed to create node-publish secret in separate namespace")

		ginkgo.By("Creating StorageClass referencing cross-namespace secrets")

		scName := fmt.Sprintf("cross-ns-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":       provSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace":  secretNamespace.Name, // Different namespace
				"csi.storage.k8s.io/node-publish-secret-name":      nodeSecretName,
				"csi.storage.k8s.io/node-publish-secret-namespace": secretNamespace.Name, // Different namespace
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create cross-namespace StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with cross-namespace secret references")

		pvcName := fmt.Sprintf("cross-ns-pvc-%s", uuid.NewString()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name, // PVC in original namespace
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &sc.Name,
			},
		}

		_, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		ginkgo.By("Verifying cross-namespace authentication works")

		WaitForPVCToBeBoundWithTimeout(ctx, f, pvc.Name, f.Namespace.Name, 120*time.Second, 5*time.Second)
	})

	// Test bucket ownership verification with account other than driver default
	ginkgo.It("should create bucket with account other than driver default when both provisioner and node secrets use same account", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating StorageClass with account2 credentials for both provisioner and node secrets")

		// Create account2 provisioner and node-publish secrets
		lisaProvSecretName, err := CreateLisaProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create account2 provisioner secret")

		lisaNodeSecretName, err := CreateLisaNodePublishSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create account2 node-publish secret")

		// Create StorageClass with both account2 secrets
		scName := fmt.Sprintf("account2-auth-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":       lisaProvSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace":  f.Namespace.Name,
				"csi.storage.k8s.io/node-publish-secret-name":      lisaNodeSecretName,
				"csi.storage.k8s.io/node-publish-secret-namespace": f.Namespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass with Lisa's credentials")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with account2 authentication-enabled StorageClass")

		pvcName := fmt.Sprintf("account2-pvc-%s", uuid.NewString()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &sc.Name,
			},
		}

		_, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC with account2 StorageClass")

		ginkgo.By("Waiting for PVC to be bound with account2 authentication")

		var boundPV *v1.PersistentVolume
		gomega.Eventually(func(ctx context.Context) bool {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil || updatedPVC.Status.Phase != v1.ClaimBound {
				return false
			}

			boundPV, err = f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, updatedPVC.Spec.VolumeName, metav1.GetOptions{})
			return err == nil && boundPV != nil
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.BeTrue(), "PVC should be bound and PV should be available")

		// Get the bucket name from the PV
		bucketName := boundPV.Spec.CSI.VolumeAttributes["bucketName"]
		framework.Logf("Dynamic provisioning created bucket: %s", bucketName)

		ginkgo.By("Mounting the volume and creating a test file")

		// Create a test pod to mount the volume and write a file
		podName := fmt.Sprintf("account2-test-pod-%s", uuid.NewString()[:8])
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test-container",
						Image: "busybox:1.36",
						Command: []string{
							"/bin/sh",
							"-c",
							"while true; do sleep 30; done",
						},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "account2-volume",
								MountPath: "/mnt/account2-data",
							},
						},
					},
				},
				Volumes: []v1.Volume{
					{
						Name: "account2-volume",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
				RestartPolicy: v1.RestartPolicyNever,
			},
		}

		_, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create test pod")

		defer func() {
			_ = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		}()

		// Wait for pod to be ready
		framework.ExpectNoError(e2epod.WaitForPodRunningInNamespace(ctx, f.ClientSet, pod), "Pod should be running")

		// Create a test file
		testFileName := "account2-ownership-test.txt"
		testFilePath := "/mnt/account2-data/" + testFileName
		testFileContent := "This file was created with account2 credentials"

		framework.Logf("Creating test file %s in pod %s", testFilePath, pod.Name)
		WriteAndVerifyFile(f, pod, testFilePath, testFileContent)

		ginkgo.By("Verifying bucket ownership using account2 S3 client")

		// Create account2 S3 client to verify ownership
		account2AccessKey := GetEnv("ACCOUNT2_ACCESS_KEY", "")
		account2SecretKey := GetEnv("ACCOUNT2_SECRET_KEY", "")
		account2CanonicalID := GetEnv("ACCOUNT2_CANONICAL_ID", "79a59df900b949e55d96a1e698fbacedfd6e09d98eacf8f8d5218e7cd47ef2bf")

		// Ensure we have account2 credentials
		if account2AccessKey == "" || account2SecretKey == "" {
			ginkgo.Fail("Account2 credentials not available for bucket ownership verification")
		}

		// Create S3 client with account2 credentials
		account2S3Client := s3client.New("", account2AccessKey, account2SecretKey)

		// Verify the test file object has account2's canonical ID as owner
		framework.Logf("Verifying object %s has account2 canonical ID: %s", testFileName, account2CanonicalID)
		ownerID, err := account2S3Client.GetObjectOwnerID(ctx, bucketName, testFileName)
		framework.ExpectNoError(err, "Failed to get object owner ID")

		gomega.Expect(ownerID).To(gomega.Equal(account2CanonicalID),
			"Object should be owned by account2 (canonical ID: %s), but got: %s. This indicates the bucket was not created with account2 credentials as expected.", account2CanonicalID, ownerID)

		framework.Logf("SUCCESS: Bucket ownership verification passed - object owner ID %s matches account2 canonical ID %s", ownerID, account2CanonicalID)

		ginkgo.By("Verifying volume context indicates secret authentication")

		// Verify the volume context contains the correct authentication source
		VerifyVolumeContext(boundPV, map[string]string{
			"authenticationSource": "secret", // Should indicate secret-based auth was used
			"bucketName":           bucketName,
		})

		framework.Logf("Bucket ownership test completed successfully - Lisa's credentials were used for dynamic provisioning")
	})
}

// VerifyVolumeContext verifies that a PersistentVolume contains the expected volume attributes
func VerifyVolumeContext(pv *v1.PersistentVolume, expected map[string]string) {
	ginkgo.GinkgoHelper()

	if pv.Spec.CSI == nil {
		ginkgo.Fail("PersistentVolume should have CSI spec")
	}

	volumeAttrs := pv.Spec.CSI.VolumeAttributes
	if volumeAttrs == nil {
		ginkgo.Fail("PersistentVolume should have volume attributes")
	}

	for key, expectedValue := range expected {
		actualValue, exists := volumeAttrs[key]
		if !exists {
			ginkgo.Fail(fmt.Sprintf("Volume attribute %s not found in PV", key))
		}
		if actualValue != expectedValue {
			ginkgo.Fail(fmt.Sprintf("Volume attribute %s has value %s, expected %s", key, actualValue, expectedValue))
		}
	}

	framework.Logf("Volume context verification passed: %v", volumeAttrs)
}
