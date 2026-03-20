package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

func TestResponseCaptureWriterPassthroughFlush(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := newResponseCaptureWriter(chimiddleware.NewWrapResponseWriter(recorder, 1), bodyCaptureLimit)

	flusher, ok := any(writer).(http.Flusher)
	if !ok {
		t.Fatalf("expected responseCaptureWriter to implement http.Flusher")
	}

	flusher.Flush()
	if !recorder.Flushed {
		t.Fatalf("expected underlying recorder to be flushed")
	}
}
