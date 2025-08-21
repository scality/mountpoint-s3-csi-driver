// This file implements comprehensive tests for mount options support in dynamic provisioning.
// It validates that the S3 CSI driver correctly handles StorageClass mountOptions when
// dynamically creating volumes, including combinations with credentials and policy enforcement.
package customsuites

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/onsi/ginkgo/v2"
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	e2evolume "k8s.io/kubernetes/test/e2e/framework/volume"
	storageframework "k8s.io/kubernetes/test/e2e/storage/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/constants"
	"github.com/scality/mountpoint-s3-csi-driver/tests/e2e/pkg/s3client"
)

// s3CSIDynamicProvisioningMountOptionsTestSuite implements the Kubernetes storage framework
// TestSuite interface to validate mount options support in dynamic provisioning scenarios.
//
// This suite verifies that:
// - StorageClass mountOptions are correctly passed to dynamically created PVs
// - Mount options work with various credential configurations
// - Mount args policy correctly filters dangerous options
// - Mount options work with different access modes and volume capabilities
type s3CSIDynamicProvisioningMountOptionsTestSuite struct {
	tsInfo storageframework.TestSuiteInfo
}

// InitS3DynamicProvisioningMountOptionsTestSuite initializes and returns a test suite
// that validates mount options functionality for dynamic provisioning in the S3 CSI driver.
func InitS3DynamicProvisioningMountOptionsTestSuite() storageframework.TestSuite {
	return &s3CSIDynamicProvisioningMountOptionsTestSuite{
		tsInfo: storageframework.TestSuiteInfo{
			Name: "dynamic-provisioning-mount-options",
			TestPatterns: []storageframework.TestPattern{
				storageframework.DefaultFsDynamicPV,
			},
		},
	}
}

func (t *s3CSIDynamicProvisioningMountOptionsTestSuite) GetTestSuiteInfo() storageframework.TestSuiteInfo {
	return t.tsInfo
}

func (t *s3CSIDynamicProvisioningMountOptionsTestSuite) SkipUnsupportedTests(_ storageframework.TestDriver, _ storageframework.TestPattern) {
}

