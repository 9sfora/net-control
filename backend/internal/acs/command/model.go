package command

import (
	"encoding/json"
	"time"
)

type Item struct {
	ID             string          `json:"id"`
	RouterID       string          `json:"router_id"`
	RouterName     string          `json:"router_name"`
	Type           string          `json:"type"`
	Status         string          `json:"status"`
	Priority       string          `json:"priority"`
	AttemptsCount  int             `json:"attempts_count"`
	MaxAttempts    int             `json:"max_attempts"`
	IdempotencyKey *string         `json:"idempotency_key"`
	AvailableAt    time.Time       `json:"available_at"`
	NextAttemptAt  *time.Time      `json:"next_attempt_at"`
	ExpiresAt      time.Time       `json:"expires_at"`
	CompletedAt    *time.Time      `json:"completed_at"`
	ErrorCode      *string         `json:"error_code"`
	ErrorMessage   *string         `json:"error_message"`
	Result         json.RawMessage `json:"result,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type Attempt struct {
	ID            string          `json:"id"`
	SessionID     string          `json:"session_id"`
	AttemptNumber int             `json:"attempt_number"`
	Status        string          `json:"status"`
	Response      json.RawMessage `json:"response,omitempty"`
	ErrorCode     *string         `json:"error_code"`
	ErrorMessage  *string         `json:"error_message"`
	DurationMS    *int            `json:"duration_ms"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at"`
}

type Event struct {
	ID         string          `json:"id"`
	FromStatus *string         `json:"from_status"`
	ToStatus   string          `json:"to_status"`
	Reason     *string         `json:"reason"`
	Metadata   json.RawMessage `json:"metadata"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Detail struct {
	Item
	Parameters json.RawMessage `json:"parameters"`
	Attempts   []Attempt       `json:"attempts"`
	Events     []Event         `json:"events"`
}

type Filter struct {
	Status   string
	RouterID string
	Limit    int
	Offset   int
}
