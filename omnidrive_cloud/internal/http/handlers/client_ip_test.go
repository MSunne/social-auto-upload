package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestResolveHeartbeatPublicIPPrefersForwardedClientIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", nil)
	req.Header.Set("X-Forwarded-For", "47.96.12.34, 10.0.0.2")
	req.RemoteAddr = "10.0.0.2:8410"

	got := resolveHeartbeatPublicIP(req, nil)
	if got == nil || *got != "47.96.12.34" {
		t.Fatalf("resolveHeartbeatPublicIP() = %v, want 47.96.12.34", got)
	}
}

func TestResolveHeartbeatPublicIPFallsBackToPayloadWhenProxyHidesClientIP(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", nil)
	req.RemoteAddr = "127.0.0.1:8410"
	payload := "8.8.8.8"

	got := resolveHeartbeatPublicIP(req, &payload)
	if got == nil || *got != "8.8.8.8" {
		t.Fatalf("resolveHeartbeatPublicIP() = %v, want 8.8.8.8", got)
	}
}

func TestResolveHeartbeatPublicIPRejectsPrivateAndReservedAddresses(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.11")
	req.Header.Set("X-Real-IP", "198.18.0.1")
	req.RemoteAddr = "10.0.0.2:8410"
	payload := "203.0.113.10"

	got := resolveHeartbeatPublicIP(req, &payload)
	if got != nil {
		t.Fatalf("resolveHeartbeatPublicIP() = %v, want nil", *got)
	}
}
