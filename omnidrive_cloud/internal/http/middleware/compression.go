package middleware

import (
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

var compressibleContentTypes = []string{
	"application/json",
	"text/plain",
	"text/html",
	"text/css",
	"application/javascript",
	"text/javascript",
	"application/xml",
	"text/xml",
}

// Compression 为常规 JSON/文本响应启用 gzip 压缩，同时跳过文件流与 SSE。
func Compression() func(http.Handler) http.Handler {
	compressor := chimiddleware.Compress(5, compressibleContentTypes...)
	return func(next http.Handler) http.Handler {
		compressed := compressor(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/v1/files/") {
				next.ServeHTTP(w, r)
				return
			}
			if strings.Contains(strings.ToLower(strings.TrimSpace(r.Header.Get("Accept"))), "text/event-stream") {
				next.ServeHTTP(w, r)
				return
			}
			compressed.ServeHTTP(w, r)
		})
	}
}
