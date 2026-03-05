/*
This suite verifies that the mounter pod (which runs mount-s3) is correctly configured
with appropriate security context settings, specifically FSGroup.

Covered behaviors:

  - fsgroup: Ensures mounter pod has PodSecurityContext.FSGroup set so the
    communication emptyDir volume is writable by the non-root mount-s3 process.
  - workload-fsgroup: Verifies that workload pods with fsGroup in their security
    context can successfully mount S3 volumes (reproduces RD-1318 / S3CSI-213).
  - dynamic-fsgroup: Same as workload-fsgroup but using dynamic provisioning
    (Immediate binding — matches customer's Argo CD deployment).
  - dynamic-fsgroup-wffc: Same but with WaitForFirstConsumer binding mode.
  - dynamic-mounter-fsgroup: Verifies mounter pod FSGroup is set for dynamic PVs.
*/
package customsuites

import (
	"context"
	"fmt"
	"os"
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
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/constants"
)

const (
	// mounterPodNamespace is the namespace where mounter pods are created by the CSI driver.
	mounterPodNamespace = "mount-s3"
	// mounterPodLabelVolumeName is the label used to find mounter pods by PV name.
	mounterPodLabelVolumeName = constants.DriverName + "/volume-name"
	// expectedMounterFSGroup is the UID/GID used by mount-s3 in vanilla Kubernetes.
	expectedMounterFSGroup = int64(1000)
)

type s3MounterPodTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3MounterPodTestSuite returns a test suite that verifies mounter pod security context.
func InitS3MounterPodTestSuite() storageframework.TestSuite {
	return &s3MounterPodTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "mounterpod",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsPreprovisionedPV,
				storageframework.DefaultFsDynamicPV,
			},
		},
	}
}

func (suite *s3MounterPodTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return suite.tsInfo
}

