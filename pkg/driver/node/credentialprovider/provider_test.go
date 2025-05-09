package credentialprovider_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/credentialprovider/awsprofile/awsprofiletest"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/util/testutil/assert"
)

const testAccessKeyID = "test-access-key-id"
const testSecretAccessKey = "test-secret-access-key"
const testSessionToken = "test-session-token"

const testRoleARN = "arn:aws:iam::111122223333:role/pod-a-role"
const testWebIdentityToken = "test-web-identity-token"

const testPodID = "2a17db00-0bf3-4052-9b3f-6c89dcee5d79"
const testVolumeID = "test-vol"
const testProfilePrefix = testPodID + "-" + testVolumeID + "-"

const testPodLevelServiceAccountToken = testPodID + "-" + testVolumeID + ".token"
const testDriverLevelServiceAccountToken = "token"

const testPodServiceAccount = "test-sa"
const testPodNamespace = "test-ns"

const testIMDSRegion = "us-east-1"

func dummyRegionProvider() (string, error) {
	return testIMDSRegion, nil
}

const testEnvPath = "/test-env"

func TestProvidingDriverLevelCredentials(t *testing.T) {
	provider := credentialprovider.New(nil, dummyRegionProvider)

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

	t.Run("only sts web identity credentials", func(t *testing.T) {
		for _, authSource := range authenticationSourceVariants {
			setEnvForStsWebIdentityCredentials(t)

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
				"AWS_ROLE_ARN":                testRoleARN,
				"AWS_WEB_IDENTITY_TOKEN_FILE": filepath.Join(testEnvPath, testDriverLevelServiceAccountToken),
			}, env)
			assertWebIdentityTokenFile(t, filepath.Join(writePath, testDriverLevelServiceAccountToken))
		}
	})

	t.Run("only profile provider", func(t *testing.T) {
		basepath := t.TempDir()
		t.Setenv("AWS_CONFIG_FILE", filepath.Join(basepath, "config"))
		t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(basepath, "credentials"))

		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourceDriver,
			WritePath:            t.TempDir(),
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
		}

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)
		assert.Equals(t, envprovider.Environment{
			"AWS_CONFIG_FILE":             filepath.Join(basepath, "config"),
			"AWS_SHARED_CREDENTIALS_FILE": filepath.Join(basepath, "credentials"),
		}, env)
	})

	t.Run("long-term and sts web identity credentials", func(t *testing.T) {
		for _, authSource := range authenticationSourceVariants {
			setEnvForLongTermCredentials(t)
			setEnvForStsWebIdentityCredentials(t)

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
				"AWS_ROLE_ARN":                testRoleARN,
				"AWS_WEB_IDENTITY_TOKEN_FILE": filepath.Join(testEnvPath, testDriverLevelServiceAccountToken),
			}, env)
			assertLongTermCredentials(t, writePath)
			assertWebIdentityTokenFile(t, filepath.Join(writePath, testDriverLevelServiceAccountToken))
		}
	})

	t.Run("incomplete long-term credentials", func(t *testing.T) {
		// Only set access key without secret
		t.Setenv("AWS_ACCESS_KEY_ID", testAccessKeyID)

		provider := credentialprovider.New(nil, dummyRegionProvider)
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
		assert.Equals(t, envprovider.Environment{}, env)

		// Only set secret key without access key
		t.Setenv("AWS_ACCESS_KEY_ID", "")
		t.Setenv("AWS_SECRET_ACCESS_KEY", testSecretAccessKey)

		env, source, err = provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)
		assert.Equals(t, envprovider.Environment{}, env)
	})

	t.Run("incomplete sts web identity credentials", func(t *testing.T) {
		// Only set role ARN without token file
		t.Setenv("AWS_ROLE_ARN", testRoleARN)

		provider := credentialprovider.New(nil, dummyRegionProvider)
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
		assert.Equals(t, envprovider.Environment{}, env)

		// Only set token file without role ARN
		tokenPath := filepath.Join(t.TempDir(), "token")
		assert.NoError(t, os.WriteFile(tokenPath, []byte(testWebIdentityToken), 0600))
		t.Setenv("AWS_ROLE_ARN", "")
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tokenPath)

		env, source, err = provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourceDriver, source)
		assert.Equals(t, envprovider.Environment{}, env)
	})

	t.Run("no credentials", func(t *testing.T) {
		for _, authSource := range authenticationSourceVariants {
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
			assert.Equals(t, envprovider.Environment{}, env)
		}
	})
}

