package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const (
	digitalHumanWorkflowKind           = "digital_human"
	digitalHumanSourceRunningHub       = "runninghub"
	digitalHumanEstimateCharsPerSecond = 4
)

type digitalHumanWorkflowConfig struct {
	Mode           string                    `json:"mode"`
	GoodsTitle     string                    `json:"goodsTitle"`
	GoodsText      string                    `json:"goodsText"`
	CharacterAsset domain.DigitalHumanAsset  `json:"characterAsset"`
	GoodsAsset     *domain.DigitalHumanAsset `json:"goodsAsset,omitempty"`
	RefAudioAsset  domain.DigitalHumanAsset  `json:"refAudioAsset"`
}

type digitalHumanWorkflowPayload struct {
	WorkflowKind       string                     `json:"workflowKind"`
	DigitalHumanConfig digitalHumanWorkflowConfig `json:"digitalHumanConfig"`
}

func isDigitalHumanWorkflowJob(job *domain.AIJob) bool {
	if job == nil {
		return false
	}
	_, ok := parseDigitalHumanWorkflowPayload(job.InputPayload)
	return ok
}

func parseDigitalHumanWorkflowPayload(raw []byte) (*digitalHumanWorkflowPayload, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var payload digitalHumanWorkflowPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, false
	}
	if !strings.EqualFold(strings.TrimSpace(payload.WorkflowKind), digitalHumanWorkflowKind) {
		return nil, false
	}
	return &payload, true
}

func (w *Worker) executeDigitalHumanVideo(ctx context.Context, job *domain.AIJob, leaseToken string) error {
	payload, ok := parseDigitalHumanWorkflowPayload(job.InputPayload)
	if !ok {
		return fmt.Errorf("digital human workflow payload is invalid")
	}

	task, err := w.app.Store.GetDigitalHumanTaskByAIJobID(ctx, job.ID)
	if err != nil {
		return err
	}
	if task == nil {
		task, err = w.createDigitalHumanWorkflowTask(ctx, job, payload)
		if err != nil {
			return err
		}
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task, err = w.app.Store.GetDigitalHumanTaskByID(ctx, task.ID)
		if err != nil {
			return err
		}
		if task == nil {
			return fmt.Errorf("linked digital human task not found")
		}

		switch strings.ToLower(strings.TrimSpace(task.Status)) {
		case "queued", "running":
			message := strings.TrimSpace(valueOrEmptyString(task.ErrorMessage))
			if task.Progress != nil && strings.TrimSpace(task.Progress.Message) != "" {
				message = strings.TrimSpace(task.Progress.Message)
			}
			if message == "" {
				message = "数字人口播生成中"
			}
			outputPayload := mustJSON(map[string]any{
				"workflowKind":       digitalHumanWorkflowKind,
				"digitalHumanTaskId": task.ID,
				"digitalHumanStatus": task.Status,
				"progress":           task.Progress,
				"modelName":          task.ModelName,
			})
			job.OutputPayload = outputPayload
			if _, err := w.syncRunningState(ctx, job, leaseToken, message, outputPayload); err != nil {
				return err
			}
		case "completed":
			return w.completeDigitalHumanWorkflowJob(ctx, job, leaseToken, task)
		case "failed", "cancelled":
			if task.ErrorMessage != nil && strings.TrimSpace(*task.ErrorMessage) != "" {
				return errors.New(strings.TrimSpace(*task.ErrorMessage))
			}
			return fmt.Errorf("digital human task %s", task.Status)
		default:
			return fmt.Errorf("unsupported digital human task status: %s", task.Status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.videoPollInterval):
		}
	}
}

func (w *Worker) createDigitalHumanWorkflowTask(ctx context.Context, job *domain.AIJob, payload *digitalHumanWorkflowPayload) (*domain.DigitalHumanTask, error) {
	if payload == nil {
		return nil, fmt.Errorf("digital human payload is required")
	}
	config := payload.DigitalHumanConfig
	mode := strings.TrimSpace(config.Mode)
	goodsText := strings.TrimSpace(config.GoodsText)
	if mode == "" || goodsText == "" {
		return nil, fmt.Errorf("digital human workflow config is incomplete")
	}

	creditsPerSecondMillis, err := w.digitalHumanCreditsPerSecondMillis(ctx)
	if err != nil {
		return nil, err
	}
	if creditsPerSecondMillis <= 0 {
		return nil, fmt.Errorf("数字人计费暂未开放，请稍后再试")
	}

	estimatedDurationSeconds := estimateDigitalHumanDurationSeconds(goodsText)
	estimatedCreditsMillis := int64(estimatedDurationSeconds) * creditsPerSecondMillis
	taskID := uuid.NewString()
	requestPayload := mustJSON(map[string]any{
		"mode":      mode,
		"modelName": strings.TrimSpace(job.ModelName),
		"source":    digitalHumanSourceRunningHub,
		"goodsText": goodsText,
		"sourceAIJob": map[string]any{
			"id":      job.ID,
			"skillId": valueOrEmptyString(job.SkillID),
			"source":  job.Source,
		},
	})

	created, err := w.app.Store.CreateDigitalHumanTask(ctx, store.CreateDigitalHumanTaskInput{
		ID:                       taskID,
		OwnerUserID:              job.OwnerUserID,
		AIJobID:                  &job.ID,
		Mode:                     mode,
		Source:                   digitalHumanSourceRunningHub,
		Status:                   "queued",
		ModelName:                strings.TrimSpace(job.ModelName),
		CharacterAsset:           mustJSON(config.CharacterAsset),
		GoodsAsset:               mustJSON(config.GoodsAsset),
		RefAudioAsset:            mustJSON(config.RefAudioAsset),
		GoodsTitle:               digitalHumanTrimmedStringPointer(config.GoodsTitle),
		GoodsText:                goodsText,
		EstimatedDurationSeconds: estimatedDurationSeconds,
		EstimatedCreditsMillis:   estimatedCreditsMillis,
		BillingStatus:            "pending",
		BillingPayload: mustJSON(map[string]any{
			"creditsPerSecond":       store.DigitalHumanCreditsFromMillis(creditsPerSecondMillis),
			"creditsPerSecondMillis": creditsPerSecondMillis,
			"estimateCharsPerSecond": digitalHumanEstimateCharsPerSecond,
			"billingMessage":         "数字人任务待预扣积分",
			"sourceType":             "ai_job",
			"sourceId":               job.ID,
		}),
		Progress: mustJSON(domain.DigitalHumanProgress{
			Current:    0,
			Total:      0,
			Percentage: 0,
			Message:    "数字人任务已创建，等待提交到生成服务",
		}),
		RequestPayload: requestPayload,
	})
	if err != nil {
		return nil, err
	}

	precharged, err := w.app.Store.PrechargeDigitalHumanTask(ctx, created.ID)
	if err != nil {
		_ = w.app.Store.DeleteDigitalHumanTask(ctx, created.ID, job.OwnerUserID)
		if errors.Is(err, store.ErrDigitalHumanBillingInsufficientBalance) {
			return nil, &executionBillingBlockedError{Result: &store.ApplyUsageBillingResult{
				BillStatus:  "failed",
				BillMessage: "当前积分不足，无法执行数字人口播",
			}}
		}
		return nil, err
	}
	return precharged, nil
}

