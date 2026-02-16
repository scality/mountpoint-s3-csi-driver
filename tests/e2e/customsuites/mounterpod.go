/*
This suite tests the mounter pod infrastructure — the dedicated Kubernetes pods
that run mount-s3 (FUSE) on behalf of workload pods.

Covered behaviors:

  - FSGroup: Ensures the mounter pod's PodSecurityContext.FSGroup is set to the
    default MountpointPodUserID (1000), so shared volumes like the emptyDir
    communication directory have correct group ownership.
  - RunAsUser: Verifies the mounter container runs as the expected non-root UID.
  - RunAsNonRoot: Confirms privilege restrictions on the mounter container.
  - Workload FSGroup compatibility: Verifies that workload pods with fsGroup set
    in their PodSecurityContext can successfully mount S3 volumes and read/write
    data. This reproduces the customer-reported issue where fsGroup on the
    workload pod caused the mounter pod's communication socket to time out.
  - Dynamic provisioning with FSGroup: Verifies that dynamically provisioned
    volumes work correctly when the workload pod has fsGroup set.
  - Volume sharing with different FSGroup: Confirms that workload pods with
    different fsGroup values sharing the same PersistentVolume get separate
    mounter pods, as the reconciler includes fsGroup in the matching criteria.

These tests validate that the CSI driver spawns properly secured mounter pods.
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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

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
			res := testRegistry.resources[i]
			func() {
				defer ginkgo.GinkgoRecover()
				ctx := context.Background()
				ginkgo.By("Deleting pv and pvc")
				err := res.CleanupResource(ctx)
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

	// --------------------------------------------------------------------
	// 2. Workload pod with fsGroup can mount and use S3 volumes
	//
	// Reproduces the customer-reported issue where workload pods with
	// fsGroup in their PodSecurityContext failed to mount S3 volumes.
	// The mounter pod's communication socket (/comm/mount.sock)
	// timed out because the emptyDir volume had incorrect group ownership.
	//
	// The fix (adding FSGroup to the mounter pod's PodSecurityContext)
	// ensures the communication directory is writable regardless of
	// what fsGroup the workload pod uses.
	//
	// Customer scenario:
	//   securityContext:
	//     fsGroup: 1001
	//     runAsGroup: 1001
	//     runAsUser: 1001
	//
	// Diagram:
	//
	//      [Workload Pod]
	//        fsGroup: 1001      ← customer sets this
	//            |
	//      NodePublishVolume
	//            |
	//            ↓
	//      [Mounter Pod]
	//        fsGroup: 1000      ← CSI driver sets this (the fix)
	//            |
	//         mount-s3
	//            |
	//            ↓
	//      [S3 FUSE Mount]      ← workload reads/writes here
	//
	// Expected results:
	// - The workload pod reaches Running state (mount succeeds)
	// - Data can be written and read back through the mounted volume
	ginkgo.It("should mount volume when workload pod has fsGroup set", func(ctx context.Context) {
		ginkgo.By("Creating a volume with standard mount options")
		res := BuildVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0644")
		testRegistry.resources = append(testRegistry.resources, res)

		ginkgo.By("Creating a workload pod with fsGroup in its PodSecurityContext")
		pod := e2epod.MakePod(testFramework.Namespace.Name, nil,
			[]*v1.PersistentVolumeClaim{res.Pvc}, admissionapi.LevelRestricted, "")
		pod.Name = fmt.Sprintf("fsgroup-pod-%s", uuid.New().String()[:8])
		podModifierNonRoot(pod)
		// Set fsGroup on the workload pod — this is the customer's configuration
		// that triggered the original bug.
		pod.Spec.SecurityContext.FSGroup = ptr.To(DefaultNonRootUser)

		pod, err := createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"workload pod with fsGroup should reach Running state — mount must not time out")
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		ginkgo.By("Writing a file through the mounted volume")
		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/fsgroup-test-%s.txt", volPath, uuid.New().String()[:8])
		testContent := "fsgroup-write-test"
		CreateFileInPod(testFramework, pod, testFile, testContent)

		ginkgo.By("Reading the file back to verify the mount is fully functional")
		e2evolume.VerifyExecInPodSucceed(testFramework, pod,
			fmt.Sprintf("cat %s | grep -q %q", testFile, testContent))
	})

	// --------------------------------------------------------------------
	// 3. Dynamic provisioning with workload pod fsGroup
	//
	// Verifies that dynamically provisioned S3 volumes work correctly when
	// the workload pod sets fsGroup in its PodSecurityContext. This combines
	// dynamic provisioning (StorageClass → PVC → automatic bucket creation)
	// with the fsGroup fix.
	//
	// Expected results:
	// - The PVC binds to a dynamically provisioned PV
	// - The workload pod with fsGroup reaches Running state
	// - Data can be written and read through the mounted volume
	ginkgo.It("should mount dynamically provisioned volume when workload pod has fsGroup set", func(ctx context.Context) {
		accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
		secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")
		if accessKey == "" || secretKey == "" {
			ginkgo.Skip("ACCOUNT1_ACCESS_KEY and ACCOUNT1_SECRET_KEY must be set for dynamic provisioning tests")
		}

		ginkgo.By("Creating S3 credential secret for dynamic provisioning")
		secretName := fmt.Sprintf("s3-secret-fsgroup-%s", uuid.New().String()[:8])
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: testFramework.Namespace.Name,
			},
			Data: map[string][]byte{
				"access_key_id":     []byte(accessKey),
				"secret_access_key": []byte(secretKey),
			},
		}
		_, err := testFramework.ClientSet.CoreV1().Secrets(testFramework.Namespace.Name).Create(ctx, secret, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create S3 credential secret")
		defer func() {
			delErr := testFramework.ClientSet.CoreV1().Secrets(testFramework.Namespace.Name).Delete(ctx, secretName, metav1.DeleteOptions{})
			if delErr != nil && !errors.IsNotFound(delErr) {
				framework.Logf("Warning: failed to delete secret %s: %v", secretName, delErr)
			}
		}()

		ginkgo.By("Creating StorageClass with mount options for non-root access")
		scName := fmt.Sprintf("fsgroup-dynamic-sc-%s", uuid.New().String()[:8])
		sc := &storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: scName},
			Provisioner: constants.DriverName,
			Parameters: map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      secretName,
				"csi.storage.k8s.io/provisioner-secret-namespace": testFramework.Namespace.Name,
			},
			MountOptions: []string{
				fmt.Sprintf("uid=%d", DefaultNonRootUser),
				fmt.Sprintf("gid=%d", DefaultNonRootGroup),
				"allow-other",
			},
			ReclaimPolicy:     &[]v1.PersistentVolumeReclaimPolicy{v1.PersistentVolumeReclaimDelete}[0],
			VolumeBindingMode: &[]storagev1.VolumeBindingMode{storagev1.VolumeBindingImmediate}[0],
		}
		sc, err = testFramework.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")
		defer func() {
			delErr := testFramework.ClientSet.StorageV1().StorageClasses().Delete(ctx, scName, metav1.DeleteOptions{})
			if delErr != nil && !errors.IsNotFound(delErr) {
				framework.Logf("Warning: failed to delete StorageClass %s: %v", scName, delErr)
			}
		}()

		ginkgo.By("Creating PVC referencing the StorageClass")
		pvcName := fmt.Sprintf("fsgroup-dynamic-pvc-%s", uuid.New().String()[:8])
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: testFramework.Namespace.Name,
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
		pvc, err = testFramework.ClientSet.CoreV1().PersistentVolumeClaims(testFramework.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		ginkgo.By("Waiting for PVC to be bound")
		WaitForPVCToBeBound(ctx, testFramework, pvc.Name, testFramework.Namespace.Name)

		ginkgo.By("Creating workload pod with fsGroup in PodSecurityContext")
		pod := e2epod.MakePod(testFramework.Namespace.Name, nil,
			[]*v1.PersistentVolumeClaim{pvc}, admissionapi.LevelRestricted, "")
		pod.Name = fmt.Sprintf("fsgroup-dynamic-pod-%s", uuid.New().String()[:8])
		podModifierNonRoot(pod)
		pod.Spec.SecurityContext.FSGroup = ptr.To(DefaultNonRootUser)

		pod, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"workload pod with fsGroup should reach Running state")
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod)

		ginkgo.By("Writing a file through the dynamically provisioned volume")
		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/fsgroup-dynamic-test-%s.txt", volPath, uuid.New().String()[:8])
		testContent := "fsgroup-dynamic-write-test"
		CreateFileInPod(testFramework, pod, testFile, testContent)

		ginkgo.By("Reading the file back to verify the mount is fully functional")
		e2evolume.VerifyExecInPodSucceed(testFramework, pod,
			fmt.Sprintf("cat %s | grep -q %q", testFile, testContent))
	})

	// --------------------------------------------------------------------
	// 4. Volume sharing: different fsGroup → separate mounter pods
	//
	// The CSI reconciler includes workload pod fsGroup in its matching
	// criteria when deciding whether to share a Mountpoint Pod. Two
	// workload pods referencing the same PersistentVolume but specifying
	// different fsGroup values must each get their own mounter pod.
	//
	// Diagram:
	//
	//      [Workload Pod 1]              [Workload Pod 2]
	//        fsGroup: 1001                 fsGroup: 3000
	//            |                              |
	//            ↓                              ↓
	//      [Mounter Pod A]              [Mounter Pod B]
	//        (volume: pv-X)               (volume: pv-X)
	//
	// Expected results:
	// - Both workload pods reach Running state
	// - Two separate mounter pods exist in the mount-s3 namespace for the same volume
	// - Both pods can independently write and read data
	ginkgo.It("should create separate mounter pods for workload pods with different fsGroup values", func(ctx context.Context) {
		ginkgo.By("Creating a shared volume with standard mount options")
		res := BuildVolumeWithOptions(ctx, testRegistry.config, pattern,
			DefaultNonRootUser, DefaultNonRootGroup, "0644")
		testRegistry.resources = append(testRegistry.resources, res)

		pvName := res.Pv.Name
		fsGroup1 := int64(1001)
		fsGroup2 := int64(3000)

		ginkgo.By(fmt.Sprintf("Creating first workload pod with fsGroup=%d", fsGroup1))
		pod1 := e2epod.MakePod(testFramework.Namespace.Name, nil,
			[]*v1.PersistentVolumeClaim{res.Pvc}, admissionapi.LevelRestricted, "")
		pod1.Name = fmt.Sprintf("fsgroup-share-1-%s", uuid.New().String()[:8])
		podModifierNonRoot(pod1)
		pod1.Spec.SecurityContext.FSGroup = ptr.To(fsGroup1)

		pod1, err := createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod1)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "first workload pod should reach Running state")
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod1)

		ginkgo.By(fmt.Sprintf("Creating second workload pod with fsGroup=%d (different)", fsGroup2))
		pod2 := e2epod.MakePod(testFramework.Namespace.Name, nil,
			[]*v1.PersistentVolumeClaim{res.Pvc}, admissionapi.LevelRestricted, "")
		pod2.Name = fmt.Sprintf("fsgroup-share-2-%s", uuid.New().String()[:8])
		podModifierNonRoot(pod2)
		pod2.Spec.SecurityContext.FSGroup = ptr.To(fsGroup2)

		pod2, err = createPod(ctx, testFramework.ClientSet, testFramework.Namespace.Name, pod2)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "second workload pod should reach Running state")
		defer e2epod.DeletePodWithWait(ctx, testFramework.ClientSet, pod2)

		ginkgo.By(fmt.Sprintf("Verifying separate mounter pods exist for volume %s", pvName))
		gomega.Eventually(func(g gomega.Gomega) {
			pods, listErr := testFramework.ClientSet.CoreV1().Pods(mounterPodNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", labelVolumeName, pvName),
			})
			g.Expect(listErr).NotTo(gomega.HaveOccurred())
			g.Expect(pods.Items).To(gomega.HaveLen(2),
				"expected 2 separate mounter pods for workload pods with different fsGroup values, got %d",
				len(pods.Items))
		}).WithTimeout(2 * time.Minute).WithPolling(5 * time.Second).Should(gomega.Succeed())

		ginkgo.By("Verifying both workload pods can write data independently")
		volPath := "/mnt/volume1"
		testFile1 := fmt.Sprintf("%s/share-test-1-%s.txt", volPath, uuid.New().String()[:8])
		testFile2 := fmt.Sprintf("%s/share-test-2-%s.txt", volPath, uuid.New().String()[:8])
		CreateFileInPod(testFramework, pod1, testFile1, "pod1-data")
		CreateFileInPod(testFramework, pod2, testFile2, "pod2-data")

		e2evolume.VerifyExecInPodSucceed(testFramework, pod1,
			fmt.Sprintf("cat %s | grep -q %q", testFile1, "pod1-data"))
		e2evolume.VerifyExecInPodSucceed(testFramework, pod2,
			fmt.Sprintf("cat %s | grep -q %q", testFile2, "pod2-data"))
	})
}
