/*
This suite tests the mounter pod infrastructure — the dedicated Kubernetes pods
that run mount-s3 (FUSE) on behalf of workload pods.

Covered behaviors:

- FSGroup: Ensures the mounter pod's PodSecurityContext.FSGroup is set to the
  default MountpointPodUserID (1000), so shared volumes like the emptyDir
  communication directory have correct group ownership.
- RunAsUser: Verifies the mounter container runs as the expected non-root UID.
- RunAsNonRoot: Confirms privilege restrictions on the mounter container.

These tests validate that the CSI driver spawns properly secured mounter pods.
*/
package customsuites

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/constants"
)

const (
	// mounterPodNamespace is the namespace where mounter pods are created.
	// Must match the Helm chart value for mountpointPod.namespace.
	mounterPodNamespace = "mount-s3"

	// Label key used on mounter pods (mirrors pkg/podmounter/mppod/creator.go LabelVolumeName).
	labelVolumeName = constants.DriverName + "/volume-name"

	// expectedFSGroup is the default MountpointPodUserID for vanilla Kubernetes.
	expectedFSGroup = int64(1000)
)

type s3CSIMounterPodTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3MounterPodTestSuite returns a test suite for mounter pod infrastructure.
//
// This suite tests:
// - Mounter pod FSGroup is set correctly in PodSecurityContext
// - Mounter container runs as the expected non-root user
func InitS3MounterPodTestSuite() storageframework.TestSuite {
	return &s3CSIMounterPodTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "mounterpod",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsPreprovisionedPV,
			},
		},
	}
}

func (suite *s3CSIMounterPodTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return suite.tsInfo
}

func (suite *s3CSIMounterPodTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

func (suite *s3CSIMounterPodTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type TestResourceRegistry struct {
		resources []*storageframework.VolumeResource
		config    *storageframework.PerTestConfig
	}
	var testRegistry TestResourceRegistry

	testFramework := framework.NewFrameworkWithCustomTimeouts("mounterpod", storageframework.GetDriverTimeouts(driver))
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

	ginkgo.BeforeEach(func(ctx context.Context) {
		testRegistry = TestResourceRegistry{}
		testRegistry.config = driver.PrepareTest(ctx, testFramework)
		ginkgo.DeferCleanup(cleanup)
	})

	// --------------------------------------------------------------------
	// 1. Mounter pod FSGroup
	//
	// When the CSI driver creates a mounter pod to run mount-s3 as a FUSE
	// process, the pod must have FSGroup set in its PodSecurityContext.
	// This ensures that shared volumes (e.g., the emptyDir communication
	// directory between the CSI node driver and the mounter process) have
	// proper group ownership, allowing the non-root mount-s3 process to
	// read/write them.
	//
	// Diagram:
	//
	//      [Workload Pod]
	//            |
	//      NodePublishVolume
	//            |
	//            ↓
	//      [Mounter Pod]  ←── spec.securityContext.fsGroup = 1000
	//            |
	//         mount-s3
	//            |
	//            ↓
	//      [S3 FUSE Mount]
	//
	// Expected results:
	// - The mounter pod in the mount-s3 namespace has PodSecurityContext.FSGroup = 1000
	// - The mounter container has SecurityContext.RunAsUser = 1000
	// - The mounter container has SecurityContext.RunAsNonRoot = true
	ginkgo.It("should set FSGroup on the mounter pod security context", func(ctx context.Context) {
		ginkgo.By("Creating a volume with standard mount options")
		res := BuildVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0644")
		testRegistry.resources = append(testRegistry.resources, res)

		pvName := res.Pv.Name

		ginkgo.By("Creating a workload pod that mounts the volume")
		pod, err := CreatePodWithVolumeAndSecurity(ctx, testFramework, res.Pvc, "",
			DefaultNonRootUser, DefaultNonRootGroup)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		ginkgo.By(fmt.Sprintf("Finding the mounter pod for volume %s in namespace %s", pvName, mounterPodNamespace))
		var mounterPodFSGroup *int64
		var mounterPodRunAsUser *int64
		var mounterPodRunAsNonRoot *bool
		gomega.Eventually(func(g gomega.Gomega) {
			pods, err := testFramework.ClientSet.CoreV1().Pods(mounterPodNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", labelVolumeName, pvName),
			})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(pods.Items).NotTo(gomega.BeEmpty(), "no mounter pod found for volume %s", pvName)

			mounterPod := pods.Items[0]
			g.Expect(mounterPod.Spec.SecurityContext).NotTo(gomega.BeNil(),
				"mounter pod should have PodSecurityContext")
			g.Expect(mounterPod.Spec.SecurityContext.FSGroup).NotTo(gomega.BeNil(),
				"mounter pod should have FSGroup set in PodSecurityContext")
			mounterPodFSGroup = mounterPod.Spec.SecurityContext.FSGroup

			g.Expect(mounterPod.Spec.Containers).NotTo(gomega.BeEmpty())
			g.Expect(mounterPod.Spec.Containers[0].SecurityContext).NotTo(gomega.BeNil())
			mounterPodRunAsUser = mounterPod.Spec.Containers[0].SecurityContext.RunAsUser
			mounterPodRunAsNonRoot = mounterPod.Spec.Containers[0].SecurityContext.RunAsNonRoot
		}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(gomega.Succeed())

		ginkgo.By("Verifying the mounter pod FSGroup matches the expected MountpointPodUserID")
		gomega.Expect(*mounterPodFSGroup).To(gomega.Equal(expectedFSGroup),
			"mounter pod FSGroup should be %d (default MountpointPodUserID)", expectedFSGroup)

		ginkgo.By("Verifying the mounter container runs as the expected non-root user")
		gomega.Expect(mounterPodRunAsUser).NotTo(gomega.BeNil())
		gomega.Expect(*mounterPodRunAsUser).To(gomega.Equal(expectedFSGroup),
			"mounter pod RunAsUser should be %d", expectedFSGroup)

		ginkgo.By("Verifying the mounter container enforces RunAsNonRoot")
		gomega.Expect(mounterPodRunAsNonRoot).NotTo(gomega.BeNil())
		gomega.Expect(*mounterPodRunAsNonRoot).To(gomega.BeTrue(),
			"mounter pod RunAsNonRoot should be true")
	})
}
