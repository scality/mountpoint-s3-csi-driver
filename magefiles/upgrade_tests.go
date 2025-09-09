//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// =============================================================================
// Test Configuration
// =============================================================================

type TestConfig struct {
	Type          string
	RSName        string // ReplicaSet name
	LabelSelector string // Label selector for pods
	PVCName       string
	PVName        string // only for static
	SCName        string // only for dynamic
	BucketName    string // only for static
	Namespace     string
	PVCTimeout    int // seconds
	PodTimeout    int // seconds
	ManifestPath  string
}

var staticConfig = TestConfig{
	Type:          "static",
	RSName:        "upgrade-test-replicaset",
	LabelSelector: "app=upgrade-test-static",
	PVCName:       "upgrade-test-pvc",
	PVName:        "upgrade-test-pv",
	BucketName:    "upgrade-test-static",
	Namespace:     "default",
	PVCTimeout:    60,
	PodTimeout:    120,
	ManifestPath:  "tests/upgrade/manifests/static",
}

var dynamicConfig = TestConfig{
	Type:          "dynamic",
	RSName:        "upgrade-test-dynamic-replicaset",
	LabelSelector: "app=upgrade-test-dynamic",
	PVCName:       "upgrade-test-dynamic-pvc",
	SCName:        "upgrade-test-sc",
	Namespace:     "default",
	PVCTimeout:    120,
	PodTimeout:    120,
	ManifestPath:  "tests/upgrade/manifests/dynamic",
}

// =============================================================================
// Public Mage Targets (Entry Points)
// =============================================================================

// SetupStaticProvisioning creates resources for static provisioning upgrade test
func SetupStaticProvisioning() error {
	return setupTest(staticConfig)
}

// SetupDynamicProvisioning creates resources for dynamic provisioning upgrade test
func SetupDynamicProvisioning() error {
	return setupTest(dynamicConfig)
}

// VerifyStaticProvisioning verifies static provisioning works after upgrade
func VerifyStaticProvisioning() error {
	return verifyTest(staticConfig, "Data written before upgrade", "Data written after upgrade")
}

// VerifyDynamicProvisioning verifies dynamic provisioning works after upgrade
func VerifyDynamicProvisioning() error {
	return verifyTest(dynamicConfig, "Dynamic data written before upgrade", "Dynamic data written after upgrade")
}

// CleanupStaticProvisioning removes static test resources
func CleanupStaticProvisioning() error {
	return cleanupTest(staticConfig)
}

// CleanupDynamicProvisioning removes dynamic test resources
func CleanupDynamicProvisioning() error {
	return cleanupTest(dynamicConfig)
}

// SetupUpgradeTests creates both static and dynamic provisioning test resources
func SetupUpgradeTests() error {
	fmt.Println("Setting up upgrade tests (static + dynamic provisioning)...")

	fmt.Println("\n--- Setting up static provisioning test ---")
	if err := SetupStaticProvisioning(); err != nil {
		return fmt.Errorf("static provisioning setup failed: %v", err)
	}

	fmt.Println("\n--- Setting up dynamic provisioning test ---")
	if err := SetupDynamicProvisioning(); err != nil {
		return fmt.Errorf("dynamic provisioning setup failed: %v", err)
	}

	fmt.Println("✓ All upgrade tests setup complete")
	return nil
}

// VerifyUpgradeTests verifies both static and dynamic provisioning after upgrade
func VerifyUpgradeTests() error {
	fmt.Println("Verifying upgrade tests (static + dynamic provisioning)...")

	fmt.Println("\n--- Verifying static provisioning ---")
	if err := VerifyStaticProvisioning(); err != nil {
		return fmt.Errorf("static provisioning verification failed: %v", err)
	}

	fmt.Println("\n--- Verifying dynamic provisioning ---")
	if err := VerifyDynamicProvisioning(); err != nil {
		return fmt.Errorf("dynamic provisioning verification failed: %v", err)
	}

	fmt.Println("✓ All upgrade tests verification complete")
	return nil
}

// CleanupUpgradeTests removes all static and dynamic test resources
func CleanupUpgradeTests() error {
	fmt.Println("Cleaning up all upgrade test resources...")

	fmt.Println("\n--- Cleaning up static provisioning ---")
	if err := CleanupStaticProvisioning(); err != nil {
		fmt.Printf("✗ Warning: Static cleanup failed: %v\n", err)
	}

	fmt.Println("\n--- Cleaning up dynamic provisioning ---")
	if err := CleanupDynamicProvisioning(); err != nil {
		fmt.Printf("✗ Warning: Dynamic cleanup failed: %v\n", err)
	}

	fmt.Println("✓ All upgrade tests cleanup complete")
	return nil
}

// =============================================================================
// Generic Test Implementation
// =============================================================================

