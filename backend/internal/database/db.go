package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// URLRecord represents a single shortened URL row persisted in SQLite.
type URLRecord struct {
	ID          int64
	OriginalURL string
	ShortCode   string
	CreatedAt   string
}

var DB *sql.DB

// InitDB opens (creating when needed) the SQLite database file under
// backend/data/urls.db and ensures the schema is migrated.
func InitDB() error {
	dir := filepath.Join("data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dir, "urls.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}

	DB = db

	const schema = `
CREATE TABLE IF NOT EXISTS urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    original_url TEXT NOT NULL,
    short_code TEXT UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_short_code ON urls(short_code);
`
	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}

	return nil
}

// InsertURL persists a new record and returns the auto-generated primary key.
func InsertURL(originalURL string) (int64, error) {
	res, err := DB.Exec(`INSERT INTO urls (original_url) VALUES (?)`, originalURL)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// UpdateShortCode sets the Base62 short code for the given record id.
func UpdateShortCode(id int64, shortCode string) error {
	_, err := DB.Exec(`UPDATE urls SET short_code = ? WHERE id = ?`, shortCode, id)
	return err
}

// GetOriginalURL resolves a short code back to the original URL.
func GetOriginalURL(shortCode string) (string, error) {
	var originalURL string

	err := DB.QueryRow(
		`SELECT original_url FROM urls WHERE short_code = ?`,
		shortCode,
	).Scan(&originalURL)

	return originalURL, err
}

// GetRecentURLs fetches the most recently shortened URLs, newest first.
func GetRecentURLs(limit int) ([]URLRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := DB.Query(`
		SELECT id, original_url, short_code, created_at
		FROM urls
		WHERE short_code IS NOT NULL
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]URLRecord, 0, limit)
	for rows.Next() {
		var rec URLRecord
		if err := rows.Scan(&rec.ID, &rec.OriginalURL, &rec.ShortCode, &rec.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}