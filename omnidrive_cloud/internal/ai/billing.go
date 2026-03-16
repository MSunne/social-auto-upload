package ai

import (
	"encoding/json"
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
				Metadata: mustJSONBytes(map[string]any{
					"jobId": job.ID,
				}),
			},
		},
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
