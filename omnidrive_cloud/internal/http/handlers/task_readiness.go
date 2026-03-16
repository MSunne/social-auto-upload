package handlers

import (
	"context"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
)

func appendReadinessIssue(readiness *domain.PublishTaskReadiness, code string, message string) {
	if strings.TrimSpace(code) != "" {
		for _, existing := range readiness.IssueCodes {
			if existing == code {
				goto appendMessage
			}
		}
		readiness.IssueCodes = append(readiness.IssueCodes, code)
	}

appendMessage:
	if strings.TrimSpace(message) == "" {
		return
	}
	for _, existing := range readiness.Issues {
		if existing == message {
			return
		}
	}
	readiness.Issues = append(readiness.Issues, message)
}

func publishTaskReadinessBlockingDimensions(readiness domain.PublishTaskReadiness) []string {
	dimensions := make([]string, 0, 4)
	if !readiness.DeviceReady {
		dimensions = append(dimensions, "device")
	}
	if !readiness.AccountReady {
		dimensions = append(dimensions, "account")
	}
	if !readiness.SkillReady || !readiness.SkillRevisionMatched || !readiness.SkillSyncedToDevice {
		dimensions = append(dimensions, "skill")
	}
	if !readiness.MaterialsReady {
		dimensions = append(dimensions, "materials")
	}
	return dimensions
}

func buildPublishTaskReadiness(
	ctx context.Context,
	app *appstate.App,
	task *domain.PublishTask,
	device *domain.Device,
	account *domain.PlatformAccount,
	skill *domain.ProductSkill,
) domain.PublishTaskReadiness {
	readiness := domain.PublishTaskReadiness{
		DeviceReady:          device != nil && device.IsEnabled,
		AccountReady:         account != nil && account.Status == "active",
		SkillReady:           task == nil || task.SkillID == nil || skill != nil && skill.IsEnabled,
		SkillRevisionMatched: true,
		SkillSyncedToDevice:  task == nil || task.SkillID == nil,
		MaterialsReady:       true,
		IssueCodes:           []string{},
		Issues:               []string{},
	}

	if task == nil {
		readiness.DeviceReady = false
		readiness.AccountReady = false
		readiness.SkillReady = false
		readiness.SkillRevisionMatched = false
		readiness.SkillSyncedToDevice = false
		readiness.MaterialsReady = false
		appendReadinessIssue(&readiness, "task_missing", "任务不存在")
		return readiness
	}

	if device == nil {
		appendReadinessIssue(&readiness, "device_missing", "设备不存在")
	} else if !device.IsEnabled {
		appendReadinessIssue(&readiness, "device_disabled", "设备已禁用")
	}

	if account == nil {
		appendReadinessIssue(&readiness, "account_missing", "账号未同步或不可用")
	} else if account.Status != "active" {
		appendReadinessIssue(&readiness, "account_inactive", "账号当前不是 active 状态")
	}

	if task.SkillID != nil {
		if skill == nil {
			readiness.SkillSyncedToDevice = false
			appendReadinessIssue(&readiness, "skill_missing", "技能不存在")
		} else if !skill.IsEnabled {
			readiness.SkillSyncedToDevice = false
			appendReadinessIssue(&readiness, "skill_disabled", "技能已禁用")
		} else {
			currentRevision := ""
			if app != nil && app.Store != nil {
				revision, revisionErr := app.Store.GetSkillRevision(ctx, skill.ID, skill.OwnerUserID)
				if revisionErr != nil {
					readiness.SkillReady = false
					readiness.SkillRevisionMatched = false
					appendReadinessIssue(&readiness, "skill_revision_check_failed", "技能版本校验失败")
				} else {
					currentRevision = revision
				}
			}
			if app != nil && app.Store != nil && device != nil {
				syncState, syncErr := app.Store.GetDeviceSkillSyncState(ctx, device.ID, skill.ID)
				if syncErr != nil {
					readiness.SkillReady = false
					readiness.SkillSyncedToDevice = false
					appendReadinessIssue(&readiness, "device_skill_sync_check_failed", "技能同步状态校验失败")
				} else if syncState == nil {
					readiness.SkillReady = false
					readiness.SkillSyncedToDevice = false
					appendReadinessIssue(&readiness, "device_skill_missing", "设备尚未同步该技能")
				} else if syncState.SyncStatus != "success" {
					readiness.SkillReady = false
					readiness.SkillSyncedToDevice = false
					appendReadinessIssue(&readiness, "device_skill_sync_incomplete", "设备上的技能同步尚未完成")
				} else if currentRevision != "" && (syncState.SyncedRevision == nil || strings.TrimSpace(*syncState.SyncedRevision) != currentRevision) {
					readiness.SkillReady = false
					readiness.SkillSyncedToDevice = false
					appendReadinessIssue(&readiness, "device_skill_outdated", "设备上的技能版本不是最新，请重新同步")
				} else {
					readiness.SkillSyncedToDevice = true
				}
			}
			if task.SkillRevision != nil && strings.TrimSpace(*task.SkillRevision) != "" {
				if currentRevision != "" && currentRevision != strings.TrimSpace(*task.SkillRevision) {
					readiness.SkillReady = false
					readiness.SkillRevisionMatched = false
					appendReadinessIssue(&readiness, "skill_revision_changed", "技能配置已更新，请复核任务")
				}
			}
		}
	}

	if app != nil && app.Store != nil {
		total, available, drifted, err := app.Store.CountPublishTaskMaterialHealth(ctx, task.ID)
		if err != nil {
			readiness.MaterialsReady = false
			appendReadinessIssue(&readiness, "material_health_check_failed", "素材可用性校验失败")
		} else {
			readiness.TotalMaterialCount = total
			readiness.AvailableMaterialCount = available
			readiness.DriftedMaterialCount = drifted
			if total > available {
				readiness.MaterialsReady = false
				readiness.MissingMaterialCount = total - available
				appendReadinessIssue(&readiness, "material_missing", "部分素材已缺失或未同步")
			}
			if drifted > 0 {
				readiness.MaterialsReady = false
				appendReadinessIssue(&readiness, "material_drifted", "部分素材已发生变化，请复核任务")
			}
		}
	}

	return readiness
}

func publishTaskReadinessAllowsExecution(readiness domain.PublishTaskReadiness) bool {
	return readiness.DeviceReady &&
		readiness.AccountReady &&
		readiness.SkillReady &&
		readiness.MaterialsReady
}
