package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type AgentHandler struct {
	app *appstate.App
}

type heartbeatRequest struct {
	DeviceCode     string      `json:"deviceCode"`
	DeviceName     string      `json:"deviceName"`
	AgentKey       string      `json:"agentKey"`
	LocalIP        *string     `json:"localIp"`
	PublicIP       *string     `json:"publicIp"`
	RuntimePayload interface{} `json:"runtimePayload"`
}

type loginEventRequest struct {
	Status              string      `json:"status"`
	Message             *string     `json:"message"`
	QRData              *string     `json:"qrData"`
	VerificationPayload interface{} `json:"verificationPayload"`
}

type syncPublishTaskRequest struct {
	ID                  string      `json:"id"`
	DeviceCode          string      `json:"deviceCode"`
	AccountID           *string     `json:"accountId"`
	SkillID             *string     `json:"skillId"`
	Platform            string      `json:"platform"`
	AccountName         string      `json:"accountName"`
	Title               string      `json:"title"`
	ContentText         *string     `json:"contentText"`
	MediaPayload        interface{} `json:"mediaPayload"`
	Status              string      `json:"status"`
	Message             *string     `json:"message"`
	VerificationPayload interface{} `json:"verificationPayload"`
}

func NewAgentHandler(app *appstate.App) *AgentHandler {
	return &AgentHandler{app: app}
}

func (h *AgentHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var payload heartbeatRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.DeviceName = strings.TrimSpace(payload.DeviceName)
	payload.AgentKey = strings.TrimSpace(payload.AgentKey)
	if payload.DeviceCode == "" || payload.DeviceName == "" || payload.AgentKey == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode, deviceName, and agentKey are required")
		return
	}

	existing, err := h.app.Store.GetDeviceByCode(r.Context(), payload.DeviceCode)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to check device")
		return
	}
	if existing != nil && existing.AgentKey != "" && existing.AgentKey != payload.AgentKey {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	var runtimePayload []byte
	if payload.RuntimePayload != nil {
		runtimePayload, err = json.Marshal(payload.RuntimePayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "runtimePayload must be valid json")
			return
		}
	}

	device, err := h.app.Store.UpsertHeartbeatDevice(r.Context(), store.HeartbeatInput{
		DeviceCode:     payload.DeviceCode,
		AgentKey:       payload.AgentKey,
		DeviceName:     payload.DeviceName,
		LocalIP:        payload.LocalIP,
		PublicIP:       payload.PublicIP,
		RuntimePayload: runtimePayload,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update heartbeat")
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"device": device,
	})
}

