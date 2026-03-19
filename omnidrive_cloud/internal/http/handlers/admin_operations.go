package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"omnidrive_cloud/internal/domain"
	httpcontext "omnidrive_cloud/internal/http/context"
	"omnidrive_cloud/internal/http/render"
	"omnidrive_cloud/internal/store"
)

type adminUpdateUserRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"isActive"`
	Notes    *string `json:"notes"`
}

type adminUpdateDeviceRequest struct {
	Name                  *string `json:"name"`
	DefaultReasoningModel *string `json:"defaultReasoningModel"`
	IsEnabled             *bool   `json:"isEnabled"`
}

type adminUpdateMediaAccountRequest struct {
	Notes *string `json:"notes"`
}

type adminUpdatePublishTaskRequest struct {
	Notes           *string   `json:"notes"`
	ExceptionReason *string   `json:"exceptionReason"`
	RiskTags        *[]string `json:"riskTags"`
}

type adminUpdateAIJobRequest struct {
	Notes           *string   `json:"notes"`
	ExceptionReason *string   `json:"exceptionReason"`
	RiskTags        *[]string `json:"riskTags"`
}

type adminBatchActionPublishTasksRequest struct {
	TaskIDs       []string    `json:"taskIds"`
	Action        string      `json:"action"`
	Message       *string     `json:"message"`
	ResolveStatus string      `json:"resolveStatus"`
	TextEvidence  *string     `json:"textEvidence"`
	Payload       interface{} `json:"payload"`
}

type adminBatchActionAIJobsRequest struct {
	JobIDs []string `json:"jobIds"`
	Action string   `json:"action"`
}

type adminBatchActionUsersRequest struct {
	UserIDs []string `json:"userIds"`
	Action  string   `json:"action"`
}

type adminBatchActionDevicesRequest struct {
	DeviceIDs []string `json:"deviceIds"`
	Action    string   `json:"action"`
}

type adminBatchActionMediaAccountsRequest struct {
	AccountIDs []string `json:"accountIds"`
	Action     string   `json:"action"`
}

func limitSlice[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func uniqueTrimmedIDs(values []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func normalizeTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func adminDeviceSummaryFromDevice(device *domain.Device) domain.AdminDeviceSummary {
	if device == nil {
		return domain.AdminDeviceSummary{}
	}
	return domain.AdminDeviceSummary{
		ID:         device.ID,
		DeviceCode: device.DeviceCode,
		Name:       device.Name,
		Status:     device.Status,
		IsEnabled:  device.IsEnabled,
		LastSeenAt: device.LastSeenAt,
	}
}

func adminAccountSummaryFromAccount(account *domain.PlatformAccount) *domain.AdminAccountSummary {
	if account == nil {
		return nil
	}
	return &domain.AdminAccountSummary{
		ID:                  account.ID,
		Platform:            account.Platform,
		AccountName:         account.AccountName,
		Status:              account.Status,
		LastMessage:         account.LastMessage,
		LastAuthenticatedAt: account.LastAuthenticatedAt,
	}
}

func adminSkillSummaryFromSkill(skill *domain.ProductSkill) *domain.AdminSkillSummary {
	if skill == nil {
		return nil
	}
	return &domain.AdminSkillSummary{
		ID:         skill.ID,
		Name:       skill.Name,
		OutputType: skill.OutputType,
		ModelName:  skill.ModelName,
		IsEnabled:  skill.IsEnabled,
	}
}

func adminUserActions(row *domain.AdminUserRow) domain.AdminUserActionState {
	if row == nil {
		return domain.AdminUserActionState{}
	}
	return domain.AdminUserActionState{
		CanUpdate:     true,
		CanDeactivate: row.User.IsActive,
		CanActivate:   !row.User.IsActive,
	}
}

func adminDeviceActions(row *domain.AdminDeviceRow) domain.AdminDeviceActionState {
	if row == nil {
		return domain.AdminDeviceActionState{}
	}
	return domain.AdminDeviceActionState{
		CanUpdate:       true,
		CanDisable:      row.Device.IsEnabled,
		CanEnable:       !row.Device.IsEnabled,
		CanForceRelease: row.Device.Load.LeasedTaskCount > 0 || row.Device.Load.LeasedAIJobCount > 0,
	}
}

func adminMediaAccountActions(row *domain.AdminMediaAccountRow) domain.AdminMediaAccountActionState {
	if row == nil {
		return domain.AdminMediaAccountActionState{}
	}
	return domain.AdminMediaAccountActionState{
		CanUpdate:   true,
		CanValidate: row.Device.IsEnabled,
		CanDelete:   row.Account.Load.TaskCount == 0 && row.Account.Load.ActiveLoginSessionCount == 0,
	}
}

func summarizeAdminPublishTaskBulkActionItems(items []domain.AdminPublishTaskBulkActionItem) domain.AdminPublishTaskBulkActionSummary {
	summary := domain.AdminPublishTaskBulkActionSummary{
		ByStatus: map[string]int64{},
		ByAction: map[string]int64{},
	}
	for _, item := range items {
		summary.SelectedCount++
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		summary.ByAction[item.Action]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "failed":
			summary.FailedCount++
		default:
			summary.SkippedCount++
		}
	}
	return summary
}

func summarizeAdminAIJobBulkActionItems(items []domain.AdminAIJobBulkActionItem) domain.AdminAIJobBulkActionSummary {
	summary := domain.AdminAIJobBulkActionSummary{
		ByStatus: map[string]int64{},
		ByAction: map[string]int64{},
	}
	for _, item := range items {
		summary.SelectedCount++
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		summary.ByAction[item.Action]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "failed":
			summary.FailedCount++
		default:
			summary.SkippedCount++
		}
	}
	return summary
}

func summarizeAdminUserBulkActionItems(items []domain.AdminUserBulkActionItem) domain.AdminUserBulkActionSummary {
	summary := domain.AdminUserBulkActionSummary{
		SelectedCount: int64(len(items)),
		ByStatus:      map[string]int64{},
		ByAction:      map[string]int64{},
	}
	for _, item := range items {
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		summary.ByAction[item.Action]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "skipped":
			summary.SkippedCount++
		default:
			summary.FailedCount++
		}
	}
	return summary
}

func summarizeAdminDeviceBulkActionItems(items []domain.AdminDeviceBulkActionItem) domain.AdminDeviceBulkActionSummary {
	summary := domain.AdminDeviceBulkActionSummary{
		SelectedCount: int64(len(items)),
		ByStatus:      map[string]int64{},
		ByAction:      map[string]int64{},
	}
	for _, item := range items {
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		summary.ByAction[item.Action]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "skipped":
			summary.SkippedCount++
		default:
			summary.FailedCount++
		}
	}
	return summary
}

func summarizeAdminMediaAccountBulkActionItems(items []domain.AdminMediaAccountBulkActionItem) domain.AdminMediaAccountBulkActionSummary {
	summary := domain.AdminMediaAccountBulkActionSummary{
		SelectedCount: int64(len(items)),
		ByStatus:      map[string]int64{},
		ByAction:      map[string]int64{},
	}
	for _, item := range items {
		summary.ProcessedCount++
		summary.ByStatus[item.Status]++
		summary.ByAction[item.Action]++
		switch item.Status {
		case "success":
			summary.SuccessCount++
		case "skipped":
			summary.SkippedCount++
		default:
			summary.FailedCount++
		}
	}
	return summary
}

func compactJSONPayload(raw json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	return json.RawMessage(trimmed)
}

func compactJSONPayloadFromBytes(raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if json.Valid(trimmed) {
		return append(json.RawMessage(nil), trimmed...)
	}
	payload, err := json.Marshal(map[string]string{"raw": string(trimmed)})
	if err != nil {
		return nil
	}
	return payload
}

func pickJobLifecycleTitle(status string) string {
	switch strings.TrimSpace(status) {
	case "queued":
		return "作业进入队列"
	case "running":
		return "AI 开始执行"
	case "completed", "success":
		return "AI 执行完成"
	case "failed":
		return "AI 执行失败"
	case "cancelled":
		return "AI 作业已取消"
	default:
		return "AI 作业状态更新"
	}
}

func pickJobLifecycleTime(job domain.AIJob) time.Time {
	if job.FinishedAt != nil {
		return *job.FinishedAt
	}
	if !job.UpdatedAt.IsZero() {
		return job.UpdatedAt
	}
	return job.CreatedAt
}

func buildAIJobExecutionLogs(workspace *domain.AdminAIJobWorkspace) []domain.AdminExecutionLog {
	if workspace == nil {
		return nil
	}

	job := workspace.Record.Job
	entries := make([]domain.AdminExecutionLog, 0, len(workspace.RecentAudits)+len(workspace.Artifacts)+len(workspace.PublishTasks)+len(workspace.BillingUsageEvents)+4)

	createdMessage := fmt.Sprintf("来源：%s", strings.TrimSpace(job.Source))
	if job.RunAt != nil {
		createdMessage += fmt.Sprintf(" · 计划执行：%s", job.RunAt.Format(time.RFC3339))
	}
	entries = append(entries, domain.AdminExecutionLog{
		ID:        "job-created",
		Stage:     "job",
		Status:    "created",
		Title:     "AI 作业已创建",
		Message:   &createdMessage,
		Source:    "system",
		Timestamp: job.CreatedAt,
		Payload:   compactJSONPayload(job.InputPayload),
	})

	if job.RunAt != nil {
		runMessage := "到达计划执行时间，准备进入模型调用。"
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "job-scheduled",
			Stage:     "schedule",
			Status:    "scheduled",
			Title:     "进入调度窗口",
			Message:   &runMessage,
			Source:    "scheduler",
			Timestamp: *job.RunAt,
		})
	}

	if !job.UpdatedAt.Equal(job.CreatedAt) || job.Message != nil || len(job.OutputPayload) > 0 || strings.TrimSpace(job.Status) != "queued" {
		payload := compactJSONPayload(job.OutputPayload)
		if payload == nil && len(job.InputPayload) > 0 && strings.TrimSpace(job.Status) == "failed" {
			payload = compactJSONPayload(job.InputPayload)
		}
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "job-lifecycle",
			Stage:     "generation",
			Status:    strings.TrimSpace(job.Status),
			Title:     pickJobLifecycleTitle(job.Status),
			Message:   job.Message,
			Source:    "ai_worker",
			Timestamp: pickJobLifecycleTime(job),
			Payload:   payload,
		})
	}

	for _, artifact := range workspace.Artifacts {
		label := artifact.ArtifactType
		if artifact.FileName != nil && strings.TrimSpace(*artifact.FileName) != "" {
			label = fmt.Sprintf("%s · %s", artifact.ArtifactType, strings.TrimSpace(*artifact.FileName))
		}
		message := fmt.Sprintf("产物来源：%s", strings.TrimSpace(artifact.Source))
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "artifact-" + artifact.ID,
			Stage:     "artifact",
			Status:    "stored",
			Title:     "生成产物已落库",
			Message:   stringPtr(label + " · " + message),
			Source:    "artifact_store",
			Timestamp: artifact.CreatedAt,
			Payload:   compactJSONPayload(artifact.Payload),
		})
	}

	for _, event := range workspace.BillingUsageEvents {
		title := "计费记录已写入"
		if event.MeterName != nil && strings.TrimSpace(*event.MeterName) != "" {
			title = fmt.Sprintf("计费：%s", strings.TrimSpace(*event.MeterName))
		}
		message := fmt.Sprintf("计量项 %s，数量 %d，状态 %s", strings.TrimSpace(event.MeterCode), event.UsageQuantity, strings.TrimSpace(event.BillStatus))
		if event.BillMessage != nil && strings.TrimSpace(*event.BillMessage) != "" {
			message += " · " + strings.TrimSpace(*event.BillMessage)
		}
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "billing-" + event.ID,
			Stage:     "billing",
			Status:    strings.TrimSpace(event.BillStatus),
			Title:     title,
			Message:   &message,
			Source:    "billing",
			Timestamp: event.CreatedAt,
			Payload:   compactJSONPayload(event.Payload),
		})
	}

	for _, task := range workspace.PublishTasks {
		message := fmt.Sprintf("%s / %s", strings.TrimSpace(task.Platform), strings.TrimSpace(task.AccountName))
		if task.Message != nil && strings.TrimSpace(*task.Message) != "" {
			message += " · " + strings.TrimSpace(*task.Message)
		}
		payload := compactJSONPayload(task.MediaPayload)
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "publish-task-" + task.ID,
			Stage:     "publish",
			Status:    strings.TrimSpace(task.Status),
			Title:     "已关联发布任务",
			Message:   &message,
			Source:    "omnibull",
			Timestamp: task.CreatedAt,
			Payload:   payload,
		})
	}

	for _, audit := range workspace.RecentAudits {
		message := audit.Message
		entries = append(entries, domain.AdminExecutionLog{
			ID:        "audit-" + audit.ID,
			Stage:     "audit",
			Status:    strings.TrimSpace(audit.Status),
			Title:     strings.TrimSpace(audit.Title),
			Message:   message,
			Source:    strings.TrimSpace(audit.Source),
			Timestamp: audit.CreatedAt,
			Payload:   compactJSONPayloadFromBytes(audit.Payload),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Timestamp.Equal(entries[j].Timestamp) {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	return entries
}

func (h *AdminConsoleHandler) recordAdminAction(ctx context.Context, resourceType string, resourceID *string, action string, title string, status string, message *string, payload []byte) {
	admin := httpcontext.CurrentAdmin(ctx)
	if admin == nil {
		return
	}
	recordAdminAuditLog(h.app, ctx, store.CreateAdminAuditLogInput{
		AdminUserID:  stringPtr(admin.ID),
		AdminEmail:   stringPtr(admin.Email),
		AdminName:    stringPtr(admin.Name),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Title:        title,
		Source:       "admin_console",
		Status:       status,
		Message:      message,
		Payload:      payload,
	})
}

func buildAdminAIJobBridgeState(job *domain.AIJob, artifactCount int64, mirroredArtifactCount int64, publishTaskCount int64) domain.AIJobBridgeState {
	if job == nil {
		return domain.AIJobBridgeState{}
	}

	stage := "queued_generation"
	if job.Source == "omnibull_local" {
		switch job.Status {
		case "running":
			stage = "generating"
		case "success", "completed":
			stage = "awaiting_omnibull_import"
		case "failed":
			stage = "failed"
		case "cancelled":
			stage = "cancelled"
		}
		switch strings.TrimSpace(job.DeliveryStatus) {
		case "imported":
			stage = "mirrored_to_omnibull"
		case "publish_queued":
			stage = "publish_queued_on_omnibull"
		case "publishing":
			stage = "publishing_on_omnibull"
		case "success", "completed":
			stage = "published_on_omnibull"
		case "failed":
			stage = "publish_failed_on_omnibull"
		case "needs_verify":
			stage = "publish_needs_verify_on_omnibull"
		case "cancelled":
			stage = "cancelled_on_omnibull"
		}
	} else {
		switch job.Status {
		case "running":
			stage = "generating"
		case "success", "completed":
			stage = "output_ready"
		case "failed":
			stage = "failed"
		case "cancelled":
			stage = "cancelled"
		}
		if publishTaskCount > 0 {
			stage = "publish_tasks_created"
		}
	}

	return domain.AIJobBridgeState{
		Source:                 job.Source,
		GenerationSide:         "omnidrive_cloud",
		TargetDeviceID:         job.DeviceID,
		LocalTaskID:            job.LocalTaskID,
		LocalPublishTaskID:     job.LocalPublishTaskID,
		DeliveryStage:          stage,
		ArtifactCount:          int(artifactCount),
		MirroredArtifactCount:  int(mirroredArtifactCount),
		LinkedPublishTaskCount: int(publishTaskCount),
	}
}

func (h *AdminConsoleHandler) decorateAdminTaskRow(ctx context.Context, row *domain.AdminPublishTaskRow, includeRuntime bool) (*domain.PublishTaskRuntimeState, error) {
	if row == nil {
		return nil, nil
	}

	row.Actions = computePublishTaskActions(&row.Task, int(row.MaterialCount))

	var runtimeState *domain.PublishTaskRuntimeState
	if includeRuntime {
		var err error
		runtimeState, err = h.app.Store.GetPublishTaskRuntimeStateByTaskID(ctx, row.Task.ID)
		if err != nil {
			return nil, err
		}
	}
	row.Bridge = buildPublishTaskBridgeState(&row.Task, runtimeState)

	if row.Owner == nil || strings.TrimSpace(row.Owner.ID) == "" {
		return runtimeState, nil
	}

	device, account, skill, err := loadPublishTaskContextForOwner(ctx, h.app, row.Owner.ID, &row.Task)
	if err != nil {
		return nil, err
	}

	row.Readiness = buildPublishTaskReadiness(ctx, h.app, &row.Task, device, account, skill)
	row.BlockingDimensions = publishTaskReadinessBlockingDimensions(row.Readiness)
	if row.Device.ID == "" && device != nil {
		row.Device = adminDeviceSummaryFromDevice(device)
	}
	if row.Account == nil {
		row.Account = adminAccountSummaryFromAccount(account)
	}
	if row.Skill == nil {
		row.Skill = adminSkillSummaryFromSkill(skill)
	}

	return runtimeState, nil
}

func (h *AdminConsoleHandler) decorateAdminAIJobRow(row *domain.AdminAIJobRow) {
	if row == nil {
		return
	}
	row.Actions = computeAIJobActions(&row.Job, int(row.ArtifactCount))
	row.Bridge = buildAdminAIJobBridgeState(&row.Job, row.ArtifactCount, row.MirroredArtifactCount, row.PublishTaskCount)
}

func (h *AdminConsoleHandler) decorateAdminUserRow(row *domain.AdminUserRow) {
	if row == nil {
		return
	}
	row.Actions = adminUserActions(row)
}

func (h *AdminConsoleHandler) decorateAdminDeviceRow(row *domain.AdminDeviceRow) {
	if row == nil {
		return
	}
	row.Actions = adminDeviceActions(row)
}

func (h *AdminConsoleHandler) decorateAdminMediaAccountRow(row *domain.AdminMediaAccountRow) {
	if row == nil {
		return
	}
	row.Actions = adminMediaAccountActions(row)
}

func (h *AdminConsoleHandler) loadAdminTaskRow(ctx context.Context, taskID string, includeRuntime bool) (*domain.AdminPublishTaskRow, *domain.PublishTaskRuntimeState, error) {
	row, err := h.app.Store.GetAdminTaskByID(ctx, taskID)
	if err != nil || row == nil {
		return row, nil, err
	}
	runtimeState, err := h.decorateAdminTaskRow(ctx, row, includeRuntime)
	if err != nil {
		return nil, nil, err
	}
	return row, runtimeState, nil
}

func (h *AdminConsoleHandler) loadAdminAIJobRow(ctx context.Context, jobID string) (*domain.AdminAIJobRow, error) {
	row, err := h.app.Store.GetAdminAIJobByID(ctx, jobID)
	if err != nil || row == nil {
		return row, err
	}
	h.decorateAdminAIJobRow(row)
	return row, nil
}

func (h *AdminConsoleHandler) loadAdminMediaAccountRow(ctx context.Context, accountID string) (*domain.AdminMediaAccountRow, error) {
	row, err := h.app.Store.GetAdminAccountByID(ctx, accountID)
	if err != nil || row == nil {
		return row, err
	}
	h.decorateAdminMediaAccountRow(row)
	return row, nil
}

func (h *AdminConsoleHandler) createAdminMediaAccountValidationSession(ctx context.Context, record *domain.AdminMediaAccountRow) (*domain.LoginSession, *string, error) {
	if record == nil {
		return nil, auditStringPtr("Media account not found"), nil
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		return nil, auditStringPtr("Media account has no owner context"), nil
	}
	if !record.Device.IsEnabled {
		return nil, auditStringPtr("Device is disabled"), nil
	}

	existingSession, err := findReusableLoginSession(ctx, h.app.Store, record.Owner.ID, record.Account.DeviceID, record.Account.Platform, record.Account.AccountName)
	if err != nil {
		return nil, nil, err
	}
	if existingSession != nil {
		return existingSession, nil, nil
	}

	message := "等待本地 OmniBull 重新验证账号"
	session, err := h.app.Store.CreateLoginSession(ctx, store.CreateLoginSessionInput{
		ID:          uuid.NewString(),
		DeviceID:    record.Account.DeviceID,
		UserID:      record.Owner.ID,
		Platform:    record.Account.Platform,
		AccountName: record.Account.AccountName,
		Status:      "pending",
		Message:     &message,
	})
	if err != nil {
		return nil, nil, err
	}

	recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
		OwnerUserID:  record.Owner.ID,
		ResourceType: "login_session",
		ResourceID:   &session.ID,
		Action:       "admin_validate_account",
		Title:        "运营后台发起账号重验",
		Source:       record.Account.Platform,
		Status:       session.Status,
		Message:      session.Message,
		Payload: mustJSONBytes(map[string]any{
			"deviceId":    record.Account.DeviceID,
			"accountId":   record.Account.ID,
			"accountName": record.Account.AccountName,
		}),
	})
	h.recordAdminAction(ctx, "media_account", &record.Account.ID, "validate", "发起媒体账号重验", "success", auditStringPtr("已创建账号重验登录会话"), mustJSONBytes(map[string]any{
		"loginSessionId": session.ID,
		"deviceId":       record.Account.DeviceID,
		"platform":       record.Account.Platform,
		"accountName":    record.Account.AccountName,
	}))
	return session, nil, nil
}

func (h *AdminConsoleHandler) createAdminRemoteLoginSession(ctx context.Context, record *domain.AdminDeviceRow, platform string, accountName string) (*domain.LoginSession, *string, error) {
	if record == nil {
		return nil, auditStringPtr("Device not found"), nil
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		return nil, auditStringPtr("Device has no owner context"), nil
	}
	if !record.Device.IsEnabled {
		return nil, auditStringPtr("Device is disabled"), nil
	}

	platform = strings.TrimSpace(platform)
	accountName = strings.TrimSpace(accountName)
	if platform == "" || accountName == "" {
		return nil, auditStringPtr("Platform and account name are required"), nil
	}

	existingSession, err := findReusableLoginSession(ctx, h.app.Store, record.Owner.ID, record.Device.ID, platform, accountName)
	if err != nil {
		return nil, nil, err
	}
	if existingSession != nil {
		return existingSession, nil, nil
	}

	message := "等待本地 OmniBull 拉起登录流程"
	session, err := h.app.Store.CreateLoginSession(ctx, store.CreateLoginSessionInput{
		ID:          uuid.NewString(),
		DeviceID:    record.Device.ID,
		UserID:      record.Owner.ID,
		Platform:    platform,
		AccountName: accountName,
		Status:      "pending",
		Message:     &message,
	})
	if err != nil {
		return nil, nil, err
	}

	recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
		OwnerUserID:  record.Owner.ID,
		ResourceType: "login_session",
		ResourceID:   &session.ID,
		Action:       "admin_remote_login",
		Title:        "运营后台发起远程账号登录",
		Source:       platform,
		Status:       session.Status,
		Message:      session.Message,
		Payload: mustJSONBytes(map[string]any{
			"deviceId":    record.Device.ID,
			"deviceCode":  record.Device.DeviceCode,
			"accountName": accountName,
		}),
	})
	h.recordAdminAction(ctx, "device", &record.Device.ID, "remote_login", "发起远程账号登录", "success", auditStringPtr("已创建远程登录会话"), mustJSONBytes(map[string]any{
		"loginSessionId": session.ID,
		"deviceId":       record.Device.ID,
		"deviceCode":     record.Device.DeviceCode,
		"platform":       platform,
		"accountName":    accountName,
	}))
	return session, nil, nil
}

func (h *AdminConsoleHandler) deleteAdminMediaAccount(ctx context.Context, record *domain.AdminMediaAccountRow) (bool, *string, int64, int64, error) {
	if record == nil {
		return false, auditStringPtr("Media account not found"), 0, 0, nil
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		return false, auditStringPtr("Media account has no owner context"), 0, 0, nil
	}

	taskCount, activeLoginSessionCount, err := h.app.Store.GetAccountUsageSummary(ctx, record.Account.ID, record.Owner.ID)
	if err != nil {
		return false, nil, 0, 0, err
	}
	if taskCount > 0 || activeLoginSessionCount > 0 {
		return false, auditStringPtr("Media account is still referenced by tasks or active login sessions"), taskCount, activeLoginSessionCount, nil
	}

	deleted, err := h.app.Store.DeleteOwnedAccount(ctx, record.Account.ID, record.Owner.ID)
	if err != nil {
		return false, nil, 0, 0, err
	}
	if !deleted {
		return false, auditStringPtr("Media account not found"), 0, 0, nil
	}

	recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
		OwnerUserID:  record.Owner.ID,
		ResourceType: "account",
		ResourceID:   &record.Account.ID,
		Action:       "admin_delete",
		Title:        "运营后台删除平台账号镜像",
		Source:       record.Account.Platform,
		Status:       "success",
		Message:      auditStringPtr("云端账号镜像已由运营后台删除"),
		Payload: mustJSONBytes(map[string]any{
			"deviceId":    record.Account.DeviceID,
			"accountName": record.Account.AccountName,
		}),
	})
	h.recordAdminAction(ctx, "media_account", &record.Account.ID, "delete", "删除媒体账号", "success", auditStringPtr("媒体账号镜像已删除"), mustJSONBytes(map[string]any{
		"deviceId":    record.Account.DeviceID,
		"platform":    record.Account.Platform,
		"accountName": record.Account.AccountName,
	}))
	return true, nil, 0, 0, nil
}

func (h *AdminConsoleHandler) forceReleaseDeviceLeases(ctx context.Context, record *domain.AdminDeviceRow) (*domain.AdminDeviceForceReleaseResult, *string, error) {
	if record == nil {
		return nil, auditStringPtr("Device not found"), nil
	}
	h.decorateAdminDeviceRow(record)

	releasedTasks, err := h.app.Store.ForceReleasePublishTaskLeasesByDevice(ctx, record.Device.ID, auditStringPtr("任务租约已由运营后台按设备释放"))
	if err != nil {
		return nil, nil, err
	}
	releasedAIJobs, err := h.app.Store.ForceReleaseAIJobLeasesByDevice(ctx, record.Device.ID, auditStringPtr("AI 任务租约已由运营后台按设备释放"))
	if err != nil {
		return nil, nil, err
	}
	if len(releasedTasks) == 0 && len(releasedAIJobs) == 0 {
		return nil, auditStringPtr("Device has no active publish task or AI job leases to release"), nil
	}

	releasedTaskIDs := make([]string, 0, len(releasedTasks))
	for index := range releasedTasks {
		task := releasedTasks[index]
		releasedTaskIDs = append(releasedTaskIDs, task.ID)
		_, _ = h.app.Store.CreatePublishTaskEvent(ctx, store.CreatePublishTaskEventInput{
			ID:        uuid.NewString(),
			TaskID:    task.ID,
			EventType: "force_released",
			Source:    "admin_console",
			Status:    task.Status,
			Message:   task.Message,
		})
		if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
			recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
				OwnerUserID:  record.Owner.ID,
				ResourceType: "publish_task",
				ResourceID:   &task.ID,
				Action:       "force_release",
				Title:        "按设备释放任务租约",
				Source:       "admin_console",
				Status:       task.Status,
				Message:      task.Message,
				Payload: mustJSONBytes(map[string]any{
					"deviceId": record.Device.ID,
				}),
			})
		}
		h.recordAdminAction(ctx, "publish_task", &task.ID, "force_release", "按设备释放发布任务租约", "success", task.Message, mustJSONBytes(map[string]any{
			"deviceId": record.Device.ID,
		}))
	}

	releasedAIJobIDs := make([]string, 0, len(releasedAIJobs))
	for index := range releasedAIJobs {
		job := releasedAIJobs[index]
		releasedAIJobIDs = append(releasedAIJobIDs, job.ID)
		recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
			OwnerUserID:  job.OwnerUserID,
			ResourceType: "ai_job",
			ResourceID:   &job.ID,
			Action:       "force_release",
			Title:        "按设备释放 AI 任务租约",
			Source:       "admin_console",
			Status:       job.Status,
			Message:      job.Message,
			Payload: mustJSONBytes(map[string]any{
				"deviceId": record.Device.ID,
			}),
		})
		h.recordAdminAction(ctx, "ai_job", &job.ID, "force_release", "按设备释放 AI 任务租约", "success", job.Message, mustJSONBytes(map[string]any{
			"deviceId": record.Device.ID,
		}))
	}

	summaryPayload := mustJSONBytes(map[string]any{
		"releasedPublishTaskIds":   releasedTaskIDs,
		"releasedAiJobIds":         releasedAIJobIDs,
		"releasedPublishTaskCount": len(releasedTaskIDs),
		"releasedAiJobCount":       len(releasedAIJobIDs),
	})
	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		recordAuditEvent(h.app, ctx, store.CreateAuditEventInput{
			OwnerUserID:  record.Owner.ID,
			ResourceType: "device",
			ResourceID:   &record.Device.ID,
			Action:       "force_release",
			Title:        "运营后台释放设备占用租约",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("设备持有的发布任务和 AI 作业租约已由运营后台释放"),
			Payload:      summaryPayload,
		})
	}
	h.recordAdminAction(ctx, "device", &record.Device.ID, "force_release", "释放设备占用租约", "success", auditStringPtr("设备持有的发布任务和 AI 作业租约已由运营后台释放"), summaryPayload)

	updated, err := h.app.Store.GetAdminDeviceByID(ctx, record.Device.ID)
	if err != nil {
		return nil, nil, err
	}
	if updated == nil {
		return nil, auditStringPtr("Device not found"), nil
	}
	h.decorateAdminDeviceRow(updated)

	return &domain.AdminDeviceForceReleaseResult{
		Record:                   *updated,
		ReleasedPublishTaskIDs:   releasedTaskIDs,
		ReleasedAIJobIDs:         releasedAIJobIDs,
		ReleasedPublishTaskCount: int64(len(releasedTaskIDs)),
		ReleasedAIJobCount:       int64(len(releasedAIJobIDs)),
		ServerTime:               time.Now().UTC(),
	}, nil, nil
}

func (h *AdminConsoleHandler) DetailUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userId"))
	if userID == "" {
		render.Error(w, http.StatusBadRequest, "userId is required")
		return
	}

	record, err := h.app.Store.GetAdminUserByID(r.Context(), userID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin user")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "User not found")
		return
	}
	h.decorateAdminUserRow(record)

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userId"))
	if userID == "" {
		render.Error(w, http.StatusBadRequest, "userId is required")
		return
	}

	var payload adminUpdateUserRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Name != nil && strings.TrimSpace(*payload.Name) == "" {
		render.Error(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if payload.Name == nil && payload.IsActive == nil && payload.Notes == nil {
		render.Error(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	record, err := h.app.Store.UpdateAdminUserTarget(r.Context(), userID, store.UpdateAdminUserTargetInput{
		Name:         payload.Name,
		IsActive:     payload.IsActive,
		Notes:        normalizeTrimmedString(payload.Notes),
		NotesTouched: payload.Notes != nil,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update user")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "User not found")
		return
	}
	h.decorateAdminUserRow(record)

	admin := httpcontext.CurrentAdmin(r.Context())
	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  record.User.ID,
		ResourceType: "user",
		ResourceID:   &record.User.ID,
		Action:       "admin_update",
		Title:        "运营后台更新用户状态",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("用户基础信息已由运营后台更新"),
		Payload: mustJSONBytes(map[string]any{
			"name":     payload.Name,
			"isActive": payload.IsActive,
			"notes":    normalizeTrimmedString(payload.Notes),
		}),
	})
	if admin != nil {
		recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
			AdminUserID:  stringPtr(admin.ID),
			AdminEmail:   stringPtr(admin.Email),
			AdminName:    stringPtr(admin.Name),
			ResourceType: "user",
			ResourceID:   &record.User.ID,
			Action:       "update",
			Title:        "更新用户",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("用户基础信息已更新"),
			Payload: mustJSONBytes(map[string]any{
				"name":     payload.Name,
				"isActive": payload.IsActive,
				"notes":    normalizeTrimmedString(payload.Notes),
			}),
		})
	}

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) BulkActionUsers(w http.ResponseWriter, r *http.Request) {
	var payload adminBatchActionUsersRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Action = strings.TrimSpace(strings.ToLower(payload.Action))
	switch payload.Action {
	case "activate", "deactivate":
	default:
		render.Error(w, http.StatusBadRequest, "action must be one of activate, deactivate")
		return
	}

	userIDs := uniqueTrimmedIDs(payload.UserIDs)
	if len(userIDs) == 0 {
		render.Error(w, http.StatusBadRequest, "userIds is required")
		return
	}

	items := make([]domain.AdminUserBulkActionItem, 0, len(userIDs))
	targetActive := payload.Action == "activate"
	for _, userID := range userIDs {
		item := domain.AdminUserBulkActionItem{
			Action: payload.Action,
			Status: "failed",
		}

		record, err := h.app.Store.GetAdminUserByID(r.Context(), userID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load user for bulk action")
			return
		}
		if record == nil {
			item.Message = auditStringPtr("User not found")
			items = append(items, item)
			continue
		}
		h.decorateAdminUserRow(record)
		item.RecordBefore = *record

		if targetActive && !record.Actions.CanActivate {
			item.Status = "skipped"
			item.Message = auditStringPtr("User is already active")
			items = append(items, item)
			continue
		}
		if !targetActive && !record.Actions.CanDeactivate {
			item.Status = "skipped"
			item.Message = auditStringPtr("User is already inactive")
			items = append(items, item)
			continue
		}

		updated, err := h.app.Store.UpdateAdminUserTarget(r.Context(), userID, store.UpdateAdminUserTargetInput{
			IsActive: &targetActive,
		})
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to execute bulk user action")
			return
		}
		if updated == nil {
			item.Message = auditStringPtr("User not found")
			items = append(items, item)
			continue
		}
		h.decorateAdminUserRow(updated)
		item.Status = "success"
		item.RecordAfter = updated
		if targetActive {
			item.Message = auditStringPtr("User activated")
		} else {
			item.Message = auditStringPtr("User deactivated")
		}

		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  updated.User.ID,
			ResourceType: "user",
			ResourceID:   &updated.User.ID,
			Action:       "admin_update",
			Title:        "运营后台批量更新用户状态",
			Source:       "admin_console",
			Status:       "success",
			Message:      item.Message,
			Payload: mustJSONBytes(map[string]any{
				"action":   payload.Action,
				"isActive": targetActive,
			}),
		})
		if targetActive {
			h.recordAdminAction(r.Context(), "user", &updated.User.ID, "activate", "批量启用用户", "success", auditStringPtr("用户已由运营后台批量启用"), nil)
		} else {
			h.recordAdminAction(r.Context(), "user", &updated.User.ID, "deactivate", "批量停用用户", "success", auditStringPtr("用户已由运营后台批量停用"), nil)
		}

		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.AdminUserBulkActionResult{
		Items:      items,
		Summary:    summarizeAdminUserBulkActionItems(items),
		ServerTime: time.Now().UTC(),
	})
}

func (h *AdminConsoleHandler) UserWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "userId"))
	if userID == "" {
		render.Error(w, http.StatusBadRequest, "userId is required")
		return
	}

	record, err := h.app.Store.GetAdminUserByID(r.Context(), userID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin user")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "User not found")
		return
	}
	h.decorateAdminUserRow(record)

	billingSummary, err := h.app.Store.GetBillingSummaryByUser(r.Context(), userID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user billing summary")
		return
	}
	devices, err := h.app.Store.ListDevicesByOwner(r.Context(), userID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user devices")
		return
	}
	accounts, err := h.app.Store.ListAccountsByOwner(r.Context(), userID, "")
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user media accounts")
		return
	}
	publishTasks, err := h.app.Store.ListPublishTasksByOwner(r.Context(), userID, store.ListPublishTasksFilter{Limit: 12})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user publish tasks")
		return
	}
	aiJobs, err := h.app.Store.ListAIJobsByOwner(r.Context(), userID, store.ListAIJobsFilter{Limit: 12})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user AI jobs")
		return
	}
	orders, err := h.app.Store.ListRechargeOrdersByUser(r.Context(), userID, 12)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user orders")
		return
	}
	walletLedgers, err := h.app.Store.ListWalletLedgerByUser(r.Context(), userID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user wallet ledgers")
		return
	}
	recentAudits, err := h.app.Store.ListRecentAdminAuditsByUserID(r.Context(), userID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load user audits")
		return
	}

	render.JSON(w, http.StatusOK, domain.AdminUserWorkspace{
		Record:         *record,
		BillingSummary: *billingSummary,
		Devices:        limitSlice(devices, 12),
		MediaAccounts:  limitSlice(accounts, 12),
		PublishTasks:   limitSlice(publishTasks, 12),
		AIJobs:         limitSlice(aiJobs, 12),
		Orders:         orders,
		WalletLedgers:  limitSlice(walletLedgers, 12),
		RecentAudits:   recentAudits,
	})
}

func (h *AdminConsoleHandler) DetailDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	record, err := h.app.Store.GetAdminDeviceByID(r.Context(), deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin device")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	h.decorateAdminDeviceRow(record)

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) UpdateDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	var payload adminUpdateDeviceRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Name != nil && strings.TrimSpace(*payload.Name) == "" {
		render.Error(w, http.StatusBadRequest, "name cannot be empty")
		return
	}
	if payload.DefaultReasoningModel != nil && strings.TrimSpace(*payload.DefaultReasoningModel) == "" {
		render.Error(w, http.StatusBadRequest, "defaultReasoningModel cannot be empty")
		return
	}
	if payload.Name == nil && payload.DefaultReasoningModel == nil && payload.IsEnabled == nil {
		render.Error(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	record, err := h.app.Store.UpdateAdminDeviceTarget(r.Context(), deviceID, store.UpdateAdminDeviceTargetInput{
		Name:                  payload.Name,
		DefaultReasoningModel: payload.DefaultReasoningModel,
		IsEnabled:             payload.IsEnabled,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update device")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	h.decorateAdminDeviceRow(record)

	admin := httpcontext.CurrentAdmin(r.Context())
	if record.Owner != nil {
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  record.Owner.ID,
			ResourceType: "device",
			ResourceID:   &record.Device.ID,
			Action:       "admin_update",
			Title:        "运营后台更新设备配置",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("设备配置已由运营后台更新"),
			Payload: mustJSONBytes(map[string]any{
				"name":                  payload.Name,
				"defaultReasoningModel": payload.DefaultReasoningModel,
				"isEnabled":             payload.IsEnabled,
			}),
		})
	}
	if admin != nil {
		recordAdminAuditLog(h.app, r.Context(), store.CreateAdminAuditLogInput{
			AdminUserID:  stringPtr(admin.ID),
			AdminEmail:   stringPtr(admin.Email),
			AdminName:    stringPtr(admin.Name),
			ResourceType: "device",
			ResourceID:   &record.Device.ID,
			Action:       "update",
			Title:        "更新设备配置",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("设备配置已更新"),
			Payload: mustJSONBytes(map[string]any{
				"name":                  payload.Name,
				"defaultReasoningModel": payload.DefaultReasoningModel,
				"isEnabled":             payload.IsEnabled,
			}),
		})
	}

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) DeviceWorkspace(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	record, err := h.app.Store.GetAdminDeviceByID(r.Context(), deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin device")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	h.decorateAdminDeviceRow(record)

	workspace := domain.AdminDeviceWorkspace{
		Record: *record,
	}
	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		workspace.RecentTasks, err = h.app.Store.ListPublishTasksByOwner(r.Context(), record.Owner.ID, store.ListPublishTasksFilter{
			DeviceID: deviceID,
			Limit:    8,
		})
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device tasks")
			return
		}
		workspace.RecentAIJobs, err = h.app.Store.ListAIJobsByOwner(r.Context(), record.Owner.ID, store.ListAIJobsFilter{
			DeviceID: deviceID,
			Limit:    8,
		})
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device AI jobs")
			return
		}
		workspace.ActiveLoginSessions, err = h.app.Store.ListActiveLoginSessionsByOwner(r.Context(), record.Owner.ID, deviceID, 6)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device login sessions")
			return
		}
		workspace.RecentAccounts, err = h.app.Store.ListAccountsByOwner(r.Context(), record.Owner.ID, deviceID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device accounts")
			return
		}
		workspace.RecentAccounts = limitSlice(workspace.RecentAccounts, 8)
		workspace.MaterialRoots, err = h.app.Store.ListMaterialRootsByOwner(r.Context(), record.Owner.ID, deviceID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device material roots")
			return
		}
		workspace.SkillSyncStates, err = h.app.Store.ListSkillSyncStatesByDevice(r.Context(), record.Owner.ID, deviceID, 12)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device skill sync states")
			return
		}
		workspace.SkillSyncStates, err = decorateSkillSyncStatesWithCurrentRevision(r.Context(), h.app, record.Owner.ID, workspace.SkillSyncStates)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to decorate skill sync states")
			return
		}
	}

	render.JSON(w, http.StatusOK, workspace)
}

func (h *AdminConsoleHandler) ForceReleaseDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(chi.URLParam(r, "deviceId"))
	if deviceID == "" {
		render.Error(w, http.StatusBadRequest, "deviceId is required")
		return
	}

	record, err := h.app.Store.GetAdminDeviceByID(r.Context(), deviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin device")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	result, message, err := h.forceReleaseDeviceLeases(r.Context(), record)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to release device leases")
		return
	}
	if result == nil {
		render.Error(w, http.StatusConflict, trimmedStringValue(message))
		return
	}

	render.JSON(w, http.StatusOK, result)
}

func (h *AdminConsoleHandler) BulkActionDevices(w http.ResponseWriter, r *http.Request) {
	var payload adminBatchActionDevicesRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Action = strings.TrimSpace(strings.ToLower(payload.Action))
	switch payload.Action {
	case "enable", "disable", "force_release":
	default:
		render.Error(w, http.StatusBadRequest, "action must be one of enable, disable, force_release")
		return
	}

	deviceIDs := uniqueTrimmedIDs(payload.DeviceIDs)
	if len(deviceIDs) == 0 {
		render.Error(w, http.StatusBadRequest, "deviceIds is required")
		return
	}

	items := make([]domain.AdminDeviceBulkActionItem, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		item := domain.AdminDeviceBulkActionItem{
			Action: payload.Action,
			Status: "failed",
		}

		record, err := h.app.Store.GetAdminDeviceByID(r.Context(), deviceID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load device for bulk action")
			return
		}
		if record == nil {
			item.Message = auditStringPtr("Device not found")
			items = append(items, item)
			continue
		}
		h.decorateAdminDeviceRow(record)
		item.RecordBefore = *record

		switch payload.Action {
		case "enable", "disable":
			targetEnabled := payload.Action == "enable"
			if targetEnabled && !record.Actions.CanEnable {
				item.Status = "skipped"
				item.Message = auditStringPtr("Device is already enabled")
				items = append(items, item)
				continue
			}
			if !targetEnabled && !record.Actions.CanDisable {
				item.Status = "skipped"
				item.Message = auditStringPtr("Device is already disabled")
				items = append(items, item)
				continue
			}

			updated, err := h.app.Store.UpdateAdminDeviceTarget(r.Context(), deviceID, store.UpdateAdminDeviceTargetInput{
				IsEnabled: &targetEnabled,
			})
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk device action")
				return
			}
			if updated == nil {
				item.Message = auditStringPtr("Device not found")
				items = append(items, item)
				continue
			}
			h.decorateAdminDeviceRow(updated)
			item.Status = "success"
			item.RecordAfter = updated
			if targetEnabled {
				item.Message = auditStringPtr("Device enabled")
			} else {
				item.Message = auditStringPtr("Device disabled")
			}

			if updated.Owner != nil {
				recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
					OwnerUserID:  updated.Owner.ID,
					ResourceType: "device",
					ResourceID:   &updated.Device.ID,
					Action:       "admin_update",
					Title:        "运营后台批量更新设备状态",
					Source:       "admin_console",
					Status:       "success",
					Message:      item.Message,
					Payload: mustJSONBytes(map[string]any{
						"action":    payload.Action,
						"isEnabled": targetEnabled,
					}),
				})
			}
			if targetEnabled {
				h.recordAdminAction(r.Context(), "device", &updated.Device.ID, "enable", "批量启用设备", "success", auditStringPtr("设备已由运营后台批量启用"), nil)
			} else {
				h.recordAdminAction(r.Context(), "device", &updated.Device.ID, "disable", "批量停用设备", "success", auditStringPtr("设备已由运营后台批量停用"), nil)
			}
		case "force_release":
			result, message, err := h.forceReleaseDeviceLeases(r.Context(), record)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk device force release")
				return
			}
			if result == nil {
				item.Status = "skipped"
				item.Message = message
				items = append(items, item)
				continue
			}
			item.Status = "success"
			item.Message = auditStringPtr("Device leases released")
			recordAfter := result.Record
			item.RecordAfter = &recordAfter
			item.ReleasedPublishTaskCount = result.ReleasedPublishTaskCount
			item.ReleasedAIJobCount = result.ReleasedAIJobCount
		}

		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.AdminDeviceBulkActionResult{
		Items:      items,
		Summary:    summarizeAdminDeviceBulkActionItems(items),
		ServerTime: time.Now().UTC(),
	})
}

func (h *AdminConsoleHandler) ListMediaAccounts(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminAccounts(r.Context(), store.AdminAccountListFilter{
		Query:    strings.TrimSpace(r.URL.Query().Get("query")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Platform: strings.TrimSpace(r.URL.Query().Get("platform")),
		UserID:   strings.TrimSpace(r.URL.Query().Get("userId")),
		DeviceID: strings.TrimSpace(r.URL.Query().Get("deviceId")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin media accounts")
		return
	}
	for index := range items {
		h.decorateAdminMediaAccountRow(&items[index])
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":    strings.TrimSpace(r.URL.Query().Get("query")),
		"status":   strings.TrimSpace(r.URL.Query().Get("status")),
		"platform": strings.TrimSpace(r.URL.Query().Get("platform")),
		"userId":   strings.TrimSpace(r.URL.Query().Get("userId")),
		"deviceId": strings.TrimSpace(r.URL.Query().Get("deviceId")),
	})
}

func (h *AdminConsoleHandler) DetailMediaAccount(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	record, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin media account")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Media account not found")
		return
	}

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) UpdateMediaAccount(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	var payload adminUpdateMediaAccountRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Notes == nil {
		render.Error(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}

	record, err := h.app.Store.UpdateAdminMediaAccountTarget(r.Context(), accountID, store.UpdateAdminMediaAccountTargetInput{
		Notes:        normalizeTrimmedString(payload.Notes),
		NotesTouched: payload.Notes != nil,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update media account")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Media account not found")
		return
	}
	h.decorateAdminMediaAccountRow(record)

	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  record.Owner.ID,
			ResourceType: "account",
			ResourceID:   &record.Account.ID,
			Action:       "admin_update",
			Title:        "运营后台更新媒体账号备注",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("媒体账号内部备注已由运营后台更新"),
			Payload: mustJSONBytes(map[string]any{
				"notes": normalizeTrimmedString(payload.Notes),
			}),
		})
	}
	h.recordAdminAction(r.Context(), "media_account", &record.Account.ID, "update", "更新媒体账号备注", "success", auditStringPtr("媒体账号内部备注已更新"), mustJSONBytes(map[string]any{
		"notes": normalizeTrimmedString(payload.Notes),
	}))

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) ValidateMediaAccount(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	record, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load media account")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Media account not found")
		return
	}
	session, message, err := h.createAdminMediaAccountValidationSession(r.Context(), record)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create validation session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusConflict, trimmedStringValue(message))
		return
	}

	render.JSON(w, http.StatusCreated, session)
}

func (h *AdminConsoleHandler) CreateRemoteLogin(w http.ResponseWriter, r *http.Request) {
	var payload createRemoteLoginRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.DeviceID = strings.TrimSpace(payload.DeviceID)
	payload.Platform = strings.TrimSpace(payload.Platform)
	payload.AccountName = strings.TrimSpace(payload.AccountName)
	if payload.DeviceID == "" || payload.Platform == "" || payload.AccountName == "" {
		render.Error(w, http.StatusBadRequest, "deviceId, platform, and accountName are required")
		return
	}

	record, err := h.app.Store.GetAdminDeviceByID(r.Context(), payload.DeviceID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load device")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Device not found")
		return
	}
	h.decorateAdminDeviceRow(record)

	session, message, err := h.createAdminRemoteLoginSession(r.Context(), record, payload.Platform, payload.AccountName)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create remote login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusConflict, trimmedStringValue(message))
		return
	}

	render.JSON(w, http.StatusCreated, session)
}

func (h *AdminConsoleHandler) GetLoginSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		render.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	session, err := h.app.Store.GetLoginSessionByID(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}

	render.JSON(w, http.StatusOK, session)
}

func (h *AdminConsoleHandler) CreateLoginSessionAction(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		render.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	session, err := h.app.Store.GetLoginSessionByID(r.Context(), sessionID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load login session")
		return
	}
	if session == nil {
		render.Error(w, http.StatusNotFound, "Login session not found")
		return
	}

	var payload createLoginActionRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.ActionType = strings.TrimSpace(payload.ActionType)
	if payload.ActionType == "" {
		render.Error(w, http.StatusBadRequest, "actionType is required")
		return
	}

	var payloadBytes []byte
	if payload.Payload != nil {
		payloadBytes, err = json.Marshal(payload.Payload)
		if err != nil {
			render.Error(w, http.StatusBadRequest, "payload must be valid json")
			return
		}
	}

	action, err := h.app.Store.CreateLoginAction(r.Context(), store.CreateLoginActionInput{
		ID:         uuid.NewString(),
		SessionID:  session.ID,
		ActionType: payload.ActionType,
		Payload:    payloadBytes,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to create login action")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  session.UserID,
		ResourceType: "login_session_action",
		ResourceID:   &action.ID,
		Action:       payload.ActionType,
		Title:        "运营后台提交登录会话动作",
		Source:       session.Platform,
		Status:       action.Status,
		Message:      auditStringPtr("登录会话动作已由运营后台下发到本地 SAU"),
		Payload: mustJSONBytes(map[string]any{
			"sessionId":     session.ID,
			"platform":      session.Platform,
			"accountName":   session.AccountName,
			"actionType":    payload.ActionType,
			"actionPayload": payload.Payload,
		}),
	})
	h.recordAdminAction(r.Context(), "login_session", &session.ID, payload.ActionType, "提交登录会话动作", "success", auditStringPtr("登录会话动作已下发"), mustJSONBytes(map[string]any{
		"actionId":      action.ID,
		"actionType":    payload.ActionType,
		"platform":      session.Platform,
		"accountName":   session.AccountName,
		"actionPayload": payload.Payload,
	}))

	if isLoginCancelAction(payload.ActionType) {
		cancelMessage := "登录会话已取消，等待本地 SAU 停止当前登录流程"
		if _, err := h.app.Store.UpdateLoginSessionEvent(r.Context(), session.ID, store.LoginEventInput{
			Status:  "cancelled",
			Message: &cancelMessage,
		}); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to cancel login session")
			return
		}
	}

	render.JSON(w, http.StatusCreated, action)
}

func (h *AdminConsoleHandler) DeleteMediaAccount(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	record, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load media account")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Media account not found")
		return
	}
	deleted, message, taskCount, activeLoginSessionCount, err := h.deleteAdminMediaAccount(r.Context(), record)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to delete media account")
		return
	}
	if !deleted {
		if taskCount > 0 || activeLoginSessionCount > 0 {
			render.JSON(w, http.StatusConflict, map[string]any{
				"error": trimmedStringValue(message),
				"usage": map[string]any{
					"publishTaskCount":        taskCount,
					"activeLoginSessionCount": activeLoginSessionCount,
				},
			})
			return
		}
		render.Error(w, http.StatusConflict, trimmedStringValue(message))
		return
	}

	render.JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (h *AdminConsoleHandler) BulkActionMediaAccounts(w http.ResponseWriter, r *http.Request) {
	var payload adminBatchActionMediaAccountsRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Action = strings.TrimSpace(strings.ToLower(payload.Action))
	switch payload.Action {
	case "validate", "delete":
	default:
		render.Error(w, http.StatusBadRequest, "action must be one of validate, delete")
		return
	}

	accountIDs := uniqueTrimmedIDs(payload.AccountIDs)
	if len(accountIDs) == 0 {
		render.Error(w, http.StatusBadRequest, "accountIds is required")
		return
	}

	items := make([]domain.AdminMediaAccountBulkActionItem, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		item := domain.AdminMediaAccountBulkActionItem{
			Action: payload.Action,
			Status: "failed",
		}

		record, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load media account for bulk action")
			return
		}
		if record == nil {
			item.Message = auditStringPtr("Media account not found")
			items = append(items, item)
			continue
		}
		item.RecordBefore = *record

		switch payload.Action {
		case "validate":
			session, message, err := h.createAdminMediaAccountValidationSession(r.Context(), record)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk media account validation")
				return
			}
			if session == nil {
				item.Status = "skipped"
				item.Message = message
				items = append(items, item)
				continue
			}
			updated, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to reload media account after bulk action")
				return
			}
			item.Status = "success"
			item.Message = auditStringPtr("Validation session created")
			item.LoginSessionID = &session.ID
			item.RecordAfter = updated
		case "delete":
			deleted, message, taskCount, activeLoginSessionCount, err := h.deleteAdminMediaAccount(r.Context(), record)
			if err != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk media account deletion")
				return
			}
			if !deleted {
				item.Status = "skipped"
				if taskCount > 0 || activeLoginSessionCount > 0 {
					item.Message = auditStringPtr(fmt.Sprintf("Media account is still referenced by %d tasks and %d active login sessions", taskCount, activeLoginSessionCount))
				} else {
					item.Message = message
				}
				items = append(items, item)
				continue
			}
			item.Status = "success"
			item.Message = auditStringPtr("Media account deleted")
			item.Deleted = true
		}

		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.AdminMediaAccountBulkActionResult{
		Items:      items,
		Summary:    summarizeAdminMediaAccountBulkActionItems(items),
		ServerTime: time.Now().UTC(),
	})
}

func (h *AdminConsoleHandler) MediaAccountWorkspace(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(chi.URLParam(r, "accountId"))
	if accountID == "" {
		render.Error(w, http.StatusBadRequest, "accountId is required")
		return
	}

	record, err := h.loadAdminMediaAccountRow(r.Context(), accountID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin media account")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Media account not found")
		return
	}

	workspace := domain.AdminMediaAccountWorkspace{Record: *record}
	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		workspace.RecentTasks, err = h.app.Store.ListPublishTasksByAccountTarget(r.Context(), record.Owner.ID, record.Account.DeviceID, record.Account.Platform, record.Account.AccountName, 10)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load media account tasks")
			return
		}
		workspace.ActiveLoginSessions, err = h.app.Store.ListLoginSessionsByAccountTarget(r.Context(), record.Owner.ID, record.Account.DeviceID, record.Account.Platform, record.Account.AccountName, 10)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load media account login sessions")
			return
		}
	}
	workspace.RecentAudits, err = h.app.Store.ListRecentAdminAuditsByMediaAccountID(r.Context(), accountID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load media account audits")
		return
	}

	render.JSON(w, http.StatusOK, workspace)
}

func (h *AdminConsoleHandler) ListPublishTasks(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminTasks(r.Context(), store.AdminTaskListFilter{
		Query:    strings.TrimSpace(r.URL.Query().Get("query")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		Platform: strings.TrimSpace(r.URL.Query().Get("platform")),
		UserID:   strings.TrimSpace(r.URL.Query().Get("userId")),
		DeviceID: strings.TrimSpace(r.URL.Query().Get("deviceId")),
		SkillID:  strings.TrimSpace(r.URL.Query().Get("skillId")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish tasks")
		return
	}

	for index := range items {
		if _, err := h.decorateAdminTaskRow(r.Context(), &items[index], false); err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to decorate admin publish tasks")
			return
		}
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":    strings.TrimSpace(r.URL.Query().Get("query")),
		"status":   strings.TrimSpace(r.URL.Query().Get("status")),
		"platform": strings.TrimSpace(r.URL.Query().Get("platform")),
		"userId":   strings.TrimSpace(r.URL.Query().Get("userId")),
		"deviceId": strings.TrimSpace(r.URL.Query().Get("deviceId")),
		"skillId":  strings.TrimSpace(r.URL.Query().Get("skillId")),
	})
}

func (h *AdminConsoleHandler) DetailPublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) UpdatePublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	var payload adminUpdatePublishTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Notes == nil && payload.ExceptionReason == nil && payload.RiskTags == nil {
		render.Error(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}
	var riskTags []string
	if payload.RiskTags != nil {
		riskTags = normalizeTrimmedStrings(*payload.RiskTags)
	}

	record, err := h.app.Store.UpdateAdminPublishTaskTarget(r.Context(), taskID, store.UpdateAdminPublishTaskTargetInput{
		Notes:                  normalizeTrimmedString(payload.Notes),
		NotesTouched:           payload.Notes != nil,
		ExceptionReason:        normalizeTrimmedString(payload.ExceptionReason),
		ExceptionReasonTouched: payload.ExceptionReason != nil,
		RiskTags:               riskTags,
		RiskTagsTouched:        payload.RiskTags != nil,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
			OwnerUserID:  record.Owner.ID,
			ResourceType: "publish_task",
			ResourceID:   &record.Task.ID,
			Action:       "admin_update",
			Title:        "运营后台更新发布任务风控归档",
			Source:       "admin_console",
			Status:       "success",
			Message:      auditStringPtr("发布任务风控归档已由运营后台更新"),
			Payload: mustJSONBytes(map[string]any{
				"notes":           normalizeTrimmedString(payload.Notes),
				"exceptionReason": normalizeTrimmedString(payload.ExceptionReason),
				"riskTags":        riskTags,
			}),
		})
	}
	h.recordAdminAction(r.Context(), "publish_task", &record.Task.ID, "update", "更新发布任务风控归档", "success", auditStringPtr("发布任务风控归档已更新"), mustJSONBytes(map[string]any{
		"notes":           normalizeTrimmedString(payload.Notes),
		"exceptionReason": normalizeTrimmedString(payload.ExceptionReason),
		"riskTags":        riskTags,
	}))

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) PublishTaskWorkspace(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, runtimeState, err := h.loadAdminTaskRow(r.Context(), taskID, true)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}

	workspace := domain.AdminPublishTaskWorkspace{
		Record:  *record,
		Runtime: runtimeState,
	}
	if record.Owner != nil && strings.TrimSpace(record.Owner.ID) != "" {
		workspace.Events, err = h.app.Store.ListPublishTaskEventsByOwner(r.Context(), taskID, record.Owner.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task events")
			return
		}
		workspace.Artifacts, err = h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), taskID, record.Owner.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task artifacts")
			return
		}
		workspace.Materials, err = h.app.Store.ListPublishTaskMaterialRefsByOwner(r.Context(), taskID, record.Owner.ID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load task materials")
			return
		}
	}
	workspace.RecentAudits, err = h.app.Store.ListRecentAdminAuditsByPublishTaskID(r.Context(), taskID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task audits")
		return
	}

	render.JSON(w, http.StatusOK, workspace)
}

func (h *AdminConsoleHandler) ListPublishTaskEvents(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.JSON(w, http.StatusOK, []domain.PublishTaskEvent{})
		return
	}

	items, err := h.app.Store.ListPublishTaskEventsByOwner(r.Context(), taskID, record.Owner.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task events")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AdminConsoleHandler) ListPublishTaskArtifacts(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.JSON(w, http.StatusOK, []domain.PublishTaskArtifact{})
		return
	}

	items, err := h.app.Store.ListPublishTaskArtifactsByOwner(r.Context(), taskID, record.Owner.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task artifacts")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AdminConsoleHandler) ListPublishTaskMaterials(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.JSON(w, http.StatusOK, []domain.PublishTaskMaterialRef{})
		return
	}

	items, err := h.app.Store.ListPublishTaskMaterialRefsByOwner(r.Context(), taskID, record.Owner.ID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load task materials")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AdminConsoleHandler) CancelPublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.Error(w, http.StatusConflict, "Publish task has no owner context")
		return
	}

	task, outcome, _, err := (&TaskHandler{app: h.app}).executeTaskCancel(r.Context(), record.Owner.ID, taskID, &record.Task)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to cancel publish task")
		return
	}
	if outcome != "success" || task == nil {
		render.Error(w, http.StatusConflict, "Publish task cannot be cancelled")
		return
	}

	updated, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload publish task")
		return
	}
	h.recordAdminAction(r.Context(), "publish_task", &taskID, "cancel", "取消发布任务", "success", auditStringPtr("发布任务已由运营后台取消"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) RetryPublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.Error(w, http.StatusConflict, "Publish task has no owner context")
		return
	}

	task, _, outcome, _, err := (&TaskHandler{app: h.app}).executeTaskRetry(r.Context(), record.Owner.ID, taskID, &record.Task)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to retry publish task")
		return
	}
	if outcome != "success" || task == nil {
		render.Error(w, http.StatusConflict, "Publish task cannot be retried")
		return
	}

	updated, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload publish task")
		return
	}
	h.recordAdminAction(r.Context(), "publish_task", &taskID, "retry", "重试发布任务", "success", auditStringPtr("发布任务已由运营后台重新排队"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) ForceReleasePublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.Error(w, http.StatusConflict, "Publish task has no owner context")
		return
	}

	task, outcome, _, err := (&TaskHandler{app: h.app}).executeTaskForceRelease(r.Context(), record.Owner.ID, taskID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to force release publish task")
		return
	}
	if outcome != "success" || task == nil {
		render.Error(w, http.StatusConflict, "Publish task is not in a releasable leased state")
		return
	}

	updated, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload publish task")
		return
	}
	h.recordAdminAction(r.Context(), "publish_task", &taskID, "force_release", "强制释放发布任务租约", "success", auditStringPtr("发布任务租约已由运营后台释放"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) ResumePublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.Error(w, http.StatusConflict, "Publish task has no owner context")
		return
	}

	var payload resumeTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil && err.Error() != "EOF" {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.Message = normalizeTrimmedString(payload.Message)

	task, outcome, _, err := (&TaskHandler{app: h.app}).executeTaskResume(r.Context(), record.Owner.ID, taskID, payload.Message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resume publish task")
		return
	}
	if outcome != "success" || task == nil {
		render.Error(w, http.StatusConflict, "Publish task is not resumable from needs_verify")
		return
	}

	updated, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload publish task")
		return
	}
	h.recordAdminAction(r.Context(), "publish_task", &taskID, "resume", "恢复发布任务自动化", "success", auditStringPtr("发布任务已由运营后台恢复执行"), mustJSONBytes(map[string]any{
		"message": payload.Message,
	}))
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) ManualResolvePublishTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(chi.URLParam(r, "taskId"))
	if taskID == "" {
		render.Error(w, http.StatusBadRequest, "taskId is required")
		return
	}

	record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load publish task")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "Publish task not found")
		return
	}
	if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
		render.Error(w, http.StatusConflict, "Publish task has no owner context")
		return
	}

	var payload manualResolveTaskRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Message = normalizeTrimmedString(payload.Message)
	payload.TextEvidence = normalizeTrimmedString(payload.TextEvidence)
	if payload.Status != "success" && payload.Status != "completed" && payload.Status != "failed" && payload.Status != "cancelled" {
		render.Error(w, http.StatusBadRequest, "status must be one of success, completed, failed, cancelled")
		return
	}

	task, _, outcome, _, err := (&TaskHandler{app: h.app}).executeTaskManualResolve(r.Context(), record.Owner.ID, taskID, payload.Status, payload.Message, payload.TextEvidence, payload.Payload)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to resolve publish task manually")
		return
	}
	if outcome != "success" || task == nil {
		render.Error(w, http.StatusConflict, "Publish task is not manually resolvable")
		return
	}

	updated, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload publish task")
		return
	}
	h.recordAdminAction(r.Context(), "publish_task", &taskID, "manual_resolve", "人工处理发布任务", "success", auditStringPtr("发布任务已由运营后台人工处理"), mustJSONBytes(map[string]any{
		"status":       payload.Status,
		"message":      payload.Message,
		"textEvidence": payload.TextEvidence,
		"payload":      payload.Payload,
	}))
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) BulkActionPublishTasks(w http.ResponseWriter, r *http.Request) {
	var payload adminBatchActionPublishTasksRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Action = strings.TrimSpace(strings.ToLower(payload.Action))
	payload.Message = normalizeTrimmedString(payload.Message)
	payload.TextEvidence = normalizeTrimmedString(payload.TextEvidence)
	switch payload.Action {
	case "cancel", "retry", "force_release", "resume", "manual_resolve":
	default:
		render.Error(w, http.StatusBadRequest, "action must be one of cancel, retry, force_release, resume, manual_resolve")
		return
	}
	if payload.Action == "manual_resolve" {
		payload.ResolveStatus = strings.TrimSpace(payload.ResolveStatus)
		switch payload.ResolveStatus {
		case "success", "completed", "failed", "cancelled":
		default:
			render.Error(w, http.StatusBadRequest, "resolveStatus must be one of success, completed, failed, cancelled")
			return
		}
	}

	taskIDs := uniqueTrimmedIDs(payload.TaskIDs)
	if len(taskIDs) == 0 {
		render.Error(w, http.StatusBadRequest, "taskIds is required")
		return
	}

	taskHandler := &TaskHandler{app: h.app}
	items := make([]domain.AdminPublishTaskBulkActionItem, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		item := domain.AdminPublishTaskBulkActionItem{
			Action: payload.Action,
			Status: "failed",
		}

		record, _, err := h.loadAdminTaskRow(r.Context(), taskID, false)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load publish task for bulk action")
			return
		}
		if record == nil {
			item.Message = auditStringPtr("Publish task not found")
			items = append(items, item)
			continue
		}
		item.RecordBefore = *record
		if record.Owner == nil || strings.TrimSpace(record.Owner.ID) == "" {
			item.Status = "skipped"
			item.Message = auditStringPtr("Publish task has no owner context")
			items = append(items, item)
			continue
		}

		var updatedRow *domain.AdminPublishTaskRow
		switch payload.Action {
		case "cancel":
			_, outcome, message, execErr := taskHandler.executeTaskCancel(r.Context(), record.Owner.ID, taskID, &record.Task)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk cancel on publish tasks")
				return
			}
			item.Status = outcome
			item.Message = auditStringPtr(message)
			if outcome == "success" {
				updatedRow, _, err = h.loadAdminTaskRow(r.Context(), taskID, false)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "Failed to reload publish task after bulk action")
					return
				}
				item.RecordAfter = updatedRow
				h.recordAdminAction(r.Context(), "publish_task", &taskID, "cancel", "批量取消发布任务", "success", auditStringPtr("发布任务已由运营后台批量取消"), nil)
			}
		case "retry":
			_, artifactCount, outcome, message, execErr := taskHandler.executeTaskRetry(r.Context(), record.Owner.ID, taskID, &record.Task)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk retry on publish tasks")
				return
			}
			item.Status = outcome
			item.Message = auditStringPtr(message)
			item.ArtifactCount = artifactCount
			if outcome == "success" {
				updatedRow, _, err = h.loadAdminTaskRow(r.Context(), taskID, false)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "Failed to reload publish task after bulk action")
					return
				}
				item.RecordAfter = updatedRow
				h.recordAdminAction(r.Context(), "publish_task", &taskID, "retry", "批量重试发布任务", "success", auditStringPtr("发布任务已由运营后台批量重试"), nil)
			}
		case "force_release":
			_, outcome, message, execErr := taskHandler.executeTaskForceRelease(r.Context(), record.Owner.ID, taskID)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk force release on publish tasks")
				return
			}
			item.Status = outcome
			item.Message = auditStringPtr(message)
			if outcome == "success" {
				updatedRow, _, err = h.loadAdminTaskRow(r.Context(), taskID, false)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "Failed to reload publish task after bulk action")
					return
				}
				item.RecordAfter = updatedRow
				h.recordAdminAction(r.Context(), "publish_task", &taskID, "force_release", "批量释放发布任务租约", "success", auditStringPtr("发布任务租约已由运营后台批量释放"), nil)
			}
		case "resume":
			_, outcome, message, execErr := taskHandler.executeTaskResume(r.Context(), record.Owner.ID, taskID, payload.Message)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk resume on publish tasks")
				return
			}
			item.Status = outcome
			item.Message = auditStringPtr(message)
			if outcome == "success" {
				updatedRow, _, err = h.loadAdminTaskRow(r.Context(), taskID, false)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "Failed to reload publish task after bulk action")
					return
				}
				item.RecordAfter = updatedRow
				h.recordAdminAction(r.Context(), "publish_task", &taskID, "resume", "批量恢复发布任务", "success", auditStringPtr("发布任务已由运营后台批量恢复"), mustJSONBytes(map[string]any{
					"message": payload.Message,
				}))
			}
		case "manual_resolve":
			_, artifactCount, outcome, message, execErr := taskHandler.executeTaskManualResolve(r.Context(), record.Owner.ID, taskID, payload.ResolveStatus, payload.Message, payload.TextEvidence, payload.Payload)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk manual resolve on publish tasks")
				return
			}
			item.Status = outcome
			item.Message = auditStringPtr(message)
			item.ArtifactCount = artifactCount
			if outcome == "success" {
				updatedRow, _, err = h.loadAdminTaskRow(r.Context(), taskID, false)
				if err != nil {
					render.Error(w, http.StatusInternalServerError, "Failed to reload publish task after bulk action")
					return
				}
				item.RecordAfter = updatedRow
				h.recordAdminAction(r.Context(), "publish_task", &taskID, "manual_resolve", "批量人工处理发布任务", "success", auditStringPtr("发布任务已由运营后台批量人工处理"), mustJSONBytes(map[string]any{
					"status":       payload.ResolveStatus,
					"message":      payload.Message,
					"textEvidence": payload.TextEvidence,
					"payload":      payload.Payload,
				}))
			}
		}

		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.AdminPublishTaskBulkActionResult{
		Items:      items,
		Summary:    summarizeAdminPublishTaskBulkActionItems(items),
		ServerTime: time.Now().UTC(),
	})
}

func (h *AdminConsoleHandler) ListAIJobs(w http.ResponseWriter, r *http.Request) {
	page := parseAdminPageQuery(r)
	items, total, summary, err := h.app.Store.ListAdminAIJobs(r.Context(), store.AdminAIJobListFilter{
		Query:    strings.TrimSpace(r.URL.Query().Get("query")),
		Status:   strings.TrimSpace(r.URL.Query().Get("status")),
		JobType:  strings.TrimSpace(r.URL.Query().Get("jobType")),
		Source:   strings.TrimSpace(r.URL.Query().Get("source")),
		UserID:   strings.TrimSpace(r.URL.Query().Get("userId")),
		DeviceID: strings.TrimSpace(r.URL.Query().Get("deviceId")),
		SkillID:  strings.TrimSpace(r.URL.Query().Get("skillId")),
		AdminPageFilter: store.AdminPageFilter{
			Page:     page.Page,
			PageSize: page.PageSize,
		},
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin AI jobs")
		return
	}

	for index := range items {
		h.decorateAdminAIJobRow(&items[index])
	}

	renderAdminList(w, page, total, items, summary, map[string]any{
		"query":    strings.TrimSpace(r.URL.Query().Get("query")),
		"status":   strings.TrimSpace(r.URL.Query().Get("status")),
		"jobType":  strings.TrimSpace(r.URL.Query().Get("jobType")),
		"source":   strings.TrimSpace(r.URL.Query().Get("source")),
		"userId":   strings.TrimSpace(r.URL.Query().Get("userId")),
		"deviceId": strings.TrimSpace(r.URL.Query().Get("deviceId")),
		"skillId":  strings.TrimSpace(r.URL.Query().Get("skillId")),
	})
}

func (h *AdminConsoleHandler) DetailAIJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) UpdateAIJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	var payload adminUpdateAIJobRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Notes == nil && payload.ExceptionReason == nil && payload.RiskTags == nil {
		render.Error(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}
	var riskTags []string
	if payload.RiskTags != nil {
		riskTags = normalizeTrimmedStrings(*payload.RiskTags)
	}

	record, err := h.app.Store.UpdateAdminAIJobTarget(r.Context(), jobID, store.UpdateAdminAIJobTargetInput{
		Notes:                  normalizeTrimmedString(payload.Notes),
		NotesTouched:           payload.Notes != nil,
		ExceptionReason:        normalizeTrimmedString(payload.ExceptionReason),
		ExceptionReasonTouched: payload.ExceptionReason != nil,
		RiskTags:               riskTags,
		RiskTagsTouched:        payload.RiskTags != nil,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to update AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	recordAuditEvent(h.app, r.Context(), store.CreateAuditEventInput{
		OwnerUserID:  record.Job.OwnerUserID,
		ResourceType: "ai_job",
		ResourceID:   &record.Job.ID,
		Action:       "admin_update",
		Title:        "运营后台更新 AI 任务风控归档",
		Source:       "admin_console",
		Status:       "success",
		Message:      auditStringPtr("AI 任务风控归档已由运营后台更新"),
		Payload: mustJSONBytes(map[string]any{
			"notes":           normalizeTrimmedString(payload.Notes),
			"exceptionReason": normalizeTrimmedString(payload.ExceptionReason),
			"riskTags":        riskTags,
		}),
	})
	h.recordAdminAction(r.Context(), "ai_job", &record.Job.ID, "update", "更新 AI 任务风控归档", "success", auditStringPtr("AI 任务风控归档已更新"), mustJSONBytes(map[string]any{
		"notes":           normalizeTrimmedString(payload.Notes),
		"exceptionReason": normalizeTrimmedString(payload.ExceptionReason),
		"riskTags":        riskTags,
	}))

	render.JSON(w, http.StatusOK, record)
}

func (h *AdminConsoleHandler) AIJobWorkspace(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	workspace := domain.AdminAIJobWorkspace{Record: *record}
	workspace.Artifacts, err = h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}
	workspace.PublishTasks, err = h.app.Store.ListPublishTasksByAIJobOwner(r.Context(), jobID, record.Job.OwnerUserID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load linked publish tasks")
		return
	}
	workspace.BillingUsageEvents, err = h.app.Store.ListBillingUsageEventsByUser(r.Context(), record.Job.OwnerUserID, store.BillingUsageEventListFilter{
		SourceType: "ai_job",
		SourceID:   jobID,
		Limit:      50,
	})
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load billing usage events")
		return
	}
	workspace.RecentAudits, err = h.app.Store.ListRecentAdminAuditsByAIJobID(r.Context(), jobID, 20)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job audits")
		return
	}
	workspace.ExecutionLogs = buildAIJobExecutionLogs(&workspace)

	render.JSON(w, http.StatusOK, workspace)
}

func (h *AdminConsoleHandler) ListAIJobArtifacts(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load admin AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	items, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}
	render.JSON(w, http.StatusOK, items)
}

func (h *AdminConsoleHandler) CancelAIJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}
	if !computeAIJobActions(&record.Job, int(record.ArtifactCount)).CanCancel {
		render.Error(w, http.StatusConflict, "AI job cannot be cancelled")
		return
	}

	message := "AI 任务已取消"
	job, err := h.app.Store.CancelAIJob(r.Context(), jobID, record.Job.OwnerUserID, &message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to cancel AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job cannot be cancelled")
		return
	}

	updated, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload AI job")
		return
	}
	h.recordAdminAction(r.Context(), "ai_job", &jobID, "cancel", "取消 AI 任务", "success", auditStringPtr("AI 任务已由运营后台取消"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) RetryAIJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}
	if !computeAIJobActions(&record.Job, int(record.ArtifactCount)).CanRetry {
		render.Error(w, http.StatusConflict, "AI job cannot be retried")
		return
	}

	existingArtifacts, err := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts")
		return
	}

	message := "AI 任务已重新排队"
	job, err := h.app.Store.RetryAIJob(r.Context(), jobID, record.Job.OwnerUserID, &message)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to retry AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job cannot be retried")
		return
	}
	_, _ = h.app.Store.DeleteAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
	cleanupAIArtifactFiles(h.app, r.Context(), existingArtifacts)

	updated, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload AI job")
		return
	}
	h.recordAdminAction(r.Context(), "ai_job", &jobID, "retry", "重试 AI 任务", "success", auditStringPtr("AI 任务已由运营后台重新排队"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) ForceReleaseAIJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(chi.URLParam(r, "jobId"))
	if jobID == "" {
		render.Error(w, http.StatusBadRequest, "jobId is required")
		return
	}

	record, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to load AI job")
		return
	}
	if record == nil {
		render.Error(w, http.StatusNotFound, "AI job not found")
		return
	}

	job, err := h.app.Store.ForceReleaseAIJobLeaseByOwner(r.Context(), jobID, record.Job.OwnerUserID, auditStringPtr("AI 任务租约已由运营后台手动释放"))
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to force release AI job")
		return
	}
	if job == nil {
		render.Error(w, http.StatusConflict, "AI job has no active lease to force release")
		return
	}

	updated, err := h.loadAdminAIJobRow(r.Context(), jobID)
	if err != nil {
		render.Error(w, http.StatusInternalServerError, "Failed to reload AI job")
		return
	}
	h.recordAdminAction(r.Context(), "ai_job", &jobID, "force_release", "强制释放 AI 任务租约", "success", auditStringPtr("AI 任务租约已由运营后台释放"), nil)
	render.JSON(w, http.StatusOK, updated)
}

func (h *AdminConsoleHandler) BulkActionAIJobs(w http.ResponseWriter, r *http.Request) {
	var payload adminBatchActionAIJobsRequest
	if err := render.DecodeJSON(r, &payload); err != nil {
		render.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payload.Action = strings.TrimSpace(strings.ToLower(payload.Action))
	switch payload.Action {
	case "cancel", "retry", "force_release":
	default:
		render.Error(w, http.StatusBadRequest, "action must be one of cancel, retry, force_release")
		return
	}

	jobIDs := uniqueTrimmedIDs(payload.JobIDs)
	if len(jobIDs) == 0 {
		render.Error(w, http.StatusBadRequest, "jobIds is required")
		return
	}

	items := make([]domain.AdminAIJobBulkActionItem, 0, len(jobIDs))
	for _, jobID := range jobIDs {
		item := domain.AdminAIJobBulkActionItem{
			Action: payload.Action,
			Status: "failed",
		}

		record, err := h.loadAdminAIJobRow(r.Context(), jobID)
		if err != nil {
			render.Error(w, http.StatusInternalServerError, "Failed to load AI job for bulk action")
			return
		}
		if record == nil {
			item.Message = auditStringPtr("AI job not found")
			items = append(items, item)
			continue
		}
		item.RecordBefore = *record

		switch payload.Action {
		case "cancel":
			if !computeAIJobActions(&record.Job, int(record.ArtifactCount)).CanCancel {
				item.Status = "skipped"
				item.Message = auditStringPtr("AI job cannot be cancelled")
				items = append(items, item)
				continue
			}
			message := "AI 任务已取消"
			job, execErr := h.app.Store.CancelAIJob(r.Context(), jobID, record.Job.OwnerUserID, &message)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk cancel on AI jobs")
				return
			}
			if job == nil {
				item.Status = "skipped"
				item.Message = auditStringPtr("AI job cannot be cancelled")
				items = append(items, item)
				continue
			}
			item.Status = "success"
			item.Message = auditStringPtr(message)
			updated, reloadErr := h.loadAdminAIJobRow(r.Context(), jobID)
			if reloadErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to reload AI job after bulk action")
				return
			}
			item.RecordAfter = updated
			h.recordAdminAction(r.Context(), "ai_job", &jobID, "cancel", "批量取消 AI 任务", "success", auditStringPtr("AI 任务已由运营后台批量取消"), nil)
		case "retry":
			if !computeAIJobActions(&record.Job, int(record.ArtifactCount)).CanRetry {
				item.Status = "skipped"
				item.Message = auditStringPtr("AI job cannot be retried")
				items = append(items, item)
				continue
			}
			existingArtifacts, execErr := h.app.Store.ListAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to load AI artifacts for bulk retry")
				return
			}
			message := "AI 任务已重新排队"
			job, execErr := h.app.Store.RetryAIJob(r.Context(), jobID, record.Job.OwnerUserID, &message)
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk retry on AI jobs")
				return
			}
			if job == nil {
				item.Status = "skipped"
				item.Message = auditStringPtr("AI job cannot be retried")
				items = append(items, item)
				continue
			}
			_, _ = h.app.Store.DeleteAIJobArtifactsByOwner(r.Context(), jobID, record.Job.OwnerUserID)
			cleanupAIArtifactFiles(h.app, r.Context(), existingArtifacts)
			item.Status = "success"
			item.Message = auditStringPtr(message)
			item.ArtifactCount = int64(len(existingArtifacts))
			updated, reloadErr := h.loadAdminAIJobRow(r.Context(), jobID)
			if reloadErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to reload AI job after bulk action")
				return
			}
			item.RecordAfter = updated
			h.recordAdminAction(r.Context(), "ai_job", &jobID, "retry", "批量重试 AI 任务", "success", auditStringPtr("AI 任务已由运营后台批量重试"), nil)
		case "force_release":
			job, execErr := h.app.Store.ForceReleaseAIJobLeaseByOwner(r.Context(), jobID, record.Job.OwnerUserID, auditStringPtr("AI 任务租约已由运营后台手动释放"))
			if execErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to execute bulk force release on AI jobs")
				return
			}
			if job == nil {
				item.Status = "skipped"
				item.Message = auditStringPtr("AI job has no active lease to force release")
				items = append(items, item)
				continue
			}
			item.Status = "success"
			item.Message = auditStringPtr("AI job lease released")
			updated, reloadErr := h.loadAdminAIJobRow(r.Context(), jobID)
			if reloadErr != nil {
				render.Error(w, http.StatusInternalServerError, "Failed to reload AI job after bulk action")
				return
			}
			item.RecordAfter = updated
			h.recordAdminAction(r.Context(), "ai_job", &jobID, "force_release", "批量释放 AI 任务租约", "success", auditStringPtr("AI 任务租约已由运营后台批量释放"), nil)
		}

		items = append(items, item)
	}

	render.JSON(w, http.StatusOK, domain.AdminAIJobBulkActionResult{
		Items:      items,
		Summary:    summarizeAdminAIJobBulkActionItems(items),
		ServerTime: time.Now().UTC(),
	})
}
