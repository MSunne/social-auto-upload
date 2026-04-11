package store

import (
	"strings"
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

func TestComputeDeviceBridgeStatusUsesRuntimeBridgeHealth(t *testing.T) {
	now := time.Now().UTC()
	lastSeenAt := now.Add(-30 * time.Second)

	if got := computeDeviceBridgeStatus(&lastSeenAt, []byte(`{"bridgeStatus":"healthy"}`)); got != "healthy" {
		t.Fatalf("expected healthy bridge status, got %q", got)
	}
	if got := computeDeviceBridgeStatus(&lastSeenAt, []byte(`{"bridgeStatus":"degraded","bridgeLastError":"timeout"}`)); got != "degraded" {
		t.Fatalf("expected degraded bridge status, got %q", got)
	}
	if got := computeDeviceBridgeStatus(&lastSeenAt, []byte(`{"cloudReachable":false}`)); got != "degraded" {
		t.Fatalf("expected cloudReachable=false to degrade bridge status, got %q", got)
	}
}

func TestComputeDeviceBridgeStatusFallsOfflineWhenHeartbeatIsStale(t *testing.T) {
	now := time.Now().UTC()
	lastSeenAt := now.Add(-3 * time.Minute)

	if got := computeDeviceBridgeStatus(&lastSeenAt, []byte(`{"bridgeStatus":"healthy"}`)); got != "offline" {
		t.Fatalf("expected stale device bridge status to be offline, got %q", got)
	}
}

func TestAIJobSelectColumnsForSummaryOmitsHeavyPayloads(t *testing.T) {
	columns := aiJobSelectColumnsFor("ai_jobs", aiJobPayloadModeSummary)
	if !strings.Contains(columns, "jsonb_build_object('skillName'") {
		t.Fatalf("expected summary columns to keep skillName, got %q", columns)
	}
	if !strings.Contains(columns, "'conversationId'") {
		t.Fatalf("expected summary columns to keep conversationId, got %q", columns)
	}
	if !strings.Contains(columns, "jsonb_build_object('stage'") {
		t.Fatalf("expected summary columns to keep output stage, got %q", columns)
	}
	if strings.Contains(columns, "ai_jobs.input_payload,") {
		t.Fatalf("expected summary columns to omit raw input payload, got %q", columns)
	}
	if strings.Contains(columns, "ai_jobs.output_payload,") {
		t.Fatalf("expected summary columns to omit raw output payload, got %q", columns)
	}
}

func TestAdminAIJobListSelectColumnsStayLightweight(t *testing.T) {
	if strings.Contains(adminAIJobListSelectColumns, "input_payload") {
		t.Fatalf("expected admin AI job list columns to omit input payload, got %q", adminAIJobListSelectColumns)
	}
	if strings.Contains(adminAIJobListSelectColumns, "output_payload") {
		t.Fatalf("expected admin AI job list columns to omit output payload, got %q", adminAIJobListSelectColumns)
	}
	if strings.Contains(adminAIJobListSelectColumns, "prompt") {
		t.Fatalf("expected admin AI job list columns to omit prompt, got %q", adminAIJobListSelectColumns)
	}
	if strings.Contains(adminAIJobListSelectColumns, "notes") {
		t.Fatalf("expected admin AI job list columns to omit notes, got %q", adminAIJobListSelectColumns)
	}
}

func TestNextPrefixBoundary(t *testing.T) {
	if got, ok := nextPrefixBoundary("user"); !ok || got != "uses" {
		t.Fatalf("expected prefix boundary to increment final byte, got %q ok=%v", got, ok)
	}
	if got, ok := nextPrefixBoundary(""); ok || got != "" {
		t.Fatalf("expected empty prefix to have no boundary, got %q ok=%v", got, ok)
	}
}
