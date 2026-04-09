package logging

import "context"

type operationContextKey struct{}

// 处理Operation相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func WithOperation(ctx context.Context, operation string) context.Context {
	if ctx == nil || operation == "" {
		return ctx
	}
	return context.WithValue(ctx, operationContextKey{}, operation)
}

// 处理Operation上下文相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func OperationFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	operation, _ := ctx.Value(operationContextKey{}).(string)
	return operation
}