func TestProvidingPodLevelCredentials(t *testing.T) {
	testutil.CleanRegionEnv(t)

	t.Run("correct values", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(serviceAccount(testPodServiceAccount, testPodNamespace, map[string]string{
			"eks.amazonaws.com/role-arn": testRoleARN,
		}))
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourcePod,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
			PodNamespace:         testPodNamespace,
			ServiceAccountName:   testPodServiceAccount,
			ServiceAccountTokens: serviceAccountTokens(t, tokens{
				"sts.amazonaws.com": {
					Token: testWebIdentityToken,
				},
			}),
		}

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourcePod, source)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE": filepath.Join(testEnvPath, testPodLevelServiceAccountToken),

			// Having a unique cache key for namespace/serviceaccount pair
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,

			// Disable long-term credentials
			"AWS_CONFIG_FILE":             "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE": "/test-env/disable-credentials",

			// Disable EC2 credentials
			"AWS_EC2_METADATA_DISABLED": "true",

			"AWS_REGION":         testIMDSRegion,
			"AWS_DEFAULT_REGION": testIMDSRegion,
		}, env)
		assertWebIdentityTokenFile(t, filepath.Join(writePath, testPodLevelServiceAccountToken))
	})

	t.Run("missing information", func(t *testing.T) {
		clientset := fake.NewSimpleClientset(
			serviceAccount(testPodServiceAccount, testPodNamespace, map[string]string{
				"eks.amazonaws.com/role-arn": testRoleARN,
			}),
			serviceAccount("test-sa-missing-role", testPodNamespace, map[string]string{}),
		)

		for name, provideCtx := range map[string]credentialprovider.ProvideContext{
			"unknown service account": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				PodNamespace:         testPodNamespace,
				ServiceAccountName:   "test-unknown-sa",
				ServiceAccountTokens: serviceAccountTokens(t, tokens{
					"sts.amazonaws.com": {
						Token: testWebIdentityToken,
					},
				}),
			},
			"missing role arn in service account": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				PodNamespace:         testPodNamespace,
				ServiceAccountName:   "test-sa-missing-role",
				ServiceAccountTokens: serviceAccountTokens(t, tokens{
					"sts.amazonaws.com": {
						Token: testWebIdentityToken,
					},
				}),
			},
			"missing service account token": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				PodNamespace:         testPodNamespace,
				ServiceAccountName:   testPodServiceAccount,
			},
			"missing sts audience in service account tokens": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				PodNamespace:         testPodNamespace,
				ServiceAccountName:   testPodServiceAccount,
				ServiceAccountTokens: serviceAccountTokens(t, tokens{
					"unknown": {
						Token: testWebIdentityToken,
					},
				}),
			},
			"missing service account name": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				PodNamespace:         testPodNamespace,
				ServiceAccountTokens: serviceAccountTokens(t, tokens{
					"sts.amazonaws.com": {
						Token: testWebIdentityToken,
					},
				}),
			},
			"missing pod namespace": {
				AuthenticationSource: credentialprovider.AuthenticationSourcePod,
				WritePath:            t.TempDir(),
				EnvPath:              testEnvPath,
				PodID:                testPodID,
				VolumeID:             testVolumeID,
				ServiceAccountName:   testPodServiceAccount,
				ServiceAccountTokens: serviceAccountTokens(t, tokens{
					"sts.amazonaws.com": {
						Token: testWebIdentityToken,
					},
				}),
			},
		} {
			t.Run(name, func(t *testing.T) {
				provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)
				_, _, err := provider.Provide(context.Background(), provideCtx)
				if err == nil {
					t.Error("it should fail with missing information")
				}
			})
		}
	})
}

