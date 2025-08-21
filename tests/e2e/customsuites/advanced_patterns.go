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

// s3AdvancedPatternsTestSuite implements TestSuite for advanced provisioning patterns
// including WaitForFirstConsumer, multiple access modes, and various StorageClass configurations
type s3AdvancedPatternsTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3AdvancedPatternsTestSuite creates the advanced patterns test suite
func InitS3AdvancedPatternsTestSuite() storageframework.TestSuite {
	return &s3AdvancedPatternsTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "s3AdvancedPatterns",
			TestPatterns: []storageframework.TestPattern{
				{
					Name:    "Advanced Dynamic PV Patterns",
					VolType: storageframework.DynamicPV,
				},
			},
		},
	}
}

// GetTestSuiteInfo returns the test suite information
func (t *s3AdvancedPatternsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

// SkipUnsupportedTests skips tests not applicable to this driver
func (t *s3AdvancedPatternsTestSuite) SkipUnsupportedTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	if pattern.VolType != storageframework.DynamicPV {
		ginkgo.Skip("S3 Advanced Patterns test only supports DynamicPV")
	}
}

// DefineTests defines the advanced pattern tests
func (t *s3AdvancedPatternsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var l local

	// Create a framework with custom timeouts
	f := framework.NewFrameworkWithCustomTimeouts("s3-advanced-patterns", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityEnforceLevel = "privileged"

	// cleanup function
	cleanup := func(ctx context.Context) {
		for _, resource := range l.resources {
			if resource != nil {
				_ = resource.CleanupResource(ctx)
			}
		}
		l.resources = nil
	}

	// Test WaitForFirstConsumer volume binding mode
	ginkgo.It("should support WaitForFirstConsumer volume binding mode", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Creating StorageClass with WaitForFirstConsumer binding mode")

		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		scName := fmt.Sprintf("wait-first-consumer-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      provSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace": f.Namespace.Name,
				// Intentionally omitting node-publish-secret to test fallback behavior.
				// Due to CSI spec limitation, node cannot access provisioner secret,
				// so it will fall back to driver credentials for mounting.
			},
			VolumeBindingMode: ptr.To(storagev1.VolumeBindingWaitForFirstConsumer),
			ReclaimPolicy:     ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		ginkgo.By("Creating PVC with WaitForFirstConsumer StorageClass")

		pvcName := fmt.Sprintf("wait-pvc-%s", uuid.NewString()[:8])
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

		ginkgo.By("Verifying PVC remains in Pending state without pod")

		// PVC should remain pending because no pod is scheduled
		gomega.Consistently(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimPending
			}
			return updatedPVC.Status.Phase
		}, 30*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimPending), "PVC should remain pending without pod")

		ginkgo.By("Creating pod to trigger volume binding")

		podName := fmt.Sprintf("wait-pod-%s", uuid.NewString()[:8])
		pod := &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PodSpec{
				SecurityContext: &v1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
					RunAsUser:    ptr.To(int64(1000)),
					RunAsGroup:   ptr.To(int64(1000)),
					SeccompProfile: &v1.SeccompProfile{
						Type: v1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Containers: []v1.Container{
					{
						Name:  "test-container",
						Image: "busybox:1.36",
						Command: []string{
							"sh", "-c", "echo 'WaitForFirstConsumer test' > /mnt/test-file && sleep 3600",
						},
						SecurityContext: &v1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
							RunAsNonRoot:             ptr.To(true),
							RunAsUser:                ptr.To(int64(1000)),
							RunAsGroup:               ptr.To(int64(1000)),
							Capabilities: &v1.Capabilities{
								Drop: []v1.Capability{"ALL"},
							},
							SeccompProfile: &v1.SeccompProfile{
								Type: v1.SeccompProfileTypeRuntimeDefault,
							},
						},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      "test-volume",
								MountPath: "/mnt",
							},
						},
					},
				},
				Volumes: []v1.Volume{
					{
						Name: "test-volume",
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcName,
							},
						},
					},
				},
			},
		}

		_, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create pod")

		defer func() {
			_ = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, podName, metav1.DeleteOptions{})
		}()

		ginkgo.By("Verifying PVC gets bound after pod creation")

		// Now PVC should get bound because pod is scheduled
		gomega.Eventually(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimPending
			}
			return updatedPVC.Status.Phase
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimBound), "PVC should be bound after pod creation")

		ginkgo.By("Verifying pod becomes ready")

		gomega.Eventually(func(ctx context.Context) v1.PodPhase {
			updatedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return v1.PodPending
			}
			return updatedPod.Status.Phase
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.PodRunning), "Pod should become running")
	})

	// Test reclaim policies
	ginkgo.It("should respect different reclaim policies", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Testing Delete reclaim policy")

		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		scName := fmt.Sprintf("reclaim-delete-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      provSecretName,
				"csi.storage.k8s.io/provisioner-secret-namespace": f.Namespace.Name,
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete), // Explicit Delete policy
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass with Delete reclaim policy")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		pvcName := fmt.Sprintf("reclaim-pvc-%s", uuid.NewString()[:8])
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

		// Wait for PVC to be bound and get PV name
		var pvName string
		gomega.Eventually(func(ctx context.Context) bool {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil || updatedPVC.Status.Phase != v1.ClaimBound {
				return false
			}
			pvName = updatedPVC.Spec.VolumeName
			return pvName != ""
		}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.BeTrue(), "PVC should be bound and have PV name")

		ginkgo.By("Verifying PV has correct reclaim policy")

		pv, err := f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get PV")

		gomega.Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(gomega.Equal(v1.PersistentVolumeReclaimDelete), "PV should have Delete reclaim policy")

		ginkgo.By("Deleting PVC and verifying PV gets deleted")

		err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Delete(ctx, pvcName, metav1.DeleteOptions{})
		framework.ExpectNoError(err, "Failed to delete PVC")

		// PV should be deleted automatically due to Delete reclaim policy
		gomega.Eventually(func(ctx context.Context) bool {
			_, err := f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
			return err != nil // PV should not exist
		}, 60*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.BeTrue(), "PV should be deleted due to Delete reclaim policy")
	})

	// Test credential fallback scenarios
	ginkgo.It("should test credential fallback behavior for all scenarios", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		// Scenario 1: No secrets provided → use driver default credentials
		ginkgo.By("Testing credential fallback scenario 1: No secrets provided")
		testCredentialFallback(ctx, f, "no-secrets", "", "", "Should use driver default credentials")

		// Scenario 2: Only provisioner secret → use provisioner for both bucket creation and mounting
		ginkgo.By("Testing credential fallback scenario 2: Only provisioner secret provided")
		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")
		testCredentialFallback(ctx, f, "provisioner-only", provSecretName, "", "Should use provisioner secret for both operations")

		// Scenario 3: Both provisioner and node secrets → use respectively
		ginkgo.By("Testing credential fallback scenario 3: Both secrets provided")
		nodeSecretName, err := CreateNodePublishSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create node-publish secret")
		testCredentialFallback(ctx, f, "both-secrets", provSecretName, nodeSecretName, "Should use secrets respectively")

		// Scenario 4: Only node secret → use node for mount, driver for provisioning
		ginkgo.By("Testing credential fallback scenario 4: Only node secret provided")
		testCredentialFallback(ctx, f, "node-only", "", nodeSecretName, "Should use node secret for mount, driver for provisioning")
	})

	// Test invalid StorageClass parameters
	ginkgo.It("should handle invalid StorageClass parameters gracefully", func(ctx context.Context) {
		l.config = driver.PrepareTest(ctx, f)
		defer cleanup(ctx)

		ginkgo.By("Testing StorageClass with missing secret namespace")

		provSecretName, err := CreateProvisionerSecret(ctx, f)
		framework.ExpectNoError(err, "Failed to create provisioner secret")

		scName := fmt.Sprintf("invalid-sc-%s", uuid.NewString()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner: "s3.csi.scality.com",
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name": provSecretName,
				// Missing secret namespace - should cause error
			},
			ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
		}

		_, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create invalid StorageClass")

		defer func() {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		}()

		pvcName := fmt.Sprintf("invalid-pvc-%s", uuid.NewString()[:8])
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
		framework.ExpectNoError(err, "Failed to create PVC with invalid StorageClass")

		ginkgo.By("Verifying PVC fails to bind due to invalid parameters")

		// PVC should remain in Pending state due to invalid StorageClass parameters
		gomega.Consistently(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
			updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
			if err != nil {
				return v1.ClaimPending
			}
			return updatedPVC.Status.Phase
		}, 60*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimPending), "PVC should remain pending due to invalid StorageClass")
	})
}