func (h *AgentHandler) ListLoginTasks(w http.ResponseWriter, r *http.Request) {
	deviceCode := strings.TrimSpace(chi.URLParam(r, "deviceCode"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if deviceCode == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode and X-Agent-Key are required")
		return
	}

	device, err := h.app.Store.GetDeviceByCode(r.Context(), deviceCode)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	if !agentKeyMatches(device, agentKey) {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	items, err := h.app.Store.ListPendingLoginTasksByDevice(r.Context(), device.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AgentHandler) PushLoginEvent(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if sessionID == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "sessionId and X-Agent-Key are required")
		return
	}

	session, err := h.app.Store.GetLoginSessionByID(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}

	device, err := h.app.Store.GetDeviceByID(r.Context(), session.DeviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil || !agentKeyMatches(device, agentKey) {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	var payload loginEventRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.Status == "" {
		render.Error(w, http.StatusBadRequest, "status is required")
		return
	}

	var verificationPayload []byte
	if payload.VerificationPayload != nil {
		verificationPayload, err = h.prepareVerificationPayload(r.Context(), "login-sessions", sessionID, payload.VerificationPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	updatedSession, err := h.app.Store.UpdateLoginSessionEvent(r.Context(), sessionID, store.LoginEventInput{
		Status:              payload.Status,
		Message:             payload.Message,
		QRData:              payload.QRData,
		VerificationPayload: verificationPayload,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update login session")
		return
	}
	if updatedSession == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}
	if err := h.app.Store.UpsertPlatformAccountFromLogin(r.Context(), updatedSession); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to mirror platform account")
		return
	}

	render.JSON(w, http.StatusOK, updatedSession)
}

func (h *AgentHandler) ListLoginActions(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if sessionID == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "sessionId and X-Agent-Key are required")
		return
	}

	session, err := h.app.Store.GetLoginSessionByID(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}

	device, err := h.app.Store.GetDeviceByID(r.Context(), session.DeviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil || !agentKeyMatches(device, agentKey) {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	actions, err := h.app.Store.ConsumePendingLoginActions(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login actions")
		return
	}
	render.JSON(w, http.StatusOK, actions)
}

func (h *AgentHandler) ListPublishTasks(w http.ResponseWriter, r *http.Request) {
	deviceCode := strings.TrimSpace(chi.URLParam(r, "deviceCode"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if deviceCode == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode and X-Agent-Key are required")
		return
	}

	device, err := h.app.Store.GetDeviceByCode(r.Context(), deviceCode)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	if !agentKeyMatches(device, agentKey) {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	items, err := h.app.Store.ListPendingPublishTasksByDevice(r.Context(), device.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AgentHandler) SyncPublishTask(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncPublishTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.ID = strings.TrimSpace(payload.ID)
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.ID == "" || payload.DeviceCode == "" || payload.Platform == "" || payload.AccountName == "" || payload.Title == "" || payload.Status == "" {
		render.Error(w, http.StatusBadRequest, "id, deviceCode, platform, accountName, title, and status are required")
		return
	}

	device, err := h.app.Store.GetDeviceByCode(r.Context(), payload.DeviceCode)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if device == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	if !agentKeyMatches(device, agentKey) {
		render.Error(w, http.StatusForbidden, "Agent key mismatch")
		return
	}

	var mediaPayload []byte
	if payload.MediaPayload != nil {
		mediaPayload, err = json.Marshal(payload.MediaPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "mediaPayload must be valid json")
			return
		}
	}
	var verificationPayload []byte
	if payload.VerificationPayload != nil {
		verificationPayload, err = h.prepareVerificationPayload(r.Context(), "publish-verify", payload.ID, payload.VerificationPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	task, err := h.app.Store.SyncPublishTask(r.Context(), store.SyncPublishTaskInput{
		ID:                  payload.ID,
		DeviceID:            device.ID,
		AccountID:           payload.AccountID,
		SkillID:             payload.SkillID,
		Platform:            payload.Platform,
		AccountName:         payload.AccountName,
		Title:               payload.Title,
		ContentText:         payload.ContentText,
		MediaPayload:        mediaPayload,
		Status:              payload.Status,
		Message:             payload.Message,
		VerificationPayload: verificationPayload,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync publish task")
		return
	}

	render.JSON(w, http.StatusOK, task)
}

func agentKeyMatches(device interface{ GetAgentKey() string }, provided string) bool {
	return provided != "" && device.GetAgentKey() == provided
}

func (h *AgentHandler) prepareVerificationPayload(ctx context.Context, folder string, entityID string, payload interface{}) ([]byte, error) {
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		return json.Marshal(payload)
	}

	rawScreenshot, ok := payloadMap["screenshotData"].(string)
	if !ok || strings.TrimSpace(rawScreenshot) == "" {
		return json.Marshal(payloadMap)
	}

	data, contentType, err := decodeBase64Payload(rawScreenshot)
	if err != nil {
		return nil, fmt.Errorf("verificationPayload screenshotData is invalid")
	}

	ext := extensionFromContentType(contentType)
	object, err := h.app.Storage.SaveBytes(
		ctx,
		fmt.Sprintf("%s/%s/%s%s", folder, entityID, uuid.NewString(), ext),
		contentType,
		data,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store verification screenshot")
	}

	delete(payloadMap, "screenshotData")
	payloadMap["screenshotUrl"] = object.PublicURL
	payloadMap["screenshotStorageKey"] = object.StorageKey
	payloadMap["screenshotContentType"] = object.ContentType
	payloadMap["screenshotSizeBytes"] = object.SizeBytes
	return json.Marshal(payloadMap)
}

func decodeBase64Payload(raw string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "data:") {
		pieces := strings.SplitN(raw, ",", 2)
		if len(pieces) != 2 {
			return nil, "", fmt.Errorf("invalid data url")
		}
		meta := pieces[0]
		body := pieces[1]
		contentType := "image/png"
		if strings.HasPrefix(meta, "data:") {
			meta = strings.TrimPrefix(meta, "data:")
			metaParts := strings.Split(meta, ";")
			if len(metaParts) > 0 && strings.TrimSpace(metaParts[0]) != "" {
				contentType = strings.TrimSpace(metaParts[0])
			}
		}
		data, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			return nil, "", err
		}
		return data, contentType, nil
	}

	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", err
	}
	return data, "image/png", nil
}

func extensionFromContentType(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/png":
		return ".png"
	default:
		ext := filepath.Ext(contentType)
		if ext != "" {
			return ext
		}
		return ".bin"
	}
}
