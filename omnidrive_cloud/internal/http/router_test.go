package http

import (
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/config"
)

func TestAgentLoginRoutesAreRegistered(t *testing.T) {
	app := &appstate.App{
		Config: config.Config{},
		Logger: slog.Default(),
	}
	router := NewRouter(app)

	cases := []struct {
		method string
		path   string
	}{
		{method: stdhttp.MethodGet, path: "/api/v1/agent/login-tasks/device-1"},
		{method: stdhttp.MethodPost, path: "/api/v1/agent/login-sessions/session-1/event"},
		{method: stdhttp.MethodGet, path: "/api/v1/agent/login-sessions/session-1/actions"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code == stdhttp.StatusNotFound {
			t.Fatalf("%s %s returned 404, route is not registered", tc.method, tc.path)
		}
	}
}

func TestDigitalHumanRoutesAreRegistered(t *testing.T) {
	app := &appstate.App{
		Config: config.Config{},
		Logger: slog.Default(),
	}
	router := NewRouter(app)

	cases := []struct {
		method string
		path   string
	}{
		{method: stdhttp.MethodGet, path: "/api/v1/digital-human/billing-preview"},
		{method: stdhttp.MethodGet, path: "/api/v1/digital-human/tasks"},
		{method: stdhttp.MethodPost, path: "/api/v1/digital-human/tasks"},
		{method: stdhttp.MethodGet, path: "/api/v1/digital-human/tasks/task-1"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code == stdhttp.StatusNotFound {
			t.Fatalf("%s %s returned 404, route is not registered", tc.method, tc.path)
		}
	}
}

func TestAdminDigitalHumanRoutesAreRegistered(t *testing.T) {
	app := &appstate.App{
		Config: config.Config{},
		Logger: slog.Default(),
	}
	router := NewRouter(app)

	cases := []struct {
		method string
		path   string
	}{
		{method: stdhttp.MethodGet, path: "/api/admin/v1/digital-human/tasks"},
		{method: stdhttp.MethodGet, path: "/api/admin/v1/digital-human/tasks/task-1"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code == stdhttp.StatusNotFound {
			t.Fatalf("%s %s returned 404, route is not registered", tc.method, tc.path)
		}
	}
}
