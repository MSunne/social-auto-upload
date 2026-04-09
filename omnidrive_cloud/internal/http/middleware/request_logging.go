package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/logging"
)

const bodyCaptureLimit = logging.DefaultPreviewLimit

// 创建请求日志中间件，在请求完成后统一记录状态码、耗时和请求标识。
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			requestMetadata := &httpcontext.RequestMetadata{}
			requestContext := httpcontext.WithRequestMetadata(r.Context(), requestMetadata)
			r = r.WithContext(requestContext)

			debugEnabled := logger.Enabled(r.Context(), slog.LevelDebug)
			if r.Body != nil && shouldCaptureBody(r.Header.Get("Content-Type")) {
				r.Body = newBodyCaptureReadCloser(r.Body, bodyCaptureLimit)
			}

			ww := newResponseCaptureWriter(chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor), bodyCaptureLimit)
			next.ServeHTTP(ww, r)
			logRequest(logger, r, ww, startedAt, requestMetadata, debugEnabled)
		})
	}
}

// 构造log请求中间件，统一处理请求进入业务层前后的公共逻辑。
func logRequest(logger *slog.Logger, r *http.Request, ww *responseCaptureWriter, startedAt time.Time, metadata *httpcontext.RequestMetadata, debugEnabled bool) {
	status := ww.Status()
	if status == 0 {
		status = http.StatusOK
	}

	attrs := []any{
		"request_id", chimiddleware.GetReqID(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"bytes_written", ww.BytesWritten(),
		"remote_addr", r.RemoteAddr,
	}
	if metadata != nil {
		if metadata.UserID != "" {
			attrs = append(attrs, "user_id", metadata.UserID)
		}
		if metadata.UserEmail != "" {
			attrs = append(attrs, "user_email", metadata.UserEmail)
		}
		if metadata.AdminID != "" {
			attrs = append(attrs, "admin_id", metadata.AdminID)
		}
		if metadata.AdminEmail != "" {
			attrs = append(attrs, "admin_email", metadata.AdminEmail)
		}
	}

	if routePattern := routePattern(r); routePattern != "" {
		attrs = append(attrs, "route", routePattern)
	}
	if rawQuery := strings.TrimSpace(r.URL.RawQuery); rawQuery != "" {
		attrs = append(attrs, "query", logging.TruncateString(rawQuery, 1024))
	}
	if userAgent := strings.TrimSpace(r.UserAgent()); userAgent != "" {
		attrs = append(attrs, "user_agent", userAgent)
	}
	responseBodyPreview := ""
	if debugEnabled || status >= http.StatusBadRequest {
		if requestBody := previewRequestBody(r); requestBody != "" {
			attrs = append(attrs, "request_body", requestBody)
		}
		if responseBody := ww.preview(); responseBody != "" {
			responseBodyPreview = responseBody
			attrs = append(attrs, "response_body", responseBody)
		}
		if requestContentType := strings.TrimSpace(r.Header.Get("Content-Type")); requestContentType != "" {
			attrs = append(attrs, "request_content_type", requestContentType)
		}
		if responseContentType := strings.TrimSpace(ww.Header().Get("Content-Type")); responseContentType != "" {
			attrs = append(attrs, "response_content_type", responseContentType)
		}
	}

	switch requestLogLevel(status, r.URL.Path, routePattern(r), responseBodyPreview) {
	case slog.LevelError:
		logger.Error("http request completed", attrs...)
	case slog.LevelWarn:
		logger.Warn("http request completed", attrs...)
	default:
		logger.Debug("http request completed", attrs...)
	}
}

// 构造请求LogLevel中间件，统一处理请求进入业务层前后的公共逻辑。
func requestLogLevel(status int, path string, route string, responseBody string) slog.Level {
	switch {
	case shouldDowngradeAgentPublishSyncConflict(status, path, route, responseBody):
		return slog.LevelDebug
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelDebug
	}
}

// 构造判断是否应当DowngradeAgent发布同步Conflict中间件，统一处理请求进入业务层前后的公共逻辑。
func shouldDowngradeAgentPublishSyncConflict(status int, path string, route string, responseBody string) bool {
	if status != http.StatusConflict {
		return false
	}
	target := strings.TrimSpace(route)
	if target == "" {
		target = strings.TrimSpace(path)
	}
	if target != "/api/v1/agent/publish-tasks/sync" {
		return false
	}
	return strings.Contains(responseBody, "Publish task belongs to a different device")
}

type bodyCaptureReadCloser struct {
	rc        io.ReadCloser
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

// 构造newBodyCapture读取Closer中间件，统一处理请求进入业务层前后的公共逻辑。
func newBodyCaptureReadCloser(rc io.ReadCloser, limit int) *bodyCaptureReadCloser {
	return &bodyCaptureReadCloser{rc: rc, limit: limit}
}

// 构造读取中间件，统一处理请求进入业务层前后的公共逻辑。
func (c *bodyCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.capture(p[:n])
	return n, err
}

// 构造Close中间件，统一处理请求进入业务层前后的公共逻辑。
func (c *bodyCaptureReadCloser) Close() error {
	return c.rc.Close()
}

// 构造预览中间件，统一处理请求进入业务层前后的公共逻辑。
func (c *bodyCaptureReadCloser) preview(contentType string) string {
	return logging.PreviewBody(contentType, c.buffer.Bytes(), c.truncated)
}

// 构造capture中间件，统一处理请求进入业务层前后的公共逻辑。
func (c *bodyCaptureReadCloser) capture(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	remaining := c.limit - c.buffer.Len()
	if remaining <= 0 {
		c.truncated = true
		return
	}
	if len(chunk) > remaining {
		_, _ = c.buffer.Write(chunk[:remaining])
		c.truncated = true
		return
	}
	_, _ = c.buffer.Write(chunk)
}

type responseCaptureWriter struct {
	chimiddleware.WrapResponseWriter
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

// 构造new响应CaptureWriter中间件，统一处理请求进入业务层前后的公共逻辑。
func newResponseCaptureWriter(w chimiddleware.WrapResponseWriter, limit int) *responseCaptureWriter {
	return &responseCaptureWriter{
		WrapResponseWriter: w,
		limit:              limit,
	}
}

// 构造Write中间件，统一处理请求进入业务层前后的公共逻辑。
func (w *responseCaptureWriter) Write(p []byte) (int, error) {
	w.capture(p)
	return w.WrapResponseWriter.Write(p)
}

// 构造WriteString中间件，统一处理请求进入业务层前后的公共逻辑。
func (w *responseCaptureWriter) WriteString(value string) (int, error) {
	w.capture([]byte(value))
	if stringWriter, ok := w.WrapResponseWriter.(io.StringWriter); ok {
		return stringWriter.WriteString(value)
	}
	return w.WrapResponseWriter.Write([]byte(value))
}

// 构造Flush中间件，统一处理请求进入业务层前后的公共逻辑。
func (w *responseCaptureWriter) Flush() {
	if flusher, ok := w.WrapResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// 构造预览中间件，统一处理请求进入业务层前后的公共逻辑。
func (w *responseCaptureWriter) preview() string {
	return logging.PreviewBody(w.Header().Get("Content-Type"), w.buffer.Bytes(), w.truncated)
}

// 构造capture中间件，统一处理请求进入业务层前后的公共逻辑。
func (w *responseCaptureWriter) capture(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	remaining := w.limit - w.buffer.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(chunk) > remaining {
		_, _ = w.buffer.Write(chunk[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.buffer.Write(chunk)
}

// 构造预览请求Body中间件，统一处理请求进入业务层前后的公共逻辑。
func previewRequestBody(r *http.Request) string {
	bodyCapture, ok := r.Body.(*bodyCaptureReadCloser)
	if !ok || bodyCapture == nil {
		return ""
	}
	return bodyCapture.preview(r.Header.Get("Content-Type"))
}

// 构造routePattern中间件，统一处理请求进入业务层前后的公共逻辑。
func routePattern(r *http.Request) string {
	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return ""
	}
	return strings.TrimSpace(routeContext.RoutePattern())
}

// 构造判断是否应当CaptureBody中间件，统一处理请求进入业务层前后的公共逻辑。
func shouldCaptureBody(contentType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if normalized == "" {
		return true
	}
	switch {
	case strings.Contains(normalized, "json"):
		return true
	case strings.Contains(normalized, "x-www-form-urlencoded"):
		return true
	case strings.HasPrefix(normalized, "text/"):
		return true
	case strings.Contains(normalized, "xml"):
		return true
	default:
		return false
	}
}
