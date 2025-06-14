package vault

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/scality/vaultclient-go/vaultclient"
	"k8s.io/kubernetes/test/e2e/framework"
)

// VaultTestClient wraps the vaultclient for E2E testing with account tracking and cleanup
type VaultTestClient struct {
	client          *vaultclient.Vault
	session         *session.Session
	createdAccounts []string // Track account names for cleanup
}

// TestAccount represents a test account with all necessary credentials and metadata
type TestAccount struct {
	Name        string
	Email       string
	AccessKey   string
	SecretKey   string
	CanonicalID string
	ARN         string
}

// NewVaultTestClient creates a new VaultTestClient with admin credentials
func NewVaultTestClient(endpoint, adminAK, adminSK string) (*VaultTestClient, error) {
	// Create AWS session with custom endpoint and credentials
	sess, err := session.NewSession(&aws.Config{
		Endpoint:                      aws.String(endpoint),
		Region:                        aws.String("us-east-1"), // Required but ignored by Vault
		Credentials:                   credentials.NewStaticCredentials(adminAK, adminSK, ""),
		CredentialsChainVerboseErrors: aws.Bool(true),
		DisableSSL:                    aws.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create Vault client using the session
	client := vaultclient.New(sess)

	return &VaultTestClient{
		client:          client,
		session:         sess,
		createdAccounts: []string{},
	}, nil
}

// CreateTestAccount creates a new account in Vault and generates access keys for it
func (v *VaultTestClient) CreateTestAccount(name string) (*TestAccount, error) {
	// Generate unique email and add timestamp, PID, and random number to ensure uniqueness across parallel processes
	timestamp := time.Now().Unix()
	pid := os.Getpid()
	random := rand.Intn(10000)
	uniqueName := fmt.Sprintf("%s-%d-%d-%d", name, timestamp, pid, random)
	email := fmt.Sprintf("%s@e2etest.local", uniqueName)

	framework.Logf("Creating Vault account: %s with email: %s", uniqueName, email)

	// 1. Create account using vaultclient.CreateAccount
	createInput := &vaultclient.CreateAccountInput{
		Name:  aws.String(uniqueName),
		Email: aws.String(email),
	}

	accountOutput, err := v.client.CreateAccount(context.Background(), createInput)
	if err != nil {
		return nil, fmt.Errorf("failed to create account %s: %w", uniqueName, err)
	}

	// Track for cleanup
	v.createdAccounts = append(v.createdAccounts, uniqueName)
	framework.Logf("Successfully created account %s, tracking for cleanup", uniqueName)

	// 2. Generate access key using vaultclient.GenerateAccountAccessKey
	generateInput := &vaultclient.GenerateAccountAccessKeyInput{
		AccountName: aws.String(uniqueName),
	}

	accessKeyOutput, err := v.client.GenerateAccountAccessKey(context.Background(), generateInput)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access key for %s: %w", uniqueName, err)
	}

	framework.Logf("Successfully generated access keys for account %s", uniqueName)

	// Extract account data from the response
	accountData := accountOutput.GetAccount()
	generatedKey := accessKeyOutput.GeneratedKey

	return &TestAccount{
		Name:        uniqueName,
		Email:       email,
		AccessKey:   aws.StringValue(generatedKey.ID),
		SecretKey:   aws.StringValue(generatedKey.Value),
		CanonicalID: aws.StringValue(accountData.CanonicalID),
		ARN:         aws.StringValue(accountData.Arn),
	}, nil
}

// DeleteTestAccount deletes a specific account from Vault
func (v *VaultTestClient) DeleteTestAccount(name string) error {
	framework.Logf("Deleting Vault account: %s", name)

	// Use vaultclient.DeleteAccount
	deleteInput := &vaultclient.DeleteAccountInput{
		AccountName: aws.String(name),
	}

	_, err := v.client.DeleteAccount(context.Background(), deleteInput)
	if err != nil {
		return fmt.Errorf("failed to delete account %s: %w", name, err)
	}

	framework.Logf("Successfully deleted account: %s", name)
	return nil
}

// CleanupAllAccounts deletes all accounts created by this client
func (v *VaultTestClient) CleanupAllAccounts() error {
	if len(v.createdAccounts) == 0 {
		framework.Logf("No Vault accounts to cleanup")
		return nil
	}

	framework.Logf("Cleaning up %d Vault accounts", len(v.createdAccounts))

	var errors []error
	for _, accountName := range v.createdAccounts {
		if err := v.DeleteTestAccount(accountName); err != nil {
			errors = append(errors, fmt.Errorf("failed to delete account %s: %w", accountName, err))
		}
	}

	// Clear the list after cleanup attempt
	v.createdAccounts = []string{}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	framework.Logf("Successfully cleaned up all Vault accounts")
	return nil
}

// GetCreatedAccountsCount returns the number of accounts currently tracked for cleanup
func (v *VaultTestClient) GetCreatedAccountsCount() int {
	return len(v.createdAccounts)
}
