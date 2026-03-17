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

func RequireUser(app *appstate.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "missing bearer token")
				render.Error(w, http.StatusUnauthorized, "Missing bearer token")
				return
			}

			rawToken := strings.TrimSpace(authHeader[7:])
			userID, err := app.Tokens.ParseToken(rawToken)
			if err != nil {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "invalid access token", "error", err)
				render.Error(w, http.StatusUnauthorized, "Invalid access token")
				return
			}

			user, err := app.Store.GetUserByID(r.Context(), userID)
			if err != nil {
				logAuthFailure(app.Logger, r, slog.LevelError, "failed to load user for access token", "user_id", userID, "error", err)
				render.Error(w, http.StatusInternalServerError, "Failed to load user")
				return
			}
			if user == nil || !user.IsActive {
				logAuthFailure(app.Logger, r, slog.LevelWarn, "inactive or missing user for access token", "user_id", userID)
				render.Error(w, http.StatusUnauthorized, "User not found")
				return
			}

			ctx := httpcontext.WithUser(r.Context(), user)
			httpcontext.SetRequestUser(ctx, user)
			app.Logger.Debug("user authenticated", "request_id", chimiddleware.GetReqID(r.Context()), "user_id", user.ID, "path", r.URL.Path)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func logAuthFailure(logger *slog.Logger, r *http.Request, level slog.Level, message string, attrs ...any) {
	if logger == nil {
		return
	}
	fields := []any{
		"request_id", chimiddleware.GetReqID(r.Context()),
		"path", r.URL.Path,
		"method", r.Method,
	}
	fields = append(fields, attrs...)
	logger.Log(r.Context(), level, message, fields...)
}
