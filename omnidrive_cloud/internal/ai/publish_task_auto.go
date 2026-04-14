package ai

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

type autoPublishTarget struct {
	AccountID   string
	Platform    string
	AccountName string
}

func (w *Worker) autoCreatePublishTaskFromAIJob(ctx context.Context, job *domain.AIJob, artifacts []domain.AIJobArtifact) (*domain.PublishTask, error) {
	if w == nil || w.app == nil || w.app.Store == nil || job == nil {
		return nil, nil
	}
	if !allowAutoPublishJobSource(job.Source) || len(artifacts) == 0 {
		return nil, nil
	}
	if job.DeviceID == nil || strings.TrimSpace(*job.DeviceID) == "" {
		return nil, fmt.Errorf("auto publish job missing device id")
	}

	target := extractAutoPublishTarget(job.InputPayload)
	if target.AccountID == "" || target.Platform == "" || target.AccountName == "" {
		return nil, fmt.Errorf("auto publish target is invalid")
	}

	device, err := w.app.Store.GetOwnedDevice(ctx, strings.TrimSpace(*job.DeviceID), job.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if device == nil || !device.IsEnabled {
		return nil, fmt.Errorf("auto publish device is invalid")
	}

	account, err := w.app.Store.GetOwnedAccountByID(ctx, target.AccountID, job.OwnerUserID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, fmt.Errorf("auto publish account is missing")
	}
	if !allowAutoPublishAccountStatus(account.Status) {
		return nil, fmt.Errorf("auto publish account is not active")
	}
	if account.DeviceID != device.ID || account.Platform != target.Platform || account.AccountName != target.AccountName {
		return nil, fmt.Errorf("auto publish target does not match synced account")
	}

	runAt, err := extractAutoPublishRunAt(job.InputPayload)
	if err != nil {
		return nil, err
	}

	mediaPayload, err := buildAutoPublishMediaPayload(job, artifacts)
	if err != nil {
		return nil, err
	}

	title := buildAutoPublishTaskTitle(job)
	contentText := resolveAutoPublishContentText(job)
	taskID := uuid.NewString()
	taskMessage := "AI 成品已生成，等待 OmniBull 发布"
	lockKey := fmt.Sprintf(
		"ai-job-publish-target:%s:%s:%s:%s:%s",
		job.ID,
		device.ID,
		target.Platform,
		target.AccountName,
		runAt.UTC().Format(time.RFC3339),
	)

	var task *domain.PublishTask
	created := false
	if err := w.app.Store.WithAdvisoryLock(ctx, lockKey, func() error {
		existingTask, err := w.app.Store.FindReusablePublishTaskByAIJobTarget(ctx, job.ID, job.OwnerUserID, device.ID, target.Platform, target.AccountName)
		if err != nil {
			return err
		}
		if existingTask != nil {
			task = existingTask
			return nil
		}

		createdTask, err := w.app.Store.CreatePublishTask(ctx, store.CreatePublishTaskInput{
			ID:           taskID,
			DeviceID:     device.ID,
			AccountID:    &account.ID,
			Platform:     target.Platform,
			AccountName:  target.AccountName,
			Title:        title,
			ContentText:  contentText,
			MediaPayload: mediaPayload,
			Status:       "pending",
			Message:      stringPtr(taskMessage),
			RunAt:        &runAt,
		})
		if err != nil {
			return err
		}
		if err := w.app.Store.LinkAIJobToPublishTask(ctx, store.LinkAIJobPublishTaskInput{
			JobID:       job.ID,
			TaskID:      createdTask.ID,
			OwnerUserID: job.OwnerUserID,
		}); err != nil {
			return err
		}
		if _, err := w.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    createdTask.ID,
			EventType: "created_from_ai_job",
			Source:    "omnidrive",
			Status:    createdTask.Status,
			Message:   stringPtr("发布任务由 AI 任务自动创建"),
			Payload: mustJSON(map[string]any{
				"aiJobId":       job.ID,
				"artifactCount": len(artifacts),
				"publishAt":     runAt.Format(time.RFC3339),
			}),
		}); err != nil {
			return err
		}
		task = createdTask
		created = true
		return nil
	}); err != nil {
		return nil, err
	}

	if task != nil && created {
		w.recordAuditEvent(ctx, job, "publish_task_auto_created", "AI 成品自动创建发布任务", "success", stringPtr(taskMessage), map[string]any{
			"taskId":      task.ID,
			"platform":    task.Platform,
			"accountName": task.AccountName,
			"publishAt":   runAt.Format(time.RFC3339),
		})
	}
	return task, nil
}

