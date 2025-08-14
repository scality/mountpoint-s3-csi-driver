package customsuites

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	"k8s.io/utils/ptr"
)

// s3DynamicRbacTestSuite implements the Kubernetes storage framework TestSuite interface.
// It validates that the S3 CSI controller properly handles RBAC permissions for dynamic provisioning,
// particularly around reading provisioner secrets from different namespaces.
type s3DynamicRbacTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3DynamicRbacTestSuite initializes and returns a test suite that validates
// RBAC functionality for dynamic provisioning with the S3 CSI driver.
//
// This suite specifically tests:
// - Controller can read provisioner-secret from any namespace
// - Invalid secret reference handling
// - Missing secret error scenarios
// - Cross-namespace secret access validation
func InitS3DynamicRbacTestSuite() storageframework.TestSuite {
	return &s3DynamicRbacTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "dynamicrbac",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPV,
			},
		},
	}
}

// GetTestSuiteInfo returns metadata about this test suite for the framework.
func (t *s3DynamicRbacTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

// SkipUnsupportedTests allows test suites to skip certain tests based on driver capabilities.
func (t *s3DynamicRbacTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, pattern storageframework.TestPattern) {
	if pattern.VolType != storageframework.DynamicPV {
		e2eskipper.Skipf("Dynamic RBAC tests only support Dynamic PV, got %v", pattern.VolType)
	}
}

// DefineTests implements the actual test suite functionality.
func (t *s3DynamicRbacTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	// local struct to maintain test state across BeforeEach/AfterEach/It blocks
	type local struct {
		resources []*storageframework.VolumeResource // tracks resources for cleanup
		config    *storageframework.PerTestConfig    // storage framework configuration
	}
	var l local

	// Create a framework with custom timeouts based on the driver's requirements
	f := framework.NewFrameworkWithCustomTimeouts("dynamicrbac", storageframework.GetDriverTimeouts(driver))
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

	ginkgo.BeforeEach(func(ctx context.Context) {
		l = local{}
		l.config = driver.PrepareTest(ctx, f)
	})

	ginkgo.AfterEach(func(ctx context.Context) {
		cleanup(ctx)
	})

	// Test that controller can read provisioner-secret from any namespace
	ginkgo.It("controller can read provisioner-secret from any namespace", func(ctx context.Context) {
		ginkgo.By("Creating a secret in a different namespace")

		// Create a new namespace for the secret
		secretNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.Namespace.Name + "-secret-ns",
			},
		}
		_, err := l.config.Framework.ClientSet.CoreV1().Namespaces().Create(ctx, secretNamespace, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create secret namespace")

		// Ensure cleanup of the namespace
		defer func() {
			_ = l.config.Framework.ClientSet.CoreV1().Namespaces().Delete(ctx, secretNamespace.Name, metav1.DeleteOptions{})
		}()

		// Create secret in the different namespace
		secretName, err := CreateCredentialSecret(ctx, &framework.Framework{
			BaseName:  f.BaseName,
			ClientSet: f.ClientSet,
			Namespace: secretNamespace,
		}, "cross-ns-provisioner",
			GetEnv("ACCOUNT1_ACCESS_KEY", ""),
			GetEnv("ACCOUNT1_SECRET_KEY", ""))
		framework.ExpectNoError(err, "Failed to create cross-namespace secret")

		ginkgo.By("Creating StorageClass referencing the cross-namespace secret")
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cross-ns-sc",
			},
			Provisioner: driver.GetDriverInfo().Name,
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      secretName,
				"csi.storage.k8s.io/provisioner-secret-namespace": secretNamespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}
		_, err = l.config.Framework.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		// Ensure cleanup of the StorageClass
		defer func() {
			_ = l.config.Framework.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC and verifying it succeeds")
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cross-ns-pvc",
				Namespace: l.config.Framework.Namespace.Name,
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

		pvc, err = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		// Wait for PVC to be bound (controller successfully read the cross-namespace secret)
		ginkgo.By("Waiting for PVC to be bound")
		err = e2epv.WaitForPersistentVolumeClaimPhase(ctx, v1.ClaimBound, l.config.Framework.ClientSet, l.config.Framework.Namespace.Name, pvc.Name, time.Second, 2*time.Minute)
		framework.ExpectNoError(err, "PVC should be bound when controller can read cross-namespace secret")

		// Cleanup PVC to trigger volume deletion
		err = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		framework.ExpectNoError(err, "Failed to delete PVC")
	})

	// Test invalid secret reference handling
	ginkgo.It("handles invalid secret references appropriately", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with invalid secret reference")
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-invalid-secret-sc",
			},
			Provisioner: driver.GetDriverInfo().Name,
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      "non-existent-secret",
				"csi.storage.k8s.io/provisioner-secret-namespace": l.config.Framework.Namespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}
		_, err := l.config.Framework.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		// Ensure cleanup of the StorageClass
		defer func() {
			_ = l.config.Framework.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with invalid secret reference")
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-invalid-secret-pvc",
				Namespace: l.config.Framework.Namespace.Name,
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

		pvc, err = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		// Ensure cleanup of the PVC
		defer func() {
			_ = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Verifying PVC remains in Pending state due to invalid secret")
		// PVC should remain pending because the secret doesn't exist
		gomega.Consistently(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimLost
			}
			return updatedPVC.Status.Phase
		}, 30*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimPending), "PVC should remain pending with invalid secret reference")
	})

	// Test missing secret namespace error scenarios
	ginkgo.It("handles missing secret namespace appropriately", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with missing secret namespace")
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-missing-ns-sc",
			},
			Provisioner: driver.GetDriverInfo().Name,
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      "some-secret",
				"csi.storage.k8s.io/provisioner-secret-namespace": "non-existent-namespace",
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}
		_, err := l.config.Framework.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		// Ensure cleanup of the StorageClass
		defer func() {
			_ = l.config.Framework.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with missing secret namespace")
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-missing-ns-pvc",
				Namespace: l.config.Framework.Namespace.Name,
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

		pvc, err = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		// Ensure cleanup of the PVC
		defer func() {
			_ = l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Delete(ctx, pvc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Verifying PVC remains in Pending state due to missing namespace")
		// PVC should remain pending because the namespace doesn't exist
		gomega.Consistently(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := l.config.Framework.ClientSet.CoreV1().PersistentVolumeClaims(l.config.Framework.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimLost
			}
			return updatedPVC.Status.Phase
		}, 30*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimPending), "PVC should remain pending with missing secret namespace")
	})
}
