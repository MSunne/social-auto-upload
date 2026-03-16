package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminRBACHandler struct {
	app *appstate.App
}

type createAdminRequest struct {
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Password string   `json:"password"`
	RoleIDs  []string `json:"roleIds"`
	IsActive *bool    `json:"isActive"`
}

type updateAdminRequest struct {
	Email    *string   `json:"email"`
	Name     *string   `json:"name"`
	Password *string   `json:"password"`
	RoleIDs  *[]string `json:"roleIds"`
	IsActive *bool     `json:"isActive"`
}

type createAdminRoleRequest struct {
	Name            string   `json:"name"`
	Description     *string  `json:"description"`
	PermissionCodes []string `json:"permissionCodes"`
}

func NewAdminRBACHandler(app *appstate.App) *AdminRBACHandler {
	return &AdminRBACHandler{app: app}
}

func (h *AdminRBACHandler) ListAdmins(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	roleID := strings.TrimSpace(r.URL.Query().Get("roleId"))

	items, total, err := h.app.Store.ListAdminIdentities(r.Context(), store.AdminIdentityListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		RoleID: roleID,
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin users")
		return
	}

	activeCount, err := h.app.Store.CountActiveAdminUsers(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin summary")
		return
	}

	roles, err := h.app.Store.ListAdminRoles(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin roles")
		return
	}

	permissions, err := h.app.Store.ListAdminPermissions(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load permission catalog")
		return
	}

	renderAdminList(w, page, total, items, map[string]any{
		"activeAdminCount": activeCount,
		"roleCount":        len(roles),
		"permissionCount":  len(permissions),
		"authMode":         "database_rbac",
	}, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"roleId":        roleID,
		"statusOptions": []string{"active", "inactive"},
		"roles":         roles,
	})
}

func (h *AdminRBACHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	var payload createAdminRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	email := strings.TrimSpace(strings.ToLower(payload.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		render.Error(w, http.StatusBadRequest, "Invalid admin email")
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		render.Error(w, http.StatusBadRequest, "Admin name is required")
		return
	}
	if len(payload.Password) < 8 {
		render.Error(w, http.StatusBadRequest, "Password must be at least 8 characters")
		return
	}

	passwordHash, err := h.app.AdminTokens.HashPassword(payload.Password)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to hash admin password")
		return
	}

	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}

	admin, err := h.app.Store.CreateAdminUser(r.Context(), store.CreateAdminUserInput{
		ID:           uuid.NewString(),
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		IsActive:     isActive,
		AuthMode:     "password",
		RoleIDs:      payload.RoleIDs,
	})
	if err != nil {
		switch {
		case isAdminUniqueViolation(err):
			render.Error(w, http.StatusConflict, "Admin email already exists")
		case isAdminInputError(err):
			render.Error(w, http.StatusBadRequest, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to create admin user")
		}
		return
	}

	currentAdmin := httpcontext.CurrentAdmin(r.Context())
	if currentAdmin != nil {
		recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
			AdminUserID:  stringPtr(currentAdmin.ID),
			AdminEmail:   stringPtr(currentAdmin.Email),
			AdminName:    stringPtr(currentAdmin.Name),
			ResourceType: "admin_user",
			ResourceID:   stringPtr(admin.ID),
			Action:       "create",
			Title:        "创建管理员",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("管理员账号已创建"),
			Payload: mustJSONBytes(map[string]any{
				"email":    admin.Email,
				"roleIds":  admin.RoleIDs,
				"isActive": admin.IsActive,
			}),
		})
	}

	render.JSON(w, http.StatusCreated, admin)
}

