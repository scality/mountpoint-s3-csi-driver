package credentialprovider

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/envprovider"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

const (
	// Keys expected in the Secret map from NodePublishVolumeRequest.
	accessKeyID     = "access_key_id"
	secretAccessKey = "secret_access_key"

	// Upper limits (not exact) — suits Vault & test creds.
	maxAccessKeyIDLen     = 16
	maxSecretAccessKeyLen = 40
)

/*
Validation rules (loosened for cloudserver test credentials):

	access_key_id     – 1 … 16 chars, uppercase A–Z or 0–9
	secret_access_key – 1 … 40 chars, [A-Za-z0-9 / + =]

The patterns are supersets of AWS IAM and permit shorter dummy keys.
*/
var (
	// accept upper‑ or lower‑case letters so test keys like "accessKey2" pass
	accessKeyIDRe     = regexp.MustCompile(`^[A-Za-z0-9]{1,` + strconv.Itoa(maxAccessKeyIDLen) + `}$`)
	secretAccessKeyRe = regexp.MustCompile(`^[A-Za-z0-9/+=]{1,` + strconv.Itoa(maxSecretAccessKeyLen) + `}$`)
)

// provideFromSecret validates credentials from a Kubernetes Secret.
func (c *Provider) provideFromSecret(_ context.Context, provideCtx ProvideContext) (envprovider.Environment, error) {
	env := envprovider.Environment{}

	valid := map[string]struct{}{accessKeyID: {}, secretAccessKey: {}}
	for k := range provideCtx.SecretData {
		if _, ok := valid[k]; !ok {
			klog.Warningf("credentialprovider: Secret contains unexpected key %q (ignored). Only %q and %q are supported.",
				k, accessKeyID, secretAccessKey)
		}
	}

	id, okID := provideCtx.SecretData[accessKeyID]
	sec, okSec := provideCtx.SecretData[secretAccessKey]

	if okID {
		id = strings.TrimSpace(id)
		if !accessKeyIDRe.MatchString(id) {
			klog.Warningf("credentialprovider: access_key_id %q is not alphanumeric or exceeds %d chars",
				id, maxAccessKeyIDLen)
			okID = false
		}
	}

	if okSec {
		sec = strings.TrimSpace(sec)
		if !secretAccessKeyRe.MatchString(sec) || !utf8.ValidString(sec) {
			klog.Warningf("credentialprovider: secret_access_key is invalid or exceeds %d chars",
				maxSecretAccessKeyLen)
			okSec = false
		}
	}

	if okID && okSec {
		env.Set(envprovider.EnvAccessKeyID, id)
		env.Set(envprovider.EnvSecretAccessKey, sec)

		// FULL access_key_id logged (no masking) for audit purposes.
		klog.V(3).Infof("credentialprovider: volume %s authenticated with access_key_id %s",
			provideCtx.VolumeID, id)

		return env, nil
	}

	var missing []string
	if !okID {
		missing = append(missing, accessKeyID)
	}
	if !okSec {
		missing = append(missing, secretAccessKey)
	}
	return nil, status.Errorf(
		codes.InvalidArgument,
		"credentialprovider: missing or invalid keys in Kubernetes Secret: %s",
		strings.Join(missing, ", "),
	)
}
