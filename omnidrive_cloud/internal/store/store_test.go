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

func TestComputeDeviceIdentityState(t *testing.T) {
	if got := computeDeviceIdentityState(nil, nil); got != "active" {
		t.Fatalf("expected empty superseded markers to yield active, got %q", got)
	}

	replacementID := "device-new"
	if got := computeDeviceIdentityState(&replacementID, nil); got != "superseded" {
		t.Fatalf("expected superseded_by_device_id to yield superseded, got %q", got)
	}

	supersededAt := time.Now().UTC()
	if got := computeDeviceIdentityState(nil, &supersededAt); got != "superseded" {
		t.Fatalf("expected superseded_at to yield superseded, got %q", got)
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
	if !strings.Contains(columns, "LEFT(ai_jobs.prompt, 160)") {
		t.Fatalf("expected summary columns to trim prompt preview, got %q", columns)
	}
}

func TestAIJobSelectColumnsForHistoryDetailKeepsOnlyHistoryFields(t *testing.T) {
	columns := aiJobSelectColumnsFor("ai_jobs", aiJobPayloadModeHistoryDetail)
	requiredSnippets := []string{
		"ai_jobs.input_payload->'messages'",
		"ai_jobs.input_payload->'attachments'",
		"ai_jobs.output_payload->'text'",
		"ai_jobs.output_payload->'artifacts'",
		"ai_jobs.output_payload->'video'->>'contentUrl'",
		"ai_jobs.output_payload->'storyboard'->>'optimizedPrompt'",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(columns, snippet) {
			t.Fatalf("expected history detail columns to contain %q, got %q", snippet, columns)
		}
	}
	for _, forbidden := range []string{"billing", "completedSegments", "referenceFrames"} {
		if strings.Contains(columns, forbidden) {
			t.Fatalf("expected history detail columns to omit %q, got %q", forbidden, columns)
		}
	}
}

func TestAIJobArtifactSelectColumnsForPreviewOmitsHeavyFields(t *testing.T) {
	columns := aiJobArtifactSelectColumnsFor("a", aiJobArtifactModePreview)
	if !strings.Contains(columns, "NULL::jsonb AS payload") {
		t.Fatalf("expected preview artifact columns to omit payload, got %q", columns)
	}
	if !strings.Contains(columns, "NULL::text AS text_content") {
		t.Fatalf("expected preview artifact columns to omit text_content, got %q", columns)
	}
	if !strings.Contains(columns, "NULL::text AS storage_key") {
		t.Fatalf("expected preview artifact columns to omit storage_key, got %q", columns)
	}
	if !strings.Contains(columns, "a.public_url") {
		t.Fatalf("expected preview artifact columns to keep public_url, got %q", columns)
	}
}

func TestAIJobArtifactSelectColumnsForChatKeepsTextWithoutPayload(t *testing.T) {
	columns := aiJobArtifactSelectColumnsFor("a", aiJobArtifactModeChat)
	if !strings.Contains(columns, "a.text_content") {
		t.Fatalf("expected chat artifact columns to keep text_content, got %q", columns)
	}
	if !strings.Contains(columns, "NULL::jsonb AS payload") {
		t.Fatalf("expected chat artifact columns to omit payload, got %q", columns)
	}
}

func TestAIJobAccountIDExpressionUsesRootAndNestedFallback(t *testing.T) {
	expression := aiJobAccountIDExpression("ai_jobs")
	if !strings.Contains(expression, "ai_jobs.input_payload->>'accountId'") {
		t.Fatalf("expected root accountId lookup, got %q", expression)
	}
	if !strings.Contains(expression, "ai_jobs.input_payload->'publishPayload'->'targets'->0->>'accountId'") {
		t.Fatalf("expected nested publish target fallback, got %q", expression)
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