// testCredentialFallback tests a specific credential fallback scenario
func testCredentialFallback(ctx context.Context, f *framework.Framework, scenarioName, provSecretName, nodeSecretName, expectedBehavior string) {
	ginkgo.By(fmt.Sprintf("Creating StorageClass for scenario: %s", scenarioName))

	scName := fmt.Sprintf("cred-fallback-%s-%s", scenarioName, uuid.NewString()[:8])

	// Build StorageClass parameters based on provided secrets
	scParams := map[string]string{}

	if provSecretName != "" {
		scParams["csi.storage.k8s.io/provisioner-secret-name"] = provSecretName
		scParams["csi.storage.k8s.io/provisioner-secret-namespace"] = f.Namespace.Name
	}

	if nodeSecretName != "" {
		scParams["csi.storage.k8s.io/node-publish-secret-name"] = nodeSecretName
		scParams["csi.storage.k8s.io/node-publish-secret-namespace"] = f.Namespace.Name
	}

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		Provisioner:   "s3.csi.scality.com",
		Parameters:    scParams,
		ReclaimPolicy: ptr.To(v1.PersistentVolumeReclaimDelete),
	}

	_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
	framework.ExpectNoError(err, fmt.Sprintf("Failed to create StorageClass for %s", scenarioName))

	defer func() {
		_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
	}()

	ginkgo.By(fmt.Sprintf("Creating PVC for scenario: %s", scenarioName))

	pvcName := fmt.Sprintf("cred-fallback-pvc-%s-%s", scenarioName, uuid.NewString()[:8])
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
	framework.ExpectNoError(err, fmt.Sprintf("Failed to create PVC for %s", scenarioName))

	ginkgo.By(fmt.Sprintf("Verifying PVC gets bound for scenario: %s (%s)", scenarioName, expectedBehavior))

	// PVC should get bound regardless of credential configuration if driver has valid fallback credentials
	gomega.Eventually(func(ctx context.Context) v1.PersistentVolumeClaimPhase {
		updatedPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return v1.ClaimPending
		}
		return updatedPVC.Status.Phase
	}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.ClaimBound),
		fmt.Sprintf("PVC should be bound for %s scenario: %s", scenarioName, expectedBehavior))

	// Test mounting by creating a pod
	ginkgo.By(fmt.Sprintf("Creating pod to test mounting for scenario: %s", scenarioName))

	podName := fmt.Sprintf("cred-fallback-pod-%s-%s", scenarioName, uuid.NewString()[:8])
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: f.Namespace.Name,
		},
		Spec: v1.PodSpec{
			SecurityContext: &v1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(int64(1000)),
				RunAsGroup:   ptr.To(int64(1000)),
				SeccompProfile: &v1.SeccompProfile{
					Type: v1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "busybox:1.36",
					Command: []string{
						"sh", "-c", fmt.Sprintf("echo 'Credential fallback test: %s' > /mnt/test-file && sleep 120", scenarioName),
					},
					SecurityContext: &v1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						RunAsNonRoot:             ptr.To(true),
						RunAsUser:                ptr.To(int64(1000)),
						RunAsGroup:               ptr.To(int64(1000)),
						Capabilities: &v1.Capabilities{
							Drop: []v1.Capability{"ALL"},
						},
						SeccompProfile: &v1.SeccompProfile{
							Type: v1.SeccompProfileTypeRuntimeDefault,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "test-volume",
							MountPath: "/mnt",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "test-volume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	_, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
	framework.ExpectNoError(err, fmt.Sprintf("Failed to create pod for %s", scenarioName))

	defer func() {
		_ = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(ctx, podName, metav1.DeleteOptions{})
	}()

	ginkgo.By(fmt.Sprintf("Verifying pod becomes ready for scenario: %s", scenarioName))

	// Verify pod becomes running (which means volume mounted successfully)
	gomega.Eventually(func(ctx context.Context) v1.PodPhase {
		updatedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return v1.PodPending
		}
		return updatedPod.Status.Phase
	}, 120*time.Second, 5*time.Second).WithContext(ctx).Should(gomega.Equal(v1.PodRunning),
		fmt.Sprintf("Pod should become running for %s scenario: %s", scenarioName, expectedBehavior))

	framework.Logf("Successfully tested credential fallback scenario: %s - %s", scenarioName, expectedBehavior)
}
