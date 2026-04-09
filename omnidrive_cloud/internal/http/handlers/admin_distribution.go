package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminDistributionHandler struct {
	app *appstate.App
}

type createDistributionRelationRequest struct {
	PromoterUserID     string  `json:"promoterUserId"`
	InviteeUserID      string  `json:"inviteeUserId"`
	PromoterEmail      string  `json:"promoterEmail"`
	InviteeEmail       string  `json:"inviteeEmail"`
	PromoterIdentifier string  `json:"promoterIdentifier"`
	InviteeIdentifier  string  `json:"inviteeIdentifier"`
	Notes              *string `json:"notes"`
}

type openPartnerProfileRequest struct {
	UserID         string `json:"userId"`
	Email          string `json:"email"`
	UserIdentifier string `json:"userIdentifier"`
}

type updatePartnerProfileRequest struct {
	Status string `json:"status"`
}

type createDistributionRuleRequest struct {
	Name                     string  `json:"name"`
	PromoterUserID           *string `json:"promoterUserId"`
	Status                   string  `json:"status"`
	CommissionRate           float64 `json:"commissionRate"`
	SettlementThresholdCents int64   `json:"settlementThresholdCents"`
	Notes                    *string `json:"notes"`
}

type createDistributionSettlementRequest struct {
	PromoterUserID *string `json:"promoterUserId"`
	Note           *string `json:"note"`
}

type updateDistributionRelationRequest struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

// 创建管理端分销Handler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewAdminDistributionHandler(app *appstate.App) *AdminDistributionHandler {
	return &AdminDistributionHandler{app: app}
}

// 处理管理端分销列表分销伙伴接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListPartners(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminPartnerProfiles(r.Context(), store.AdminPartnerProfileListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load partner profiles")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"active", "inactive"},
	})
}

// 处理管理端分销开启分销伙伴接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) OpenPartner(w http.ResponseWriter, r *http.Request) {
	var payload openPartnerProfileRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	userID, err := h.resolveDistributionUserID(r.Context(), payload.UserID, payload.UserIdentifier, payload.Email)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resolve partner user")
		return
	}
	if userID == "" {
		render.Error(w, http.StatusBadRequest, "userId is required")
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	profile, err := h.app.Store.OpenPartnerProfile(r.Context(), userID)
	if err != nil {
		switch err {
		case store.ErrPartnerProfileUserMiss:
			render.Error(w, http.StatusNotFound, "Partner user not found")
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to open partner profile")
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "partner_profile",
		ResourceID:   stringPtr(profile.UserID),
		Action:       "open",
		Title:        "代用户开通企业合作",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("企业合作伙伴档案已开通"),
		Payload: mustJSONBytes(map[string]any{
			"userId":      profile.UserID,
			"partnerCode": profile.PartnerCode,
			"status":      profile.Status,
		}),
	})

	render.JSON(w, http.StatusCreated, profile)
}

// 处理管理端分销更新分销伙伴接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) UpdatePartner(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userId"))
	if userID == "" {
		render.Error(w, http.StatusBadRequest, "userId is required")
		return
	}

	var payload updatePartnerProfileRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	profile, err := h.app.Store.SetPartnerProfileStatus(r.Context(), userID, payload.Status)
	if err != nil {
		switch err {
		case store.ErrPartnerProfileUserMiss:
			render.Error(w, http.StatusNotFound, "Partner profile not found")
		case store.ErrPartnerProfileStatusInvalid:
			render.Error(w, http.StatusBadRequest, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to update partner profile")
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "partner_profile",
		ResourceID:   stringPtr(profile.UserID),
		Action:       "status_update",
		Title:        "更新分销员资格",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分销员档案状态已更新"),
		Payload: mustJSONBytes(map[string]any{
			"userId":      profile.UserID,
			"partnerCode": profile.PartnerCode,
			"status":      profile.Status,
		}),
	})

	render.JSON(w, http.StatusOK, profile)
}

// 处理管理端分销列表Relations接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListRelations(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminDistributionRelations(r.Context(), store.AdminDistributionRelationListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load distribution relations")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"active", "inactive"},
	})
}