func TestDetectingRegionToUseForPodLevelCredentials(t *testing.T) {
	testutil.CleanRegionEnv(t)

	clientset := fake.NewSimpleClientset(serviceAccount(testPodServiceAccount, testPodNamespace, map[string]string{
		"eks.amazonaws.com/role-arn": testRoleARN,
	}))

	baseProvideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourcePod,
		WritePath:            t.TempDir(),
		EnvPath:              testEnvPath,
		PodID:                testPodID,
		VolumeID:             testVolumeID,
		PodNamespace:         testPodNamespace,
		ServiceAccountName:   testPodServiceAccount,
		ServiceAccountTokens: serviceAccountTokens(t, tokens{
			"sts.amazonaws.com": {
				Token: testWebIdentityToken,
			},
		}),
	}

	t.Run("no region", func(t *testing.T) {
		provider := credentialprovider.New(clientset.CoreV1(), func() (string, error) {
			return "", errors.New("unknown region")
		})

		_, _, err := provider.Provide(context.Background(), baseProvideCtx)
		if err == nil {
			t.Error("it should fail if there is not any region information")
		}
	})

	t.Run("region from imds", func(t *testing.T) {
		provider := credentialprovider.New(clientset.CoreV1(), func() (string, error) {
			return "us-east-2", nil
		})

		env, _, err := provider.Provide(context.Background(), baseProvideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "us-east-2",
			"AWS_DEFAULT_REGION":            "us-east-2",
		}, env)
	})

	t.Run("region from env", func(t *testing.T) {
		t.Setenv("AWS_REGION", "eu-west-1")
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		env, _, err := provider.Provide(context.Background(), baseProvideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "eu-west-1",
			"AWS_DEFAULT_REGION":            "eu-west-1",
		}, env)
	})

	t.Run("default region from env", func(t *testing.T) {
		t.Setenv("AWS_DEFAULT_REGION", "eu-north-1")
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		env, _, err := provider.Provide(context.Background(), baseProvideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "eu-north-1",
			"AWS_DEFAULT_REGION":            "eu-north-1",
		}, env)
	})

	t.Run("default and regular region from env", func(t *testing.T) {
		t.Setenv("AWS_REGION", "eu-west-1")
		t.Setenv("AWS_DEFAULT_REGION", "eu-north-1")
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		env, _, err := provider.Provide(context.Background(), baseProvideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "eu-west-1",
			"AWS_DEFAULT_REGION":            "eu-north-1",
		}, env)
	})

	t.Run("region from options", func(t *testing.T) {
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		provideCtx := baseProvideCtx
		provideCtx.BucketRegion = "us-west-1"
		env, _, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "us-west-1",
			"AWS_DEFAULT_REGION":            "us-west-1",
		}, env)
	})

	t.Run("region from options with default region from env", func(t *testing.T) {
		t.Setenv("AWS_DEFAULT_REGION", "eu-north-1")
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		provideCtx := baseProvideCtx
		provideCtx.BucketRegion = "us-west-1"
		env, _, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "us-west-1",
			"AWS_DEFAULT_REGION":            "eu-north-1",
		}, env)
	})

	t.Run("region from volume context", func(t *testing.T) {
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		provideCtx := baseProvideCtx
		provideCtx.StsRegion = "ap-south-1"
		env, _, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "ap-south-1",
			"AWS_DEFAULT_REGION":            "ap-south-1",
		}, env)
	})

	t.Run("region from volume context with default region from env", func(t *testing.T) {
		t.Setenv("AWS_DEFAULT_REGION", "eu-north-1")
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		provideCtx := baseProvideCtx
		provideCtx.StsRegion = "ap-south-1"
		env, _, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodLevelServiceAccountToken),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    "ap-south-1",
			"AWS_DEFAULT_REGION":            "eu-north-1",
		}, env)
	})
}

