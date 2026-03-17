package logging

import "context"

type operationContextKey struct{}

func WithOperation(ctx context.Context, operation string) context.Context {
	if ctx == nil || operation == "" {
		return ctx
	}
	return context.WithValue(ctx, operationContextKey{}, operation)
}

func OperationFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	operation, _ := ctx.Value(operationContextKey{}).(string)
	return operation
}
