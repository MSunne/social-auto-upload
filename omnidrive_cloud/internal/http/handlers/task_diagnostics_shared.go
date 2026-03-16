package handlers

import (
	"context"
	"strings"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
)

func loadPublishTaskContextForOwner(ctx context.Context, app *appstate.App, ownerUserID string, task *domain.PublishTask) (*domain.Device, *domain.PlatformAccount, *domain.ProductSkill, error) {
	device, err := app.Store.GetOwnedDevice(ctx, task.DeviceID, ownerUserID)
	if err != nil {
		return nil, nil, nil, err
	}

	var account *domain.PlatformAccount
	if task.AccountID != nil && strings.TrimSpace(*task.AccountID) != "" {
		account, err = app.Store.GetOwnedAccountByID(ctx, strings.TrimSpace(*task.AccountID), ownerUserID)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if account == nil {
		account, err = app.Store.GetAccountByDeviceTarget(ctx, task.DeviceID, task.Platform, task.AccountName)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	var skill *domain.ProductSkill
	if task.SkillID != nil && strings.TrimSpace(*task.SkillID) != "" {
		skill, err = app.Store.GetOwnedSkillByID(ctx, strings.TrimSpace(*task.SkillID), ownerUserID)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return device, account, skill, nil
}

func buildPublishTaskDiagnosticItem(ctx context.Context, app *appstate.App, ownerUserID string, task *domain.PublishTask) (domain.PublishTaskDiagnosticItem, error) {
	device, account, skill, err := loadPublishTaskContextForOwner(ctx, app, ownerUserID, task)
	if err != nil {
		return domain.PublishTaskDiagnosticItem{}, err
	}
	readiness := buildPublishTaskReadiness(ctx, app, task, device, account, skill)
	return domain.PublishTaskDiagnosticItem{
		Task:               *task,
		Readiness:          readiness,
		BlockingDimensions: publishTaskReadinessBlockingDimensions(readiness),
	}, nil
}

func summarizePublishTaskDiagnosticItems(items []domain.PublishTaskDiagnosticItem) domain.PublishTaskDiagnosticSummary {
	summary := domain.PublishTaskDiagnosticSummary{
		ByStatus:    map[string]int64{},
		ByDimension: map[string]int64{},
		ByIssueCode: map[string]int64{},
	}
	for _, item := range items {
		isReady := publishTaskReadinessAllowsExecution(item.Readiness)
		summary.TotalCount++
		summary.ByStatus[item.Task.Status]++
		if isReady {
			summary.ReadyCount++
		} else {
			summary.BlockedCount++
		}
		for _, dimension := range item.BlockingDimensions {
			summary.ByDimension[dimension]++
		}
		for _, issueCode := range item.Readiness.IssueCodes {
			summary.ByIssueCode[issueCode]++
		}
	}
	return summary
}