func TestProvidingPodLevelCredentialsForDifferentPods(t *testing.T) {
	testutil.CleanRegionEnv(t)

	clientset := fake.NewSimpleClientset(
		serviceAccount("test-sa-1", testPodNamespace, map[string]string{
			"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/Test1",
		}),
		serviceAccount("test-sa-2", testPodNamespace, map[string]string{
			"eks.amazonaws.com/role-arn": "arn:aws:iam::123456789012:role/Test2",
		}),
	)
	provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

	baseProvideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourcePod,
		WritePath:            t.TempDir(),
		EnvPath:              testEnvPath,
		PodNamespace:         testPodNamespace,
		VolumeID:             testVolumeID,
	}

	provideCtxOne := baseProvideCtx
	provideCtxOne.PodID = "pod1"
	provideCtxOne.ServiceAccountName = "test-sa-1"
	provideCtxOne.ServiceAccountTokens = serviceAccountTokens(t, tokens{
		"sts.amazonaws.com": {Token: "token1"},
	})

	provideCtxTwo := baseProvideCtx
	provideCtxTwo.PodID = "pod2"
	provideCtxTwo.ServiceAccountName = "test-sa-2"
	provideCtxTwo.ServiceAccountTokens = serviceAccountTokens(t, tokens{
		"sts.amazonaws.com": {Token: "token2"},
	})

	envOne, sourceOne, err := provider.Provide(context.Background(), provideCtxOne)
	assert.NoError(t, err)
	assert.Equals(t, credentialprovider.AuthenticationSourcePod, sourceOne)
	assert.Equals(t, envprovider.Environment{
		"AWS_ROLE_ARN":                  "arn:aws:iam::123456789012:role/Test1",
		"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, "pod1-"+testVolumeID+".token"),
		"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/test-sa-1",
		"AWS_CONFIG_FILE":               "/test-env/disable-config",
		"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
		"AWS_EC2_METADATA_DISABLED":     "true",
		"AWS_REGION":                    testIMDSRegion,
		"AWS_DEFAULT_REGION":            testIMDSRegion,
	}, envOne)

	tokenOneContent, err := os.ReadFile(filepath.Join(provideCtxOne.WritePath, "pod1-"+testVolumeID+".token"))
	assert.NoError(t, err)
	assert.Equals(t, []byte("token1"), tokenOneContent)

	envTwo, sourceTwo, err := provider.Provide(context.Background(), provideCtxTwo)
	assert.NoError(t, err)
	assert.Equals(t, credentialprovider.AuthenticationSourcePod, sourceTwo)
	assert.Equals(t, envprovider.Environment{
		"AWS_ROLE_ARN":                  "arn:aws:iam::123456789012:role/Test2",
		"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, "pod2-"+testVolumeID+".token"),
		"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/test-sa-2",
		"AWS_CONFIG_FILE":               "/test-env/disable-config",
		"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
		"AWS_EC2_METADATA_DISABLED":     "true",
		"AWS_REGION":                    testIMDSRegion,
		"AWS_DEFAULT_REGION":            testIMDSRegion,
	}, envTwo)

	tokenContent2, err := os.ReadFile(filepath.Join(provideCtxTwo.WritePath, "pod2-"+testVolumeID+".token"))
	assert.NoError(t, err)
	assert.Equals(t, []byte("token2"), tokenContent2)
}

func TestProvidingPodLevelCredentialsWithSlashInIDs(t *testing.T) {
	testutil.CleanRegionEnv(t)

	clientset := fake.NewSimpleClientset(serviceAccount(testPodServiceAccount, testPodNamespace, map[string]string{
		"eks.amazonaws.com/role-arn": testRoleARN,
	}))
	provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

	baseProvideCtx := credentialprovider.ProvideContext{
		AuthenticationSource: credentialprovider.AuthenticationSourcePod,
		WritePath:            t.TempDir(),
		EnvPath:              testEnvPath,
		PodID:                testPodID,
		VolumeID:             testVolumeID,
		PodNamespace:         testPodNamespace,
		ServiceAccountName:   testPodServiceAccount,
		ServiceAccountTokens: serviceAccountTokens(t, tokens{
			"sts.amazonaws.com": {Token: testWebIdentityToken},
		}),
	}

	t.Run("slash in volume id", func(t *testing.T) {
		provideCtx := baseProvideCtx
		provideCtx.VolumeID = "vol/1"

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourcePod, source)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, testPodID+"-vol~1.token"),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    testIMDSRegion,
			"AWS_DEFAULT_REGION":            testIMDSRegion,
		}, env)

		tokenContent, err := os.ReadFile(filepath.Join(provideCtx.WritePath, testPodID+"-vol~1.token"))
		assert.NoError(t, err)
		assert.Equals(t, []byte(testWebIdentityToken), tokenContent)
	})

	t.Run("slash in pod id", func(t *testing.T) {
		provideCtx := baseProvideCtx
		provideCtx.PodID = "pod/123"

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourcePod, source)
		assert.Equals(t, envprovider.Environment{
			"AWS_ROLE_ARN":                  testRoleARN,
			"AWS_WEB_IDENTITY_TOKEN_FILE":   filepath.Join(testEnvPath, "pod~123-"+testVolumeID+".token"),
			"UNSTABLE_MOUNTPOINT_CACHE_KEY": testPodNamespace + "/" + testPodServiceAccount,
			"AWS_CONFIG_FILE":               "/test-env/disable-config",
			"AWS_SHARED_CREDENTIALS_FILE":   "/test-env/disable-credentials",
			"AWS_EC2_METADATA_DISABLED":     "true",
			"AWS_REGION":                    testIMDSRegion,
			"AWS_DEFAULT_REGION":            testIMDSRegion,
		}, env)

		tokenContent, err := os.ReadFile(filepath.Join(provideCtx.WritePath, "pod~123-"+testVolumeID+".token"))
		assert.NoError(t, err)
		assert.Equals(t, []byte(testWebIdentityToken), tokenContent)
	})
}

