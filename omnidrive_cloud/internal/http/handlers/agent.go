package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
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
	ID                  string                           `json:"id"`
	DeviceCode          string                           `json:"deviceCode"`
	AccountID           *string                          `json:"accountId"`
	SkillID             *string                          `json:"skillId"`
	Platform            string                           `json:"platform"`
	AccountName         string                           `json:"accountName"`
	Title               string                           `json:"title"`
	ContentText         *string                          `json:"contentText"`
	MediaPayload        interface{}                      `json:"mediaPayload"`
	Status              string                           `json:"status"`
	Message             *string                          `json:"message"`
	ExecutionPayload    interface{}                      `json:"executionPayload"`
	VerificationPayload interface{}                      `json:"verificationPayload"`
	Artifacts           []syncPublishTaskArtifactRequest `json:"artifacts"`
	LeaseToken          *string                          `json:"leaseToken"`
}

type syncPublishTaskArtifactRequest struct {
	ArtifactKey  string      `json:"artifactKey"`
	ArtifactType string      `json:"artifactType"`
	Source       string      `json:"source"`
	Title        *string     `json:"title"`
	FileName     *string     `json:"fileName"`
	MimeType     *string     `json:"mimeType"`
	TextContent  *string     `json:"textContent"`
	Payload      interface{} `json:"payload"`
	Data         *string     `json:"data"`
	Base64Data   *string     `json:"base64Data"`
}

type syncAccountRequest struct {
	DeviceCode          string  `json:"deviceCode"`
	Platform            string  `json:"platform"`
	AccountName         string  `json:"accountName"`
	Status              string  `json:"status"`
	LastMessage         *string `json:"lastMessage"`
	LastAuthenticatedAt *string `json:"lastAuthenticatedAt"`
}

type claimPublishTaskRequest struct {
	DeviceCode string `json:"deviceCode"`
}

type renewPublishTaskLeaseRequest struct {
	DeviceCode string `json:"deviceCode"`
	LeaseToken string `json:"leaseToken"`
}

type releasePublishTaskLeaseRequest struct {
	DeviceCode string  `json:"deviceCode"`
	LeaseToken string  `json:"leaseToken"`
	Message    *string `json:"message"`
}

type syncSkillStateRequest struct {
	DeviceCode string `json:"deviceCode"`
	Items      []struct {
		SkillID        string  `json:"skillId"`
		SyncStatus     string  `json:"syncStatus"`
		SyncedRevision *string `json:"syncedRevision"`
		AssetCount     *int64  `json:"assetCount"`
		Message        *string `json:"message"`
		LastSyncedAt   *string `json:"lastSyncedAt"`
	} `json:"items"`
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

func (h *AgentHandler) SyncAccount(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncAccountRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	payload.Status = strings.TrimSpace(payload.Status)
	if payload.DeviceCode == "" || payload.Platform == "" || payload.AccountName == "" || payload.Status == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode, platform, accountName, and status are required")
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

	var lastAuthenticatedAt *time.Time
	if payload.LastAuthenticatedAt != nil && strings.TrimSpace(*payload.LastAuthenticatedAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*payload.LastAuthenticatedAt))
		if parseErr != nil {
			render.Error(w, http.StatusBadRequest, "lastAuthenticatedAt must be RFC3339")
			return
		}
		lastAuthenticatedAt = &parsed
	}

	account, err := h.app.Store.UpsertPlatformAccount(
		r.Context(),
		device.ID,
		payload.Platform,
		payload.AccountName,
		payload.Status,
		payload.LastMessage,
		lastAuthenticatedAt,
	)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync account")
		return
	}

	render.JSON(w, http.StatusOK, account)
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}

	h.recordRecoveredPublishTasks(r.Context(), device)

	items, err := h.app.Store.ListPendingPublishTasksByDevice(r.Context(), device.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish tasks")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AgentHandler) ListSkills(w http.ResponseWriter, r *http.Request) {
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}
	if device.OwnerUserID == nil || strings.TrimSpace(*device.OwnerUserID) == "" {
		render.JSON(w, http.StatusOK, map[string]any{
			"items": []domain.AgentSkillPackage{},
		})
		return
	}

	var since *time.Time
	if rawSince := strings.TrimSpace(r.URL.Query().Get("since")); rawSince != "" {
		parsed, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "since must be RFC3339")
			return
		}
		since = &parsed
	}

	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		var parsed int
		if _, err := fmt.Sscanf(rawLimit, "%d", &parsed); err != nil || parsed < 0 {
			render.Error(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	skills, err := h.app.Store.ListEnabledSkillsByOwner(r.Context(), *device.OwnerUserID, since, limit)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load skills")
		return
	}

	items := make([]domain.AgentSkillPackage, 0, len(skills))
	for _, skill := range skills {
		assets, err := h.app.Store.ListSkillAssets(r.Context(), skill.ID, *device.OwnerUserID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load skill assets")
			return
		}
		syncState, err := h.app.Store.GetDeviceSkillSyncState(r.Context(), device.ID, skill.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load skill sync state")
			return
		}
		items = append(items, domain.AgentSkillPackage{
			Revision: buildSkillRevision(&skill),
			Skill:    skill,
			Assets:   assets,
			Sync:     syncState,
		})
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"serverTime": time.Now().UTC(),
	})
}

