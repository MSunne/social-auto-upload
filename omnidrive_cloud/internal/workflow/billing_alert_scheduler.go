package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"omnidrive_cloud/internal/ai"
	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/sms"
	"omnidrive_cloud/internal/store"
)

const (
	billingAlertSchedulerPollInterval = 10 * time.Minute
	billingAlertSchedulerLimit        = 2000
	billingAlertSMSAction             = "billing_balance_alert_sms_sent"
	billingAlertSMSResourceType       = "billing_summary"
	billingAlertSMSSignName           = "禾硕天成深圳科技"
	billingAlertSMSTemplateCode       = "SMS_505065010"
	billingAlertTimezone              = "Asia/Shanghai"
)

type BillingAlertScheduler struct {
	app          *appstate.App
	pollInterval time.Duration
}

type billingAlertCandidate struct {
	Job              domain.AIJob
	Preview          store.UsageBillingQueuePreviewItem
	ProjectedBalance int64
}

// 创建计费告警Scheduler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewBillingAlertScheduler(app *appstate.App) (*BillingAlertScheduler, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}
	return &BillingAlertScheduler{
		app:          app,
		pollInterval: billingAlertSchedulerPollInterval,
	}, nil
}

// 启动计费告警调度流程，持续处理后续轮询、调度或后台任务。
func (s *BillingAlertScheduler) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	s.app.Logger.Info("billing alert scheduler started", "poll_interval", s.pollInterval.String())

	go func() {
		defer wg.Done()
		s.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		s.app.Logger.Info("billing alert scheduler stopped")
	}
}

// 运行计费告警调度流程，按当前上下文驱动一次或持续的业务处理。
func (s *BillingAlertScheduler) run(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// 执行一轮 AI 作业拉取与处理循环，供后台 worker 按固定节奏持续轮询。
func (s *BillingAlertScheduler) runOnce(ctx context.Context) {
	location := billingAlertLocation()
	now := time.Now().In(location)
	startToday, startTomorrow, endTomorrow := billingAlertWindowBounds(now)

	jobs, err := s.app.Store.ListPendingExecutableAIJobsBefore(ctx, endTomorrow.UTC(), billingAlertSchedulerLimit)
	if err != nil {
		s.app.Logger.Error("billing alert scheduler failed to list pending ai jobs", "error", err)
		return
	}
	if len(jobs) == 0 {
		return
	}

	smsConfig, smsReady, smsErr := s.loadBillingAlertSMSConfig(ctx)
	if smsErr != nil {
		s.app.Logger.Error("billing alert scheduler failed to load sms config", "error", smsErr)
		return
	}

	groupedJobs := groupAIJobsByOwner(jobs)
	sentCount := 0
	for ownerUserID, ownerJobs := range groupedJobs {
		if len(ownerJobs) == 0 {
			continue
		}

		alreadySentToday, err := s.app.Store.HasRecentAuditEvent(ctx, ownerUserID, billingAlertSMSAction, startToday.UTC())
		if err != nil {
			s.app.Logger.Error("billing alert scheduler failed to check recent sms audit", "user_id", ownerUserID, "error", err)
			continue
		}
		if alreadySentToday {
			continue
		}

		inputs := make([]store.ApplyUsageBillingInput, 0, len(ownerJobs))
		for _, job := range ownerJobs {
			inputs = append(inputs, ai.BuildEstimatedUsageBillingInput(&job))
		}

		previews, err := s.app.Store.PreviewUsageBillingQueue(ctx, inputs)
		if err != nil {
			s.app.Logger.Error("billing alert scheduler failed to preview billing queue", "user_id", ownerUserID, "error", err)
			continue
		}

		candidate := firstTomorrowBillingShortage(ownerJobs, previews, startTomorrow, endTomorrow, location)
		if candidate == nil {
			continue
		}
		if !smsReady {
			s.app.Logger.Warn("billing alert scheduler skipped sms because config is incomplete", "user_id", ownerUserID, "job_id", candidate.Job.ID)
			continue
		}

		user, err := s.app.Store.GetUserByID(ctx, ownerUserID)
		if err != nil {
			s.app.Logger.Error("billing alert scheduler failed to load user", "user_id", ownerUserID, "error", err)
			continue
		}
		phone := normalizeBillingAlertPhone(user)
		if phone == "" {
			s.app.Logger.Warn("billing alert scheduler skipped sms because user phone is empty", "user_id", ownerUserID, "job_id", candidate.Job.ID)
			continue
		}

		templateParam, err := buildBillingAlertTemplateParam(candidate.ProjectedBalance)
		if err != nil {
			s.app.Logger.Error("billing alert scheduler failed to build sms template param", "user_id", ownerUserID, "error", err)
			continue
		}

		result, err := sms.SendTemplateSMS(
			smsConfig,
			phone,
			uuid.NewString(),
			billingAlertSMSSignName,
			billingAlertSMSTemplateCode,
			templateParam,
		)
		if err != nil {
			s.app.Logger.Error("billing alert scheduler failed to send sms", "user_id", ownerUserID, "job_id", candidate.Job.ID, "error", err)
			continue
		}

		message := fmt.Sprintf("已发送次日任务余额不足提醒短信，预计余额 %d", candidate.ProjectedBalance)
		if err := s.app.Store.CreateAuditEvent(ctx, store.CreateAuditEventInput{
			ID:           uuid.NewString(),
			OwnerUserID:  ownerUserID,
			ResourceType: billingAlertSMSResourceType,
			ResourceID:   stringPtr(ownerUserID),
			Action:       billingAlertSMSAction,
			Title:        "次日任务余额不足短信提醒",
			Source:       "billing_alert_scheduler",
			Status:       "success",
			Message:      stringPtr(message),
			Payload: mustBillingAlertJSONBytes(map[string]any{
				"jobId":              candidate.Job.ID,
				"jobRunAt":           effectiveJobRunAt(candidate.Job).UTC().Format(time.RFC3339),
				"projectedBalance":   candidate.ProjectedBalance,
				"billMessage":        candidate.Preview.Result.BillMessage,
				"smsRequestId":       result.RequestID,
				"smsBizId":           result.BizID,
				"smsProvider":        result.Provider,
				"smsTemplateCode":    billingAlertSMSTemplateCode,
				"smsTemplateSign":    billingAlertSMSSignName,
				"smsTemplatePayload": templateParam,
			}),
		}); err != nil {
			s.app.Logger.Error("billing alert scheduler failed to record sms audit", "user_id", ownerUserID, "job_id", candidate.Job.ID, "error", err)
			continue
		}

		sentCount++
		s.app.Logger.Info("billing alert scheduler sent insufficient-balance sms", "user_id", ownerUserID, "job_id", candidate.Job.ID, "projected_balance", candidate.ProjectedBalance)
	}

	if sentCount > 0 {
		s.app.Logger.Info("billing alert scheduler completed reminder scan", "sent_count", sentCount, "window_end", endTomorrow.Format(time.RFC3339))
	}
}

// 加载计费告警短信配置，供计费告警调度继续处理当前业务状态。
func (s *BillingAlertScheduler) loadBillingAlertSMSConfig(ctx context.Context) (sms.RegistrationConfig, bool, error) {
	record, err := s.app.Store.GetAdminSystemSettings(ctx)
	if err != nil {
		return sms.RegistrationConfig{}, false, err
	}

	config := sms.RegistrationConfig{
		Provider:     sms.ProviderAliyunDysmsapi,
		Endpoint:     "dysmsapi.aliyuncs.com",
		SignName:     billingAlertSMSSignName,
		TemplateCode: billingAlertSMSTemplateCode,
	}
	if record == nil {
		return config, false, nil
	}

	config.AccessKeyID = strings.TrimSpace(record.SMSRegistration.AccessKeyID)
	config.AccessKeySecret = strings.TrimSpace(record.SMSRegistration.AccessKeySecret)
	endpoint := strings.TrimSpace(record.SMSRegistration.Endpoint)
	if endpoint != "" && !strings.EqualFold(endpoint, "dypnsapi.aliyuncs.com") {
		config.Endpoint = endpoint
	}

	ready := config.AccessKeyID != "" && config.AccessKeySecret != ""
	return config, ready, nil
}

// 处理groupAI作业s归属方相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func groupAIJobsByOwner(jobs []domain.AIJob) map[string][]domain.AIJob {
	grouped := make(map[string][]domain.AIJob)
	for _, job := range jobs {
		ownerUserID := strings.TrimSpace(job.OwnerUserID)
		if ownerUserID == "" {
			continue
		}
		grouped[ownerUserID] = append(grouped[ownerUserID], job)
	}
	return grouped
}

