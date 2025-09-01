//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// =============================================================================
// Constants and Variables
// =============================================================================

const (
	staticTestBucketName = "upgrade-test-static"
	staticTestPVName     = "upgrade-test-pv"
	staticTestPVCName    = "upgrade-test-pvc"
	staticTestPodName    = "upgrade-test-pod"
	testNamespace        = "default"
)

// =============================================================================
// Public Mage Targets (Entry Points)
// =============================================================================

// SetupStaticProvisioning creates resources for static provisioning upgrade test
func SetupStaticProvisioning() error {
	fmt.Println("🔧 Setting up static provisioning upgrade test...")

	// Ensure credentials are loaded
	mg.Deps(LoadCredentials)

	// Create S3 bucket
	if err := createStaticTestBucket(); err != nil {
		return fmt.Errorf("failed to create test bucket: %v", err)
	}

	// Create PV
	if err := createStaticTestPV(); err != nil {
		return fmt.Errorf("failed to create PV: %v", err)
	}

	// Create PVC
	if err := createStaticTestPVC(); err != nil {
		return fmt.Errorf("failed to create PVC: %v", err)
	}

	// Create Pod (PVC will bind when pod is created for static provisioning)
	if err := createStaticTestPod(); err != nil {
		return fmt.Errorf("failed to create Pod: %v", err)
	}

	// Wait for pod to be ready (this also ensures PVC is bound)
	if err := waitForPodReady(); err != nil {
		return fmt.Errorf("Pod failed to be ready: %v", err)
	}

	// Write initial test data
	if err := writeTestData("before-upgrade.txt", "Data written before upgrade"); err != nil {
		return fmt.Errorf("failed to write test data: %v", err)
	}

	fmt.Println("✅ Static provisioning test setup complete")
	return nil
}

// VerifyStaticProvisioning verifies static provisioning works after upgrade
func VerifyStaticProvisioning() error {
	fmt.Println("🔍 Verifying static provisioning after upgrade...")

	// Check pod is still running
	if err := verifyPodRunning(); err != nil {
		return fmt.Errorf("❌ Pod check failed: %v", err)
	}

	// Verify old data persists
	if err := verifyTestDataExists("before-upgrade.txt", "Data written before upgrade"); err != nil {
		return fmt.Errorf("❌ Data persistence check failed: %v", err)
	}

	// Write new data after upgrade
	if err := writeTestData("after-upgrade.txt", "Data written after upgrade"); err != nil {
		return fmt.Errorf("❌ New write check failed: %v", err)
	}

	// Verify new data
	if err := verifyTestDataExists("after-upgrade.txt", "Data written after upgrade"); err != nil {
		return fmt.Errorf("❌ New data verification failed: %v", err)
	}

	fmt.Println("✅ Static provisioning upgrade verification successful!")
	return nil
}

// CleanupStaticProvisioning removes static test resources
func CleanupStaticProvisioning() error {
	fmt.Println("🧹 Cleaning up static provisioning test resources...")

	// Ensure credentials are loaded for S3 bucket deletion
	mg.Deps(LoadCredentials)

	// Delete Pod
	_ = sh.Run("kubectl", "delete", "pod", staticTestPodName, "-n", testNamespace, "--ignore-not-found=true")

	// Delete PVC
	_ = sh.Run("kubectl", "delete", "pvc", staticTestPVCName, "-n", testNamespace, "--ignore-not-found=true")

	// Delete PV
	_ = sh.Run("kubectl", "delete", "pv", staticTestPVName, "--ignore-not-found=true")

	// Delete S3 bucket
	if err := deleteStaticTestBucket(); err != nil {
		fmt.Printf("Warning: Failed to delete test bucket: %v\n", err)
	}

	fmt.Println("✅ Static provisioning test cleanup complete")
	return nil
}

// =============================================================================
// S3 Client and Operations
// =============================================================================

// getS3Client creates and returns an S3 client configured with credentials and endpoint
func getS3Client() (*s3.Client, error) {
	accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
	secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("credentials not loaded")
	}

	// Load AWS config with static credentials
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(aws.NewCredentialsCache(
			aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     accessKey,
					SecretAccessKey: secretKey,
				}, nil
			}),
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create S3 client with custom endpoint resolver
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(GetS3EndpointURL())
		o.UsePathStyle = true
	})

	return client, nil
}

func createStaticTestBucket() error {
	fmt.Printf("Creating S3 bucket: %s\n", staticTestBucketName)

	client, err := getS3Client()
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	_, err = client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(staticTestBucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %v", staticTestBucketName, err)
	}

	fmt.Printf("✓ S3 bucket %s created\n", staticTestBucketName)
	return nil
}

