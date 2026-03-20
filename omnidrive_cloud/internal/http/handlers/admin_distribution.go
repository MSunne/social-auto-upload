package handlers

import (
	"errors"
	"net/http"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AdminDistributionHandler struct {
	app *appstate.App
}

type createDistributionRelationRequest struct {
	PromoterUserID string  `json:"promoterUserId"`
	InviteeUserID  string  `json:"inviteeUserId"`
	Notes          *string `json:"notes"`
}

type openPartnerProfileRequest struct {
	UserID string `json:"userId"`
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

func NewAdminDistributionHandler(app *appstate.App) *AdminDistributionHandler {
	return &AdminDistributionHandler{app: app}
}

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

func (h *AdminDistributionHandler) OpenPartner(w http.ResponseWriter, r *http.Request) {
	var payload openPartnerProfileRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := strings.TrimSpace(payload.UserID)
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

func (h *AdminDistributionHandler) CreateRelation(w http.ResponseWriter, r *http.Request) {
	var payload createDistributionRelationRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	admin := httpcontext.CurrentAdmin(r.Context())
	record, err := h.app.Store.CreateDistributionRelation(r.Context(), store.CreateDistributionRelationInput{
		PromoterUserID:   strings.TrimSpace(payload.PromoterUserID),
		InviteeUserID:    strings.TrimSpace(payload.InviteeUserID),
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