func (h *AdminRBACHandler) UpdateAdmin(w http.ResponseWriter, r *http.Request) {
	adminID := strings.TrimSpace(chi.URLParam(r, "adminId"))
	if adminID == "" {
		render.Error(w, http.StatusBadRequest, "adminId is required")
		return
	}

	existing, err := h.app.Store.GetAdminIdentityByID(r.Context(), adminID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin user")
		return
	}
	if existing == nil {
		render.Error(w, http.StatusNotFound, "Admin user not found")
		return
	}

	var payload updateAdminRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	updateInput := store.UpdateAdminUserInput{
		Email:    payload.Email,
		Name:     payload.Name,
		IsActive: payload.IsActive,
	}

	if payload.Email != nil {
		trimmed := strings.TrimSpace(strings.ToLower(*payload.Email))
		if _, err := mail.ParseAddress(trimmed); err != nil {
			render.Error(w, http.StatusBadRequest, "Invalid admin email")
			return
		}
		updateInput.Email = &trimmed
	}
	if payload.Name != nil {
		trimmed := strings.TrimSpace(*payload.Name)
		if trimmed == "" {
			render.Error(w, http.StatusBadRequest, "Admin name is required")
			return
		}
		updateInput.Name = &trimmed
	}
	if payload.Password != nil {
		if len(*payload.Password) < 8 {
			render.Error(w, http.StatusBadRequest, "Password must be at least 8 characters")
			return
		}
		passwordHash, err := h.app.AdminTokens.HashPassword(*payload.Password)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to hash admin password")
			return
		}
		updateInput.PasswordHash = &passwordHash
	}
	if payload.RoleIDs != nil {
		updateInput.RoleIDs = *payload.RoleIDs
		updateInput.RoleIDsTouched = true
	}

	if err := ensureSuperAdminRemains(r.Context(), h.app.Store, existing, updateInput); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin, err := h.app.Store.UpdateAdminUser(r.Context(), adminID, updateInput)
	if err != nil {
		switch {
		case isAdminUniqueViolation(err):
			render.Error(w, http.StatusConflict, "Admin email already exists")
		case isAdminInputError(err):
			render.Error(w, http.StatusBadRequest, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to update admin user")
		}
		return
	}
	if admin == nil {
		render.Error(w, http.StatusNotFound, "Admin user not found")
		return
	}

	currentAdmin := httpcontext.CurrentAdmin(r.Context())
	if currentAdmin != nil {
		recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
			AdminUserID:  stringPtr(currentAdmin.ID),
			AdminEmail:   stringPtr(currentAdmin.Email),
			AdminName:    stringPtr(currentAdmin.Name),
			ResourceType: "admin_user",
			ResourceID:   stringPtr(admin.ID),
			Action:       "update",
			Title:        "更新管理员",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("管理员资料已更新"),
			Payload: mustJSONBytes(map[string]any{
				"roleIds":  admin.RoleIDs,
				"isActive": admin.IsActive,
			}),
		})
	}

	render.JSON(w, http.StatusOK, admin)
}

func (h *AdminRBACHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListAdminRoles(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin roles")
		return
	}

	permissions, err := h.app.Store.ListAdminPermissions(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load permission catalog")
		return
	}

	systemRoleCount := 0
	for _, item := range items {
		if item.IsSystem {
			systemRoleCount++
		}
	}

	page := adminPageQuery{Page: 1, PageSize: max(1, len(items))}
	renderAdminList(w, page, int64(len(items)), items, map[string]any{
		"roleCount":         len(items),
		"systemRoleCount":   systemRoleCount,
		"permissionCount":   len(permissions),
		"permissionCatalog": permissions,
	}, nil)
}

func (h *AdminRBACHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	var payload createAdminRoleRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		render.Error(w, http.StatusBadRequest, "Role name is required")
		return
	}

	role, err := h.app.Store.CreateAdminRole(r.Context(), store.CreateAdminRoleInput{
		ID:              uuid.NewString(),
		Name:            name,
		Description:     trimOptionalString(payload.Description),
		PermissionCodes: payload.PermissionCodes,
	})
	if err != nil {
		switch {
		case isAdminUniqueViolation(err):
			render.Error(w, http.StatusConflict, "Role name already exists")
		case isAdminInputError(err):
			render.Error(w, http.StatusBadRequest, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to create admin role")
		}
		return
	}

	currentAdmin := httpcontext.CurrentAdmin(r.Context())
	if currentAdmin != nil {
		recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
			AdminUserID:  stringPtr(currentAdmin.ID),
			AdminEmail:   stringPtr(currentAdmin.Email),
			AdminName:    stringPtr(currentAdmin.Name),
			ResourceType: "admin_role",
			ResourceID:   stringPtr(role.ID),
			Action:       "create",
			Title:        "创建角色",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("管理员角色已创建"),
			Payload: mustJSONBytes(map[string]any{
				"name":            role.Name,
				"permissionCodes": role.Permissions,
			}),
		})
	}

	render.JSON(w, http.StatusCreated, role)
}

func ensureSuperAdminRemains(ctx context.Context, s *store.Store, existingAdmin *domain.AdminIdentity, update store.UpdateAdminUserInput) error {
	if existingAdmin == nil {
		return nil
	}

	hasSuperAdmin := containsString(existingAdmin.RoleIDs, appstate.SuperAdminRoleID)
	if !hasSuperAdmin {
		return nil
	}

	willRemainActive := existingAdmin.IsActive
	if update.IsActive != nil {
		willRemainActive = *update.IsActive
	}

	willKeepSuperAdmin := hasSuperAdmin
	if update.RoleIDsTouched {
		willKeepSuperAdmin = containsString(update.RoleIDs, appstate.SuperAdminRoleID)
	}

	if willRemainActive && willKeepSuperAdmin {
		return nil
	}

	count, err := s.CountActiveAdminsByRole(ctx, appstate.SuperAdminRoleID)
	if err != nil {
		return errors.New("failed to validate super admin guard")
	}
	if count <= 1 {
		return errors.New("at least one active super admin must remain")
	}
	return nil
}

func containsString(items []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, item := range items {
		if strings.TrimSpace(item) == target {
			return true
		}
	}
	return false
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isAdminInputError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "required") || strings.Contains(message, "invalid")
}

func isAdminUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
