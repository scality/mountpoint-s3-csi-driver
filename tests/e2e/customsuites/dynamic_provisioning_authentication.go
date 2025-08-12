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
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/utils/ptr"
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

		gomega.Eventually(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimPending
			}
			return updatedPVC.Status.Phase
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimBound), "PVC should be bound with authentication")

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
		dynamicDriver, ok := driver.(storageframework.DynamicPVTestDriver)
		if !ok {
			ginkgo.Skip("Driver does not support dynamic provisioning")
		}

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

		gomega.Eventually(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimPending
			}
			return updatedPVC.Status.Phase
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimBound), "Cross-namespace authentication should work")
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
