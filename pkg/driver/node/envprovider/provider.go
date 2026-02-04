// Package envprovider provides utilities for accessing environment variables to pass Mountpoint.
package envprovider

import (
	"fmt"
	"maps"
	"os"
	"slices"
)

const (
	EnvRegion                = "AWS_REGION"
	EnvMaxAttempts           = "AWS_MAX_ATTEMPTS"
	EnvEndpointURL           = "AWS_ENDPOINT_URL"
	EnvProfile               = "AWS_PROFILE"
	EnvConfigFile            = "AWS_CONFIG_FILE"
	EnvSharedCredentialsFile = "AWS_SHARED_CREDENTIALS_FILE"
	EnvAccessKeyID           = "AWS_ACCESS_KEY_ID"
	EnvSecretAccessKey       = "AWS_SECRET_ACCESS_KEY"
	EnvSessionToken          = "AWS_SESSION_TOKEN"
	EnvMountpointCacheKey    = "UNSTABLE_MOUNTPOINT_CACHE_KEY"

	// TLS configuration environment variables for custom CA certificates
	EnvTLSCACertSecret           = "TLS_CA_CERT_SECRET"
	EnvTLSInitImage              = "TLS_INIT_IMAGE"
	EnvTLSInitImagePullPolicy    = "TLS_INIT_IMAGE_PULL_POLICY"
	EnvTLSInitResourcesReqCPU    = "TLS_INIT_RESOURCES_REQUESTS_CPU"
	EnvTLSInitResourcesReqMemory = "TLS_INIT_RESOURCES_REQUESTS_MEMORY"
	EnvTLSInitResourcesLimMemory = "TLS_INIT_RESOURCES_LIMITS_MEMORY"
)

// Key represents an environment variable name.
type Key = string

// Value represents an environment variable value.
type Value = string

// Environment represents a list of environment variables as key-value pairs.
type Environment map[Key]Value

// envAllowlist is the list of environment variables to pass-by by default.
// If any of these set, it will be returned as-is in [Default].
var envAllowlist = []Key{
	EnvRegion,
	EnvEndpointURL,
}

// Default returns list of environment variables to pass Mountpoint.
func Default() Environment {
	environment := make(Environment)
	for _, key := range envAllowlist {
		val := os.Getenv(key)
		if val != "" {
			environment[key] = val
		}
	}
	return environment
}

// List returns a sorted slice of environment variables in "KEY=VALUE" format.
func (env Environment) List() []string {
	list := []string{}
	for key, val := range env {
		list = append(list, format(key, val))
	}
	slices.Sort(list)
	return list
}

// Delete deletes the environment variable with the specified key.
func (env Environment) Delete(key Key) {
	delete(env, key)
}

// Set adds or updates the environment variable with the specified key and value.
func (env Environment) Set(key Key, value Value) {
	env[key] = value
}

// Merge adds all key-value pairs from the given environment to the current environment.
// If a key exists in both environments, the value from the given environment takes precedence.
func (env Environment) Merge(other Environment) {
	maps.Copy(env, other)
}

// format formats given key and value to be used as an environment variable.
func format(key Key, value Value) string {
	return fmt.Sprintf("%s=%s", key, value)
}

// TLSConfig holds TLS configuration for custom CA certificates.
type TLSConfig struct {
	// CACertSecretName is the name of the Kubernetes Secret containing custom CA certificate(s)
	CACertSecretName string
	// InitImage is the container image for the CA certificate installation initContainer
	InitImage string
	// InitImagePullPolicy is the pull policy for the init container image
	InitImagePullPolicy string
	// InitResourcesReqCPU is the CPU request for the init container
	InitResourcesReqCPU string
	// InitResourcesReqMemory is the memory request for the init container
	InitResourcesReqMemory string
	// InitResourcesLimMemory is the memory limit for the init container
	InitResourcesLimMemory string
}

// GetTLSConfig returns TLS configuration from environment variables.
// Returns nil if no TLS configuration is set (i.e., TLS_CA_CERT_SECRET is empty).
func GetTLSConfig() *TLSConfig {
	caCertSecret := os.Getenv(EnvTLSCACertSecret)
	if caCertSecret == "" {
		return nil
	}

	return &TLSConfig{
		CACertSecretName:       caCertSecret,
		InitImage:              os.Getenv(EnvTLSInitImage),
		InitImagePullPolicy:    os.Getenv(EnvTLSInitImagePullPolicy),
		InitResourcesReqCPU:    os.Getenv(EnvTLSInitResourcesReqCPU),
		InitResourcesReqMemory: os.Getenv(EnvTLSInitResourcesReqMemory),
		InitResourcesLimMemory: os.Getenv(EnvTLSInitResourcesLimMemory),
	}
}
