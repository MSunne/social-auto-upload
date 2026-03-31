package store

import (
	"testing"
	"time"
)

func TestComputeDeviceStatusUsesHeartbeatGraceWindow(t *testing.T) {
	now := time.Now().UTC()
	lastSeenAt := now.Add(-70 * time.Second)

	if got := computeDeviceStatus(&lastSeenAt, nil); got != "online" {
		t.Fatalf("expected default grace window to keep device online, got %q", got)
	}

	runtimePayload := []byte(`{"heartbeatIntervalSeconds":30}`)
	if got := computeDeviceStatus(&lastSeenAt, runtimePayload); got != "online" {
		t.Fatalf("expected heartbeat-aware grace window to keep device online, got %q", got)
	}
}

func TestComputeDeviceStatusFallsOfflinePastHeartbeatWindow(t *testing.T) {
	now := time.Now().UTC()
	lastSeenAt := now.Add(-3 * time.Minute)

	runtimePayload := []byte(`{"heartbeatIntervalSeconds":30}`)
	if got := computeDeviceStatus(&lastSeenAt, runtimePayload); got != "offline" {
		t.Fatalf("expected stale device to be offline, got %q", got)
	}
}