// 处理管理端分销创建Relation接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) CreateRelation(w http.ResponseWriter, r *http.Request) {
	var payload createDistributionRelationRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	promoterUserID, err := h.resolveDistributionUserID(r.Context(), payload.PromoterUserID, payload.PromoterIdentifier, payload.PromoterEmail)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resolve promoter user")
		return
	}
	inviteeUserID, err := h.resolveDistributionUserID(r.Context(), payload.InviteeUserID, payload.InviteeIdentifier, payload.InviteeEmail)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resolve invitee user")
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.CreateDistributionRelation(r.Context(), store.CreateDistributionRelationInput{
		PromoterUserID:   promoterUserID,
		InviteeUserID:    inviteeUserID,
		Notes:            trimmedStringPtr(valueOrEmpty(payload.Notes)),
		CreatedByAdminID: stringPtr(admin.ID),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrDistributionRelationUserNotFound):
			render.Error(w, http.StatusNotFound, "Distribution relation user not found")
		case errors.Is(err, store.ErrDistributionRelationSelfInvite):
			render.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, store.ErrDistributionRelationInviteeBound):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "distribution_relation",
		ResourceID:   stringPtr(record.ID),
		Action:       "create",
		Title:        "创建分销关系",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分销关系已创建"),
		Payload: mustJSONBytes(map[string]any{
			"promoterUserId": record.Promoter.ID,
			"inviteeUserId":  record.Invitee.ID,
			"status":         record.Status,
		}),
	})

	render.JSON(w, http.StatusCreated, record)
}

// 处理管理端分销更新Relation接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) UpdateRelation(w http.ResponseWriter, r *http.Request) {
	relationID := strings.TrimSpace(chi.URLParam(r, "relationId"))
	if relationID == "" {
		render.Error(w, http.StatusBadRequest, "relationId is required")
		return
	}

	var payload updateDistributionRelationRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.UpdateDistributionRelation(r.Context(), store.UpdateDistributionRelationInput{
		RelationID: relationID,
		Status:     payload.Status,
		Notes:      payload.Notes,
	})
	if err != nil {
		switch err {
		case store.ErrDistributionRelationNotFound:
			render.Error(w, http.StatusNotFound, "Distribution relation not found")
		case store.ErrDistributionRelationStatusInvalid:
			render.Error(w, http.StatusBadRequest, err.Error())
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to update distribution relation")
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "distribution_relation",
		ResourceID:   stringPtr(record.ID),
		Action:       "status_update",
		Title:        "更新分销关系",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分销关系状态已更新"),
		Payload: mustJSONBytes(map[string]any{
			"relationId":     record.ID,
			"promoterUserId": record.Promoter.ID,
			"inviteeUserId":  record.Invitee.ID,
			"status":         record.Status,
		}),
	})

	render.JSON(w, http.StatusOK, record)
}

// 处理管理端分销列表规则接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListAdminDistributionRules(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load distribution rules")
		return
	}

	activeCount := 0
	for _, item := range items {
		if item.Status == "active" {
			activeCount++
		}
	}
	page := adminPageQuery{Page: 1, PageSize: max(1, len(items))}
	renderAdminList(w, page, int64(len(items)), items, map[string]any{
		"activeCount":   activeCount,
		"inactiveCount": len(items) - activeCount,
	}, nil)
}

