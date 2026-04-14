package ai

import (
	"strings"
	"time"

	"omnidrive_cloud/internal/domain"
)

const (
	longTaskFastPollWindow   = 5 * time.Minute
	longTaskFastPollInterval = 15 * time.Second
	longTaskSlowPollInterval = 3 * time.Minute
)

func isLongRunningAIJobType(jobType string) bool {
	switch strings.ToLower(strings.TrimSpace(jobType)) {
	case "video", "digital_human":
		return true
	default:
		return false
	}
}

func mediaExecutionPollInterval(anchor time.Time, now time.Time) time.Duration {
	if anchor.IsZero() {
		return longTaskFastPollInterval
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	anchor = anchor.UTC()
	now = now.UTC()
	if now.Before(anchor) {
		return longTaskFastPollInterval
	}
	if now.Sub(anchor) <= longTaskFastPollWindow {
		return longTaskFastPollInterval
	}
	return longTaskSlowPollInterval
}

func digitalHumanTaskExecutionAnchor(task *domain.DigitalHumanTask) time.Time {
	if task == nil {
		return time.Time{}
	}
	if task.StartedAt != nil {
		return task.StartedAt.UTC()
	}
	return task.CreatedAt.UTC()
}

func shouldContinueDigitalHumanInBackground(task *domain.DigitalHumanTask, timeout time.Duration, now time.Time) bool {
	if task == nil || timeout <= 0 {
		return false
	}
	anchor := digitalHumanTaskExecutionAnchor(task)
	if anchor.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !now.UTC().Before(anchor.Add(timeout))
}
