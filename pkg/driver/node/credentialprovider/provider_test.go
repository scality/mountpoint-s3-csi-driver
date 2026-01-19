package credentialprovider_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider/awsprofile/awsprofiletest"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

const (
	testAccessKeyID     = "test-access-key-id"
	testSecretAccessKey = "test-secret-access-key"
	testSessionToken    = "test-session-token"
)

const (
	testPodID         = "2a17db00-0bf3-4052-9b3f-6c89dcee5d79"
	testVolumeID      = "test-vol"
	testProfilePrefix = testPodID + "-" + testVolumeID + "-"
)

const testEnvPath = "/test-env"

func TestProvidingDriverLevelCredentials(t *testing.T) {
	provider := credentialprovider.New(nil)

	authenticationSourceVariants := []string{
		credentialprovider.AuthenticationSourceDriver,
		// It should fallback to Driver-level if authentication source is unspecified.
		credentialprovider.AuthenticationSourceUnspecified,
	}

	t.Run("only long-term credentials", func(t *testing.T) {
		for _, authSource := range authenticationSourceVariants {
			setEnvForLongTermCredentials(t)

			writePath := t.TempDir()
			provideCtx := credentialprovider.ProvideContext{
				AuthenticationSource: authSource,
				WritePath:            writePath,
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
			}

			env, source, err := provider.Provide(context.Background(), provideCtx)
			assert.NoError(t, err)
			assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)
			assert.Equals(t, envprovider.Environment{
				"AWS_PROFILE":                 testProfilePrefix + "s3-csi",
				"AWS_CONFIG_FILE":             "/test-env/" + testProfilePrefix + "s3-csi-config",
				"AWS_SHARED_CREDENTIALS_FILE": "/test-env/" + testProfilePrefix + "s3-csi-credentials",
			}, env)
			assertLongTermCredentials(t, writePath)
		}
	})

	t.Run("missing credentials", func(t *testing.T) {
		// Clear environment variables to test credential validation
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "")
		t.Setenv("AWS_SESSION_TOKEN", "")

		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
		}

		_, _, err := provider.Provide(context.Background(), provideCtx)
		assert.Equals(t, "credentialprovider: static IAM credentials not provided via environment variables", err.Error())
	})

	t.Run("missing access key", func(t *testing.T) {
		// Only set secret access key without access key
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SECRET_ACCESS_KEY", testSecretAccessKey)

		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
		}

		_, _, err := provider.Provide(context.Background(), provideCtx)
		assert.Equals(t, "credentialprovider: static IAM credentials not provided via environment variables", err.Error())
	})

	t.Run("missing secret key", func(t *testing.T) {
		// Only set access key without secret
		t.Setenv("AWS_ACCESS_KEY_ID", testAccessKeyID)
		t.Setenv("AWS_SECRET_ACCESS_KEY", "")

		provider := credentialprovider.New(nil)
		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
		}

		_, _, err := provider.Provide(context.Background(), provideCtx)
		assert.Equals(t, "credentialprovider: static IAM credentials not provided via environment variables", err.Error())
	})
}

func TestCleanup(t *testing.T) {
	t.Run("cleanup driver level", func(t *testing.T) {
		// Provide/create long-term AWS credentials first
		setEnvForLongTermCredentials(t)
		provider := credentialprovider.New(nil)

		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
		}

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)
		assert.Equals(t, testProfilePrefix+"s3-csi", env["AWS_PROFILE"])
		assert.Equals(t, "/test-env/"+testProfilePrefix+"s3-csi-config", env["AWS_CONFIG_FILE"])
		assert.Equals(t, "/test-env/"+testProfilePrefix+"s3-csi-credentials", env["AWS_SHARED_CREDENTIALS_FILE"])
		assertLongTermCredentials(t, writePath)

		// Perform cleanup
		err = provider.Cleanup(credentialprovider.CleanupContext{
			WritePath: writePath,
			PodID:     testPodID,
			VolumeID:  testVolumeID,
		})
		assert.NoError(t, err)

		// Verify files were removed
		_, err = os.Stat(filepath.Join(writePath, testProfilePrefix+"s3-csi-config"))
		if err == nil {
			t.Fatalf("AWS Config should be cleaned up")
		}
		assert.Equals(t, fs.ErrNotExist, err)

		_, err = os.Stat(filepath.Join(writePath, testProfilePrefix+"s3-csi-credentials"))
		if err == nil {
			t.Fatalf("AWS Credentials should be cleaned up")
		}
		assert.Equals(t, fs.ErrNotExist, err)
	})
}

