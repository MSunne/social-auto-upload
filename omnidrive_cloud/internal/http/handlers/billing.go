package handlers

import (
	"net/http"

	appstate "omnidrive_cloud/internal/app"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
)

type BillingHandler struct {
	app *appstate.App
}

func NewBillingHandler(app *appstate.App) *BillingHandler {
	return &BillingHandler{app: app}
}

func (h *BillingHandler) ListPackages(w http.ResponseWriter, r *http.Request) {
	items, err := h.app.Store.ListBillingPackages(r.Context())
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing packages")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *BillingHandler) Ledger(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListWalletLedgerByUser(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load wallet ledger")
		return
	}
	render.JSON(w, http.StatusOK, items)
}
