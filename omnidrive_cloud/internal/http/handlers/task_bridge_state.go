package handlers

import (
	"encoding/json"
	"strings"

	"omnidrive_cloud/internal/domain"
)

// 构建发布任务桥接状态，为任务桥接状态生成后续步骤所需的派生参数或载荷。
func buildPublishTaskBridgeState(task *domain.PublishTask, runtime *domain.PublishTaskRuntimeState) domain.PublishTaskBridgeState {
	bridge := domain.PublishTaskBridgeState{
		Origin:         "cloud",
		HasActiveLease: task != nil && task.LeaseOwnerDeviceID != nil && task.LeaseExpiresAt != nil,
	}
	if runtime == nil {
		return bridge
	}

	bridge.LastAgentSyncAt = runtime.LastAgentSyncAt
	if len(runtime.ExecutionPayload) == 0 {
		return bridge
	}

	var payload map[string]any
	if err := json.Unmarshal(runtime.ExecutionPayload, &payload); err != nil {
		return bridge
	}

	source := normalizeBridgeString(payload["source"])
	stage := normalizeBridgeString(payload["stage"])
	localStatus := normalizeBridgeString(payload["localStatus"])
	workerName := normalizeBridgeString(payload["workerName"])
	updatedAt := normalizeBridgeString(payload["updatedAt"])
	startedAt := normalizeBridgeString(payload["startedAt"])
	finishedAt := normalizeBridgeString(payload["finishedAt"])

	if source != nil {
		bridge.LocalSource = source
		switch strings.TrimSpace(*source) {
		case "local_api", "openclaw_skill":
			bridge.Origin = "local"
		case "omnidrive_agent":
			bridge.Origin = "imported"
		}
	}
	bridge.Stage = stage
	bridge.LocalStatus = localStatus
	bridge.WorkerName = workerName
	bridge.UpdatedAt = updatedAt
	bridge.StartedAt = startedAt
	bridge.FinishedAt = finishedAt
	return bridge
}

// 规范化桥接String，统一任务桥接状态链路的输入格式和后续处理行为。
func normalizeBridgeString(value any) *string {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	return &text
}
