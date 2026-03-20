package handlers

import (
	"net/http"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type PartnerHandler struct {
	app *appstate.App
}

func NewPartnerHandler(app *appstate.App) *PartnerHandler {
	return &PartnerHandler{app: app}
}

func (h *PartnerHandler) MyOverview(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	overview, err := h.app.Store.GetPartnerOverviewByUserID(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load partner overview")
		return
	}
	render.JSON(w, http.StatusOK, overview)
}

func (h *PartnerHandler) OpenMyProfile(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	profile, err := h.app.Store.OpenPartnerProfile(r.Context(), user.ID)
	if err != nil {
		switch err {
		case store.ErrPartnerProfileUserMiss:
			render.Error(w, http.StatusNotFound, "Partner user not found")
		default:
			render.Error(w, http.StatusInternalServerError, "Failed to open partner profile")
		}
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "partner_profile",
		ResourceID:   stringPtr(profile.UserID),
		Action:       "open",
		Title:        "开通企业合作",
		Source:       "omnidrive_cloud",
		Status:       "success",
		Message:      auditStringPtr("企业合作入口已开通"),
		Payload: mustJSONBytes(map[string]any{
			"partnerCode": profile.PartnerCode,
			"status":      profile.Status,
		}),
	})

	overview, err := h.app.Store.GetPartnerOverviewByUserID(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load partner overview")
		return
	}
	render.JSON(w, http.StatusOK, overview)
}
