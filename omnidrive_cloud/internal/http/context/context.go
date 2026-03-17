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

func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

func CurrentUser(ctx context.Context) *domain.User {
	user, _ := ctx.Value(userContextKey{}).(*domain.User)
	return user
}

func WithAdmin(ctx context.Context, admin *domain.AdminIdentity) context.Context {
	return context.WithValue(ctx, adminContextKey{}, admin)
}

func CurrentAdmin(ctx context.Context) *domain.AdminIdentity {
	admin, _ := ctx.Value(adminContextKey{}).(*domain.AdminIdentity)
	return admin
}

func WithRequestMetadata(ctx context.Context, metadata *RequestMetadata) context.Context {
	return context.WithValue(ctx, requestMetadataContextKey{}, metadata)
}

func CurrentRequestMetadata(ctx context.Context) *RequestMetadata {
	metadata, _ := ctx.Value(requestMetadataContextKey{}).(*RequestMetadata)
	return metadata
}

func SetRequestUser(ctx context.Context, user *domain.User) {
	if metadata := CurrentRequestMetadata(ctx); metadata != nil && user != nil {
		metadata.UserID = user.ID
		metadata.UserEmail = user.Email
	}
}

func SetRequestAdmin(ctx context.Context, admin *domain.AdminIdentity) {
	if metadata := CurrentRequestMetadata(ctx); metadata != nil && admin != nil {
		metadata.AdminID = admin.ID
		metadata.AdminEmail = admin.Email
	}
}