// 处理首个Tomorrow计费Shortage相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func firstTomorrowBillingShortage(jobs []domain.AIJob, previews []store.UsageBillingQueuePreviewItem, startTomorrow time.Time, endTomorrow time.Time, location *time.Location) *billingAlertCandidate {
	if len(jobs) == 0 || len(previews) == 0 {
		return nil
	}
	limit := len(jobs)
	if len(previews) < limit {
		limit = len(previews)
	}

	for index := 0; index < limit; index++ {
		preview := previews[index]
		if !usageBillingLooksInsufficient(preview.Result) {
			continue
		}

		jobRunAt := effectiveJobRunAt(jobs[index]).In(location)
		if jobRunAt.Before(startTomorrow) || !jobRunAt.Before(endTomorrow) {
			continue
		}

		return &billingAlertCandidate{
			Job:              jobs[index],
			Preview:          preview,
			ProjectedBalance: preview.CreditBalanceBefore,
		}
	}
	return nil
}

// 处理用量计费LooksInsufficient相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func usageBillingLooksInsufficient(result store.ApplyUsageBillingResult) bool {
	if !strings.EqualFold(strings.TrimSpace(result.BillStatus), "failed") {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(result.BillMessage))
	return strings.Contains(message, "wallet credits insufficient") || strings.Contains(message, "积分不足")
}

// 构建计费告警TemplateParam，为计费告警调度生成后续步骤所需的派生参数或载荷。
func buildBillingAlertTemplateParam(balance int64) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"balance": fmt.Sprintf("%d", balance),
	})
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// 处理计费告警Location相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func billingAlertLocation() *time.Location {
	location, err := time.LoadLocation(billingAlertTimezone)
	if err != nil {
		return time.Local
	}
	return location
}

// 处理计费告警窗口Bounds相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func billingAlertWindowBounds(now time.Time) (time.Time, time.Time, time.Time) {
	location := now.Location()
	startToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	startTomorrow := startToday.Add(24 * time.Hour)
	endTomorrow := startTomorrow.Add(24 * time.Hour)
	return startToday, startTomorrow, endTomorrow
}

// 处理生效作业执行时间相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func effectiveJobRunAt(job domain.AIJob) time.Time {
	if job.RunAt != nil {
		return job.RunAt.UTC()
	}
	return job.CreatedAt.UTC()
}

// 规范化计费告警手机，统一计费告警调度链路的输入格式和后续处理行为。
func normalizeBillingAlertPhone(user *domain.User) string {
	if user == nil {
		return ""
	}
	return strings.TrimSpace(user.Phone)
}

// 处理must计费告警JSONBytes相关逻辑，结合当前上下文完成必要的状态转换或结果组装。
func mustBillingAlertJSONBytes(value any) []byte {
	if value == nil {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return payload
}