func TestCleanup(t *testing.T) {
	testutil.CleanRegionEnv(t)

	t.Run("cleanup driver level", func(t *testing.T) {
		// Provide/create long-term credentials first
		setEnvForLongTermCredentials(t)

		provider := credentialprovider.New(nil, dummyRegionProvider)
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
		assert.Equals(t, envprovider.Environment{
			"AWS_PROFILE":                 testProfilePrefix + "s3-csi",
			"AWS_CONFIG_FILE":             "/test-env/" + testProfilePrefix + "s3-csi-config",
			"AWS_SHARED_CREDENTIALS_FILE": "/test-env/" + testProfilePrefix + "s3-csi-credentials",
		}, env)
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
			t.Fatalf("AWS Config file should be cleaned up")
		}
		assert.Equals(t, fs.ErrNotExist, err)

		_, err = os.Stat(filepath.Join(writePath, testProfilePrefix+"s3-csi-credentials"))
		if err == nil {
			t.Fatalf("AWS Credentials file should be cleaned up")
		}
		assert.Equals(t, fs.ErrNotExist, err)
	})

	t.Run("cleanup pod level", func(t *testing.T) {
		// Provide/create STS Web Identity credentials first
		clientset := fake.NewSimpleClientset(serviceAccount(testPodServiceAccount, testPodNamespace, map[string]string{
			"eks.amazonaws.com/role-arn": testRoleARN,
		}))
		provider := credentialprovider.New(clientset.CoreV1(), dummyRegionProvider)

		writePath := t.TempDir()
		provideCtx := credentialprovider.ProvideContext{
			AuthenticationSource: credentialprovider.AuthenticationSourcePod,
			WritePath:            writePath,
			EnvPath:              testEnvPath,
			PodID:                testPodID,
			VolumeID:             testVolumeID,
			PodNamespace:         testPodNamespace,
			ServiceAccountName:   testPodServiceAccount,
			ServiceAccountTokens: serviceAccountTokens(t, tokens{
				"sts.amazonaws.com": {
					Token: testWebIdentityToken,
				},
			}),
		}

		env, source, err := provider.Provide(context.Background(), provideCtx)
		assert.NoError(t, err)
		assert.Equals(t, credentialprovider.AuthenticationSourcePod, source)
		assert.Equals(t, testRoleARN, env["AWS_ROLE_ARN"])
		assert.Equals(t, filepath.Join(testEnvPath, testPodLevelServiceAccountToken), env["AWS_WEB_IDENTITY_TOKEN_FILE"])
		assertWebIdentityTokenFile(t, filepath.Join(writePath, testPodLevelServiceAccountToken))

		// Perform cleanup
		err = provider.Cleanup(credentialprovider.CleanupContext{
			WritePath: writePath,
			PodID:     testPodID,
			VolumeID:  testVolumeID,
		})
		assert.NoError(t, err)

		// Verify file was removed
		_, err = os.Stat(filepath.Join(writePath, testPodLevelServiceAccountToken))
		if err == nil {
			t.Fatalf("Service Account Token should be cleaned up")
		}
		assert.Equals(t, fs.ErrNotExist, err)
	})

	t.Run("cleanup with non-existent files", func(t *testing.T) {
		writePath := t.TempDir()
		provider := credentialprovider.New(nil, dummyRegionProvider)

		// Cleanup should not fail if files don't exist
		err := provider.Cleanup(credentialprovider.CleanupContext{
			WritePath: writePath,
			PodID:     testPodID,
			VolumeID:  testVolumeID,
		})
		assert.NoError(t, err)
	})
}

