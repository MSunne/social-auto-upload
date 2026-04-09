package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	appstate "omnidrive_cloud/internal/app"
)

const quotaExpirySchedulerBatchSize = 200

type QuotaExpiryScheduler struct {
	app          *appstate.App
	pollInterval time.Duration
}

// 创建额度过期Scheduler相关实例，组装运行所需依赖并返回给上层流程复用。
func NewQuotaExpiryScheduler(app *appstate.App) (*QuotaExpiryScheduler, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}

	return &QuotaExpiryScheduler{
		app:          app,
		pollInterval: time.Minute,
	}, nil
}

// 启动额度过期调度流程，持续处理后续轮询、调度或后台任务。
func (s *QuotaExpiryScheduler) Start(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(1)

	s.app.Logger.Info("quota expiry scheduler started", "poll_interval", s.pollInterval.String())

	go func() {
		defer wg.Done()
		s.run(ctx)
	}()

	return func() {
		cancel()
		wg.Wait()
		s.app.Logger.Info("quota expiry scheduler stopped")
	}
}

// 运行额度过期调度流程，按当前上下文驱动一次或持续的业务处理。
func (s *QuotaExpiryScheduler) run(ctx context.Context) {
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
func (s *QuotaExpiryScheduler) runOnce(ctx context.Context) {
	result, err := s.app.Store.ExpireDueQuotaAccounts(ctx, quotaExpirySchedulerBatchSize)
	if err != nil {
		s.app.Logger.Error("quota expiry scheduler failed to expire due quota accounts", "error", err)
		return
	}
	if result == nil || result.ExpiredCount == 0 {
		return
	}

	s.app.Logger.Info(
		"quota expiry scheduler expired due quota accounts",
		"expired_count", result.ExpiredCount,
		"cleared_quota_total", result.ClearedQuotaTotal,
		"distribution_release_credits", result.DistributionReleaseCredits,
	)
}
