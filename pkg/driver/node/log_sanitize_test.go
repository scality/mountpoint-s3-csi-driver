package node

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/scality/mountpoint-s3-csi-driver/pkg/driver/node/volumecontext"
)

func TestLogSafeNodePublishVolumeRequest(t *testing.T) {
	req := &csi.NodePublishVolumeRequest{
		VolumeId:   "test-volume-id",
		TargetPath: "/test/path",
		Secrets: map[string]string{
			"access_key_id":     "AKIAIOSFODNN7EXAMPLE",
			"secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
		VolumeContext: map[string]string{
			"bucketName":                          "my-bucket",
			volumecontext.CSIServiceAccountTokens: "eyJhbGciOiJSUzI1NiIs...",
		},
	}

	safe := logSafeNodePublishVolumeRequest(req)

	// Secrets field must be nil (completely excluded from copy)
	if safe.Secrets != nil {
		t.Errorf("Secrets should be nil, got %v", safe.Secrets)
	}

	// Service account tokens must be stripped from VolumeContext
	if _, ok := safe.VolumeContext[volumecontext.CSIServiceAccountTokens]; ok {
		t.Error("Service account tokens should be removed from VolumeContext")
	}

	// Non-sensitive fields must be preserved
	if safe.VolumeId != "test-volume-id" {
		t.Errorf("VolumeId not preserved: %s", safe.VolumeId)
	}
	if safe.TargetPath != "/test/path" {
		t.Errorf("TargetPath not preserved: %s", safe.TargetPath)
	}
	if safe.VolumeContext["bucketName"] != "my-bucket" {
		t.Errorf("bucketName not preserved in VolumeContext")
	}

	// Original request must not be mutated
	if _, ok := req.VolumeContext[volumecontext.CSIServiceAccountTokens]; !ok {
		t.Error("Original request VolumeContext was mutated")
	}
}