func TestMountKindConstants(t *testing.T) {
	t.Run("MountKind constant values", func(t *testing.T) {
		// Test that MountKind constants have expected string values
		assert.Equals(t, "pod", credentialprovider.MountKindPod)
		assert.Equals(t, "systemd", credentialprovider.MountKindSystemd)
	})

	t.Run("MountKind type assignment", func(t *testing.T) {
		// Test that MountKind can be assigned and compared
		var mountKind credentialprovider.MountKind
		mountKind = credentialprovider.MountKindPod
		assert.Equals(t, credentialprovider.MountKindPod, mountKind)

		mountKind = credentialprovider.MountKindSystemd
		assert.Equals(t, credentialprovider.MountKindSystemd, mountKind)
	})
}

func TestAuthenticationSourceConstants(t *testing.T) {
	t.Run("AuthenticationSource constant values", func(t *testing.T) {
		// Test that AuthenticationSource constants have expected string values
		assert.Equals(t, "", credentialprovider.AuthenticationSourceUnspecified)
		assert.Equals(t, "driver", credentialprovider.AuthenticationSourceDriver)
		assert.Equals(t, "secret", credentialprovider.AuthenticationSourceSecret)
	})
}

func TestCleanupContextWithMountKind(t *testing.T) {
	provider := credentialprovider.New(nil)

	testCases := []struct {
		name      string
		mountKind credentialprovider.MountKind
	}{
		{
			name:      "should cleanup credentials for pod-based mount strategy",
			mountKind: credentialprovider.MountKindPod,
		},
		{
			name:      "should cleanup credentials for systemd mount strategy",
			mountKind: credentialprovider.MountKindSystemd,
		},
		{
			name:      "should cleanup credentials when mount kind is not specified",
			mountKind: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up credentials for testing cleanup
			setEnvForLongTermCredentials(t)
			writePath := t.TempDir()

			// First provide credentials to create files
			provideCtx := credentialprovider.ProvideContext{
				AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
				WritePath:            writePath,
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
			}

			_, _, err := provider.Provide(context.Background(), provideCtx)
			assert.NoError(t, err)

			// Verify files exist before cleanup
			_, err = os.Stat(filepath.Join(writePath, testProfilePrefix+"s3-csi-config"))
			assert.NoError(t, err)

			// Perform cleanup with MountKind field
			err = provider.Cleanup(credentialprovider.CleanupContext{
				WritePath: writePath,
				PodID:     testPodID,
				VolumeID:  testVolumeID,
				MountKind: tc.mountKind,
			})
			assert.NoError(t, err)

			// Verify files were removed (cleanup should work regardless of MountKind)
			_, err = os.Stat(filepath.Join(writePath, testProfilePrefix+"s3-csi-config"))
			if err == nil {
				t.Fatalf("S3 Config should be cleaned up regardless of MountKind")
			}
			assert.Equals(t, fs.ErrNotExist, err)
		})
	}
}

func setEnvForLongTermCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", testAccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", testSecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", testSessionToken)
}

func assertLongTermCredentials(t *testing.T, basepath string) {
	config, err := awsprofiletest.ReadConfig(filepath.Join(basepath, testProfilePrefix+"s3-csi-config"))
	assert.NoError(t, err)
	assert.Equals(t, map[string]map[string]string{
		"profile " + testProfilePrefix + "s3-csi": {},
	}, config)

	credentials, err := awsprofiletest.ReadCredentials(filepath.Join(basepath, testProfilePrefix+"s3-csi-credentials"))
	assert.NoError(t, err)
	assert.Equals(t, map[string]map[string]string{
		testProfilePrefix + "s3-csi": {
			"aws_access_key_id":     testAccessKeyID,
			"aws_secret_access_key": testSecretAccessKey,
			"aws_session_token":     testSessionToken,
		},
	}, credentials)
}