func extractAutoPublishTarget(raw []byte) autoPublishTarget {
	payload := decodePayloadMap(raw)
	target := autoPublishTarget{
		AccountID: strings.TrimSpace(stringValue(payload["accountId"])),
	}
	publishPayload, _ := payload["publishPayload"].(map[string]any)
	targets, _ := publishPayload["targets"].([]any)
	if len(targets) == 0 {
		return target
	}
	firstTarget, _ := targets[0].(map[string]any)
	if firstTarget == nil {
		return target
	}
	if target.AccountID == "" {
		target.AccountID = strings.TrimSpace(stringValue(firstTarget["accountId"]))
	}
	target.Platform = strings.TrimSpace(stringValue(firstTarget["platform"]))
	target.AccountName = strings.TrimSpace(stringValue(firstTarget["accountName"]))
	return target
}

func extractAutoPublishRunAt(raw []byte) (time.Time, error) {
	payload := decodePayloadMap(raw)
	for _, value := range []any{
		payload["publishAt"],
		stringValueFromMap(mapValue(payload, "publishPayload"), "runAt", "requestedRun"),
	} {
		text := strings.TrimSpace(stringValue(value))
		if text == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("account skill publishAt is invalid")
}

func buildAutoPublishMediaPayload(job *domain.AIJob, artifacts []domain.AIJobArtifact) ([]byte, error) {
	items := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.PublicURL == nil || strings.TrimSpace(*artifact.PublicURL) == "" {
			continue
		}
		items = append(items, map[string]any{
			"artifactKey":  artifact.ArtifactKey,
			"artifactType": artifact.ArtifactType,
			"publicUrl":    artifact.PublicURL,
			"fileName":     artifact.FileName,
			"mimeType":     artifact.MimeType,
			"source":       artifact.Source,
			"storageKey":   artifact.StorageKey,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("ai job artifacts are missing public urls")
	}

	return json.Marshal(map[string]any{
		"source":      "ai_job",
		"aiJobId":     job.ID,
		"jobType":     job.JobType,
		"modelName":   job.ModelName,
		"artifacts":   items,
		"generatedAt": firstNonNilTime(job.FinishedAt, time.Now().UTC()).Format(time.RFC3339),
	})
}

func buildAutoPublishTaskTitle(job *domain.AIJob) string {
	payload := decodePayloadMap(job.InputPayload)
	if value := strings.TrimSpace(stringValue(payload["skillName"])); value != "" {
		return value
	}
	if job.Prompt != nil && strings.TrimSpace(*job.Prompt) != "" {
		return strings.TrimSpace(*job.Prompt)
	}
	return "AI 成品发布"
}

func resolveAutoPublishContentText(job *domain.AIJob) *string {
	outputPayload := decodePayloadMap(job.OutputPayload)
	if storyboardPayload, ok := outputPayload["storyboard"].(map[string]any); ok {
		if value := strings.TrimSpace(stringValueFromMap(storyboardPayload, "optimizedContentText")); value != "" {
			return stringPtr(value)
		}
	}
	if publishPayload, ok := outputPayload["publish"].(map[string]any); ok {
		if value := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText")); value != "" {
			return stringPtr(value)
		}
	}
	inputPayload := decodePayloadMap(job.InputPayload)
	if publishPayload, ok := inputPayload["publishPayload"].(map[string]any); ok {
		if value := strings.TrimSpace(stringValueFromMap(publishPayload, "contentText", "contentTemplate")); value != "" {
			return stringPtr(value)
		}
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

func allowAutoPublishJobSource(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "account_skill_binding", "openclaw_skill":
		return true
	default:
		return false
	}
}

func mapValue(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, _ := payload[key].(map[string]any)
	return value
}
