package credentialprovider

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util"
)

const (
	driverLevelServiceAccountTokenName = "token"
)

// provideFromDriver provides driver-level AWS credentials.
func (c *Provider) provideFromDriver(provideCtx ProvideContext) (envprovider.Environment, error) {
	klog.V(4).Infof("credentialprovider: Using driver identity")

	env := envprovider.Environment{}

	// Long-term AWS credentials
	accessKeyID := os.Getenv(envprovider.EnvAccessKeyID)
	secretAccessKey := os.Getenv(envprovider.EnvSecretAccessKey)
	if accessKeyID != "" && secretAccessKey != "" {
		sessionToken := os.Getenv(envprovider.EnvSessionToken)
		longTermCredsEnv, err := provideLongTermCredentialsFromDriver(provideCtx, accessKeyID, secretAccessKey, sessionToken)
		if err != nil {
			klog.V(4).ErrorS(err, "credentialprovider: Failed to provide long-term AWS credentials")
			return nil, err
		}

		env.Merge(longTermCredsEnv)
	}

	// STS Web Identity provider
	webIdentityTokenFile := os.Getenv(envprovider.EnvWebIdentityTokenFile)
	roleARN := os.Getenv(envprovider.EnvRoleARN)
	if webIdentityTokenFile != "" && roleARN != "" {
		stsWebIdentityCredsEnv, err := provideStsWebIdentityCredentialsFromDriver(provideCtx)
		if err != nil {
			klog.V(4).ErrorS(err, "credentialprovider: Failed to provide STS Web Identity credentials from driver")
			return nil, err
		}

		env.Merge(stsWebIdentityCredsEnv)
	}

	return env, nil
}

// cleanupFromDriver removes any credential files that were created for driver-level authentication via [Provider.provideFromDriver].
func (c *Provider) cleanupFromDriver(cleanupCtx CleanupContext) error {
	return nil
}

// provideStsWebIdentityCredentialsFromDriver provides credentials for STS Web Identity from the driver's service account.
// It basically copies driver's injected service account token to [provideCtx.WritePath].
func provideStsWebIdentityCredentialsFromDriver(provideCtx ProvideContext) (envprovider.Environment, error) {
	driverServiceAccountTokenFile := os.Getenv(envprovider.EnvWebIdentityTokenFile)
	tokenFile := filepath.Join(provideCtx.WritePath, driverLevelServiceAccountTokenName)
	err := util.ReplaceFile(tokenFile, driverServiceAccountTokenFile, CredentialFilePerm)
	if err != nil {
		return nil, fmt.Errorf("credentialprovider: sts-web-identity: failed to copy driver's service account token: %w", err)
	}

	return envprovider.Environment{
		envprovider.EnvRoleARN:              os.Getenv(envprovider.EnvRoleARN),
		envprovider.EnvWebIdentityTokenFile: filepath.Join(provideCtx.EnvPath, driverLevelServiceAccountTokenName),
	}, nil
}

// provideLongTermCredentialsFromDriver provides long-term AWS credentials from the driver's environment variables.
// It directly sets the access key, secret key, and session token in the returned environment.
func provideLongTermCredentialsFromDriver(provideCtx ProvideContext, accessKeyID, secretAccessKey, sessionToken string) (envprovider.Environment, error) {
	// Create an environment with the provided credentials
	env := envprovider.Environment{
		envprovider.EnvAccessKeyID:     accessKeyID,
		envprovider.EnvSecretAccessKey: secretAccessKey,
	}

	// Add session token if provided
	if sessionToken != "" {
		env.Set(envprovider.EnvSessionToken, sessionToken)
	}

	return env, nil
}