func TestProvideWithSecretAuthSource(t *testing.T) {
	tests := []struct {
		name         string
		secretData   map[string]string
		expectError  bool
		expectedAuth credentialprovider.AuthenticationSource
	}{
		{
			name: "valid credentials",
			secretData: map[string]string{
				"access_key_id":     "ACCESS123",
				"secret_access_key": "SECRET456",
			},
			expectError:  false,
			expectedAuth: credentialprovider.AuthenticationSourceSecret,
		},
		{
			name: "missing access_key_id",
			secretData: map[string]string{
				"secret_access_key": "SECRET456",
			},
			expectError: true,
		},
		{
			name: "missing secret_access_key",
			secretData: map[string]string{
				"access_key_id": "ACCESS123",
			},
			expectError: true,
		},
		{
			name:        "empty secret",
			secretData:  map[string]string{},
			expectError: true,
		},
		{
			name: "invalid access_key_id format",
			secretData: map[string]string{
				"access_key_id":     "Invalid@Key", // Contains non-alphanumeric character
				"secret_access_key": "SECRET456",
			},
			expectError: true,
		},
		{
			name: "invalid secret_access_key format",
			secretData: map[string]string{
				"access_key_id":     "ACCESS123",
				"secret_access_key": "Invalid@Secret", // Contains invalid character
			},
			expectError: true,
		},
		{
			name: "unexpected keys",
			secretData: map[string]string{
				"access_key_id":     "ACCESS123",
				"secret_access_key": "SECRET456",
				"extra_key":         "ignored",
			},
			expectError:  false, // Should ignore the extra key
			expectedAuth: credentialprovider.AuthenticationSourceSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := credentialprovider.New(nil)

			provideCtx := credentialprovider.ProvideContext{
				VolumeID:             "test-volume-id",
				AuthenticationSource: credentialprovider.AuthenticationSourceSecret,
				SecretData:           tt.secretData,
			}

			env, authSource, err := provider.Provide(context.Background(), provideCtx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				assert.NoError(t, err)
				assert.Equals(t, tt.expectedAuth, authSource)
				if env == nil {
					t.Errorf("Expected environment to be not nil")
				}
			}
		})
	}
}

func TestProvideWithUnknownAuthSource(t *testing.T) {
	provider := credentialprovider.New(nil)

	writePath := t.TempDir()
	provideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: "unknown-source", // Using an unknown authentication source
		WritePath:            writePath,
		EnvPath:              testEnvPath,
		PodID:                testPodID,
		VolumeID:             testVolumeID,
	}

	env, source, err := provider.Provide(context.Background(), provideCtx)

	// Verify error was returned
	if err == nil {
		t.Errorf("Expected error for unknown authentication source, got nil")
	}

	// Verify error message contains all supported auth sources
	expectedErrMsg := "unknown `authenticationSource`: unknown-source, only `driver` (default option if not specified) and `secret` supported"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}

	// Verify returned values
	assert.Equals(t, credentialprovider.AuthenticationSourceUnspecified, source)
	assert.Equals(t, envprovider.Environment(nil), env)
}

func TestCredentialFallback(t *testing.T) {
	// This test verifies the credential fallback behavior when only provisioner-secret
	// is configured in StorageClass but no node-publish-secret is provided.
	// Due to CSI spec limitations, the node service cannot access provisioner secrets,
	// so it must fall back to driver credentials.

	// Set up environment variables for driver-level credentials
	setEnvForLongTermCredentials(t)

	provider := credentialprovider.New(nil)

	writePath := t.TempDir()
	provideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourceSecret, // Request secret authentication
		SecretData:           nil,                                           // No node-publish secret data available
		WritePath:            writePath,
		EnvPath:              testEnvPath,
		PodID:                testPodID,
		VolumeID:             testVolumeID,
	}

	env, source, err := provider.Provide(context.Background(), provideCtx)

	// Should succeed with driver authentication as fallback
	// (Cannot use provisioner secret - CSI spec limitation)
	assert.NoError(t, err)
	assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)

	// Verify environment contains driver credentials (same as driver-level credentials test)
	assert.Equals(t, envprovider.Environment{
		"AWS_PROFILE":                 testProfilePrefix + "s3-csi",
		"AWS_CONFIG_FILE":             "/test-env/" + testProfilePrefix + "s3-csi-config",
		"AWS_SHARED_CREDENTIALS_FILE": "/test-env/" + testProfilePrefix + "s3-csi-credentials",
	}, env)

	// Verify credential files were created
	assertLongTermCredentials(t, writePath)
}

