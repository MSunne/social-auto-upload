package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

func RequireAdmin(app *appstate.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "missing admin bearer token")
				render.Error(w, http.StatusUnauthorized, "Missing bearer token")
				return
			}

			rawToken := strings.TrimSpace(authHeader[7:])
			subject, err := app.AdminTokens.ParseToken(rawToken)
			if err != nil {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "invalid admin access token", "error", err)
				render.Error(w, http.StatusUnauthorized, "Invalid admin access token")
				return
			}

			admin, err := app.ResolveAdminIdentity(r.Context(), subject)
			if err != nil {
				logAuthFailure(app.Logger, r, slog.LevelError, "failed to resolve admin identity", "error", err)
				render.Error(w, http.StatusInternalServerError, "Failed to resolve admin identity")
				return
			}
			if admin == nil {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "admin identity not found for token")
				render.Error(w, http.StatusUnauthorized, "Admin not found")
				return
			}

			ctx := httpcontext.WithAdmin(r.Context(), admin)
			httpcontext.SetRequestAdmin(ctx, admin)
			app.Logger.Debug("admin authenticated", "request_id", chimiddleware.GetReqID(r.Context()), "admin_id", admin.ID, "path", r.URL.Path)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireAdminPermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := httpcontext.CurrentAdmin(r.Context())
			if admin == nil {
				logAuthFailure(slog.Default(), r, slog.LevelWarn, "admin permission check without admin context", "required_permission", permission)
				render.Error(w, http.StatusUnauthorized, "Admin not found")
				return
			}

			for _, item := range admin.Permissions {
				if item == permission {
					next.ServeHTTP(w, r)
					return
				}
			}

			logAuthFailure(slog.Default(), r, slog.LevelWarn, "admin permission denied", "admin_id", admin.ID, "required_permission", permission)
			render.Error(w, http.StatusForbidden, "Admin permission denied")
		})
	}
}
