package handlers

import (
	"context"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
)

// 处理decorate技能同步StatesCurrentRevision相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func decorateSkillSyncStatesWithCurrentRevision(ctx context.Context, app *appstate.App, ownerUserID string, items []domain.DeviceSkillSyncState) ([]domain.DeviceSkillSyncState, error) {
	results := make([]domain.DeviceSkillSyncState, 0, len(items))
	for _, item := range items {
		revision, err := app.Store.GetSkillRevision(ctx, item.SkillID, ownerUserID)
		if err != nil {
			return nil, err
		}
		item.DesiredRevision = normalizeTrimmedStringPtr(revision)
		item.IsCurrent = item.SyncStatus == "success" &&
			item.DesiredRevision != nil &&
			item.SyncedRevision != nil &&
			strings.TrimSpace(*item.SyncedRevision) == strings.TrimSpace(*item.DesiredRevision)
		item.NeedsSync = !item.IsCurrent
		results = append(results, item)
	}
	return results, nil
}
