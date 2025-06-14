# Vault Client Package

This package provides a VaultClient wrapper for E2E testing that integrates with the [Scality VaultClient Go library](https://github.com/scality/vaultclient-go) to dynamically create and manage test accounts.

## Overview

The VaultClient package enables E2E tests to:

- Dynamically create test accounts with unique credentials
- Generate access keys for each account
- Track and cleanup all created accounts automatically
- Support both Vault-based dynamic credentials and traditional static credentials

## Usage

### Basic Setup

```go
// Create a VaultClient for testing
vaultClient, err := vault.NewVaultTestClient(
    "https://vault.example.com",  // Vault endpoint
    "admin-access-key",           // Admin access key
    "admin-secret-key",           // Admin secret key
)
if err != nil {
    log.Fatalf("Failed to create VaultClient: %v", err)
}

// Create a test account
testAccount, err := vaultClient.CreateTestAccount("MyTestAccount")
if err != nil {
    log.Fatalf("Failed to create test account: %v", err)
}

// Use the generated credentials
s3Client := s3client.New("", testAccount.AccessKey, testAccount.SecretKey)

// Cleanup all accounts when done
defer vaultClient.CleanupAllAccounts()
```

### Integration with E2E Tests

The package is integrated into the E2E test framework through command-line flags:

```bash
# Run tests with Vault-based dynamic credentials
go test ./... \
  --vault-endpoint=https://vault.example.com \
  --vault-admin-access-key=admin-key \
  --vault-admin-secret-key=admin-secret \
  --s3-endpoint-url=https://s3.example.com

# Run tests with traditional static credentials (backward compatibility)
go test ./... \
  --access-key-id=static-key \
  --secret-access-key=static-secret \
  --s3-endpoint-url=https://s3.example.com
```

### Test Account Structure

Each `TestAccount` contains:

```go
type TestAccount struct {
    Name        string  // Unique account name with timestamp
    Email       string  // Generated email address
    AccessKey   string  // Generated S3 access key
    SecretKey   string  // Generated S3 secret key
    CanonicalID string  // Account's canonical ID
    ARN         string  // Account's ARN
}
```

## Features

### Automatic Account Tracking

The VaultClient automatically tracks all created accounts for cleanup:

```go
// All accounts are tracked internally
account1, _ := vaultClient.CreateTestAccount("Test1")
account2, _ := vaultClient.CreateTestAccount("Test2")

// Get count of tracked accounts
count := vaultClient.GetCreatedAccountsCount() // Returns 2

// Cleanup all at once
vaultClient.CleanupAllAccounts()
```

### Unique Account Names

Account names are automatically made unique by appending a timestamp:

```go
// Creates account named "MyTest-1640995200"
account := vaultClient.CreateTestAccount("MyTest")
```

### Error Handling

The package provides detailed error messages for common issues:

- Vault endpoint connectivity problems
- Authentication failures
- Account creation/deletion errors
- Access key generation failures

## Credentials Test Suite Integration

The `credentials` test suite automatically uses VaultClient when available:

```go
// In credentials.go, the test suite detects VaultClient availability
func getCredentialsTestAccounts(ctx context.Context, vaultClient *vault.VaultTestClient) {
    if vaultClient != nil {
        // Create dynamic accounts for Lisa and Bart
        lisaAccount, _ := vaultClient.CreateTestAccount("CredentialsTestLisa")
        bartAccount, _ := vaultClient.CreateTestAccount("CredentialsTestBart")
        return lisaAccount.AccessKey, lisaAccount.SecretKey, // ...
    } else {
        // Fallback to hardcoded credentials
        return "accessKey2", "verySecretKey2", // ...
    }
}
```

## Configuration

### Required Environment/Flags

When using Vault-based credentials:

- `--vault-endpoint`: Vault server endpoint URL
- `--vault-admin-access-key`: Admin access key for account management
- `--vault-admin-secret-key`: Admin secret key for account management
- `--s3-endpoint-url`: S3 endpoint URL (still required)

### Backward Compatibility

The package maintains full backward compatibility with static credentials:

- If Vault flags are not provided, tests use traditional static credentials
- Existing test scripts and CI/CD pipelines continue to work unchanged

## Implementation Details

### AWS SDK Integration

The package uses AWS SDK v1 (required by vaultclient-go) for Vault operations while the rest of the project uses AWS SDK v2. This is handled transparently:

```go
// Creates AWS v1 session for VaultClient
sess, err := session.NewSession(&aws.Config{
    Endpoint:    aws.String(endpoint),
    Credentials: credentials.NewStaticCredentials(adminAK, adminSK, ""),
    // ...
})

// VaultClient uses the session
client := vaultclient.New(sess)
```

### Cleanup Strategy

Cleanup happens at multiple levels:

1. **Global cleanup**: All accounts created by the VaultClient are cleaned up when tests complete
2. **Test-specific cleanup**: Individual tests can cleanup their specific accounts
3. **Graceful failure**: Cleanup errors are logged but don't fail the tests

## Troubleshooting

### Common Issues

1. **Vault endpoint not reachable**

   ```bash
   Error: Failed to initialize Vault client: failed to create AWS session
   ```

   - Check that the Vault endpoint URL is correct and accessible
   - Verify network connectivity to the Vault server

2. **Authentication failures**

   ```bash
   Error: Failed to create default test account: failed to create account
   ```

   - Verify admin access key and secret key are correct
   - Ensure the admin account has permissions to create accounts

3. **Account creation limits**

   ```bash
   Error: Failed to create account: account limit exceeded
   ```

   - Check if the Vault server has account creation limits
   - Clean up old test accounts manually if needed

### Debug Logging

Enable verbose logging to troubleshoot issues:

```bash
go test ./... -v \
  --vault-endpoint=https://vault.example.com \
  --vault-admin-access-key=admin-key \
  --vault-admin-secret-key=admin-secret \
  --s3-endpoint-url=https://s3.example.com
```

## Dependencies

- [github.com/scality/vaultclient-go](https://github.com/scality/vaultclient-go) v0.0.2
- [github.com/aws/aws-sdk-go](https://github.com/aws/aws-sdk-go) (required by vaultclient-go)
- Kubernetes E2E test framework
