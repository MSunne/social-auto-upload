package middleware

import (
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

func RequireUser(app *appstate.App) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				render.Error(w, http.StatusUnauthorized, "Missing bearer token")
				return
			}

			rawToken := strings.TrimSpace(authHeader[7:])
			userID, err := app.Tokens.ParseToken(rawToken)
			if err != nil {
				render.Error(w, http.StatusUnauthorized, "Invalid access token")
				return
			}

			user, err := app.Store.GetUserByID(r.Context(), userID)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to load user")
				return
			}
			if user == nil || !user.IsActive {
				render.Error(w, http.StatusUnauthorized, "User not found")
				return
			}

			next.ServeHTTP(w, r.WithContext(httpcontext.WithUser(r.Context(), user)))
		})
	}
}