func TestCredentialFallbackEmptySecretData(t *testing.T) {
	// This test verifies fallback behavior when secret authentication is requested
	// but the SecretData map is empty (no node-publish secrets provided).
	// This simulates the case where only provisioner-secret is in the StorageClass.

	// Set up environment variables for driver-level credentials
	setEnvForLongTermCredentials(t)

	provider := credentialprovider.New(nil)

	writePath := t.TempDir()
	provideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourceSecret, // Request secret authentication
		SecretData:           map[string]string{},                           // Empty map - no node-publish secrets
		WritePath:            writePath,
		EnvPath:              testEnvPath,
		PodID:                testPodID,
		VolumeID:             testVolumeID,
	}

	env, source, err := provider.Provide(context.Background(), provideCtx)

	// Should succeed with driver authentication as fallback
	assert.NoError(t, err)
	assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)

	// Verify environment contains driver credentials (same as driver-level credentials test)
	assert.Equals(t, envprovider.Environment{
		"AWS_PROFILE":                 testProfilePrefix + "s3-csi",
		"AWS_CONFIG_FILE":             "/test-env/" + testProfilePrefix + "s3-csi-config",
		"AWS_SHARED_CREDENTIALS_FILE": "/test-env/" + testProfilePrefix + "s3-csi-credentials",
	}, env)

	// Verify credential files were created
	assertLongTermCredentials(t, writePath)
}

