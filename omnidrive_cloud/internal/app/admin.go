package app

import "omnidrive_cloud/internal/domain"

const BootstrapAdminID = "bootstrap-admin"

var bootstrapAdminPermissions = []string{
	"user.read",
	"user.update",
	"user.freeze",
	"device.read",
	"device.update",
	"task.read",
	"task.operate",
	"finance.read",
	"finance.adjust",
	"support_recharge.review",
	"distribution.read",
	"distribution.settle",
	"withdrawal.review",
	"system.config",
	"admin.manage",
}

func (a *App) BootstrapAdminIdentity() *domain.AdminIdentity {
	return &domain.AdminIdentity{
		ID:          BootstrapAdminID,
		Email:       a.Config.AdminEmail,
		Name:        a.Config.AdminName,
		Role:        "super_admin",
		Roles:       []string{"super_admin"},
		Permissions: append([]string(nil), bootstrapAdminPermissions...),
		AuthMode:    "bootstrap_env",
	}
}

func (a *App) ResolveAdminIdentity(subject string) *domain.AdminIdentity {
	if subject != BootstrapAdminID {
		return nil
	}
	return a.BootstrapAdminIdentity()
}