func deleteStaticTestBucket() error {
	fmt.Printf("Deleting S3 bucket: %s\n", staticTestBucketName)

	client, err := getS3Client()
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	// First list and delete all objects in the bucket
	listResp, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(staticTestBucketName),
	})
	if err != nil {
		fmt.Printf("Warning: Failed to list objects in bucket %s: %v\n", staticTestBucketName, err)
	} else if listResp.Contents != nil && len(listResp.Contents) > 0 {
		for _, obj := range listResp.Contents {
			_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(staticTestBucketName),
				Key:    obj.Key,
			})
			if err != nil {
				fmt.Printf("Warning: Failed to delete object %s: %v\n", *obj.Key, err)
			}
		}
	}

	// Then delete the bucket
	_, err = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
		Bucket: aws.String(staticTestBucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %v", staticTestBucketName, err)
	}

	fmt.Printf("✓ S3 bucket %s deleted\n", staticTestBucketName)
	return nil
}

// =============================================================================
// Kubernetes Resource Operations
// =============================================================================

func createStaticTestPV() error {
	fmt.Printf("Creating PV: %s\n", staticTestPVName)

	if err := sh.Run("kubectl", "apply", "-f", "tests/upgrade/manifests/static-pv.yaml"); err != nil {
		return fmt.Errorf("failed to create PV: %v", err)
	}

	fmt.Printf("✓ PV %s created\n", staticTestPVName)
	return nil
}

func createStaticTestPVC() error {
	fmt.Printf("Creating PVC: %s\n", staticTestPVCName)

	if err := sh.Run("kubectl", "apply", "-f", "tests/upgrade/manifests/static-pvc.yaml"); err != nil {
		return fmt.Errorf("failed to create PVC: %v", err)
	}

	fmt.Printf("✓ PVC %s created\n", staticTestPVCName)
	return nil
}

func createStaticTestPod() error {
	fmt.Printf("Creating Pod: %s\n", staticTestPodName)

	if err := sh.Run("kubectl", "apply", "-f", "tests/upgrade/manifests/static-pod.yaml"); err != nil {
		return fmt.Errorf("failed to create Pod: %v", err)
	}

	fmt.Printf("✓ Pod %s created\n", staticTestPodName)
	return nil
}

func waitForPVCBound() error {
	fmt.Println("Waiting for PVC to be bound...")

	if err := sh.Run("kubectl", "wait", "--for=condition=bound",
		fmt.Sprintf("pvc/%s", staticTestPVCName),
		"-n", testNamespace,
		"--timeout=60s"); err != nil {
		return fmt.Errorf("PVC did not bind within timeout: %v", err)
	}

	fmt.Println("✓ PVC bound successfully")
	return nil
}

func waitForPodReady() error {
	fmt.Println("Waiting for pod to be ready...")

	if err := sh.Run("kubectl", "wait", "--for=condition=Ready",
		fmt.Sprintf("pod/%s", staticTestPodName),
		"-n", testNamespace,
		"--timeout=120s"); err != nil {
		return fmt.Errorf("Pod did not become ready within timeout: %v", err)
	}

	fmt.Println("✓ Pod ready")
	return nil
}

// =============================================================================
// Test Data Operations
// =============================================================================

func writeTestData(filename, content string) error {
	fmt.Printf("Writing test data to %s...\n", filename)

	if err := sh.Run("kubectl", "exec", staticTestPodName, "-n", testNamespace, "--",
		"sh", "-c", fmt.Sprintf("echo '%s' > /data/%s", content, filename)); err != nil {
		return fmt.Errorf("failed to write test data: %v", err)
	}

	fmt.Printf("✓ Test data written to %s\n", filename)
	return nil
}

func verifyTestDataExists(filename, expectedContent string) error {
	fmt.Printf("Verifying test data in %s...\n", filename)

	output, err := sh.Output("kubectl", "exec", staticTestPodName, "-n", testNamespace, "--",
		"cat", fmt.Sprintf("/data/%s", filename))
	if err != nil {
		return fmt.Errorf("failed to read test data: %v", err)
	}

	if !strings.Contains(output, expectedContent) {
		return fmt.Errorf("test data mismatch - expected: %s, got: %s", expectedContent, output)
	}

	fmt.Printf("✓ Test data verified in %s\n", filename)
	return nil
}

// =============================================================================
// Verification Operations
// =============================================================================

func verifyPodRunning() error {
	fmt.Println("Checking pod status...")

	output, err := sh.Output("kubectl", "get", "pod", staticTestPodName,
		"-n", testNamespace,
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to get pod status: %v", err)
	}

	if strings.TrimSpace(output) != "Running" {
		return fmt.Errorf("pod not running, status: %s", output)
	}

	fmt.Println("✓ Pod is running")
	return nil
}
