package dashboard

import "time"

type RouterCounts struct {
	Total          int64 `json:"total"`
	Online         int64 `json:"online"`
	Stale          int64 `json:"stale"`
	Offline        int64 `json:"offline"`
	NeverConnected int64 `json:"never_connected"`
	Blocked        int64 `json:"blocked"`
}

type CommandCounts struct {
	Pending         int64 `json:"pending"`
	WaitingDevice   int64 `json:"waiting_device"`
	Processing      int64 `json:"processing"`
	RetryScheduled  int64 `json:"retry_scheduled"`
	Failed          int64 `json:"failed"`
	Expired         int64 `json:"expired"`
	CompletedLast24 int64 `json:"completed_last_24_hours"`
}

type RecentConnection struct {
	RouterID     string    `json:"router_id"`
	RouterName   string    `json:"router_name"`
	SerialNumber string    `json:"serial_number"`
	IPAddress    *string   `json:"ip_address"`
	LastSeenAt   time.Time `json:"last_seen_at"`
}

type RecentCommand struct {
	CommandID    string     `json:"command_id"`
	RouterID     string     `json:"router_id"`
	RouterName   string     `json:"router_name"`
	Type         string     `json:"type"`
	Status       string     `json:"status"`
	ErrorCode    *string    `json:"error_code"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
}

type Snapshot struct {
	GeneratedAt       time.Time          `json:"generated_at"`
	Routers           RouterCounts       `json:"routers"`
	Commands          CommandCounts      `json:"commands"`
	RecentConnections []RecentConnection `json:"recent_connections"`
	RecentActivity    []RecentCommand    `json:"recent_activity"`
	RecentFailures    []RecentCommand    `json:"recent_failures"`
}
