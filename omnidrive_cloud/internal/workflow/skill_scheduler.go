package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	appstate "omnidrive_cloud/internal/app"
	"omnidrive_cloud/internal/domain"
	"omnidrive_cloud/internal/store"
)

const (
	skillSchedulerLookAhead = 36 * time.Hour
)

type SkillScheduler struct {
	app          *appstate.App
	pollInterval time.Duration
}

func NewSkillScheduler(app *appstate.App) (*SkillScheduler, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}
	pollSeconds := app.Config.AIWorkerPollSeconds
	if pollSeconds <= 0 {
		pollSeconds = 5
	}
	return &SkillScheduler{
		app:          app,
		pollInterval: time.Duration(pollSeconds) * time.Second,
	}, nil
}

func (s *SkillScheduler) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	s.app.Logger.Info("skill scheduler started", "poll_interval", s.pollInterval.String())

	go func() {
		defer wg.Done()
		s.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		s.app.Logger.Info("skill scheduler stopped")
	}
}

func (s *SkillScheduler) run(ctx context.Context) {
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

func (s *SkillScheduler) runOnce(ctx context.Context) {
	promoted, err := s.app.Store.PromoteDueScheduledAIJobs(ctx, 200)
	if err != nil {
		s.app.Logger.Error("skill scheduler failed to promote scheduled ai jobs", "error", err)
	} else if len(promoted) > 0 {
		s.app.Logger.Debug("skill scheduler promoted scheduled ai jobs", "count", len(promoted))
	}

	if err := s.ensureRecurringAccountSkillRuns(ctx); err != nil {
		s.app.Logger.Error("skill scheduler failed to ensure recurring account skill runs", "error", err)
	}

	// Skill-level execution times are deprecated. New timed runs are created from
	// account-bound skill tasks in OmniBull.
}

func (s *SkillScheduler) ensureRecurringAccountSkillRuns(ctx context.Context) error {
	jobs, err := s.app.Store.ListRecurringAccountSkillTemplateJobs(ctx, 200)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := s.ensureRecurringAccountSkillRun(ctx, job); err != nil {
			s.app.Logger.Error("skill scheduler failed to ensure recurring account skill run", "job_id", job.ID, "error", err)
		}
	}
	return nil
}

func (s *SkillScheduler) ensureRecurringAccountSkillRun(ctx context.Context, seed domain.AIJob) error {
	config, ok := ParseAccountSkillScheduleConfig(seed.InputPayload)
	if !ok || !config.RepeatDaily || strings.TrimSpace(config.ScheduleKey) == "" {
		return nil
	}

	activeJob, err := s.app.Store.FindActiveAccountSkillJobByScheduleKey(ctx, config.ScheduleKey)
	if err != nil {
		return err
	}
	if activeJob != nil {
		return nil
	}

	if seed.SkillID == nil || strings.TrimSpace(*seed.SkillID) == "" {
		return nil
	}
	skill, err := s.app.Store.GetOwnedSkillByID(ctx, strings.TrimSpace(*seed.SkillID), seed.OwnerUserID)
	if err != nil {
		return err
	}
	if skill == nil || !skill.IsEnabled || skill.DeviceID == nil || strings.TrimSpace(*skill.DeviceID) == "" {
		return nil
	}

	accountID := ExtractAccountSkillTargetAccountID(seed.InputPayload)
	if accountID == "" {
		return nil
	}
	account, err := s.app.Store.GetOwnedAccountByID(ctx, accountID, seed.OwnerUserID)
	if err != nil {
		return err
	}
	if account == nil || !AccountAllowedForAutoPublish(account.Status) {
		return nil
	}

	publishAt, err := NextAccountSkillPublishAt(config.TimeOfDay, config.Timezone, time.Now())
	if err != nil {
		return err
	}
	prepared, err := PrepareAccountSkillRun(ctx, s.app, *skill, *account, publishAt, config)
	if err != nil {
		return err
	}
	job, err := s.app.Store.CreateAIJob(ctx, store.CreateAIJobInput{
		ID:           uuid.NewString(),
		OwnerUserID:  seed.OwnerUserID,
		DeviceID:     &account.DeviceID,
		SkillID:      &skill.ID,
		Source:       "account_skill_binding",
		LocalTaskID:  nil,
		JobType:      prepared.JobType,
		ModelName:    prepared.ModelName,
		Prompt:       stringPtr(prepared.Prompt),
		InputPayload: prepared.InputPayload,
		Status:       prepared.Status,
		Message:      stringPtr(prepared.Message),
		RunAt:        &prepared.GenerateAt,
	})
	if err != nil {
		return err
	}
	s.app.Logger.Info(
		"skill scheduler created recurring account skill run",
		"seed_job_id", seed.ID,
		"job_id", job.ID,
		"skill_id", skill.ID,
		"account_id", account.ID,
		"time_of_day", config.TimeOfDay,
		"publish_at", prepared.PublishAt.Format(time.RFC3339),
	)
	return nil
}

