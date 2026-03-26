package store

import (
	"testing"

	"omnidrive_cloud/internal/domain"
)

func TestAIJobDeliveryChangedSkipsRepeatedDeliveryHeartbeat(t *testing.T) {
	job := &domain.AIJob{
		DeliveryStatus:     "success",
		DeliveryMessage:    stringPtr("发布任务执行成功"),
		LocalPublishTaskID: stringPtr("publish-task-1"),
	}

	if aiJobDeliveryChanged(job, "success", stringPtr("发布任务执行成功"), stringPtr("publish-task-1")) {
		t.Fatalf("expected repeated delivery update to be treated as unchanged")
	}
}

func TestAIJobDeliveryChangedDetectsMeaningfulUpdates(t *testing.T) {
	job := &domain.AIJob{
		DeliveryStatus:     "publish_queued",
		DeliveryMessage:    stringPtr("等待发布"),
		LocalPublishTaskID: stringPtr("publish-task-1"),
	}

	if !aiJobDeliveryChanged(job, "publishing", stringPtr("发布中"), stringPtr("publish-task-1")) {
		t.Fatalf("expected status/message change to be treated as changed")
	}
	if !aiJobDeliveryChanged(job, "publish_queued", stringPtr("等待发布"), stringPtr("publish-task-2")) {
		t.Fatalf("expected publish task binding change to be treated as changed")
	}
}