func (t *s3CSIDynamicProvisioningMountOptionsTestSuite) DefineTests(driver storageframework.TestDriver, pattern storageframework.TestPattern) {
	type local struct {
		resources    []*storageframework.VolumeResource
		config       *storageframework.PerTestConfig
		storageClass *storagev1.StorageClass
		s3Client     *s3client.Client
	}
	var l local

	f := framework.NewFrameworkWithCustomTimeouts("dynamic-provisioning-mount-options", storageframework.GetDriverTimeouts(driver))
	f.NamespacePodSecurityLevel = admissionapi.LevelRestricted

	cleanup := func(ctx context.Context) {
		var errs []error
		for _, resource := range l.resources {
			if err := resource.CleanupResource(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			framework.Logf("Cleanup errors: %v", errs)
		}

		// Clean up custom StorageClass if created
		if l.storageClass != nil {
			err := f.ClientSet.StorageV1().StorageClasses().Delete(ctx, l.storageClass.Name, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				framework.Logf("Failed to delete StorageClass %s: %v", l.storageClass.Name, err)
			}
		}

		// Clean up s3-secret if it was created by this test
		err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Delete(ctx, "s3-secret", metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			framework.Logf("Failed to delete s3-secret from test namespace: %v", err)
		}
	}

	// createDefaultS3Secret creates the s3-secret in the test namespace
	// that tests expect to exist when no explicit provisioner secret is configured
	createDefaultS3Secret := func(ctx context.Context) {
		accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
		secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")

		// Fallback to test credentials if environment variables are not set
		if accessKey == "" {
			accessKey = "test-access-key"
		}
		if secretKey == "" {
			secretKey = "test-secret-key"
		}

		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "s3-secret",
				Namespace: f.Namespace.Name,
			},
			Data: map[string][]byte{
				"access_key_id":     []byte(accessKey),
				"secret_access_key": []byte(secretKey),
			},
		}

		_, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			framework.ExpectNoError(err, "Failed to create s3-secret in test namespace")
		} else if errors.IsAlreadyExists(err) {
			_, err = f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Update(ctx, secret, metav1.UpdateOptions{})
			framework.ExpectNoError(err, "Failed to update s3-secret in test namespace")
		}
	}

	ginkgo.BeforeEach(func(ctx context.Context) {
		l = local{}
		l.config = driver.PrepareTest(ctx, f)
		l.s3Client = s3client.New("", "", "")

		createDefaultS3Secret(ctx)

		ginkgo.DeferCleanup(cleanup)
	})

	// createStorageClassWithMountOptions creates a custom StorageClass with specified mount options
	createStorageClassWithMountOptions := func(ctx context.Context, mountOptions []string, parameters map[string]string, suffix string) *storagev1.StorageClass {
		scName := fmt.Sprintf("mount-options-test-%s", suffix)

		driverName := constants.DriverName

		if parameters == nil {
			parameters = map[string]string{}
		}

		if _, hasProvisionerSecret := parameters["csi.storage.k8s.io/provisioner-secret-name"]; !hasProvisionerSecret {
			parameters["csi.storage.k8s.io/provisioner-secret-name"] = "s3-secret"
			parameters["csi.storage.k8s.io/provisioner-secret-namespace"] = f.Namespace.Name
		}

		sc := &storagev1.StorageClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: scName,
			},
			Provisioner:       driverName,
			Parameters:        parameters,
			MountOptions:      mountOptions,
			ReclaimPolicy:     &[]v1.PersistentVolumeReclaimPolicy{v1.PersistentVolumeReclaimDelete}[0],
			VolumeBindingMode: &[]storagev1.VolumeBindingMode{storagev1.VolumeBindingImmediate}[0],
		}

		var err error
		sc, err = f.ClientSet.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create StorageClass")

		return sc
	}

	// createPVCWithStorageClass creates a PVC that uses the specified StorageClass
	createPVCWithStorageClass := func(ctx context.Context, sc *storagev1.StorageClass, pvcName string) *v1.PersistentVolumeClaim {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: f.Namespace.Name,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{
					v1.ReadWriteMany,
				},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
				StorageClassName: &sc.Name,
			},
		}

		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Create(ctx, pvc, metav1.CreateOptions{})
		framework.ExpectNoError(err, "Failed to create PVC")

		err = e2epv.WaitForPersistentVolumeClaimPhase(ctx, v1.ClaimBound, f.ClientSet, pvc.Namespace, pvc.Name, framework.Poll, framework.ClaimProvisionTimeout)
		framework.ExpectNoError(err, "PVC should be bound")

		return pvc
	}

	ginkgo.It("should apply basic mount options from StorageClass to dynamically provisioned volume", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with basic mount options")
		mountOptions := []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other",
			"file-mode=644",
			"dir-mode=755",
		}

		l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, nil, "basic")

		ginkgo.By("Creating PVC with the StorageClass")
		pvc := createPVCWithStorageClass(ctx, l.storageClass, "basic-mount-options-pvc")

		ginkgo.By("Creating pod that mounts the dynamically provisioned volume")
		pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
		pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/mount-options-test.txt", volPath)
		testContent := "testing mount options in dynamic provisioning"

		ginkgo.By("Verifying volume can be written to with correct permissions")
		WriteAndVerifyFile(f, pod, testFile, testContent)

		ginkgo.By("Verifying file has correct ownership and permissions")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -L -c '%%a %%g %%u' %s | grep '644 %d %d'",
			testFile, DefaultNonRootGroup, DefaultNonRootUser))

		ginkgo.By("Verifying directory has correct ownership and permissions")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -L -c '%%a %%g %%u' %s | grep '755 %d %d'",
			volPath, DefaultNonRootGroup, DefaultNonRootUser))
	})

	ginkgo.It("should work with prefix mount option", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with prefix mount option")
		prefix := "dynamic-test-prefix/"
		mountOptions := []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other",
			fmt.Sprintf("prefix=%s", prefix),
		}

		l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, nil, "prefix")

		ginkgo.By("Creating PVC with the StorageClass")
		pvc := createPVCWithStorageClass(ctx, l.storageClass, "prefix-mount-options-pvc")

		pvc, err := f.ClientSet.CoreV1().PersistentVolumeClaims(f.Namespace.Name).Get(ctx, pvc.Name, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get updated PVC")

		if pvc.Spec.VolumeName == "" {
			framework.Failf("PVC %s is bound but Spec.VolumeName is empty", pvc.Name)
		}

		pv, err := f.ClientSet.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		framework.ExpectNoError(err, "Failed to get PV")

		bucketName := pv.Spec.CSI.VolumeAttributes["bucketName"]
		framework.ExpectNoError(err, "Failed to get bucket name from PV")

		ginkgo.By("Creating pod that mounts the volume with prefix")
		pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
		pod, err = createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		volPath := "/mnt/volume1"
		testFileName := "prefix-test.txt"
		testFile := fmt.Sprintf("%s/%s", volPath, testFileName)
		testContent := "testing prefix mount options in dynamic provisioning"

		ginkgo.By("Writing file to volume with prefix")
		WriteAndVerifyFile(f, pod, testFile, testContent)

		ginkgo.By("Verifying file exists under prefix in S3")
		err = l.s3Client.VerifyObjectsExistInS3(ctx, bucketName, prefix, []string{testFileName})
		framework.ExpectNoError(err, "File should exist under prefix in S3")
	})

	ginkgo.It("should work with read-only mount option", func(ctx context.Context) {
		ginkgo.By("Creating StorageClass with read-only mount option")
		mountOptions := []string{
			fmt.Sprintf("uid=%d", DefaultNonRootUser),
			fmt.Sprintf("gid=%d", DefaultNonRootGroup),
			"allow-other",
			"read-only",
		}

		l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, nil, "readonly")

		ginkgo.By("Creating PVC with the StorageClass")
		pvc := createPVCWithStorageClass(ctx, l.storageClass, "readonly-mount-options-pvc")

		ginkgo.By("Creating pod that mounts the read-only volume")
		pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
		pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
		framework.ExpectNoError(err)
		defer func() {
			framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
		}()

		volPath := "/mnt/volume1"
		testFile := fmt.Sprintf("%s/should-fail.txt", volPath)

		ginkgo.By("Verifying read access works")
		e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("ls -la %s", volPath))

		ginkgo.By("Verifying write access is denied")
		_, stderr, err := e2evolume.PodExec(f, pod, fmt.Sprintf("touch %s", testFile))
		if err == nil {
			framework.Failf("Expected write to fail on read-only volume")
		}
		if !strings.Contains(stderr, "Read-only file system") {
			framework.Failf("Expected 'Read-only file system' error, got: %s", stderr)
		}
	})

	ginkgo.Describe("Mount options with credential configurations", func() {
		createSecretForTest := func(ctx context.Context, secretName string) *v1.Secret {
			accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
			secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")

			// Fallback to test credentials if environment variables are not set
			if accessKey == "" {
				accessKey = "test-access-key"
			}
			if secretKey == "" {
				secretKey = "test-secret-key"
			}

			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: f.Namespace.Name,
				},
				Data: map[string][]byte{
					"access_key_id":     []byte(accessKey),
					"secret_access_key": []byte(secretKey),
				},
			}

			secret, err := f.ClientSet.CoreV1().Secrets(f.Namespace.Name).Create(ctx, secret, metav1.CreateOptions{})
			framework.ExpectNoError(err)
			return secret
		}

		ginkgo.It("should work with provisioner secret and mount options", func(ctx context.Context) {
			ginkgo.By("Creating provisioner secret")
			provisionerSecret := createSecretForTest(ctx, "provisioner-secret")

			ginkgo.By("Creating StorageClass with provisioner secret and mount options")
			mountOptions := []string{
				fmt.Sprintf("uid=%d", DefaultNonRootUser),
				fmt.Sprintf("gid=%d", DefaultNonRootGroup),
				"allow-other",
			}

			parameters := map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":      provisionerSecret.Name,
				"csi.storage.k8s.io/provisioner-secret-namespace": provisionerSecret.Namespace,
			}

			l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, parameters, "provisioner-cred")

			ginkgo.By("Creating PVC with the StorageClass")
			pvc := createPVCWithStorageClass(ctx, l.storageClass, "provisioner-cred-pvc")

			ginkgo.By("Creating pod that uses the dynamically provisioned volume")
			pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
			pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err)
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			volPath := "/mnt/volume1"
			testFile := fmt.Sprintf("%s/provisioner-cred-test.txt", volPath)

			ginkgo.By("Verifying volume works with provisioner secret and mount options")
			WriteAndVerifyFile(f, pod, testFile, "provisioner secret + mount options test")
		})

		ginkgo.It("should work with node-publish secret and mount options", func(ctx context.Context) {
			ginkgo.By("Creating node-publish secret")
			nodeSecret := createSecretForTest(ctx, "node-secret")

			ginkgo.By("Creating StorageClass with node-publish secret and mount options")
			mountOptions := []string{
				fmt.Sprintf("uid=%d", DefaultNonRootUser),
				fmt.Sprintf("gid=%d", DefaultNonRootGroup),
				"allow-other",
				"debug",
			}

			parameters := map[string]string{
				"csi.storage.k8s.io/node-publish-secret-name":      nodeSecret.Name,
				"csi.storage.k8s.io/node-publish-secret-namespace": nodeSecret.Namespace,
			}

			l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, parameters, "node-cred")

			ginkgo.By("Creating PVC with the StorageClass")
			pvc := createPVCWithStorageClass(ctx, l.storageClass, "node-cred-pvc")

			ginkgo.By("Creating pod that uses the dynamically provisioned volume")
			pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
			pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err)
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			volPath := "/mnt/volume1"
			testFile := fmt.Sprintf("%s/node-cred-test.txt", volPath)

			ginkgo.By("Verifying volume works with node-publish secret and mount options")
			WriteAndVerifyFile(f, pod, testFile, "node secret + mount options test")
		})

		ginkgo.It("should work with both provisioner and node-publish secrets plus mount options", func(ctx context.Context) {
			ginkgo.By("Creating both provisioner and node-publish secrets")
			provisionerSecret := createSecretForTest(ctx, "both-provisioner-secret")
			nodeSecret := createSecretForTest(ctx, "both-node-secret")

			ginkgo.By("Creating StorageClass with both secrets and mount options")
			mountOptions := []string{
				fmt.Sprintf("uid=%d", DefaultNonRootUser),
				fmt.Sprintf("gid=%d", DefaultNonRootGroup),
				"allow-other",
				"file-mode=644",
				"dir-mode=755",
			}

			parameters := map[string]string{
				"csi.storage.k8s.io/provisioner-secret-name":       provisionerSecret.Name,
				"csi.storage.k8s.io/provisioner-secret-namespace":  provisionerSecret.Namespace,
				"csi.storage.k8s.io/node-publish-secret-name":      nodeSecret.Name,
				"csi.storage.k8s.io/node-publish-secret-namespace": nodeSecret.Namespace,
			}

			l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, parameters, "both-creds")

			ginkgo.By("Creating PVC with the StorageClass")
			pvc := createPVCWithStorageClass(ctx, l.storageClass, "both-creds-pvc")

			ginkgo.By("Creating pod that uses the dynamically provisioned volume")
			pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
			pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err)
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			volPath := "/mnt/volume1"
			testFile := fmt.Sprintf("%s/both-creds-test.txt", volPath)
			testDir := fmt.Sprintf("%s/both-creds-dir", volPath)

			ginkgo.By("Verifying volume works with both secrets and mount options")
			WriteAndVerifyFile(f, pod, testFile, "both secrets + mount options test")

			ginkgo.By("Creating directory to test dir permissions")
			CreateDirInPod(f, pod, testDir)

			ginkgo.By("Verifying file and directory permissions are correct")
			e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -L -c '%%a %%g %%u' %s | grep '644 %d %d'",
				testFile, DefaultNonRootGroup, DefaultNonRootUser))
			e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -L -c '%%a %%g %%u' %s | grep '755 %d %d'",
				testDir, DefaultNonRootGroup, DefaultNonRootUser))
		})
	})

	ginkgo.Describe("Mount args policy enforcement in dynamic provisioning", func() {
		testPolicyEnforcement := func(ctx context.Context, badMountOptions []string, testName string) {
			ginkgo.By(fmt.Sprintf("Creating StorageClass with disallowed mount options: %v", badMountOptions))

			mountOptions := []string{
				fmt.Sprintf("uid=%d", DefaultNonRootUser),
				fmt.Sprintf("gid=%d", DefaultNonRootGroup),
				"allow-other",
			}
			mountOptions = append(mountOptions, badMountOptions...)

			l.storageClass = createStorageClassWithMountOptions(ctx, mountOptions, nil, testName)

			ginkgo.By("Creating PVC with the StorageClass")
			pvc := createPVCWithStorageClass(ctx, l.storageClass, fmt.Sprintf("%s-pvc", testName))

			ginkgo.By("Creating pod that mounts the volume")
			pod := MakeNonRootPodWithVolume(f.Namespace.Name, []*v1.PersistentVolumeClaim{pvc}, "")
			pod, err := createPod(ctx, f.ClientSet, f.Namespace.Name, pod)
			framework.ExpectNoError(err)
			defer func() {
				framework.ExpectNoError(e2epod.DeletePodWithWait(ctx, f.ClientSet, pod))
			}()

			volPath := "/mnt/volume1"
			testFile := fmt.Sprintf("%s/policy-test.txt", volPath)

			ginkgo.By("Verifying volume works despite disallowed options (proving they were stripped)")
			WriteAndVerifyFile(f, pod, testFile, fmt.Sprintf("policy enforcement test: %s", testName))

			ginkgo.By("Verifying file ownership is correct (proving valid options work)")
			e2evolume.VerifyExecInPodSucceed(f, pod, fmt.Sprintf("stat -L -c '%%g %%u' %s | grep '%d %d'",
				testFile, DefaultNonRootGroup, DefaultNonRootUser))
		}

		ginkgo.It("should strip --endpoint-url from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"--endpoint-url=https://wrong.example.com"}, "endpoint-url")
		})

		ginkgo.It("should strip --profile from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"--profile=bad-profile"}, "profile")
		})

		ginkgo.It("should strip --cache-xz from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"--cache-xz"}, "cache-xz")
		})

		ginkgo.It("should strip --incremental-upload from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"--incremental-upload"}, "incremental-upload")
		})

		ginkgo.It("should strip --storage-class from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"--storage-class=EXPRESS_ONEZONE"}, "storage-class")
		})

		ginkgo.It("should strip -o from mount options", func(ctx context.Context) {
			testPolicyEnforcement(ctx, []string{"-o"}, "fs-tab")
		})

		ginkgo.It("should strip multiple disallowed mount options", func(ctx context.Context) {
			badOptions := []string{
				"--endpoint-url=https://wrong.example.com",
				"--profile=bad-profile",
				"--cache-xz",
				"--incremental-upload",
				"--storage-class=EXPRESS_ONEZONE",
			}
			testPolicyEnforcement(ctx, badOptions, "multiple-bad")
		})
	})
}