func (h *AgentHandler) SyncSkillStates(w http.ResponseWriter, r *http.Request) {
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if agentKey == "" {
		render.Error(w, http.StatusBadRequest, "X-Agent-Key is required")
		return
	}

	var payload syncSkillStateRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	if payload.DeviceCode == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode is required")
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
	if device.OwnerUserID == nil || strings.TrimSpace(*device.OwnerUserID) == "" {
		render.Error(w, http.StatusConflict, "Device is not claimed")
		return
	}

	results := make([]domain.DeviceSkillSyncState, 0, len(payload.Items))
	for _, item := range payload.Items {
		skillID := strings.TrimSpace(item.SkillID)
		syncStatus := strings.TrimSpace(item.SyncStatus)
		if skillID == "" || syncStatus == "" {
			continue
		}
		skill, err := h.app.Store.GetOwnedSkillByID(r.Context(), skillID, *device.OwnerUserID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to validate skill sync payload")
			return
		}
		if skill == nil {
			continue
		}
		var lastSyncedAt *time.Time
		if item.LastSyncedAt != nil && strings.TrimSpace(*item.LastSyncedAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*item.LastSyncedAt))
			if err != nil {
				render.Error(w, http.StatusBadRequest, "lastSyncedAt must be RFC3339")
				return
			}
			lastSyncedAt = &parsed
		}
		syncedRevision := normalizeTrimmedString(item.SyncedRevision)
		message := normalizeTrimmedString(item.Message)
		assetCount := int64(0)
		if item.AssetCount != nil {
			assetCount = *item.AssetCount
		}
		state, err := h.app.Store.UpsertDeviceSkillSyncState(r.Context(), store.UpsertDeviceSkillSyncStateInput{
			DeviceID:       device.ID,
			SkillID:        skillID,
			SyncStatus:     syncStatus,
			SyncedRevision: syncedRevision,
			AssetCount:     assetCount,
			Message:        message,
			LastSyncedAt:   lastSyncedAt,
		})
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to sync skill states")
			return
		}
		results = append(results, *state)
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"items": results,
	})
}

func (h *AgentHandler) PublishTaskPackage(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	deviceCode := strings.TrimSpace(r.URL.Query().Get("deviceCode"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if taskID == "" || deviceCode == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "taskId, deviceCode, and X-Agent-Key are required")
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}

	task, err := h.app.Store.GetPublishTaskByID(r.Context(), taskID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if task == nil || task.DeviceID != device.ID {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}

	var account *domain.PlatformAccount
	if task.AccountID != nil && strings.TrimSpace(*task.AccountID) != "" && device.OwnerUserID != nil {
		account, err = h.app.Store.GetOwnedAccountByID(r.Context(), strings.TrimSpace(*task.AccountID), *device.OwnerUserID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task account")
			return
		}
	}
	if account == nil {
		account, err = h.app.Store.GetAccountByDeviceTarget(r.Context(), device.ID, task.Platform, task.AccountName)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task account")
			return
		}
	}

	var skill *domain.ProductSkill
	skillAssets := make([]domain.ProductSkillAsset, 0)
	if task.SkillID != nil && strings.TrimSpace(*task.SkillID) != "" && device.OwnerUserID != nil {
		skill, err = h.app.Store.GetOwnedSkillByID(r.Context(), strings.TrimSpace(*task.SkillID), *device.OwnerUserID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task skill")
			return
		}
		if skill != nil {
			skillAssets, err = h.app.Store.ListSkillAssets(r.Context(), skill.ID, *device.OwnerUserID)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to load task skill assets")
				return
			}
		}
	}

	materials, err := h.app.Store.ListPublishTaskMaterialRefsByTaskID(r.Context(), task.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task materials")
		return
	}
	runtimeState, err := h.app.Store.GetPublishTaskRuntimeStateByTaskID(r.Context(), task.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task runtime state")
		return
	}
	readiness := buildPublishTaskReadiness(r.Context(), h.app, task, device, account, skill)

	render.JSON(w, http.StatusOK, domain.AgentPublishTaskPackage{
		Task:        *task,
		Account:     account,
		Skill:       skill,
		SkillAssets: skillAssets,
		Materials:   materials,
		Readiness:   readiness,
		Runtime:     runtimeState,
	})
}

