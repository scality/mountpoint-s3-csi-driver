package credentialprovider

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider/awsprofile"
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
	} else {
		// Profile provider
		// TODO: This is not officially supported and won't work by default with containerization.
		configFile := os.Getenv(envprovider.EnvConfigFile)
		sharedCredentialsFile := os.Getenv(envprovider.EnvSharedCredentialsFile)
		if configFile != "" && sharedCredentialsFile != "" {
			env.Set(envprovider.EnvConfigFile, configFile)
			env.Set(envprovider.EnvSharedCredentialsFile, sharedCredentialsFile)
		}
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
	prefix := driverLevelLongTermCredentialsProfilePrefix(cleanupCtx.PodID, cleanupCtx.VolumeID)
	return awsprofile.Cleanup(awsprofile.Settings{
		Basepath: cleanupCtx.WritePath,
		Prefix:   prefix,
	})
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
// These variables injected to driver's Pod from a configured Kubernetes secret if configured, here it basically
// created a AWS Profile from these credentials in [provideCtx.WritePath].
func provideLongTermCredentialsFromDriver(provideCtx ProvideContext, accessKeyID, secretAccessKey, sessionToken string) (envprovider.Environment, error) {
	prefix := driverLevelLongTermCredentialsProfilePrefix(provideCtx.PodID, provideCtx.VolumeID)
	awsProfile, err := awsprofile.Create(awsprofile.Settings{
		Basepath: provideCtx.WritePath,
		Prefix:   prefix,
		FilePerm: CredentialFilePerm,
	}, awsprofile.Credentials{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		SessionToken:    sessionToken,
	})
	if err != nil {
		return nil, fmt.Errorf("credentialprovider: long-term: failed to create aws profile: %w", err)
	}

	profile := awsProfile.Name
	configFile := filepath.Join(provideCtx.EnvPath, awsProfile.ConfigFilename)
	credentialsFile := filepath.Join(provideCtx.EnvPath, awsProfile.CredentialsFilename)

	return envprovider.Environment{
		envprovider.EnvProfile:               profile,
		envprovider.EnvConfigFile:            configFile,
		envprovider.EnvSharedCredentialsFile: credentialsFile,
	}, nil
}

// driverLevelLongTermCredentialsProfilePrefix generates a prefix for AWS credential profile names
// when using driver-level authentication. The prefix includes both pod and volume IDs to ensure uniqueness.
func driverLevelLongTermCredentialsProfilePrefix(podID, volumeID string) string {
	return escapedVolumeIdentifier(podID, volumeID) + "-"
}
