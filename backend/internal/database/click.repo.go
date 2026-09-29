package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// BatchInsertClicks inserts multiple click records using PostgreSQL COPY for maximum performance.
func BatchInsertClicks(ctx context.Context, clicks []ClickRecord) error {
	if len(clicks) == 0 {
		return nil
	}

	rows := make([][]interface{}, len(clicks))
	for i, c := range clicks {
		rows[i] = []interface{}{
			c.URLID,
			c.ClickedAt,
			c.IPHash,
			c.UserAgent,
			c.Referer,
			c.DeviceType,
		}
	}

	_, err := Pool.CopyFrom(
		ctx,
		pgx.Identifier{"clicks"},
		[]string{"url_id", "clicked_at", "ip_hash", "user_agent", "referer", "device_type"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy from clicks: %w", err)
	}

	return nil
}

// GetURLStats aggregates analytics for a specific short code.
func GetURLStats(ctx context.Context, shortCode string) (*URLStats, error) {
	var stats URLStats
	var urlID int64

	urlQuery := `
		SELECT id, short_code, original_url, created_at, is_ephemeral, expires_at
		FROM urls
		WHERE short_code = $1
	`
	err := Pool.QueryRow(ctx, urlQuery, shortCode).Scan(
		&urlID, &stats.ShortCode, &stats.OriginalURL, &stats.CreatedAt,
		&stats.IsEphemeral, &stats.ExpiresAt,
	)
	if err != nil {
		return nil, ErrNotFound
	}

	// Total clicks & unique visitors
	aggQuery := `
		SELECT COUNT(*), COUNT(DISTINCT ip_hash)
		FROM clicks
		WHERE url_id = $1
	`
	_ = Pool.QueryRow(ctx, aggQuery, urlID).Scan(&stats.TotalClicks, &stats.UniqueVisitors)

	// Daily clicks (past 14 days)
	dailyQuery := `
		SELECT TO_CHAR(clicked_at, 'YYYY-MM-DD') AS day, COUNT(*) AS count
		FROM clicks
		WHERE url_id = $1
		GROUP BY day
		ORDER BY day DESC
		LIMIT 14
	`
	dailyRows, err := Pool.Query(ctx, dailyQuery, urlID)
	if err == nil {
		defer dailyRows.Close()
		stats.ClicksByDay = make([]DailyClicks, 0)
		for dailyRows.Next() {
			var d DailyClicks
			if err := dailyRows.Scan(&d.Date, &d.Clicks); err == nil {
				stats.ClicksByDay = append(stats.ClicksByDay, d)
			}
		}
	}

	// Top Referers
	refQuery := `
		SELECT COALESCE(NULLIF(referer, ''), 'direct') AS ref, COUNT(*) AS count
		FROM clicks
		WHERE url_id = $1
		GROUP BY ref
		ORDER BY count DESC
		LIMIT 5
	`
	refRows, err := Pool.Query(ctx, refQuery, urlID)
	if err == nil {
		defer refRows.Close()
		stats.TopReferers = make([]RefererStat, 0)
		for refRows.Next() {
			var r RefererStat
			if err := refRows.Scan(&r.Referer, &r.Clicks); err == nil {
				stats.TopReferers = append(stats.TopReferers, r)
			}
		}
	}

	// Device breakdown
	devQuery := `
		SELECT COALESCE(NULLIF(device_type, ''), 'desktop') AS dev, COUNT(*) AS count
		FROM clicks
		WHERE url_id = $1
		GROUP BY dev
		ORDER BY count DESC
	`
	devRows, err := Pool.Query(ctx, devQuery, urlID)
	if err == nil {
		defer devRows.Close()
		stats.DeviceBreakdown = make([]DeviceStat, 0)
		for devRows.Next() {
			var d DeviceStat
			if err := devRows.Scan(&d.Device, &d.Clicks); err == nil {
				stats.DeviceBreakdown = append(stats.DeviceBreakdown, d)
			}
		}
	}

	return &stats, nil
}

// GetURLClicks lists raw click records for a short code with pagination.
func GetURLClicks(ctx context.Context, shortCode string, limit, offset int) ([]ClickRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT c.id, c.url_id, c.clicked_at, c.ip_hash, c.user_agent, c.referer, c.device_type
		FROM clicks c
		JOIN urls u ON u.id = c.url_id
		WHERE u.short_code = $1
		ORDER BY c.clicked_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := Pool.Query(ctx, query, shortCode, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get url clicks: %w", err)
	}
	defer rows.Close()

	clicks := make([]ClickRecord, 0, limit)
	for rows.Next() {
		var c ClickRecord
		if err := rows.Scan(&c.ID, &c.URLID, &c.ClickedAt, &c.IPHash, &c.UserAgent, &c.Referer, &c.DeviceType); err != nil {
			return nil, err
		}
		clicks = append(clicks, c)
	}

	return clicks, rows.Err()
}