func setupTest(config TestConfig) error {
	fmt.Printf("Setting up %s provisioning upgrade test...\n", config.Type)

	// Ensure credentials are loaded
	mg.Deps(LoadCredentials)

	// Static-specific setup
	if config.Type == "static" {
		if err := createS3Bucket(config.BucketName); err != nil {
			return fmt.Errorf("failed to create S3 bucket: %v", err)
		}
		if err := applyManifest(config, "pv"); err != nil {
			return fmt.Errorf("failed to create PV: %v", err)
		}
	}

	// Dynamic-specific setup
	if config.Type == "dynamic" {
		if err := applyManifest(config, "storageclass"); err != nil {
			return fmt.Errorf("failed to create StorageClass: %v", err)
		}
	}

	// Common setup steps
	if err := applyManifest(config, "pvc"); err != nil {
		return fmt.Errorf("failed to create PVC: %v", err)
	}

	if err := waitForPVCBound(config); err != nil {
		return fmt.Errorf("PVC failed to bind: %v", err)
	}

	if err := applyManifest(config, "replicaset"); err != nil {
		return fmt.Errorf("failed to create ReplicaSet: %v", err)
	}

	if err := waitForPodReady(config); err != nil {
		return fmt.Errorf("Pod failed to be ready: %v", err)
	}

	// Write initial test data
	dataContent := "Data written before upgrade"
	if config.Type == "dynamic" {
		dataContent = "Dynamic data written before upgrade"
	}
	if err := writeTestData(config, "before-upgrade.txt", dataContent); err != nil {
		return fmt.Errorf("failed to write test data: %v", err)
	}

	fmt.Printf("✓ %s provisioning test setup complete\n", strings.Title(config.Type))
	return nil
}

func verifyTest(config TestConfig, beforeContent, afterContent string) error {
	fmt.Printf("Verifying %s provisioning after upgrade...\n", config.Type)

	// Check pod is still running
	if err := verifyPodRunning(config); err != nil {
		return fmt.Errorf("✗ Pod check failed: %v", err)
	}

	// Verify old data persists
	if err := verifyTestDataExists(config, "before-upgrade.txt", beforeContent); err != nil {
		return fmt.Errorf("✗ Data persistence check failed: %v", err)
	}

	// Write new data after upgrade
	if err := writeTestData(config, "after-upgrade.txt", afterContent); err != nil {
		return fmt.Errorf("✗ New write check failed: %v", err)
	}

	// Verify new data
	if err := verifyTestDataExists(config, "after-upgrade.txt", afterContent); err != nil {
		return fmt.Errorf("✗ New data verification failed: %v", err)
	}

	fmt.Printf("✓ %s provisioning upgrade verification successful!\n", strings.Title(config.Type))
	return nil
}

func cleanupTest(config TestConfig) error {
	fmt.Printf("Cleaning up %s provisioning test resources...\n", config.Type)

	// Delete ReplicaSet
	_ = sh.Run("kubectl", "delete", "replicaset", config.RSName, "-n", config.Namespace, "--ignore-not-found=true")

	// Delete PVC
	_ = sh.Run("kubectl", "delete", "pvc", config.PVCName, "-n", config.Namespace, "--ignore-not-found=true")

	// Static-specific cleanup
	if config.Type == "static" {
		_ = sh.Run("kubectl", "delete", "pv", config.PVName, "--ignore-not-found=true")

		// Delete S3 bucket (requires credentials)
		mg.Deps(LoadCredentials)
		if err := deleteS3Bucket(config.BucketName); err != nil {
			fmt.Printf("✗ Warning: Failed to delete test bucket: %v\n", err)
		}
	}

	// Dynamic-specific cleanup
	if config.Type == "dynamic" {
		_ = sh.Run("kubectl", "delete", "storageclass", config.SCName, "--ignore-not-found=true")
	}

	fmt.Printf("✓ %s provisioning test cleanup complete\n", strings.Title(config.Type))
	return nil
}

// =============================================================================
// Generic Helper Functions
// =============================================================================

// getPodNameFromReplicaSet gets the pod name managed by the ReplicaSet
func getPodNameFromReplicaSet(config TestConfig) (string, error) {
	output, err := sh.Output("kubectl", "get", "pods",
		"-l", config.LabelSelector,
		"-n", config.Namespace,
		"-o", "jsonpath={.items[0].metadata.name}")
	if err != nil {
		return "", fmt.Errorf("failed to get pod from ReplicaSet: %v", err)
	}

	podName := strings.TrimSpace(output)
	if podName == "" {
		return "", fmt.Errorf("no pod found for ReplicaSet %s", config.RSName)
	}

	return podName, nil
}

func applyManifest(config TestConfig, resourceType string) error {
	manifestFile := fmt.Sprintf("%s-%s.yaml", config.ManifestPath, resourceType)

	resourceName := getResourceName(config, resourceType)
	fmt.Printf("Creating %s: %s\n", strings.Title(resourceType), resourceName)

	if err := sh.Run("kubectl", "apply", "-f", manifestFile); err != nil {
		return fmt.Errorf("failed to create %s: %v", resourceType, err)
	}

	fmt.Printf("✓ %s %s created\n", strings.Title(resourceType), resourceName)
	return nil
}

