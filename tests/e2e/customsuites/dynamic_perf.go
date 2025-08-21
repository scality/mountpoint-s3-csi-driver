package customsuites

import (
	"context"
	"fmt"
	"sync"
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

// s3DynamicProvisioningPerfTestSuite implements TestSuite for performance and stress testing
// of dynamic provisioning scenarios
type s3DynamicProvisioningPerfTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3DynamicProvisioningPerfTestSuite creates the performance test suite for dynamic provisioning
func InitS3DynamicProvisioningPerfTestSuite() storageframework.TestSuite {
	return &s3DynamicProvisioningPerfTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "s3DynamicProvisioningPerf",
			TestPatterns: []storageframework.TestPattern{
				// Use DynamicPV pattern for performance testing
				{
					Name:    "Dynamic PV Performance Test",
					VolType: storageframework.DynamicPV,
				},
			},
		},
	}
}

// GetTestSuiteInfo returns the test suite information
func (t *s3DynamicProvisioningPerfTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

// SkipUnsupportedTests skips tests not applicable to this driver
func (t *s3DynamicProvisioningPerfTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	if pattern.VolType != storageframework.DynamicPV {
		ginkgo.Skip("S3 Dynamic Provisioning Performance test only supports DynamicPV")
	}
}

// DefineTests defines the performance and stress tests for dynamic provisioning
func (t *s3DynamicProvisioningPerfTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var l local

	// Create a framework with custom timeouts based on the driver's requirements
	f := framework.NewFrameworkWithCustomTimeouts("s3-dynamic-perf", storageframework.GetDriverTimeouts(driver))
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

	// Performance test: Multiple concurrent provisions
	ginkgo.It("should handle multiple concurrent dynamic provisions efficiently", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating StorageClass for concurrent provisioning test")

		// Create provisioner secret for authentication
		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		// Create StorageClass with authentication
		scName := fmt.Sprintf("concurrent-perf-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      provSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace": f.Namespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating multiple PVCs concurrently")

		// Test concurrent provisioning of 5 volumes
		const numVolumes = 5
		var wg sync.WaitGroup
		pvcNames := make([]string, numVolumes)
		errors := make([]error, numVolumes)
		startTime := time.Now()

		for i := range numVolumes {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				pvcName := fmt.Sprintf("concurrent-pvc-%d-%s", index, uuid.NewString()[:8])
				pvcNames[index] = pvcName

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

				_, errors[index] = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
			}(i)
		}

		wg.Wait()
		creationTime := time.Since(startTime)

		// Verify no creation errors
		for i, err := range errors {
			framework.ExpectNoError(err, fmt.Sprintf("Failed to create PVC %d", i))
		}

		ginkgo.By(fmt.Sprintf("Waiting for all %d PVCs to be bound", numVolumes))

		// Wait for all PVCs to be bound
		bindStartTime := time.Now()
		for _, pvcName := range pvcNames {
			WaitForPVCToBeBoundWithTimeout(ctx, f, pvcName, f.Namespace.Name, 120*time.Second, 5*time.Second)
		}

		totalTime := time.Since(startTime)
		bindTime := time.Since(bindStartTime)

		framework.Logf("Performance Results:")
		framework.Logf("  - Created %d PVCs in %v (avg: %v per PVC)", numVolumes, creationTime, creationTime/numVolumes)
		framework.Logf("  - All PVCs bound in %v (avg: %v per PVC)", bindTime, bindTime/numVolumes)
		framework.Logf("  - Total time: %v", totalTime)

		// Performance assertion: All volumes should be provisioned within reasonable time
		gomega.Expect(totalTime).To(gomega.BeNumerically("<", 300*time.Second), "All volumes should be provisioned within 5 minutes")
	})

	// Stress test: Secret lookup performance
	ginkgo.It("should handle rapid secret-based provisioning efficiently", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating multiple secrets for rapid lookup test")

		// Create multiple secrets
		const numSecrets = 3
		secretNames := make([]string, numSecrets)
		for i := range numSecrets {
			secretName, err := CreateProvisionerSecret(ctx, f)
			framework.ExpectNoError(err, fmt.Sprintf("Failed to create secret %d", i))
			secretNames[i] = secretName
		}

		ginkgo.By("Creating PVCs rapidly using different secrets")

		startTime := time.Now()
		const numProvisions = 6 // 2 per secret
		pvcNames := make([]string, numProvisions)

		for i := range numProvisions {
			secretName := secretNames[i%numSecrets]

			// Create unique StorageClass for each PVC to test secret lookup
			scName := fmt.Sprintf("rapid-sc-%d-%s", i, uuid.NewString()[:8])
			sc := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: scName,
				},
				Provisioner: "s3.csi.scality.com",
				Parameters: map[string]string{
					"csi.storage.k8s.io/provisioner-secret-name":      secretName,
					"csi.storage.k8s.io/provisioner-secret-namespace": f.Namespace.Name,
				},
				ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
			}

			_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
			framework.ExpectNoError(err, fmt.Sprintf("Failed to create StorageClass %d", i))

			defer func(scName string) {
				_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, scName, metav1.DeleteOptions{})
			}(sc.Name)

			// Create PVC
			pvcName := fmt.Sprintf("rapid-pvc-%d-%s", i, uuid.NewString()[:8])
			pvcNames[i] = pvcName

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
			framework.ExpectNoError(err, fmt.Sprintf("Failed to create PVC %d", i))

			// Small delay to simulate rapid but not simultaneous creation
			time.Sleep(100 * time.Millisecond)
		}

		creationTime := time.Since(startTime)

		ginkgo.By("Verifying all rapid provisions complete successfully")

		// Wait for all PVCs to be bound
		for _, pvcName := range pvcNames {
			WaitForPVCToBeBoundWithTimeout(ctx, f, pvcName, f.Namespace.Name, 120*time.Second, 5*time.Second)
		}

		totalTime := time.Since(startTime)

		framework.Logf("Rapid Provisioning Results:")
		framework.Logf("  - Created %d PVCs with %d different secrets in %v", numProvisions, numSecrets, creationTime)
		framework.Logf("  - Total binding time: %v", totalTime)
		framework.Logf("  - Average time per volume: %v", totalTime/numProvisions)

		// Performance assertion: Rapid secret-based provisioning should be efficient
		gomega.Expect(totalTime).To(gomega.BeNumerically("<", 240*time.Second), "Rapid provisioning should complete within 4 minutes")
	})

	// Large-scale scenario test
	ginkgo.It("should handle large-scale dynamic provisioning scenarios", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Testing large-scale provisioning with mixed patterns")

		// Create secrets for different authentication scenarios
		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		nodeSecretName, err := CreateNodePublishSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create node-publish secret")

		// Create StorageClasses with different configurations
		storageClasses := []struct {
			name   string
			params map[string]string
		}{
			{
				name: "large-scale-prov-only",
				params: map[string]string{
					"csi.storage.k8s.io/provisioner-secret-name":      provSecretName,
					"csi.storage.k8s.io/provisioner-secret-namespace": f.Namespace.Name,
				},
			},
			{
				name: "large-scale-full-auth",
				params: map[string]string{
					"csi.storage.k8s.io/provisioner-secret-name":       provSecretName,
					"csi.storage.k8s.io/provisioner-secret-namespace":  f.Namespace.Name,
					"csi.storage.k8s.io/node-publish-secret-name":      nodeSecretName,
					"csi.storage.k8s.io/node-publish-secret-namespace": f.Namespace.Name,
				},
			},
		}

		// Create StorageClasses
		for i, scConfig := range storageClasses {
			scName := fmt.Sprintf("%s-%s", scConfig.name, uuid.NewString()[:8])
			sc := &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: scName,
				},
				Provisioner:   "s3.csi.scality.com",
				Parameters:    scConfig.params,
				ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
			}

			_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
			framework.ExpectNoError(err, fmt.Sprintf("Failed to create StorageClass %d", i))

			defer func(scName string) {
				_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, scName, metav1.DeleteOptions{})
			}(sc.Name)

			// Update the struct to store the actual created name
			storageClasses[i].name = scName
		}

		ginkgo.By("Creating large-scale mixed workload")

		const volumesPerClass = 4
		totalVolumes := len(storageClasses) * volumesPerClass
		startTime := time.Now()

		var allPVCNames []string
		for scIndex, scConfig := range storageClasses {
			for volIndex := range volumesPerClass {
				pvcName := fmt.Sprintf("large-scale-pvc-%d-%d-%s", scIndex, volIndex, uuid.NewString()[:8])
				allPVCNames = append(allPVCNames, pvcName)

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
						StorageClassName: &scConfig.name,
					},
				}

				_, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
				framework.ExpectNoError(err, fmt.Sprintf("Failed to create PVC %s", pvcName))

				// Small staggered delay to simulate realistic workload patterns
				time.Sleep(50 * time.Millisecond)
			}
		}

		creationTime := time.Since(startTime)

		ginkgo.By(fmt.Sprintf("Waiting for all %d large-scale PVCs to be bound", totalVolumes))

		// Wait for all volumes to be bound
		for _, pvcName := range allPVCNames {
			WaitForPVCToBeBoundWithTimeout(ctx, f, pvcName, f.Namespace.Name, 180*time.Second, 10*time.Second)
		}

		totalTime := time.Since(startTime)

		framework.Logf("Large-Scale Provisioning Results:")
		framework.Logf("  - Created %d volumes across %d StorageClasses", totalVolumes, len(storageClasses))
		framework.Logf("  - Creation time: %v", creationTime)
		framework.Logf("  - Total binding time: %v", totalTime)
		framework.Logf("  - Average time per volume: %v", totalTime/time.Duration(totalVolumes))
		framework.Logf("  - Volumes per minute: %.1f", float64(totalVolumes)/totalTime.Minutes())

		// Performance assertion: Large-scale provisioning should complete within reasonable time
		gomega.Expect(totalTime).To(gomega.BeNumerically("<", 480*time.Second), "Large-scale provisioning should complete within 8 minutes")
	})
}
