package node_test

import (
	"strings"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
)

func TestProtoSanitizerRedactsSecrets(t *testing.T) {
	// Create a NodePublishVolumeRequest with secrets
	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume-id",
		TargetPath: "/test/path",
		Secrets: map[string]string{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"session_token":     "FQoGZXIvYXdzEHcaDBPsK5E7XBqt...",
		},
		VolumeContext: map[string]string{
			"bucketName": "my-bucket",
		},
	}

	// Use protosanitizer to strip secrets
	sanitized := protosanitizer.StripSecrets(req).String()

	// Verify that ALL secrets are redacted (the entire secrets map is marked as csi_secret)
	if strings.Contains(sanitized, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") {
		t.Errorf("Secret access key was not redacted: %s", sanitized)
	}

	if strings.Contains(sanitized, "FQoGZXIvYXdzEHcaDBPsK5E7XBqt") {
		t.Errorf("Session token was not redacted: %s", sanitized)
	}

	if strings.Contains(sanitized, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("Access key ID was not redacted: %s", sanitized)
	}

	// Verify that the secrets field is present but redacted
	if !strings.Contains(sanitized, `"secrets":"***stripped***"`) {
		t.Errorf("Secrets field was not properly redacted, expected '***stripped***': %s", sanitized)
	}

	// Verify that non-secret fields are preserved
	if !strings.Contains(sanitized, "test-volume-id") {
		t.Errorf("Volume ID was unexpectedly redacted: %s", sanitized)
	}

	if !strings.Contains(sanitized, "/test/path") {
		t.Errorf("Target path was unexpectedly redacted: %s", sanitized)
	}

	if !strings.Contains(sanitized, "my-bucket") {
		t.Errorf("Bucket name was unexpectedly redacted: %s", sanitized)
	}
}
