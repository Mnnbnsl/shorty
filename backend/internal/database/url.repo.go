package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound      = errors.New("url not found or expired")
	ErrAlreadyExists = errors.New("short code already in use")
)

// GetNextURLSequence retrieves the next sequence value for generating Base62 short codes.
func GetNextURLSequence(ctx context.Context) (int64, error) {
	var nextVal int64
	query := `SELECT nextval(pg_get_serial_sequence('urls', 'id'))`
	err := Pool.QueryRow(ctx, query).Scan(&nextVal)
	if err != nil {
		return 0, fmt.Errorf("nextval: %w", err)
	}
	return nextVal, nil
}

// InsertURL stores a new URL mapping.
func InsertURL(ctx context.Context, id *int64, originalURL, shortCode string, isCustom bool, userID *string, isEphemeral bool, expiresAt *time.Time) (*URLRecord, error) {
	var rec URLRecord
	var query string
	var err error

	if id != nil {
		query = `
			INSERT INTO urls (id, original_url, short_code, is_custom, user_id, is_ephemeral, expires_at)
			OVERRIDING SYSTEM VALUE
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, original_url, short_code, is_custom, user_id, is_ephemeral, expires_at, created_at, updated_at
		`
		err = Pool.QueryRow(ctx, query, *id, originalURL, shortCode, isCustom, userID, isEphemeral, expiresAt).Scan(
			&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.IsCustom, &rec.UserID,
			&rec.IsEphemeral, &rec.ExpiresAt, &rec.CreatedAt, &rec.UpdatedAt,
		)
	} else {
		query = `
			INSERT INTO urls (original_url, short_code, is_custom, user_id, is_ephemeral, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, original_url, short_code, is_custom, user_id, is_ephemeral, expires_at, created_at, updated_at
		`
		err = Pool.QueryRow(ctx, query, originalURL, shortCode, isCustom, userID, isEphemeral, expiresAt).Scan(
			&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.IsCustom, &rec.UserID,
			&rec.IsEphemeral, &rec.ExpiresAt, &rec.CreatedAt, &rec.UpdatedAt,
		)
	}

	if err != nil {
		// PostgreSQL unique_violation code 23505
		if isUniqueViolation(err) {
			return nil, ErrAlreadyExists
		}
		return nil, fmt.Errorf("insert url: %w", err)
	}

	return &rec, nil
}

// GetOriginalURL finds an active (non-expired) URL by short code.
func GetOriginalURL(ctx context.Context, shortCode string) (*URLRecord, error) {
	var rec URLRecord
	query := `
		SELECT id, original_url, short_code, is_custom, user_id, is_ephemeral, expires_at, created_at, updated_at
		FROM urls
		WHERE short_code = $1 AND (expires_at IS NULL OR expires_at > now())
	`
	err := Pool.QueryRow(ctx, query, shortCode).Scan(
		&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.IsCustom, &rec.UserID,
		&rec.IsEphemeral, &rec.ExpiresAt, &rec.CreatedAt, &rec.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get original url: %w", err)
	}

	return &rec, nil
}

// GetRecentURLs lists the most recently created non-expired URLs.
func GetRecentURLs(ctx context.Context, limit int) ([]URLRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT u.id, u.original_url, u.short_code, u.is_custom, u.user_id, u.is_ephemeral, u.expires_at, u.created_at, u.updated_at,
		       COALESCE(COUNT(c.id), 0) AS clicks_count
		FROM urls u
		LEFT JOIN clicks c ON c.url_id = u.id
		WHERE u.expires_at IS NULL OR u.expires_at > now()
		GROUP BY u.id
		ORDER BY u.id DESC
		LIMIT $1
	`
	rows, err := Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent urls: %w", err)
	}
	defer rows.Close()

	records := make([]URLRecord, 0, limit)
	for rows.Next() {
		var rec URLRecord
		if err := rows.Scan(
			&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.IsCustom, &rec.UserID,
			&rec.IsEphemeral, &rec.ExpiresAt, &rec.CreatedAt, &rec.UpdatedAt, &rec.ClicksCount,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// GetUserURLs lists URLs owned by a specific user.
func GetUserURLs(ctx context.Context, userID string, limit, offset int) ([]URLRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT u.id, u.original_url, u.short_code, u.is_custom, u.user_id, u.is_ephemeral, u.expires_at, u.created_at, u.updated_at,
		       COALESCE(COUNT(c.id), 0) AS clicks_count
		FROM urls u
		LEFT JOIN clicks c ON c.url_id = u.id
		WHERE u.user_id = $1
		GROUP BY u.id
		ORDER BY u.created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get user urls: %w", err)
	}
	defer rows.Close()

	records := make([]URLRecord, 0, limit)
	for rows.Next() {
		var rec URLRecord
		if err := rows.Scan(
			&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.IsCustom, &rec.UserID,
			&rec.IsEphemeral, &rec.ExpiresAt, &rec.CreatedAt, &rec.UpdatedAt, &rec.ClicksCount,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// DeleteUserURL deletes a URL belonging to a user.
func DeleteUserURL(ctx context.Context, shortCode, userID string) (bool, error) {
	res, err := Pool.Exec(ctx, `DELETE FROM urls WHERE short_code = $1 AND user_id = $2`, shortCode, userID)
	if err != nil {
		return false, fmt.Errorf("delete url: %w", err)
	}
	return res.RowsAffected() > 0, nil
}

// DeleteExpiredURLs removes ephemeral URLs whose expiration timestamp has passed.
func DeleteExpiredURLs(ctx context.Context) (int64, error) {
	res, err := Pool.Exec(ctx, `DELETE FROM urls WHERE is_ephemeral = true AND expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("delete expired urls: %w", err)
	}
	return res.RowsAffected(), nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "ERROR: duplicate key value violates unique constraint \"urls_short_code_key\" (SQLSTATE 23505)" ||
		err.Error() == "ERROR: duplicate key value violates unique constraint \"idx_urls_short_code\" (SQLSTATE 23505)" ||
		(len(err.Error()) >= 5 && err.Error()[:5] == "23505") ||
		containsSubstring(err.Error(), "duplicate key")
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}
