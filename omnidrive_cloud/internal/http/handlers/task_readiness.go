package handlers

import (
	"context"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
)

func buildPublishTaskReadiness(
	ctx context.Context,
	app *appstate.App,
	task *domain.PublishTask,
	device *domain.Device,
	account *domain.PlatformAccount,
	skill *domain.ProductSkill,
) domain.PublishTaskReadiness {
	readiness := domain.PublishTaskReadiness{
		DeviceReady:    device != nil && device.IsEnabled,
		AccountReady:   account != nil && account.Status == "active",
		SkillReady:     task == nil || task.SkillID == nil || skill != nil && skill.IsEnabled,
		MaterialsReady: true,
		Issues:         []string{},
	}

	if task == nil {
		readiness.DeviceReady = false
		readiness.AccountReady = false
		readiness.SkillReady = false
		readiness.MaterialsReady = false
		readiness.Issues = append(readiness.Issues, "任务不存在")
		return readiness
	}

	if device == nil {
		readiness.Issues = append(readiness.Issues, "设备不存在")
	} else if !device.IsEnabled {
		readiness.Issues = append(readiness.Issues, "设备已禁用")
	}

	if account == nil {
		readiness.Issues = append(readiness.Issues, "账号未同步或不可用")
	} else if account.Status != "active" {
		readiness.Issues = append(readiness.Issues, "账号当前不是 active 状态")
	}

	if task.SkillID != nil {
		if skill == nil {
			readiness.Issues = append(readiness.Issues, "技能不存在")
		} else if !skill.IsEnabled {
			readiness.Issues = append(readiness.Issues, "技能已禁用")
		}
	}

	if app != nil && app.Store != nil {
		total, available, err := app.Store.CountPublishTaskAvailableMaterials(ctx, task.ID)
		if err != nil {
			readiness.MaterialsReady = false
			readiness.Issues = append(readiness.Issues, "素材可用性校验失败")
		} else {
			readiness.TotalMaterialCount = total
			readiness.AvailableMaterialCount = available
			if total > available {
				readiness.MaterialsReady = false
				readiness.MissingMaterialCount = total - available
				readiness.Issues = append(readiness.Issues, "部分素材已缺失或未同步")
			}
		}
	}

	return readiness
}