func (h *AgentHandler) ClaimPublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if taskID == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "taskId and X-Agent-Key are required")
		return
	}

	var payload claimPublishTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	if payload.DeviceCode == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode is required")
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
	if !device.IsEnabled {
		render.Error(w, http.StatusConflict, "Device is disabled")
		return
	}

	h.recordRecoveredPublishTasks(r.Context(), device)

	leaseToken := uuid.NewString()
	leaseExpiresAt := time.Now().Add(store.PublishTaskLeaseTTL())
	task, err := h.app.Store.ClaimPublishTaskLease(r.Context(), taskID, device.ID, leaseToken, leaseExpiresAt)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to claim publish task")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Publish task is not claimable")
		return
	}

	if device.OwnerUserID != nil {
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  *device.OwnerUserID,
			ResourceType: "publish_task",
			ResourceID:   &task.ID,
			Action:       "claim",
			Title:        "本地设备认领发布任务",
			Source:       task.Platform,
			Status:       task.Status,
			Message:      auditStringPtr("任务已被本地执行器认领"),
			Payload: mustJSONBytes(map[string]any{
				"deviceId":       device.ID,
				"leaseExpiresAt": leaseExpiresAt,
			}),
		})
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: "claimed",
		Source:    "agent",
		Status:    task.Status,
		Message:   auditStringPtr("任务已被本地执行器认领"),
		Payload: mustJSONBytes(map[string]any{
			"deviceId":       device.ID,
			"leaseExpiresAt": leaseExpiresAt,
		}),
	})

	render.JSON(w, http.StatusOK, map[string]any{
		"task":           task,
		"leaseToken":     leaseToken,
		"leaseExpiresAt": leaseExpiresAt.UTC(),
	})
}

func (h *AgentHandler) RenewPublishTaskLease(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if taskID == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "taskId and X-Agent-Key are required")
		return
	}

	var payload renewPublishTaskLeaseRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.LeaseToken = strings.TrimSpace(payload.LeaseToken)
	if payload.DeviceCode == "" || payload.LeaseToken == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode and leaseToken are required")
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

	leaseExpiresAt := time.Now().Add(store.PublishTaskLeaseTTL())
	task, err := h.app.Store.RenewPublishTaskLease(r.Context(), taskID, device.ID, payload.LeaseToken, leaseExpiresAt)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to renew publish task lease")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Publish task lease is not renewable")
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{
		"task":           task,
		"leaseToken":     payload.LeaseToken,
		"leaseExpiresAt": leaseExpiresAt.UTC(),
	})
}

