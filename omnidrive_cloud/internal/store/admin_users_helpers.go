package store

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"omnidrive_cloud/internal/domain"
)

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func normalizeAdminStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	sort.Strings(items)
	return items
}

func normalizeTextValues(values []string) []string {
	return normalizeAdminStringList(values)
}

func adminPrimaryRole(roles []string) string {
	items := normalizeAdminStringList(roles)
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func sortAdminIdentity(identity *domain.AdminIdentity) {
	if identity == nil {
		return
	}
	identity.RoleIDs = normalizeAdminStringList(identity.RoleIDs)
	identity.Roles = normalizeAdminStringList(identity.Roles)
	identity.Permissions = normalizeAdminStringList(identity.Permissions)

	identity.Role = adminPrimaryRole(identity.Roles)
	if identity.Role == "" {
		identity.Role = adminPrimaryRole(identity.RoleIDs)
	}
}

func ensureAdminRolesExist(ctx context.Context, tx pgx.Tx, roleIDs []string) error {
	normalized := normalizeAdminStringList(roleIDs)
	if len(normalized) == 0 {
		return nil
	}

	var count int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM admin_roles
		WHERE id = ANY($1)
	`, normalized).Scan(&count); err != nil {
		return err
	}
	if count != len(normalized) {
		return fmt.Errorf("one or more admin roles are invalid")
	}
	return nil
}
