package middleware

import (
	"net/http"
	"strings"

	"omnidrive_cloud/internal/config"
)

var corsAllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
var corsAllowedHeaders = []string{"Accept", "Authorization", "Content-Type", "Origin", "X-Requested-With"}
var corsExposedHeaders = []string{"Content-Length", "Content-Type"}

func CORS(cfg config.Config) func(http.Handler) http.Handler {
	allowedOrigins := append([]string(nil), cfg.CORSAllowedOrigins...)
	allowAllOrigins := false
	allowedOriginSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			allowAllOrigins = true
			continue
		}
		allowedOriginSet[trimmed] = struct{}{}
	}

	allowMethodsValue := strings.Join(corsAllowedMethods, ", ")
	allowHeadersValue := strings.Join(corsAllowedHeaders, ", ")
	exposeHeadersValue := strings.Join(corsExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if !allowAllOrigins {
				if _, ok := allowedOriginSet[origin]; !ok {
					if r.Method == http.MethodOptions {
						http.Error(w, "CORS origin denied", http.StatusForbidden)
						return
					}
					next.ServeHTTP(w, r)
					return
				}
			}

			headers := w.Header()
			headers.Set("Access-Control-Allow-Origin", origin)
			headers.Set("Access-Control-Allow-Credentials", "true")
			headers.Set("Access-Control-Allow-Methods", allowMethodsValue)
			headers.Set("Access-Control-Allow-Headers", allowHeadersValue)
			headers.Set("Access-Control-Expose-Headers", exposeHeadersValue)
			headers.Add("Vary", "Origin")
			headers.Add("Vary", "Access-Control-Request-Method")
			headers.Add("Vary", "Access-Control-Request-Headers")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