func (h *AgentHandler) ReleasePublishTaskLease(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	agentKey := strings.TrimSpace(r.Header.Get("X-Agent-Key"))
	if taskID == "" || agentKey == "" {
		render.Error(w, http.StatusBadRequest, "taskId and X-Agent-Key are required")
		return
	}

	var payload releasePublishTaskLeaseRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.DeviceCode = strings.TrimSpace(payload.DeviceCode)
	payload.LeaseToken = strings.TrimSpace(payload.LeaseToken)
	payload.Message = normalizeTrimmedString(payload.Message)
	if payload.DeviceCode == "" || payload.LeaseToken == "" {
		render.Error(w, http.StatusBadRequest, "deviceCode and leaseToken are required")
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

	task, err := h.app.Store.ReleasePublishTaskLeaseByAgent(r.Context(), taskID, device.ID, payload.LeaseToken, payload.Message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to release publish task lease")
		return
	}
	if task == nil {
		render.Error(w, http.StatusConflict, "Publish task lease is not releasable")
		return
	}
	if err := h.app.Store.DeletePublishTaskRuntimeState(r.Context(), task.ID); err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to clear task runtime state")
		return
	}

	eventType := "released"
	defaultMessage := "本地执行器已释放任务租约并重新排队"
	if task.Status == "cancelled" {
		eventType = "cancelled"
		defaultMessage = "本地执行器已确认取消任务"
	}
	message := payload.Message
	if message == nil {
		message = &defaultMessage
	}
	_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
		ID:        uuid.NewString(),
		TaskID:    task.ID,
		EventType: eventType,
		Source:    "agent",
		Status:    task.Status,
		Message:   message,
		Payload: mustJSONBytes(map[string]any{
			"deviceId": device.ID,
		}),
	})
	if device.OwnerUserID != nil {
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  *device.OwnerUserID,
			ResourceType: "publish_task",
			ResourceID:   &task.ID,
			Action:       "agent_release",
			Title:        "本地设备释放任务租约",
			Source:       task.Platform,
			Status:       task.Status,
			Message:      message,
			Payload: mustJSONBytes(map[string]any{
				"deviceId": device.ID,
			}),
		})
	}

	render.JSON(w, http.StatusOK, task)
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
	if payload.LeaseToken != nil {
		trimmed := strings.TrimSpace(*payload.LeaseToken)
		payload.LeaseToken = &trimmed
		if trimmed == "" {
			payload.LeaseToken = nil
		}
	}
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

	existingTask, err := h.app.Store.GetPublishTaskByID(r.Context(), payload.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect existing publish task")
		return
	}
	if existingTask != nil && existingTask.DeviceID != device.ID {
		render.Error(w, http.StatusConflict, "Publish task belongs to a different device")
		return
	}
	if existingTask != nil && !isAllowedAgentPublishTaskTransition(existingTask.Status, payload.Status) {
		render.Error(w, http.StatusConflict, "Publish task status transition is not allowed")
		return
	}
	if existingTask != nil && existingTask.LeaseOwnerDeviceID != nil && existingTask.LeaseToken != nil {
		if *existingTask.LeaseOwnerDeviceID == device.ID && existingTask.LeaseExpiresAt != nil && existingTask.LeaseExpiresAt.After(time.Now().UTC()) {
			if payload.LeaseToken == nil || *payload.LeaseToken != *existingTask.LeaseToken {
				render.Error(w, http.StatusConflict, "Publish task lease token mismatch")
				return
			}
		}
	}

	var mediaPayload []byte
	if payload.MediaPayload != nil {
		mediaPayload, err = json.Marshal(payload.MediaPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "mediaPayload must be valid json")
			return
		}
	}
	var executionPayload []byte
	executionTouched := payload.ExecutionPayload != nil
	if executionTouched {
		executionPayload, err = json.Marshal(payload.ExecutionPayload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "executionPayload must be valid json")
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
		LeaseToken:          payload.LeaseToken,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync publish task")
		return
	}

	existingArtifacts, err := h.app.Store.ListPublishTaskArtifactsByTaskID(r.Context(), task.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to inspect existing publish task artifacts")
		return
	}
	artifactInputs, err := h.preparePublishTaskArtifacts(r.Context(), task.ID, verificationPayload, payload.Artifacts)
	if err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(artifactInputs) > 0 {
		updatedArtifacts, err := h.app.Store.UpsertPublishTaskArtifacts(r.Context(), artifactInputs)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to sync publish task artifacts")
			return
		}
		cleanupReplacedArtifactFiles(h.app, r.Context(), existingArtifacts, updatedArtifacts)
	}
	lastAgentSyncAt := time.Now().UTC()
	runtimeState, err := h.app.Store.UpsertPublishTaskRuntimeState(r.Context(), store.UpsertPublishTaskRuntimeStateInput{
		TaskID:           task.ID,
		ExecutionPayload: executionPayload,
		ExecutionTouched: executionTouched,
		LastAgentSyncAt:  &lastAgentSyncAt,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to sync publish task runtime state")
		return
	}
	if task.Status == "pending" || task.Status == "cancelled" || task.Status == "failed" || task.Status == "success" || task.Status == "completed" {
		if err := h.app.Store.DeletePublishTaskRuntimeState(r.Context(), task.ID); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to clear publish task runtime state")
			return
		}
		runtimeState = nil
	}
	if device.OwnerUserID != nil {
		_, _ = h.app.Store.CreatePublishTaskEvent(r.Context(), store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    task.ID,
			EventType: publishTaskEventTypeFromStatus(task.Status),
			Source:    "agent",
			Status:    task.Status,
			Message:   task.Message,
			Payload: mustJSONBytes(map[string]any{
				"accountId":           task.AccountID,
				"skillId":             task.SkillID,
				"verificationPayload": json.RawMessage(task.VerificationPayload),
				"artifactCount":       len(artifactInputs),
				"hasRuntimeState":     runtimeState != nil,
				"finishedAt":          task.FinishedAt,
			}),
		})
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  *device.OwnerUserID,
			ResourceType: "publish_task",
			ResourceID:   &task.ID,
			Action:       "agent_sync",
			Title:        "发布任务状态同步",
			Source:       task.Platform,
			Status:       task.Status,
			Message:      task.Message,
			Payload: mustJSONBytes(map[string]any{
				"deviceId":            task.DeviceID,
				"accountName":         task.AccountName,
				"verificationPayload": json.RawMessage(task.VerificationPayload),
			}),
		})
	}

	render.JSON(w, http.StatusOK, task)
}

