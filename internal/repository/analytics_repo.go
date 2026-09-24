package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/coderkamlesh/portfolio_api/internal/database"
	"github.com/coderkamlesh/portfolio_api/internal/models"
)

// AnalyticsRepository reads and writes resume_downloads.
type AnalyticsRepository struct {
	base
}

// NewAnalyticsRepository binds the repository to the database connection.
func NewAnalyticsRepository(db *database.DB) *AnalyticsRepository {
	return &AnalyticsRepository{base{db: db}}
}

// RecordDownload appends one download row.
func (r *AnalyticsRepository) RecordDownload(ctx context.Context, download *models.ResumeDownload) error {
	const q = `INSERT INTO resume_downloads (id, downloaded_at, ip_hash, user_agent, referrer)
	           VALUES (?, ?, ?, ?, ?)`
	if _, err := r.db.ExecContext(ctx, q, download.ID, database.FormatTime(download.DownloadedAt),
		nullString(download.IPHash), nullString(download.UserAgent), nullString(download.Referrer)); err != nil {
		return fmt.Errorf("analytics: record download: %w", err)
	}
	return nil
}

// DownloadTotals aggregates downloads in [from, to].
//
// The daily grouping is done in SQL on the date prefix so the database, not Go,
// decides the bucket boundaries. Rows with an empty ip_hash are excluded from the
// distinct count rather than being counted as one shared visitor: a deployment
// without a hash secret should report zero unique visitors, not one.
func (r *AnalyticsRepository) DownloadTotals(ctx context.Context, from, to time.Time) (*models.DownloadSummary, error) {
	fromText := database.FormatTime(from)
	toText := database.FormatTime(to)

	var total, unique int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT ip_hash)
		   FROM resume_downloads
		  WHERE downloaded_at >= ? AND downloaded_at <= ? AND ip_hash IS NOT NULL AND ip_hash <> ''`,
		fromText, toText).Scan(&total, &unique); err != nil {
		return nil, fmt.Errorf("analytics: download totals: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT substr(downloaded_at, 1, 10) AS day, COUNT(*)
		   FROM resume_downloads
		  WHERE downloaded_at >= ? AND downloaded_at <= ?
		  GROUP BY day
		  ORDER BY day`,
		fromText, toText)
	if err != nil {
		return nil, fmt.Errorf("analytics: downloads by day: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var day string
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return nil, fmt.Errorf("analytics: scan day: %w", err)
		}
		counts[day] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analytics: day rows: %w", err)
	}

	// The day map is sparse, so the response fills in every day in the range with
	// an explicit zero: a chart needs a continuous axis.
	byDay := make([]models.DownloadPoint, 0)
	for day := from.UTC().Format("2006-01-02"); day <= to.UTC().Format("2006-01-02"); day = nextDay(day) {
		byDay = append(byDay, models.DownloadPoint{Date: day, Count: counts[day]})
	}

	return &models.DownloadSummary{
		Total:     total,
		UniqueIPs: unique,
		ByDay:     byDay,
		From:      from.UTC().Format("2006-01-02"),
		To:        to.UTC().Format("2006-01-02"),
	}, nil
}

// nextDay advances a YYYY-MM-DD string by one day.
func nextDay(day string) string {
	parsed, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return parsed.AddDate(0, 0, 1).Format("2006-01-02")
}