func TestSecretCredentialValidation(t *testing.T) {
	// This test validates the credential validation logic in provider_secret.go
	// to ensure it properly handles various key lengths and formats.

	tests := []struct {
		name        string
		accessKeyID string
		secretKey   string
		expectError bool
		description string
	}{
		// Valid short test credentials
		{
			name:        "valid short test credentials",
			accessKeyID: "TESTKEY123",
			secretKey:   "TESTSECRET456",
			expectError: false,
			description: "Short test credentials should be accepted",
		},

		// AWS IAM standard credentials (20 characters for access key)
		{
			name:        "valid AWS IAM 20-character access key",
			accessKeyID: "AKIAIOSFODNN7EXAMPLE", // Standard AWS format: 20 chars starting with AKIA
			secretKey:   "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			expectError: false,
			description: "AWS IAM 20-character access keys should be accepted (bug S3CSI-195)",
		},
		{
			name:        "valid AWS IAM 20-character access key (alternate format)",
			accessKeyID: "14RAIKRQ3288T9H0YO6X", // 20 chars (from bug report)
			secretKey:   "mr8LiennC2aGvMxCB/TxaXT0I1Vj9NxxSo97FY6p",
			expectError: false,
			description: "Real AWS IAM key from bug report should be accepted",
		},

		// Lower-case keys (for test credentials)
		{
			name:        "valid lower-case access key",
			accessKeyID: "accessKey2",
			secretKey:   "secretKey2",
			expectError: false,
			description: "Lower-case keys should be accepted for test credentials",
		},
		{
			name:        "valid mixed-case access key",
			accessKeyID: "AcCeSsKeY123",
			secretKey:   "SeCrEtKeY456",
			expectError: false,
			description: "Mixed-case keys should be accepted",
		},

		// Secret key with base64 characters
		{
			name:        "valid secret key with base64 characters",
			accessKeyID: "AKIAIOSFODNN7EXAMPLE",
			secretKey:   "wJalrXUtnFEMI/K7MDENG+bPxRfiCY/EXAMPLEKEY==",
			expectError: false,
			description: "Secret keys with base64 characters (/, +, =) should be accepted",
		},

		// Secret key with UUID format (e.g., Scaleway)
		{
			name:        "valid secret key with UUID format",
			accessKeyID: "SCWXXXXXXXXXXXXXXXXXX",
			secretKey:   "1a111a11-1a1a-1a11-11a1-a111a1111a1a",
			expectError: false,
			description: "Secret keys with UUID format (hyphens) should be accepted",
		},

		// Maximum length (128 characters)
		{
			name:        "valid 128-character access key (maximum allowed)",
			accessKeyID: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRST", // 128 chars
			secretKey:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRST",
			expectError: false,
			description: "128-character keys should be accepted (maximum limit)",
		},

		// Invalid: exceeds maximum length
		{
			name:        "invalid access key exceeds 128 characters",
			accessKeyID: "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTU", // 129 chars
			secretKey:   "TESTSECRET456",
			expectError: true,
			description: "Access keys exceeding 128 characters should be rejected",
		},
		{
			name:        "invalid secret key exceeds 128 characters",
			accessKeyID: "AKIAIOSFODNN7EXAMPLE",
			secretKey:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTU", // 129 chars
			expectError: true,
			description: "Secret keys exceeding 128 characters should be rejected",
		},

		// Invalid: special characters (not alphanumeric or base64)
		{
			name:        "invalid access key with special characters",
			accessKeyID: "INVALID@KEY!",
			secretKey:   "TESTSECRET456",
			expectError: true,
			description: "Access keys with special characters should be rejected",
		},
		{
			name:        "invalid access key with spaces",
			accessKeyID: "INVALID KEY",
			secretKey:   "TESTSECRET456",
			expectError: true,
			description: "Access keys with spaces should be rejected",
		},
		{
			name:        "invalid secret key with invalid special characters",
			accessKeyID: "AKIAIOSFODNN7EXAMPLE",
			secretKey:   "INVALID@SECRET!",
			expectError: true,
			description: "Secret keys with invalid special characters should be rejected",
		},

		// Edge cases: empty strings
		{
			name:        "invalid empty access key",
			accessKeyID: "",
			secretKey:   "TESTSECRET456",
			expectError: true,
			description: "Empty access key should be rejected",
		},
		{
			name:        "invalid empty secret key",
			accessKeyID: "AKIAIOSFODNN7EXAMPLE",
			secretKey:   "",
			expectError: true,
			description: "Empty secret key should be rejected",
		},

		// Edge cases: whitespace handling
		{
			name:        "valid access key with leading/trailing spaces (trimmed)",
			accessKeyID: "  AKIAIOSFODNN7EXAMPLE  ",
			secretKey:   "  wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY  ",
			expectError: false,
			description: "Keys with leading/trailing spaces should be trimmed and accepted",
		},
		{
			name:        "valid 128-character access key with leading/trailing spaces (trimmed to 128)",
			accessKeyID: "  ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRST  ", // 128 chars after trim
			secretKey:   "  ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRST  ",
			expectError: false,
			description: "Maximum length keys with spaces should be trimmed first, then validated (length checked after trim)",
		},

		// Single character keys (minimum valid length)
		{
			name:        "valid single character keys",
			accessKeyID: "A",
			secretKey:   "S",
			expectError: false,
			description: "Single character keys should be accepted (minimum length)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := credentialprovider.New(nil)

			provideCtx := credentialprovider.ProvideContext{
				VolumeID:             "test-volume-id",
				AuthenticationSource: credentialprovider.AuthenticationSourceSecret,
				SecretData: map[string]string{
					"access_key_id":     tt.accessKeyID,
					"secret_access_key": tt.secretKey,
				},
			}

			env, authSource, err := provider.Provide(context.Background(), provideCtx)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: Expected error but got nil", tt.description)
					return
				}
				// Verify error returns nil environment and still returns AuthenticationSourceSecret
				// (Provider.Provide returns AuthenticationSourceSecret even on validation failure)
				if env != nil {
					t.Errorf("%s: Expected nil environment on error, got %v", tt.description, env)
				}
				if authSource != credentialprovider.AuthenticationSourceSecret {
					t.Errorf("%s: Expected secret auth source on validation error, got %s", tt.description, authSource)
				}
			} else {
				if err != nil {
					t.Errorf("%s: Expected success but got error: %v", tt.description, err)
					return
				}
				assert.NoError(t, err)
				assert.Equals(t, credentialprovider.AuthenticationSourceSecret, authSource)
				if env == nil {
					t.Errorf("%s: Expected environment to be not nil", tt.description)
				}
				// Verify environment contains the credentials
				if env[envprovider.EnvAccessKeyID] != strings.TrimSpace(tt.accessKeyID) {
					t.Errorf("%s: Expected access_key_id %q in environment, got %q",
						tt.description, strings.TrimSpace(tt.accessKeyID), env[envprovider.EnvAccessKeyID])
				}
				if env[envprovider.EnvSecretAccessKey] != strings.TrimSpace(tt.secretKey) {
					t.Errorf("%s: Expected secret_access_key in environment",
						tt.description)
				}
			}
		})
	}
}