func (h *AgentHandler) preparePublishTaskArtifacts(ctx context.Context, taskID string, verificationPayload []byte, items []syncPublishTaskArtifactRequest) ([]store.UpsertPublishTaskArtifactInput, error) {
	results := make([]store.UpsertPublishTaskArtifactInput, 0, len(items)+1)

	for _, item := range items {
		prepared, err := h.preparePublishTaskArtifact(ctx, taskID, item)
		if err != nil {
			return nil, err
		}
		if prepared != nil {
			results = append(results, *prepared)
		}
	}

	if verificationArtifact, err := deriveVerificationArtifact(taskID, verificationPayload); err != nil {
		return nil, err
	} else if verificationArtifact != nil {
		results = append(results, *verificationArtifact)
	}

	return results, nil
}

func (h *AgentHandler) preparePublishTaskArtifact(ctx context.Context, taskID string, item syncPublishTaskArtifactRequest) (*store.UpsertPublishTaskArtifactInput, error) {
	artifactType := strings.TrimSpace(item.ArtifactType)
	if artifactType == "" {
		artifactType = "attachment"
	}
	source := strings.TrimSpace(item.Source)
	if source == "" {
		source = "agent"
	}

	title := normalizeTrimmedString(item.Title)
	fileName := normalizeTrimmedString(item.FileName)
	mimeType := normalizeTrimmedString(item.MimeType)
	textContent := normalizeTrimmedString(item.TextContent)
	artifactKey := buildPublishTaskArtifactKey(item.ArtifactKey, artifactType, fileName, title)
	if artifactKey == "" {
		return nil, nil
	}

	var payload []byte
	if item.Payload != nil {
		encoded, err := json.Marshal(item.Payload)
		if err != nil {
			return nil, fmt.Errorf("artifacts payload must be valid json")
		}
		payload = encoded
	}

	var storageKey *string
	var publicURL *string
	var sizeBytes *int64
	rawData := firstNonEmptyString(item.Data, item.Base64Data)
	if rawData != nil {
		data, contentType, err := decodeBase64Payload(*rawData)
		if err != nil {
			return nil, fmt.Errorf("artifacts data is invalid")
		}
		ext := extensionFromContentType(contentType)
		object, err := h.app.Storage.SaveBytes(
			ctx,
			fmt.Sprintf("publish-artifacts/%s/%s%s", taskID, artifactKey, ext),
			contentType,
			data,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to store publish task artifact")
		}
		storageKey = &object.StorageKey
		publicURL = &object.PublicURL
		sizeBytes = &object.SizeBytes
		if mimeType == nil {
			mimeType = &object.ContentType
		}
		if fileName == nil {
			derivedFileName := artifactKey + ext
			fileName = &derivedFileName
		}
	}

	return &store.UpsertPublishTaskArtifactInput{
		TaskID:       taskID,
		ArtifactKey:  artifactKey,
		ArtifactType: artifactType,
		Source:       source,
		Title:        title,
		FileName:     fileName,
		MimeType:     mimeType,
		StorageKey:   storageKey,
		PublicURL:    publicURL,
		SizeBytes:    sizeBytes,
		TextContent:  textContent,
		Payload:      payload,
	}, nil
}

