package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

func buildChatBillingInput(job *domain.AIJob, result *ChatResult) store.ApplyUsageBillingInput {
	metrics := make([]store.ApplyUsageMetricInput, 0, 2)
	usage := map[string]any{}
	if result != nil && result.Usage != nil {
		usage = result.Usage
	}

	if promptTokens := usageInt64(usage["prompt_tokens"], usage["input_tokens"]); promptTokens > 0 {
		metrics = append(metrics, store.ApplyUsageMetricInput{
			MeterCode: "chat_input_tokens",
			Quantity:  promptTokens,
			Metadata: mustJSONBytes(map[string]any{
				"promptTokens": promptTokens,
			}),
		})
	}
	if completionTokens := usageInt64(usage["completion_tokens"], usage["output_tokens"], usage["candidates_token_count"]); completionTokens > 0 {
		metrics = append(metrics, store.ApplyUsageMetricInput{
			MeterCode: "chat_output_tokens",
			Quantity:  completionTokens,
			Metadata: mustJSONBytes(map[string]any{
				"completionTokens": completionTokens,
			}),
		})
	}

	return store.ApplyUsageBillingInput{
		UserID:     strings.TrimSpace(job.OwnerUserID),
		SourceType: "ai_job",
		SourceID:   strings.TrimSpace(job.ID),
		ModelName:  strings.TrimSpace(job.ModelName),
		JobType:    strings.TrimSpace(job.JobType),
		Metrics:    metrics,
	}
}

func buildImageBillingInput(job *domain.AIJob, imageCount int) store.ApplyUsageBillingInput {
	metrics := []store.ApplyUsageMetricInput{}
	if imageCount > 0 {
		metrics = append(metrics, store.ApplyUsageMetricInput{
			MeterCode: "image_generations",
			Quantity:  int64(imageCount),
			Metadata: mustJSONBytes(map[string]any{
				"imageCount": imageCount,
			}),
		})
	}
	return store.ApplyUsageBillingInput{
		UserID:     strings.TrimSpace(job.OwnerUserID),
		SourceType: "ai_job",
		SourceID:   strings.TrimSpace(job.ID),
		ModelName:  strings.TrimSpace(job.ModelName),
		JobType:    strings.TrimSpace(job.JobType),
		Metrics:    metrics,
	}
}

func buildVideoBillingInput(job *domain.AIJob) store.ApplyUsageBillingInput {
	payload := decodePayloadMap(job.InputPayload)
	metadata := map[string]any{
		"jobId": job.ID,
	}
	if durationSeconds := intPtrFromMap(payload, "durationSeconds", "duration"); durationSeconds != nil && *durationSeconds > 0 {
		metadata["durationSeconds"] = *durationSeconds
	}
	return store.ApplyUsageBillingInput{
		UserID:     strings.TrimSpace(job.OwnerUserID),
		SourceType: "ai_job",
		SourceID:   strings.TrimSpace(job.ID),
		ModelName:  strings.TrimSpace(job.ModelName),
		JobType:    strings.TrimSpace(job.JobType),
		Metrics: []store.ApplyUsageMetricInput{
			{
				MeterCode: "video_generations",
				Quantity:  1,
				Metadata:  mustJSONBytes(metadata),
			},
		},
	}
}

func BuildEstimatedUsageBillingInput(job *domain.AIJob) store.ApplyUsageBillingInput {
	if job == nil {
		return store.ApplyUsageBillingInput{}
	}
	switch strings.TrimSpace(strings.ToLower(job.JobType)) {
	case "image":
		payload := decodePayloadMap(job.InputPayload)
		imageCount := 1
		if count := intPtrFromMap(payload, "imageCount", "count", "n"); count != nil && *count > 0 {
			imageCount = *count
		}
		return buildImageBillingInput(job, imageCount)
	case "video":
		return buildVideoBillingInput(job)
	default:
		return store.ApplyUsageBillingInput{
			UserID:     strings.TrimSpace(job.OwnerUserID),
			SourceType: "ai_job",
			SourceID:   strings.TrimSpace(job.ID),
			ModelName:  strings.TrimSpace(job.ModelName),
			JobType:    strings.TrimSpace(job.JobType),
			Metrics:    []store.ApplyUsageMetricInput{},
		}
	}
}

func BuildUsageBillingBlockMessage(result *store.ApplyUsageBillingResult) string {
	if result == nil {
		return "当前积分不足，任务开始前需要预扣费，请先充值后再试。"
	}
	raw := strings.TrimSpace(strings.ToLower(result.BillMessage))
	switch {
	case strings.Contains(raw, "wallet credits insufficient for fallback debit"):
		return "当前套餐额度已用尽，且钱包积分不足，任务开始前需要预扣费，请先充值后再试。"
	case strings.Contains(raw, "wallet credits insufficient"):
		return "当前钱包积分不足，任务开始前需要预扣费，请先充值后再试。"
	case strings.Contains(raw, "pricing rule not found"):
		return "当前任务缺少可用计费规则，暂时无法启动，请联系管理员检查计费配置。"
	case strings.TrimSpace(result.BillMessage) != "":
		return fmt.Sprintf("任务启动前计费预检未通过：%s", strings.TrimSpace(result.BillMessage))
	default:
		return "当前积分不足，任务开始前需要预扣费，请先充值后再试。"
	}
}

func usageInt64(values ...any) int64 {
	for _, value := range values {
		switch typed := value.(type) {
		case int64:
			if typed > 0 {
				return typed
			}
		case int:
			if typed > 0 {
				return int64(typed)
			}
		case float64:
			if typed > 0 {
				return int64(typed)
			}
		case json.Number:
			if parsed, err := typed.Int64(); err == nil && parsed > 0 {
				return parsed
			}
		case string:
			if parsed, err := json.Number(strings.TrimSpace(typed)).Int64(); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func mustJSONBytes(value any) []byte {
	if value == nil {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return data
}
