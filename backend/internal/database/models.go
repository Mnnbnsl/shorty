package database

import "time"

// User represents an account holder in the system.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	DisplayName  string    `json:"displayName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// URLRecord represents a shortened URL entry.
type URLRecord struct {
	ID          int64      `json:"id"`
	OriginalURL string     `json:"originalUrl"`
	ShortCode   string     `json:"shortCode"`
	IsCustom    bool       `json:"isCustom"`
	UserID      *string    `json:"userId,omitempty"`
	IsEphemeral bool       `json:"isEphemeral"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClicksCount int64      `json:"clicksCount"`
}

// ClickRecord represents a recorded click event.
type ClickRecord struct {
	ID         int64     `json:"id"`
	URLID      int64     `json:"urlId"`
	ClickedAt  time.Time `json:"clickedAt"`
	IPHash     string    `json:"ipHash,omitempty"`
	UserAgent  string    `json:"userAgent,omitempty"`
	Referer    string    `json:"referer,omitempty"`
	DeviceType string    `json:"deviceType,omitempty"`
}

// RefreshToken stores hashed refresh tokens.
type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// DailyClicks represents click counts for a single calendar day.
type DailyClicks struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

// RefererStat represents visitor counts per referral domain.
type RefererStat struct {
	Referer string `json:"referer"`
	Clicks  int64  `json:"clicks"`
}

// DeviceStat represents breakdown by device category.
type DeviceStat struct {
	Device string `json:"device"`
	Clicks int64  `json:"clicks"`
}

// URLStats represents aggregated metrics for a short code.
type URLStats struct {
	ShortCode       string        `json:"shortCode"`
	OriginalURL     string        `json:"originalUrl"`
	CreatedAt       time.Time     `json:"createdAt"`
	IsEphemeral     bool          `json:"isEphemeral"`
	ExpiresAt       *time.Time    `json:"expiresAt,omitempty"`
	TotalClicks     int64         `json:"totalClicks"`
	UniqueVisitors  int64         `json:"uniqueVisitors"`
	ClicksByDay     []DailyClicks `json:"clicksByDay"`
	TopReferers     []RefererStat `json:"topReferers"`
	DeviceBreakdown []DeviceStat  `json:"deviceBreakdown"`
}