func deriveVerificationArtifact(taskID string, verificationPayload []byte) (*store.UpsertPublishTaskArtifactInput, error) {
	if len(verificationPayload) == 0 {
		return nil, nil
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(verificationPayload, &payload); err != nil {
		return nil, fmt.Errorf("verification payload is invalid")
	}

	publicURL, _ := payload["screenshotUrl"].(string)
	storageKey, _ := payload["screenshotStorageKey"].(string)
	contentType, _ := payload["screenshotContentType"].(string)
	if strings.TrimSpace(publicURL) == "" && strings.TrimSpace(storageKey) == "" {
		return nil, nil
	}

	var sizeBytes *int64
	switch value := payload["screenshotSizeBytes"].(type) {
	case float64:
		converted := int64(value)
		sizeBytes = &converted
	case int64:
		converted := value
		sizeBytes = &converted
	}

	title := "人工验证截图"
	return &store.UpsertPublishTaskArtifactInput{
		TaskID:       taskID,
		ArtifactKey:  "verification-screenshot",
		ArtifactType: "verification_screenshot",
		Source:       "agent",
		Title:        &title,
		MimeType:     normalizeTrimmedStringPtr(contentType),
		StorageKey:   normalizeTrimmedStringPtr(storageKey),
		PublicURL:    normalizeTrimmedStringPtr(publicURL),
		SizeBytes:    sizeBytes,
		Payload:      verificationPayload,
	}, nil
}

func publishTaskEventTypeFromStatus(status string) string {
	switch status {
	case "cancel_requested":
		return "cancel_requested"
	case "cancelled":
		return "cancelled"
	case "needs_verify":
		return "needs_verify"
	case "success", "completed":
		return "completed"
	case "failed":
		return "failed"
	case "running":
		return "running"
	default:
		return "synced"
	}
}

func isAllowedAgentPublishTaskTransition(current string, next string) bool {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" || next == "" {
		return false
	}
	if current == next {
		return true
	}

	switch current {
	case "pending":
		return next == "needs_verify" || next == "failed" || next == "success" || next == "completed"
	case "running":
		return next == "needs_verify" || next == "failed" || next == "success" || next == "completed" || next == "cancelled" || next == "cancel_requested"
	case "needs_verify":
		return next == "success" || next == "completed" || next == "failed" || next == "cancelled"
	case "cancel_requested":
		return next == "cancelled" || next == "failed" || next == "success" || next == "completed"
	case "failed", "cancelled", "success", "completed":
		return false
	default:
		return true
	}
}

func (h *AgentHandler) recordRecoveredPublishTasks(ctx context.Context, device *domain.Device) {
	if device == nil {
		return
	}
	items, err := h.app.Store.RecoverExpiredPublishTaskLeases(ctx, device.ID)
	if err != nil || len(items) == 0 {
		return
	}
	for _, task := range items {
		_, _ = h.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    task.ID,
			EventType: "lease_recovered",
			Source:    "system",
			Status:    task.Status,
			Message:   task.Message,
		})
		if device.OwnerUserID != nil {
			recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
				OwnerUserID:  *device.OwnerUserID,
				ResourceType: "publish_task",
				ResourceID:   &task.ID,
				Action:       "lease_recovered",
				Title:        "发布任务租约回收",
				Source:       task.Platform,
				Status:       task.Status,
				Message:      task.Message,
				Payload: mustJSONBytes(map[string]any{
					"deviceId": device.ID,
				}),
			})
		}
	}
}

func normalizeTrimmedString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeTrimmedStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func buildSkillRevision(skill *domain.ProductSkill) string {
	if skill == nil {
		return ""
	}
	return skill.UpdatedAt.UTC().Format(time.RFC3339Nano)
}

func firstNonEmptyString(values ...*string) *string {
	for _, value := range values {
		if normalized := normalizeTrimmedString(value); normalized != nil {
			return normalized
		}
	}
	return nil
}

func buildPublishTaskArtifactKey(raw string, artifactType string, fileName *string, title *string) string {
	key := strings.TrimSpace(raw)
	key = strings.ReplaceAll(key, " ", "-")
	key = strings.Trim(key, "-_/")
	if key != "" {
		return key
	}
	for _, candidate := range []string{stringValue(fileName), stringValue(title), artifactType} {
		candidate = strings.TrimSpace(candidate)
		candidate = strings.ReplaceAll(candidate, " ", "-")
		candidate = strings.Trim(candidate, "-_/")
		if candidate != "" {
			return candidate
		}
	}
	return uuid.NewString()
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
