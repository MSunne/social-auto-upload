package middleware

import (
	"log/slog"
	"net/http"
	"testing"
)

func TestRequestLogLevelDowngradesExpectedAgentPublishSyncConflict(t *testing.T) {
	level := requestLogLevel(
		http.StatusConflict,
		"/api/v1/agent/publish-tasks/sync",
		"/api/v1/agent/publish-tasks/sync",
		`{"error":"Publish task belongs to a different device"}`,
	)
	if level != slog.LevelDebug {
		t.Fatalf("expected debug level, got %v", level)
	}
}

func TestRequestLogLevelKeepsUnexpectedConflictsAsWarn(t *testing.T) {
	level := requestLogLevel(
		http.StatusConflict,
		"/api/v1/agent/publish-tasks/sync",
		"/api/v1/agent/publish-tasks/sync",
		`{"error":"Publish task status transition is not allowed"}`,
	)
	if level != slog.LevelWarn {
		t.Fatalf("expected warn level, got %v", level)
	}
}
