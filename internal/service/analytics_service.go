package service

import (
	"context"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AnalyticsDeps wires the analytics service.
type AnalyticsDeps struct {
	Analytics AnalyticsStore
	Now       func() time.Time
}

// AnalyticsService serves the admin download dashboard.
type AnalyticsService struct {
	analytics AnalyticsStore
	now       func() time.Time
}

// NewAnalyticsService builds the service.
func NewAnalyticsService(deps AnalyticsDeps) *AnalyticsService {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &AnalyticsService{analytics: deps.Analytics, now: now}
}

// Analytics range bounds. The admin panel asks for a chart over a recent window,
// and an unbounded range would make the per-day series grow without limit.
const (
	defaultAnalyticsDays = 30
	maxAnalyticsDays     = 365
)

// DownloadStats returns the download totals for a window ending today.
//
// A zero or negative days value falls back to the default, and the range is
// clamped to maxAnalyticsDays so a crafted query cannot ask for years of daily
// points.
func (s *AnalyticsService) DownloadStats(ctx context.Context, days int) (*models.DownloadSummary, error) {
	if days <= 0 {
		days = defaultAnalyticsDays
	}
	if days > maxAnalyticsDays {
		days = maxAnalyticsDays
	}

	// Both bounds are derived from the start of today. Adding a day to a mid-day
	// timestamp would land on the same time tomorrow rather than the end of
	// today, which would report a future day and add a phantom zero to the
	// series.
	today := s.now().UTC()
	startOfToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	from := startOfToday.AddDate(0, 0, -(days - 1))
	to := startOfToday.AddDate(0, 0, 1).Add(-time.Nanosecond)

	summary, err := s.analytics.DownloadTotals(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if summary.ByDay == nil {
		summary.ByDay = []models.DownloadPoint{}
	}
	return summary, nil
}
