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

func NewQuotaExpiryScheduler(app *appstate.App) (*QuotaExpiryScheduler, error) {
	if app == nil {
		return nil, fmt.Errorf("app is required")
	}

	return &QuotaExpiryScheduler{
		app:          app,
		pollInterval: time.Minute,
	}, nil
}

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