func (s *SkillScheduler) ensureScheduledJob(ctx context.Context, skill domain.ProductSkill) error {
	if skill.DeviceID == nil || strings.TrimSpace(*skill.DeviceID) == "" || skill.NextRunAt == nil {
		return nil
	}
	publishAt := skill.NextRunAt.UTC()
	generateAt := ScheduledSkillGenerationTime(publishAt)
	existing, err := s.app.Store.FindScheduledOrActiveAIJobBySkillRun(ctx, skill.ID, generateAt)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	hasAccountBindings, err := s.app.Store.HasActiveAIJobsBySkillAndSource(ctx, skill.OwnerUserID, skill.ID, "account_skill_binding")
	if err != nil {
		return err
	}
	if hasAccountBindings {
		nextRunAt := nextScheduledSkillRunAt(skill, publishAt)
		if _, err := s.app.Store.UpdateSkillScheduleState(ctx, skill.ID, nextRunAt, skill.LastRunAt); err != nil {
			return err
		}
		s.app.Logger.Info(
			"skill scheduler skipped omnibull_local job because account-bound skill runs are active",
			"skill_id", skill.ID,
			"publish_at", publishAt.Format(time.RFC3339),
			"next_run_at", formatOptionalRFC3339(nextRunAt),
		)
		return nil
	}

	jobType, ok := MapSkillOutputTypeToJobType(skill.OutputType)
	if !ok {
		return fmt.Errorf("unsupported skill output type: %s", skill.OutputType)
	}

	payload, err := s.buildJobPayload(ctx, skill, generateAt, publishAt, jobType)
	if err != nil {
		return err
	}

	status := "scheduled"
	message := stringPtr("等待定时生成")
	if !generateAt.After(time.Now().UTC()) {
		status = "queued"
		message = stringPtr("等待云端生成")
	}

	prompt := strings.TrimSpace(stringValue(skill.PromptTemplate))
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Description)
	}
	if prompt == "" {
		prompt = strings.TrimSpace(skill.Name)
	}

	job, err := s.app.Store.CreateAIJob(ctx, store.CreateAIJobInput{
		ID:           uuid.NewString(),
		OwnerUserID:  skill.OwnerUserID,
		DeviceID:     skill.DeviceID,
		SkillID:      &skill.ID,
		Source:       "omnibull_local",
		LocalTaskID:  stringPtr(uuid.NewString()),
		JobType:      jobType,
		ModelName:    strings.TrimSpace(skill.ModelName),
		Prompt:       stringPtr(prompt),
		InputPayload: payload,
		Status:       status,
		Message:      message,
		RunAt:        &generateAt,
	})
	if err != nil {
		return err
	}

	nextRunAt := nextScheduledSkillRunAt(skill, publishAt)
	if _, err := s.app.Store.UpdateSkillScheduleState(ctx, skill.ID, nextRunAt, skill.LastRunAt); err != nil {
		return err
	}

	s.app.Logger.Info(
		"skill scheduler created ai job",
		"skill_id", skill.ID,
		"job_id", job.ID,
		"job_type", job.JobType,
		"generate_at", generateAt.Format(time.RFC3339),
		"publish_at", publishAt.Format(time.RFC3339),
		"status", job.Status,
	)
	return nil
}

func nextScheduledSkillRunAt(skill domain.ProductSkill, publishAt time.Time) *time.Time {
	if !skill.RepeatDaily {
		return nil
	}
	next := publishAt.Add(24 * time.Hour)
	return &next
}

func formatOptionalRFC3339(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func scheduledSkillGenerationTime(publishAt time.Time) time.Time {
	return ScheduledSkillGenerationTime(publishAt)
}

func (s *SkillScheduler) buildJobPayload(ctx context.Context, skill domain.ProductSkill, generateAt time.Time, publishAt time.Time, jobType string) ([]byte, error) {
	accounts, err := s.app.Store.ListAccountsByOwner(ctx, skill.OwnerUserID, stringValue(skill.DeviceID))
	if err != nil {
		return nil, err
	}

	targets := make([]PublishTarget, 0)
	for _, account := range accounts {
		if !AccountAllowedForAutoPublish(account.Status) {
			continue
		}
		targets = append(targets, PublishTarget{
			AccountID:   stringPtr(account.ID),
			Platform:    account.Platform,
			AccountName: account.AccountName,
		})
	}

	return BuildSkillAIJobPayload(ctx, s.app, skill, generateAt, publishAt, jobType, targets)
}

func accountAllowedForAutoPublish(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "invalid", "disabled", "deleted":
		return false
	default:
		return true
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