func (w *Worker) completeDigitalHumanWorkflowJob(ctx context.Context, job *domain.AIJob, leaseToken string, task *domain.DigitalHumanTask) error {
	if task == nil || task.ResultAsset == nil {
		return fmt.Errorf("digital human task completed without result asset")
	}

	fileName := strings.TrimSpace(task.ResultAsset.FileName)
	if fileName == "" {
		fileName = "digital-human.mp4"
	}
	mimeType := strings.TrimSpace(task.ResultAsset.MimeType)
	if mimeType == "" {
		mimeType = "video/mp4"
	}
	artifactKey := safeArtifactKey(fileName, "digital-human.mp4")
	artifacts, err := w.app.Store.UpsertAIJobArtifacts(ctx, []store.UpsertAIJobArtifactInput{{
		JobID:        job.ID,
		ArtifactKey:  artifactKey,
		ArtifactType: "video",
		Source:       "digital_human",
		Title:        stringPtr("数字人口播成品"),
		FileName:     &fileName,
		MimeType:     &mimeType,
		StorageKey:   digitalHumanTrimmedStringPointer(task.ResultAsset.StorageKey),
		PublicURL:    digitalHumanTrimmedStringPointer(task.ResultAsset.PublicURL),
		SizeBytes:    task.ResultAsset.SizeBytes,
		Payload: mustJSON(map[string]any{
			"digitalHumanTaskId": task.ID,
			"modelName":          task.ModelName,
			"billingStatus":      task.BillingStatus,
			"actualDuration":     task.ActualDurationSeconds,
		}),
	}})
	if err != nil {
		return err
	}

	outputPayload := mustJSON(map[string]any{
		"workflowKind":       digitalHumanWorkflowKind,
		"digitalHumanTaskId": task.ID,
		"digitalHumanStatus": task.Status,
		"modelName":          task.ModelName,
		"billingStatus":      task.BillingStatus,
		"artifacts":          summarizeArtifacts(artifacts),
		"completedAt":        firstNonNilTime(task.CompletedAt, time.Now().UTC()).Format(time.RFC3339),
	})
	message := "数字人口播生成完成"
	if _, err := w.completeJob(ctx, job, leaseToken, message, outputPayload, nil); err != nil {
		return err
	}
	w.recordAuditEvent(ctx, job, "cloud_generate_success", "数字人口播生成完成", "success", stringPtr(message), map[string]any{
		"jobType":            job.JobType,
		"modelName":          task.ModelName,
		"digitalHumanTaskId": task.ID,
		"artifactCount":      len(artifacts),
	})
	return nil
}

func (w *Worker) digitalHumanCreditsPerSecondMillis(ctx context.Context) (int64, error) {
	record, err := w.app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return 0, err
	}
	if record == nil {
		return 0, nil
	}
	if record.DigitalHumanCreditsPerSecondMillis > 0 {
		return record.DigitalHumanCreditsPerSecondMillis, nil
	}
	if record.DigitalHumanCreditsPerSecond > 0 {
		return record.DigitalHumanCreditsPerSecond * store.DigitalHumanCreditMillisScale, nil
	}
	return 0, nil
}

func estimateDigitalHumanDurationSeconds(goodsText string) int {
	nonWhitespaceRunes := 0
	for _, r := range goodsText {
		if unicode.IsSpace(r) {
			continue
		}
		nonWhitespaceRunes++
	}
	if nonWhitespaceRunes == 0 {
		return 1
	}
	estimated := (nonWhitespaceRunes + digitalHumanEstimateCharsPerSecond - 1) / digitalHumanEstimateCharsPerSecond
	if estimated <= 0 {
		return 1
	}
	return estimated
}

func digitalHumanTrimmedStringPointer(value string) *string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return &trimmed
	}
	return nil
}
