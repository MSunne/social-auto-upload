package database

import (
	"context"
	"log/slog"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/logging"
)

type queryTracer struct {
	logger *slog.Logger
}

type traceQueryState struct {
	startedAt time.Time
	sql       string
	args      []any
}

type traceBatchState struct {
	startedAt time.Time
}

type queryTraceKey struct{}
type batchTraceKey struct{}

func newQueryTracer(logger *slog.Logger) *queryTracer {
	if logger == nil {
		logger = slog.Default()
	}
	return &queryTracer{logger: logger}
}

func (t *queryTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	if data.ConnConfig != nil {
		t.logger.Debug("db connect start",
			"host", data.ConnConfig.Host,
			"port", data.ConnConfig.Port,
			"database", data.ConnConfig.Database,
			"user", data.ConnConfig.User,
		)
	}
	return ctx
}

func (t *queryTracer) TraceConnectEnd(ctx context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		t.logger.Error("db connect failed", "error", data.Err)
		return
	}
	t.logger.Debug("db connect succeeded")
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceKey{}, traceQueryState{
		startedAt: time.Now(),
		sql:       normalizeSQL(data.SQL),
		args:      logging.PreviewArgs(data.Args, 256),
	})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	state, _ := ctx.Value(queryTraceKey{}).(traceQueryState)
	requestID := chimiddleware.GetReqID(ctx)
	operation := logging.OperationFromContext(ctx)
	attrs := []any{
		"request_id", requestID,
		"duration_ms", time.Since(state.startedAt).Milliseconds(),
		"sql", state.sql,
		"args", state.args,
		"command_tag", data.CommandTag.String(),
	}
	if operation != "" {
		attrs = append(attrs, "operation", operation)
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
		t.logger.Error("db query completed", attrs...)
		return
	}
	if shouldSkipIdleBackgroundQuery(requestID, operation, data.CommandTag.String()) {
		return
	}
	t.logger.Debug("db query completed", attrs...)
}

func (t *queryTracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return context.WithValue(ctx, batchTraceKey{}, traceBatchState{startedAt: time.Now()})
}

func (t *queryTracer) TraceBatchQuery(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchQueryData) {
	attrs := []any{
		"request_id", chimiddleware.GetReqID(ctx),
		"sql", normalizeSQL(data.SQL),
		"args", logging.PreviewArgs(data.Args, 256),
		"command_tag", data.CommandTag.String(),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
		t.logger.Error("db batch query completed", attrs...)
		return
	}
	t.logger.Debug("db batch query completed", attrs...)
}

func (t *queryTracer) TraceBatchEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceBatchEndData) {
	state, _ := ctx.Value(batchTraceKey{}).(traceBatchState)
	attrs := []any{
		"request_id", chimiddleware.GetReqID(ctx),
		"duration_ms", time.Since(state.startedAt).Milliseconds(),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
		t.logger.Error("db batch completed", attrs...)
		return
	}
	t.logger.Debug("db batch completed", attrs...)
}

func (t *queryTracer) TracePrepareStart(ctx context.Context, _ *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	t.logger.Debug("db prepare start", "request_id", chimiddleware.GetReqID(ctx), "name", data.Name, "sql", normalizeSQL(data.SQL))
	return ctx
}

func (t *queryTracer) TracePrepareEnd(ctx context.Context, _ *pgx.Conn, data pgx.TracePrepareEndData) {
	attrs := []any{
		"request_id", chimiddleware.GetReqID(ctx),
		"already_prepared", data.AlreadyPrepared,
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
		t.logger.Error("db prepare completed", attrs...)
		return
	}
	t.logger.Debug("db prepare completed", attrs...)
}

func normalizeSQL(sql string) string {
	sql = strings.Join(strings.Fields(strings.TrimSpace(sql)), " ")
	return logging.TruncateString(sql, 1200)
}

func shouldSkipIdleBackgroundQuery(requestID string, operation string, commandTag string) bool {
	if requestID != "" || operation != "ai_worker_poll" {
		return false
	}

	commandTag = strings.TrimSpace(commandTag)
	return strings.HasSuffix(commandTag, " 0")
}
