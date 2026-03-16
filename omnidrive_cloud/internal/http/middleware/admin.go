package middleware

import (
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

func RequireAdmin(app *appstate.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				render.Error(w, http.StatusUnauthorized, "Missing bearer token")
				return
			}

			rawToken := strings.TrimSpace(authHeader[7:])
			subject, err := app.AdminTokens.ParseToken(rawToken)
			if err != nil {
				render.Error(w, http.StatusUnauthorized, "Invalid admin access token")
				return
			}

			admin := app.ResolveAdminIdentity(subject)
			if admin == nil {
				render.Error(w, http.StatusUnauthorized, "Admin not found")
				return
			}

			next.ServeHTTP(w, r.WithContext(httpcontext.WithAdmin(r.Context(), admin)))
		})
	}
}

func RequireAdminPermission(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := httpcontext.CurrentAdmin(r.Context())
			if admin == nil {
				render.Error(w, http.StatusUnauthorized, "Admin not found")
				return
			}

			for _, item := range admin.Permissions {
				if item == permission {
					next.ServeHTTP(w, r)
					return
				}
			}

			render.Error(w, http.StatusForbidden, "Admin permission denied")
		})
	}
}
