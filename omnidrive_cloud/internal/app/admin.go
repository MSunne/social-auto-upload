package app

import (
	"context"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const (
	BootstrapAdminID          = "bootstrap-admin"
	SuperAdminRoleID          = "super_admin"
	adminSessionSubjectPrefix = "admin_session:"
)

var adminPermissionCatalog = []store.UpsertAdminPermissionInput{
	{Code: "user.read", Name: "Read Users", Description: "查看用户列表、资料与账务摘要", Category: "user"},
	{Code: "user.update", Name: "Update Users", Description: "更新用户内部资料和状态", Category: "user"},
	{Code: "user.freeze", Name: "Freeze Users", Description: "冻结或恢复用户", Category: "user"},
	{Code: "device.read", Name: "Read Devices", Description: "查看设备列表、状态与工作负载", Category: "device"},
	{Code: "device.update", Name: "Update Devices", Description: "更新设备配置、启停和绑定支持动作", Category: "device"},
	{Code: "task.read", Name: "Read Tasks", Description: "查看发布任务、账号登录与 AI 作业", Category: "task"},
	{Code: "task.operate", Name: "Operate Tasks", Description: "重试、取消、强制释放或人工处理任务", Category: "task"},
	{Code: "finance.read", Name: "Read Finance", Description: "查看订单、钱包流水、套餐和定价规则", Category: "finance"},
	{Code: "finance.adjust", Name: "Adjust Finance", Description: "发起财务调整或补偿", Category: "finance"},
	{Code: "support_recharge.review", Name: "Review Support Recharge", Description: "审核客服充值单并执行入账或驳回", Category: "finance"},
	{Code: "distribution.read", Name: "Read Distribution", Description: "查看分销关系、佣金与结算数据", Category: "distribution"},
	{Code: "distribution.settle", Name: "Settle Distribution", Description: "发起或确认分销结算", Category: "distribution"},
	{Code: "withdrawal.review", Name: "Review Withdrawals", Description: "审核提现申请和付款状态", Category: "finance"},
	{Code: "system.config", Name: "Manage System Config", Description: "查看或修改系统配置", Category: "system"},
	{Code: "admin.manage", Name: "Manage Admins", Description: "管理管理员账号、角色和权限", Category: "admin"},
}

var systemAdminRoleCatalog = []store.UpsertAdminRoleInput{
	{
		ID:          SuperAdminRoleID,
		Name:        "Super Admin",
		Description: stringPtr("拥有管理后台全部权限的系统角色"),
		IsSystem:    true,
		PermissionCodes: []string{
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
		},
	},
	{
		ID:          "operations",
		Name:        "Operations",
		Description: stringPtr("运营支持角色，负责用户、设备和任务巡检"),
		IsSystem:    true,
		PermissionCodes: []string{
			"user.read",
			"user.update",
			"device.read",
			"device.update",
			"task.read",
			"task.operate",
		},
	},
	{
		ID:          "support",
		Name:        "Support",
		Description: stringPtr("客服角色，处理用户问题与客服充值审核"),
		IsSystem:    true,
		PermissionCodes: []string{
			"user.read",
			"device.read",
			"task.read",
			"support_recharge.review",
			"finance.read",
		},
	},
	{
		ID:          "finance",
		Name:        "Finance",
		Description: stringPtr("财务角色，负责订单、钱包、分销和提现审核"),
		IsSystem:    true,
		PermissionCodes: []string{
			"user.read",
			"finance.read",
			"finance.adjust",
			"support_recharge.review",
			"distribution.read",
			"distribution.settle",
			"withdrawal.review",
		},
	},
	{
		ID:          "auditor",
		Name:        "Auditor",
		Description: stringPtr("审计与合规角色，只读访问关键运营与财务数据"),
		IsSystem:    true,
		PermissionCodes: []string{
			"user.read",
			"device.read",
			"task.read",
			"finance.read",
			"distribution.read",
		},
	},
	{
		ID:          "engineering_support",
		Name:        "Engineering Support",
		Description: stringPtr("工程支持角色，用于排障和系统配置检查"),
		IsSystem:    true,
		PermissionCodes: []string{
			"user.read",
			"device.read",
			"device.update",
			"task.read",
			"task.operate",
			"system.config",
		},
	},
}

func (a *App) EnsureAdminBootstrap(ctx context.Context) error {
	if err := a.Store.EnsureAdminRBACCatalog(ctx, adminPermissionCatalog, systemAdminRoleCatalog); err != nil {
		return fmt.Errorf("ensure admin rbac catalog: %w", err)
	}

	count, err := a.Store.CountAdminUsers(ctx)
	if err != nil {
		return fmt.Errorf("count admin users: %w", err)
	}
	if count > 0 {
		return nil
	}

	passwordHash, err := a.AdminTokens.HashPassword(a.Config.AdminPassword)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}

	_, err = a.Store.CreateAdminUser(ctx, store.CreateAdminUserInput{
		ID:           BootstrapAdminID,
		Email:        a.Config.AdminEmail,
		Name:         a.Config.AdminName,
		PasswordHash: passwordHash,
		IsActive:     true,
		AuthMode:     "bootstrap_seed",
		RoleIDs:      []string{SuperAdminRoleID},
	})
	if err != nil {
		return fmt.Errorf("create bootstrap admin user: %w", err)
	}
	return nil
}

func (a *App) ResolveAdminIdentity(ctx context.Context, subject string) (*domain.AdminIdentity, error) {
	sessionID, ok := parseAdminSessionSubject(subject)
	if !ok {
		return nil, nil
	}
	return a.Store.GetAdminIdentityBySessionID(ctx, sessionID)
}

func BuildAdminSessionSubject(sessionID string) string {
	return adminSessionSubjectPrefix + sessionID
}

func parseAdminSessionSubject(subject string) (string, bool) {
	if !strings.HasPrefix(subject, adminSessionSubjectPrefix) {
		return "", false
	}
	sessionID := strings.TrimSpace(strings.TrimPrefix(subject, adminSessionSubjectPrefix))
	if sessionID == "" {
		return "", false
	}
	return sessionID, true
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copied := value
	return &copied
}
