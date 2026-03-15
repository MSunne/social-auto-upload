package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type DeviceHandler struct {
	app *appstate.App
}

type claimDeviceRequest struct {
	DeviceCode string `json:"deviceCode"`
}

type updateDeviceRequest struct {
	Name                  *string `json:"name"`
	DefaultReasoningModel *string `json:"defaultReasoningModel"`
	IsEnabled             *bool   `json:"isEnabled"`
}

func NewDeviceHandler(app *appstate.App) *DeviceHandler {
	return &DeviceHandler{app: app}
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	items, err := h.app.Store.ListDevicesByOwner(r.Context(), user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load devices")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *DeviceHandler) Detail(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	device, err := h.app.Store.GetOwnedDevice(r.Context(), deviceID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}

	render.JSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) Workspace(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	device, err := h.app.Store.GetOwnedDevice(r.Context(), deviceID, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}

	recentTasks, err := h.app.Store.ListPublishTasksByOwner(r.Context(), user.ID, store.ListPublishTasksFilter{
		DeviceID: deviceID,
		Limit:    8,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device tasks")
		return
	}

	activeLoginSessions, err := h.app.Store.ListActiveLoginSessionsByOwner(r.Context(), user.ID, deviceID, 6)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device login sessions")
		return
	}

	recentAccounts, err := h.app.Store.ListAccountsByOwner(r.Context(), user.ID, deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device accounts")
		return
	}
	if len(recentAccounts) > 8 {
		recentAccounts = recentAccounts[:8]
	}

	materialRoots, err := h.app.Store.ListMaterialRootsByOwner(r.Context(), user.ID, deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device material roots")
		return
	}

	skillSyncStates, err := h.app.Store.ListSkillSyncStatesByDevice(r.Context(), user.ID, deviceID, 12)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device skill sync states")
		return
	}

	render.JSON(w, http.StatusOK, domain.DeviceWorkspace{
		Device:              *device,
		RecentTasks:         recentTasks,
		ActiveLoginSessions: activeLoginSessions,
		RecentAccounts:      recentAccounts,
		MaterialRoots:       materialRoots,
		SkillSyncStates:     skillSyncStates,
	})
}

func (h *DeviceHandler) Claim(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())

	var payload claimDeviceRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	if payload.DeviceCode == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode is required")
		return
	}

	device, err := h.app.Store.ClaimDevice(r.Context(), payload.DeviceCode, user.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to claim device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device code not found or already claimed by another user")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "device",
		ResourceID:   &device.ID,
		Action:       "claim",
		Title:        "认领 OmniBull 设备",
		Source:       "devices",
		Status:       "success",
		Message:      auditStringPtr("设备已绑定到当前云端账户"),
		Payload: mustJSONBytes(map[string]any{
			"deviceCode": device.DeviceCode,
			"name":       device.Name,
		}),
	})

	render.JSON(w, http.StatusOK, device)
}

func (h *DeviceHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := httpcontext.CurrentUser(r.Context())
	deviceID := chi.URLParam(r, "deviceId")
	if strings.TrimSpace(deviceID) == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	var payload updateDeviceRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	device, err := h.app.Store.UpdateDevice(r.Context(), deviceID, user.ID, store.UpdateDeviceInput{
		Name:                  payload.Name,
		DefaultReasoningModel: payload.DefaultReasoningModel,
		IsEnabled:             payload.IsEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  user.ID,
		ResourceType: "device",
		ResourceID:   &device.ID,
		Action:       "update",
		Title:        "更新设备配置",
		Source:       "devices",
		Status:       "success",
		Message:      auditStringPtr("设备配置已更新"),
		Payload: mustJSONBytes(map[string]any{
			"name":                  payload.Name,
			"defaultReasoningModel": payload.DefaultReasoningModel,
			"isEnabled":             payload.IsEnabled,
		}),
	})

	render.JSON(w, http.StatusOK, device)
}