func getResourceName(config TestConfig, resourceType string) string {
	switch resourceType {
	case "replicaset":
		return config.RSName
	case "pvc":
		return config.PVCName
	case "pv":
		return config.PVName
	case "storageclass":
		return config.SCName
	default:
		return ""
	}
}

func waitForPVCBound(config TestConfig) error {
	fmt.Printf("Waiting for PVC to be bound (%s provisioning)...\n", config.Type)

	for i := 0; i < config.PVCTimeout; i++ {
		output, err := sh.Output("kubectl", "get", "pvc", config.PVCName, "-n", config.Namespace,
			"-o", "jsonpath={.status.phase}")
		if err != nil {
			return fmt.Errorf("failed to get PVC status: %v", err)
		}

		if output == "Bound" {
			fmt.Println("✓ PVC bound successfully")
			return nil
		}

		fmt.Printf("PVC status: %s, waiting...\n", output)
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("PVC did not bind within timeout")
}

func waitForPodReady(config TestConfig) error {
	fmt.Println("Waiting for ReplicaSet pod to be ready...")

	timeoutStr := fmt.Sprintf("%ds", config.PodTimeout)
	if err := sh.Run("kubectl", "wait", "--for=condition=Ready",
		"pod", "-l", config.LabelSelector,
		"-n", config.Namespace,
		fmt.Sprintf("--timeout=%s", timeoutStr)); err != nil {
		return fmt.Errorf("ReplicaSet pod did not become ready within timeout: %v", err)
	}

	fmt.Println("✓ ReplicaSet pod ready")
	return nil
}

func writeTestData(config TestConfig, filename, content string) error {
	fmt.Printf("Writing test data to %s...\n", filename)

	// Get the pod name from ReplicaSet
	podName, err := getPodNameFromReplicaSet(config)
	if err != nil {
		return fmt.Errorf("failed to get pod name: %v", err)
	}

	if err := sh.Run("kubectl", "exec", podName, "-n", config.Namespace, "--",
		"sh", "-c", fmt.Sprintf("echo '%s' > /data/%s", content, filename)); err != nil {
		return fmt.Errorf("failed to write test data: %v", err)
	}

	fmt.Printf("✓ Test data written to %s\n", filename)
	return nil
}

func verifyTestDataExists(config TestConfig, filename, expectedContent string) error {
	fmt.Printf("Verifying test data in %s...\n", filename)

	// Get the pod name from ReplicaSet
	podName, err := getPodNameFromReplicaSet(config)
	if err != nil {
		return fmt.Errorf("failed to get pod name: %v", err)
	}

	output, err := sh.Output("kubectl", "exec", podName, "-n", config.Namespace, "--",
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

func verifyPodRunning(config TestConfig) error {
	fmt.Println("Checking ReplicaSet pod status...")

	// Get the pod name from ReplicaSet
	podName, err := getPodNameFromReplicaSet(config)
	if err != nil {
		return fmt.Errorf("failed to get pod name: %v", err)
	}

	output, err := sh.Output("kubectl", "get", "pod", podName,
		"-n", config.Namespace,
		"-o", "jsonpath={.status.phase}")
	if err != nil {
		return fmt.Errorf("failed to get pod status: %v", err)
	}

	if strings.TrimSpace(output) != "Running" {
		return fmt.Errorf("pod not running, status: %s", output)
	}

	fmt.Println("✓ ReplicaSet pod is running")
	return nil
}

// =============================================================================
// S3 Client and Operations (Unchanged)
// =============================================================================

func getS3Client() (*s3.Client, error) {
	accessKey := os.Getenv("ACCOUNT1_ACCESS_KEY")
	secretKey := os.Getenv("ACCOUNT1_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("credentials not loaded")
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
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

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(GetS3EndpointURL())
		o.UsePathStyle = true
	})

	return client, nil
}

func createS3Bucket(bucketName string) error {
	fmt.Printf("Creating S3 bucket: %s\n", bucketName)

	client, err := getS3Client()
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	_, err = client.CreateBucket(context.Background(), &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to create bucket %s: %v", bucketName, err)
	}

	fmt.Printf("✓ S3 bucket %s created\n", bucketName)
	return nil
}

func deleteS3Bucket(bucketName string) error {
	fmt.Printf("Deleting S3 bucket: %s\n", bucketName)

	client, err := getS3Client()
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %v", err)
	}

	// First list and delete all objects in the bucket
	listResp, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		fmt.Printf("✗ Warning: Failed to list objects in bucket %s: %v\n", bucketName, err)
	} else if listResp.Contents != nil && len(listResp.Contents) > 0 {
		for _, obj := range listResp.Contents {
			_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
				Bucket: aws.String(bucketName),
				Key:    obj.Key,
			})
			if err != nil {
				fmt.Printf("✗ Warning: Failed to delete object %s: %v\n", *obj.Key, err)
			}
		}
	}

	// Then delete the bucket
	_, err = client.DeleteBucket(context.Background(), &s3.DeleteBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete bucket %s: %v", bucketName, err)
	}

	fmt.Printf("✓ S3 bucket %s deleted\n", bucketName)
	return nil
}