//-- Utilities for tests

func setEnvForLongTermCredentials(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", testAccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", testSecretAccessKey)
	t.Setenv("AWS_SESSION_TOKEN", testSessionToken)
}

func assertLongTermCredentials(t *testing.T, basepath string) {
	t.Helper()

	awsprofiletest.AssertCredentialsFromAWSProfile(
		t,
		testProfilePrefix+"s3-csi",
		credentialprovider.CredentialFilePerm,
		filepath.Join(basepath, testProfilePrefix+"s3-csi-config"),
		filepath.Join(basepath, testProfilePrefix+"s3-csi-credentials"),
		testAccessKeyID,
		testSecretAccessKey,
		testSessionToken,
	)
}

func setEnvForStsWebIdentityCredentials(t *testing.T) {
	t.Helper()

	tokenPath := filepath.Join(t.TempDir(), "token")
	assert.NoError(t, os.WriteFile(tokenPath, []byte(testWebIdentityToken), 0600))

	t.Setenv("AWS_ROLE_ARN", testRoleARN)
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", tokenPath)
}

func assertWebIdentityTokenFile(t *testing.T, path string) {
	t.Helper()

	got, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equals(t, []byte(testWebIdentityToken), got)
}

type tokens = map[string]struct {
	Token               string `json:"token"`
	ExpirationTimestamp time.Time
}

func serviceAccountTokens(t *testing.T, tokens tokens) string {
	buf, err := json.Marshal(&tokens)
	assert.NoError(t, err)
	return string(buf)
}

func serviceAccount(name, namespace string, annotations map[string]string) *v1.ServiceAccount {
	return &v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Annotations: annotations,
	}}
}

// TestProvideWithSecretAuthSource tests authentication with Secret credentials
func TestProvideWithSecretAuthSource(t *testing.T) {
	tests := []struct {
		name         string
		secretData   map[string]string
		expectError  bool
		expectedAuth credentialprovider.AuthenticationSource
	}{
		{
			name: "valid secret credentials",
			secretData: map[string]string{
				"key_id":     "AKIA123456789ABC",
				"access_key": "SECRET123456789ABCDEFGHIJKLMNOPQRSTUV",
			},
			expectError:  false,
			expectedAuth: credentialprovider.AuthenticationSourceSecret,
		},
		{
			name: "invalid secret credentials",
			secretData: map[string]string{
				"key_id": "invalid",
			},
			expectError: true,
		},
		{
			name: "secret with unexpected keys",
			secretData: map[string]string{
				"key_id":     "AKIA123456789ABC",
				"access_key": "SECRET123456789ABCDEFGHIJKLMNOPQRSTUV",
				"unexpected": "value", // This will trigger the unexpected key warning
			},
			expectError:  false,
			expectedAuth: credentialprovider.AuthenticationSourceSecret,
		},
		{
			name: "invalid access_key format",
			secretData: map[string]string{
				"key_id": "AKIA123456789ABC",
				// This will trigger the invalid access_key warning
				"access_key": "SECRET!@#$%^&*()",
			},
			expectError: true,
		},
		{
			name: "access_key exceeds max length",
			secretData: map[string]string{
				"key_id": "AKIA123456789ABC",
				// This will trigger the max length warning for access_key
				"access_key": "SECRET123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := credentialprovider.New(nil, nil)

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
	provider := credentialprovider.New(nil, dummyRegionProvider)

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
	expectedErrMsg := "unknown `authenticationSource`: unknown-source, only `driver` (default option if not specified), `pod`, and `secret` supported"
	if err.Error() != expectedErrMsg {
		t.Errorf("Expected error message %q, got %q", expectedErrMsg, err.Error())
	}

	// Verify returned values
	assert.Equals(t, credentialprovider.AuthenticationSourceUnspecified, source)
	assert.Equals(t, envprovider.Environment(nil), env)
}
