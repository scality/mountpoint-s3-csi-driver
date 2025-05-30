// Package awsprofiletest provides testing utilities for AWS Profiles.
//
// Note on implementation choice:
// Although the AWS SDK provides an ini parser (github.com/aws/aws-sdk-go-v2/internal/ini),
// it's an internal package that cannot be imported directly. While we could use the
// config.LoadSharedConfigProfile function for all our needs, we maintain a simple
// custom ini parser for test verification purposes where we need direct access to
// the raw config/credentials file content.
//
// If AWS makes their ini parser public in the future, we should consider replacing
// this implementation with the official one to ensure complete compatibility.
package awsprofiletest

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

func AssertCredentialsFromAWSProfile(t *testing.T, profileName string, filePerm os.FileMode, configFile, credentialsFile, accessKeyID, secretAccessKey, sessionToken string) {
	t.Helper()

	assertCredentialFilePermissions(t, configFile, filePerm)
	assertCredentialFilePermissions(t, credentialsFile, filePerm)

	credentials := parseAWSProfile(t, profileName, configFile, credentialsFile)
	assert.Equals(t, accessKeyID, credentials.AccessKeyID)
	assert.Equals(t, secretAccessKey, credentials.SecretAccessKey)
	assert.Equals(t, sessionToken, credentials.SessionToken)
}

func assertCredentialFilePermissions(t *testing.T, file string, filePerm os.FileMode) {
	fileInfo, err := os.Stat(file)
	assert.NoError(t, err)
	assert.Equals(t, filePerm, fileInfo.Mode().Perm())
}

func parseAWSProfile(t *testing.T, profileName, configFile, credentialsFile string) aws.Credentials {
	sharedConfig, err := config.LoadSharedConfigProfile(context.Background(), profileName, func(c *config.LoadSharedConfigOptions) {
		c.ConfigFiles = []string{configFile}
		c.CredentialsFiles = []string{credentialsFile}
	})
	assert.NoError(t, err)
	return sharedConfig.Credentials
}

// ReadConfig reads an AWS config file and returns a map of profile and key-value pairs.
// Note: This function is needed for test verification purposes only.
func ReadConfig(path string) (map[string]map[string]string, error) {
	return readIniFile(path)
}

// ReadCredentials reads an AWS credentials file and returns a map of profile and key-value pairs.
// Note: This function is needed for test verification purposes only.
func ReadCredentials(path string) (map[string]map[string]string, error) {
	return readIniFile(path)
}

// readIniFile reads AWS ini format files (both config and credentials) and parses them into a map.
// This implementation uses a simple file parser since we only need basic functionality for tests.
// Production AWS credential loading should use the AWS SDK Config package.
func readIniFile(path string) (map[string]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		// In this case we'll ignore the close error since it's a read operation
		// and the data has already been processed
		_ = file.Close()
	}()

	profiles := make(map[string]map[string]string)
	scanner := bufio.NewScanner(file)

	var currentProfile string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for profile header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profileName := line[1 : len(line)-1]
			currentProfile = profileName
			profiles[currentProfile] = make(map[string]string)
			continue
		}

		// Parse key=value pairs
		if currentProfile != "" && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				profiles[currentProfile][key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return profiles, nil
}
