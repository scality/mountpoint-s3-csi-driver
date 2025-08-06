package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestControllerGetCapabilities(t *testing.T) {
	driver := &Driver{}

	req := &csi.ControllerGetCapabilitiesRequest{}
	resp, err := driver.ControllerGetCapabilities(context.Background(), req)
	if err != nil {
		t.Fatalf("ControllerGetCapabilities failed: %v", err)
	}

	if resp == nil {
		t.Fatal("ControllerGetCapabilities returned nil response")
	}

	if len(resp.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(resp.Capabilities))
	}

	capability := resp.Capabilities[0]
	if capability.GetRpc() == nil {
		t.Fatal("Expected RPC capability, got nil")
	}

	if capability.GetRpc().Type != csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME {
		t.Fatalf("Expected CREATE_DELETE_VOLUME capability, got %v", capability.GetRpc().Type)
	}
}

func TestGetPluginCapabilities(t *testing.T) {
	driver := &Driver{}

	req := &csi.GetPluginCapabilitiesRequest{}
	resp, err := driver.GetPluginCapabilities(context.Background(), req)
	if err != nil {
		t.Fatalf("GetPluginCapabilities failed: %v", err)
	}

	if resp == nil {
		t.Fatal("GetPluginCapabilities returned nil response")
	}

	if len(resp.Capabilities) != 1 {
		t.Fatalf("Expected 1 capability, got %d", len(resp.Capabilities))
	}

	capability := resp.Capabilities[0]
	if capability.GetService() == nil {
		t.Fatal("Expected Service capability, got nil")
	}

	if capability.GetService().Type != csi.PluginCapability_Service_CONTROLLER_SERVICE {
		t.Fatalf("Expected CONTROLLER_SERVICE capability, got %v", capability.GetService().Type)
	}
}