func (suite *s3MounterPodTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

func (suite *s3MounterPodTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var l local

	f := framework.NewFrameworkWithCustomTimeouts("mounterpod", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	cleanup := func(ctx context.Context) {
		for _, r := range l.resources {
			if r != nil {
				_ = r.CleanupResource(ctx)
			}
		}
		l.resources = nil
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		l = local{}
		l.config = driver.PrepareTest(ctx, f)
		ginkgo.DeferCleanup(cleanup)
	})

	// findMounterPod locates the mounter pod for a given PV name by label selector.
	findMounterPod := func(ctx context.Context, pvName string) *v1.Pod {
		var mounterPod *v1.Pod
		gomega.Eventually(func(ctx context.Context) error {
			pods, err := f.ClientSet.CoreV1().Pods(mounterPodNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", mounterPodLabelVolumeName, pvName),
			})
			if err != nil {
				return err
			}
			if len(pods.Items) == 0 {
				return fmt.Errorf("no mounter pod found for PV %s", pvName)
			}
			mounterPod = &pods.Items[0]
			return nil
		}, 2*time.Minute, 5*time.Second).WithContext(ctx).Should(gomega.Succeed(),
			"mounter pod should be found for PV %s", pvName)
		return mounterPod
	}

	// makeFSGroupPod creates a pod spec with the customer's exact security context (fsGroup=1001).
	makeFSGroupPod := func(pvcs []*v1.PersistentVolumeClaim) *v1.Pod {
		pod := e2epod.MakePod(f.Namespace.Name, nil, pvcs, admissionapi.LevelRestricted, "")
		pod.Spec.SecurityContext = &v1.PodSecurityContext{
			FSGroup:      ptr.To(DefaultNonRootUser),
			RunAsGroup:   ptr.To(DefaultNonRootUser),
			RunAsUser:    ptr.To(DefaultNonRootUser),
			RunAsNonRoot: ptr.To(true),
		}
		for i := range pod.Spec.Containers {
			pod.Spec.Containers[i].SecurityContext = &v1.SecurityContext{
				RunAsUser:                ptr.To(DefaultNonRootUser),
				RunAsGroup:               ptr.To(DefaultNonRootUser),
				RunAsNonRoot:             ptr.To(true),
				AllowPrivilegeEscalation: ptr.To(false),
				SeccompProfile: &v1.SeccompProfile{
					Type: v1.SeccompProfileTypeRuntimeDefault,
				},
				Capabilities: &v1.Capabilities{
					Drop: []v1.Capability{"ALL"},
				},
			}
		}
		return pod
	}

	// verifyMounterPodFSGroup finds the mounter pod and asserts its security context.
	// On OpenShift, the SCC assigns UIDs from the namespace range (MountpointPodUserID
	// returns nil), so we only verify FSGroup is non-nil. On vanilla K8s, we check
	// the exact value (1000).
	verifyMounterPodFSGroup := func(ctx context.Context, pvName string) {
		mounterPod := findMounterPod(ctx, pvName)
		isOpenShift := os.Getenv("CLUSTER_TYPE") == "openshift"

		ginkgo.By("Verifying mounter pod PodSecurityContext.FSGroup is set")
		gomega.Expect(mounterPod.Spec.SecurityContext).NotTo(gomega.BeNil(),
			"mounter pod should have PodSecurityContext")
		gomega.Expect(mounterPod.Spec.SecurityContext.FSGroup).NotTo(gomega.BeNil(),
			"mounter pod should have FSGroup set")
		if !isOpenShift {
			gomega.Expect(*mounterPod.Spec.SecurityContext.FSGroup).To(gomega.Equal(expectedMounterFSGroup),
				"mounter pod FSGroup should be %d", expectedMounterFSGroup)
		}

		ginkgo.By("Verifying mounter pod container SecurityContext")
		gomega.Expect(mounterPod.Spec.Containers).NotTo(gomega.BeEmpty())
		sc := mounterPod.Spec.Containers[0].SecurityContext
		gomega.Expect(sc).NotTo(gomega.BeNil())
		gomega.Expect(sc.RunAsNonRoot).NotTo(gomega.BeNil())
		gomega.Expect(*sc.RunAsNonRoot).To(gomega.BeTrue())
		if !isOpenShift {
			gomega.Expect(sc.RunAsUser).NotTo(gomega.BeNil())
			gomega.Expect(*sc.RunAsUser).To(gomega.Equal(expectedMounterFSGroup))
		}
	}

	// createDynamicVolumeWithFSGroupPod creates a StorageClass, PVC, and workload pod with fsGroup.
	// Returns the created pod and PV name for mounter pod verification.
	createDynamicVolumeWithFSGroupPod := func(ctx context.Context, bindingMode storagev1.VolumeBindingMode) (*v1.Pod, string) {
		dynamicDriver := driver.(storageframework.DynamicPVTestDriver)
		sc := dynamicDriver.GetDynamicProvisionStorageClass(ctx, l.config, "")
		sc.VolumeBindingMode = &bindingMode
		sc.MountOptions = []string{
			fmt.Sprintf("--uid=%d", DefaultNonRootUser),
			fmt.Sprintf("--gid=%d", DefaultNonRootGroup),
			"--allow-other",
			"--allow-delete",
			"--allow-overwrite",
		}

		ginkgo.By(fmt.Sprintf("Creating StorageClass with %s binding mode", bindingMode))
		_, err := f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err)
		ginkgo.DeferCleanup(func(ctx context.Context) {
			_ = f.ClientSet.StorageV1().StorageClasses().Delete(ctx, sc.Name, metav1.DeleteOptions{})
		})

		ginkgo.By("Creating PVC referencing the StorageClass")
		pvcName := fmt.Sprintf("fsgroup-pvc-%s", uuid.NewString()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteMany},
				StorageClassName: &sc.Name,
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}
		_, err = f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err)

		ginkgo.By("Creating workload pod with fsGroup=1001")
		pod := makeFSGroupPod([]*v1.PersistentVolumeClaim{pvc})
		pod, err = createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err, "workload pod with fsGroup should start successfully with dynamic provisioning")
		ginkgo.DeferCleanup(func(ctx context.Context) {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		})

		// Get the PV name after binding
		boundPVC, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvcName, metav1.GetOptions{})
		framework.ExpectNoError(err)

		return pod, boundPVC.Spec.VolumeName
	}

	// Test 1: Mounter pod FSGroup verification (preprovisioned PV only)
	if pattern.VolType == storageframework.PreprovisionedPV {
		ginkgo.It("should set FSGroup on mounter pod security context", func(ctx context.Context) {
			ginkgo.By("Creating a volume with mount options for non-root access")
			resource := BuildVolumeWithOptions(ctx, l.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "")
			l.resources = append(l.resources, resource)

			ginkgo.By("Creating a workload pod that mounts the volume")
			pod, err := CreatePodWithVolumeAndSecurity(ctx, f, resource.Pvc, "", DefaultNonRootUser, DefaultNonRootGroup)
			framework.ExpectNoError(err)
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			verifyMounterPodFSGroup(ctx, resource.Pv.Name)
		})

		// Test 2: Customer bug reproduction — workload pod with fsGroup can mount (preprovisioned PV)
		ginkgo.It("should allow workload pod with fsGroup to mount volume [RD-1318]", func(ctx context.Context) {
			ginkgo.By("Creating a volume with mount options for non-root access")
			resource := BuildVolumeWithOptions(ctx, l.config, pattern, DefaultNonRootUser, DefaultNonRootGroup, "")
			l.resources = append(l.resources, resource)

			ginkgo.By("Creating workload pod with fsGroup=1001 (customer configuration)")
			pod := makeFSGroupPod([]*v1.PersistentVolumeClaim{resource.Pvc})
			pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err, "workload pod with fsGroup should start successfully")
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			ginkgo.By("Verifying data can be written and read through the mounted volume")
			volPath := pod.Spec.Containers[0].VolumeMounts[0].MountPath
			testFile := fmt.Sprintf("%s/fsgroup-test-%s.txt", volPath, uuid.NewString()[:8])
			WriteAndVerifyFile(f, pod, testFile, "fsgroup-test-data")
		})
	}

	// Dynamic provisioning tests (DynamicPV only)
	if pattern.VolType == storageframework.DynamicPV {
		// Test 3: Dynamic provisioning with Immediate binding + fsGroup (customer's Argo CD scenario)
		ginkgo.It("should allow workload pod with fsGroup to mount dynamically provisioned volume [RD-1318]", func(ctx context.Context) {
			pod, _ := createDynamicVolumeWithFSGroupPod(ctx, storagev1.VolumeBindingImmediate)

			ginkgo.By("Verifying data can be written and read through the mounted volume")
			volPath := pod.Spec.Containers[0].VolumeMounts[0].MountPath
			testFile := fmt.Sprintf("%s/fsgroup-dynamic-test-%s.txt", volPath, uuid.NewString()[:8])
			WriteAndVerifyFile(f, pod, testFile, "fsgroup-dynamic-test-data")
		})

		// Test 4: Dynamic provisioning with WaitForFirstConsumer binding + fsGroup
		ginkgo.It("should allow workload pod with fsGroup to mount WaitForFirstConsumer volume [RD-1318]", func(ctx context.Context) {
			pod, _ := createDynamicVolumeWithFSGroupPod(ctx, storagev1.VolumeBindingWaitForFirstConsumer)

			ginkgo.By("Verifying data can be written and read through the mounted volume")
			volPath := pod.Spec.Containers[0].VolumeMounts[0].MountPath
			testFile := fmt.Sprintf("%s/fsgroup-wffc-test-%s.txt", volPath, uuid.NewString()[:8])
			WriteAndVerifyFile(f, pod, testFile, "fsgroup-wffc-test-data")
		})

		// Test 5: Verify mounter pod FSGroup with dynamic provisioning
		ginkgo.It("should set FSGroup on mounter pod with dynamically provisioned volume", func(ctx context.Context) {
			_, pvName := createDynamicVolumeWithFSGroupPod(ctx, storagev1.VolumeBindingImmediate)
			verifyMounterPodFSGroup(ctx, pvName)
		})
	}
}
