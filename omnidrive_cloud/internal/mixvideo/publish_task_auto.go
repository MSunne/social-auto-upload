package mixvideo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

func (w *Worker) autoCreatePublishTaskFromMixVideoTask(ctx context.Context, task *domain.MixVideoTask) (*domain.PublishTask, error) {
	if w == nil || w.app == nil || w.app.Store == nil || task == nil {
		return nil, nil
	}
	if task.ResultAsset == nil || task.DeviceID == nil || task.AccountID == nil || task.Platform == nil || task.AccountName == nil || task.RunAt == nil {
		return nil, nil
	}

	device, err := w.app.Store.GetOwnedDevice(ctx, strings.TrimSpace(*task.DeviceID), task.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if device == nil || !device.IsEnabled {
		return nil, fmt.Errorf("auto publish device is invalid")
	}

	account, err := w.app.Store.GetOwnedAccountByID(ctx, strings.TrimSpace(*task.AccountID), task.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("auto publish account is missing")
	}
	if !allowAutoPublishAccountStatus(account.Status) {
		return nil, fmt.Errorf("auto publish account is not active")
	}
	if account.DeviceID != device.ID || account.Platform != strings.TrimSpace(*task.Platform) || account.AccountName != strings.TrimSpace(*task.AccountName) {
		return nil, fmt.Errorf("auto publish target does not match synced account")
	}

	title := buildMixVideoPublishTaskTitle(task)
	contentText := resolveMixVideoPublishContentText(task)
	mediaPayload, err := json.Marshal(map[string]any{
		"source":         "mix_video_task",
		"mixVideoTaskId": task.ID,
		"scriptText":     task.ScriptText,
		"generatedAt":    firstNonNilTime(task.CompletedAt, time.Now().UTC()).Format(time.RFC3339),
		"artifacts": []map[string]any{{
			"artifactType": "video",
			"publicUrl":    task.ResultAsset.PublicURL,
			"fileName":     task.ResultAsset.FileName,
			"mimeType":     task.ResultAsset.MimeType,
			"storageKey":   task.ResultAsset.StorageKey,
			"sizeBytes":    task.ResultAsset.SizeBytes,
		}},
	})
	if err != nil {
		return nil, err
	}

	taskID := uuid.NewString()
	taskMessage := "混剪成片已生成，等待 OmniBull 发布"
	lockKey := fmt.Sprintf(
		"mix-video-publish-target:%s:%s:%s:%s:%s",
		task.ID,
		device.ID,
		account.Platform,
		account.AccountName,
		task.RunAt.UTC().Format(time.RFC3339),
	)

	var publishTask *domain.PublishTask
	created := false
	if err := w.app.Store.WithAdvisoryLock(ctx, lockKey, func() error {
		existingTask, err := w.app.Store.FindReusablePublishTaskByMixVideoTarget(ctx, task.ID, task.OwnerUserID, device.ID, account.Platform, account.AccountName)
		if err != nil {
			return err
		}
		if existingTask != nil {
			publishTask = existingTask
			return nil
		}

		createdTask, err := w.app.Store.CreatePublishTask(ctx, store.CreatePublishTaskInput{
			ID:           taskID,
			DeviceID:     device.ID,
			AccountID:    &account.ID,
			SkillID:      task.SkillID,
			Platform:     account.Platform,
			AccountName:  account.AccountName,
			Title:        title,
			ContentText:  contentText,
			MediaPayload: mediaPayload,
			Status:       "pending",
			Message:      publishTaskStringPtr(taskMessage),
			RunAt:        task.RunAt,
		})
		if err != nil {
			return err
		}
		if err := w.app.Store.LinkMixVideoTaskToPublishTask(ctx, store.LinkMixVideoTaskPublishTaskInput{
			TaskID:             task.ID,
			LocalPublishTaskID: createdTask.ID,
			OwnerUserID:        task.OwnerUserID,
		}); err != nil {
			return err
		}
		if _, err := w.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    createdTask.ID,
			EventType: "created_from_mix_video_task",
			Source:    "omnidrive",
			Status:    createdTask.Status,
			Message:   publishTaskStringPtr("发布任务由混剪任务自动创建"),
			Payload: publishTaskMustJSON(map[string]any{
				"mixVideoTaskId": task.ID,
				"publishAt":      task.RunAt.Format(time.RFC3339),
			}),
		}); err != nil {
			return err
		}
		publishTask = createdTask
		created = true
		return nil
	}); err != nil {
		return nil, err
	}

	if publishTask != nil && created {
		w.app.Logger.Info("mix video auto created publish task", "mix_video_task_id", task.ID, "publish_task_id", publishTask.ID)
	}
	return publishTask, nil
}

func buildMixVideoPublishTaskTitle(task *domain.MixVideoTask) string {
	payload := decodePayloadMap(task.RequestPayload)
	if value := strings.TrimSpace(stringValue(payload["skillName"])); value != "" {
		return value
	}
	if value := strings.TrimSpace(stringValue(payload["scriptTemplate"])); value != "" {
		return value
	}
	return "混剪成片发布"
}

func resolveMixVideoPublishContentText(task *domain.MixVideoTask) *string {
	payload := decodePayloadMap(task.RequestPayload)
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	if value := strings.TrimSpace(stringValue(publishPayload["contentText"])); value != "" {
		return publishTaskStringPtr(value)
	}
	if value := strings.TrimSpace(stringValue(publishPayload["contentTemplate"])); value != "" {
		return publishTaskStringPtr(value)
	}
	return nil
}

func allowAutoPublishAccountStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "invalid", "disabled", "deleted", "inactive":
		return false
	default:
		return true
	}
}

func decodePayloadMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func publishTaskStringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	return &trimmed
}

func firstNonNilTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil || value.IsZero() {
		return fallback.UTC()
	}
	return value.UTC()
}

func publishTaskMustJSON(value any) []byte {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}
