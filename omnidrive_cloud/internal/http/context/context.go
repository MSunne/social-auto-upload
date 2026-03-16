package context

import (
	"context"

	"omnidrive_cloud/internal/domain"
)

type userContextKey struct{}
type adminContextKey struct{}

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
