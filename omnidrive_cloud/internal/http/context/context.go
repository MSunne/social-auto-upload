package context

import (
	"context"

	"omnidrive_cloud/internal/domain"
)

type userContextKey struct{}
type adminContextKey struct{}
type requestMetadataContextKey struct{}

type RequestMetadata struct {
	UserID     string
	UserEmail  string
	AdminID    string
	AdminEmail string
}

// 处理请求上下文中的用户，供中间件和处理器在同一链路内共享状态。
func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// 处理请求上下文中的Current用户，供中间件和处理器在同一链路内共享状态。
func CurrentUser(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey{}).(*domain.User)
	return user
}

// 处理请求上下文中的管理端，供中间件和处理器在同一链路内共享状态。
func WithAdmin(ctx context.Context, admin *domain.AdminIdentity) context.Context {
	return context.WithValue(ctx, adminContextKey{}, admin)
}

// 处理请求上下文中的Current管理端，供中间件和处理器在同一链路内共享状态。
func CurrentAdmin(ctx context.Context) *domain.AdminIdentity {
	admin, _ := ctx.Value(adminContextKey{}).(*domain.AdminIdentity)
	return admin
}

// 处理请求上下文中的请求Metadata，供中间件和处理器在同一链路内共享状态。
func WithRequestMetadata(ctx context.Context, metadata *RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

// 处理请求上下文中的Current请求Metadata，供中间件和处理器在同一链路内共享状态。
func CurrentRequestMetadata(ctx context.Context) *RequestMetadata {
	metadata, _ := ctx.Value(requestMetadataContextKey{}).(*RequestMetadata)
	return metadata
}

// 处理请求上下文中的Set请求用户，供中间件和处理器在同一链路内共享状态。
func SetRequestUser(ctx context.Context, user *domain.User) {
	if metadata := CurrentRequestMetadata(ctx); metadata != nil && user != nil {
		metadata.UserID = user.ID
		if user.Email != "" {
			metadata.UserEmail = user.Email
		} else {
			metadata.UserEmail = user.Phone
		}
	}
}

// 处理请求上下文中的Set请求管理端，供中间件和处理器在同一链路内共享状态。
func SetRequestAdmin(ctx context.Context, admin *domain.AdminIdentity) {
	if metadata := CurrentRequestMetadata(ctx); metadata != nil && admin != nil {
		metadata.AdminID = admin.ID
		metadata.AdminEmail = admin.Email
	}
}