// 处理管理端分销创建规则接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var payload createDistributionRuleRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.CreateDistributionRule(r.Context(), store.CreateDistributionRuleInput{
		Name:                     strings.TrimSpace(payload.Name),
		PromoterUserID:           trimmedStringPtr(valueOrEmpty(payload.PromoterUserID)),
		Status:                   strings.TrimSpace(payload.Status),
		CommissionRate:           payload.CommissionRate,
		SettlementThresholdCents: payload.SettlementThresholdCents,
		Notes:                    trimmedStringPtr(valueOrEmpty(payload.Notes)),
		CreatedByAdminID:         stringPtr(admin.ID),
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrDistributionRuleInvalidRate):
			render.Error(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, store.ErrDistributionRelationUserNotFound):
			render.Error(w, http.StatusNotFound, "Promoter user not found")
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "distribution_rule",
		ResourceID:   stringPtr(record.ID),
		Action:       "create",
		Title:        "创建分销规则",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分销规则已创建"),
		Payload: mustJSONBytes(map[string]any{
			"scope":                    record.Scope,
			"status":                   record.Status,
			"commissionRate":           record.CommissionRate,
			"settlementThresholdCents": record.SettlementThresholdCents,
			"promoterUserId":           valueOrEmpty(payload.PromoterUserID),
		}),
	})

	render.JSON(w, http.StatusCreated, record)
}

// 处理管理端分销列表Commissions接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListCommissions(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminCommissions(r.Context(), store.AdminCommissionListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load distribution commissions")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"pending_consume", "pending_settlement", "settled"},
	})
}

// 处理管理端分销列表CommissionReleases接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListCommissionReleases(w http.ResponseWriter, r *http.Request) {
	commissionID := strings.TrimSpace(chi.URLParam(r, "commissionId"))
	if commissionID == "" {
		render.Error(w, http.StatusBadRequest, "commissionId is required")
		return
	}

	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	items, err := h.app.Store.ListAdminCommissionReleaseEvents(r.Context(), commissionID, limit)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load commission release events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

// 处理管理端分销列表Settlements接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) ListSettlements(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminSettlements(r.Context(), store.AdminSettlementListFilter{
		Query:  strings.TrimSpace(r.URL.Query().Get("query")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load distribution settlements")
		return
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":         strings.TrimSpace(r.URL.Query().Get("query")),
		"status":        strings.TrimSpace(r.URL.Query().Get("status")),
		"statusOptions": []string{"pending", "completed"},
	})
}

// 处理管理端分销创建Settlement接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) CreateSettlement(w http.ResponseWriter, r *http.Request) {
	var payload createDistributionSettlementRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.CreateDistributionSettlementBatch(r.Context(), store.CreateDistributionSettlementInput{
		PromoterUserID: trimmedStringPtr(valueOrEmpty(payload.PromoterUserID)),
		Note:           trimmedStringPtr(valueOrEmpty(payload.Note)),
		AdminID:        admin.ID,
		AdminEmail:     admin.Email,
		AdminName:      admin.Name,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrDistributionSettlementPromoterMiss):
			render.Error(w, http.StatusNotFound, "Settlement promoter not found")
		case errors.Is(err, store.ErrDistributionSettlementNoEligible):
			render.Error(w, http.StatusConflict, err.Error())
		default:
			render.Error(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: "distribution_settlement_batch",
		ResourceID:   stringPtr(record.ID),
		Action:       "create",
		Title:        "发起分销结算批次",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("分销结算批次已创建"),
		Payload: mustJSONBytes(map[string]any{
			"batchNo":          record.BatchNo,
			"status":           record.Status,
			"itemCount":        record.ItemCount,
			"totalAmountCents": record.TotalAmountCents,
			"promoterUserId":   valueOrEmpty(payload.PromoterUserID),
		}),
	})

	render.JSON(w, http.StatusCreated, record)
}

// 处理管理端分销解析分销用户ID接口，解析请求参数并调用应用状态或存储层完成业务动作。
func (h *AdminDistributionHandler) resolveDistributionUserID(ctx context.Context, candidates ...string) (string, error) {
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}
		user, err := h.app.Store.GetUserByID(ctx, trimmed)
		if err != nil {
			return "", err
		}
		if user != nil {
			return user.ID, nil
		}
		byEmail, err := h.app.Store.GetUserByEmail(ctx, trimmed)
		if err != nil {
			return "", err
		}
		if byEmail != nil {
			return byEmail.User.ID, nil
		}
		byPhone, err := h.app.Store.GetUserByPhone(ctx, trimmed)
		if err != nil {
			return "", err
		}
		if byPhone != nil {
			return byPhone.User.ID, nil
		}
	}
	return "", nil
}
