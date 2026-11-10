package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	ChannelInapp    = "inapp"
	ChannelEmail    = "email"
	ChannelRealtime = "realtime"
	ChannelSMS      = "sms"
	ChannelPush     = "push"

	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusSent       = "sent"
	StatusDelivered  = "delivered"
	StatusRead       = "read"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"

	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// Notification is the API projection.
type Notification struct {
	UUID            uuid.UUID         `json:"uuid"`
	Channel         string            `json:"channel"`
	Status          string            `json:"status"`
	Priority        string            `json:"priority"`
	Title           string            `json:"title"`
	Body            string            `json:"body"`
	Payload         map[string]any    `json:"payload"`
	ActionURL       *string           `json:"action_url,omitempty"`
	SignedActionURL *string           `json:"signed_action_url,omitempty"`
	Recipient       *string           `json:"recipient,omitempty"`
	TemplateCode    *string           `json:"template_code,omitempty"`
	SourceEvent     *string           `json:"source_event,omitempty"`
	SentAt          *time.Time        `json:"sent_at,omitempty"`
	ReadAt          *time.Time        `json:"read_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	User            *NotificationUser `json:"user,omitempty"`
}

// NotificationUser is the target user on a platform notification row.
type NotificationUser struct {
	UUID    uuid.UUID `json:"uuid"`
	Name    string    `json:"name"`
	Surname string    `json:"surname"`
	Email   string    `json:"email"`
}

// PushDevice is a registered mobile device (Expo push token, TEC-1186). Current marks the
// device registered from the caller's own session.
type PushDevice struct {
	UUID           uuid.UUID  `json:"uuid"`
	Token          string     `json:"token"`
	Platform       string     `json:"platform"`
	Locale         *string    `json:"locale"`
	AppVersion     *string    `json:"app_version"`
	DeviceName     *string    `json:"device_name"`
	Current        bool       `json:"current"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	DisabledAt     *time.Time `json:"disabled_at"`
	DisabledReason *string    `json:"disabled_reason"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Preferences is the user notification preference set.
type Preferences struct {
	EmailEnabled    bool `json:"email_enabled"`
	InappEnabled    bool `json:"inapp_enabled"`
	RealtimeEnabled bool `json:"realtime_enabled"`
	PushEnabled     bool `json:"push_enabled"`
}

// EnqueueInput creates one or more queued notifications.
type EnqueueInput struct {
	TenantID *int64
	// AccountID is the account the notification is about. Its public uuid is stored in the
	// payload as account_uuid (mobile push data, TEC-1187).
	AccountID    *int64
	WorkspaceID  *int64
	UserID       *int64
	UserUUID     *uuid.UUID
	Channels     []string
	Priority     string
	Title        string
	Body         string
	Payload      map[string]any
	ActionURL    *string
	Recipient    *string
	TemplateCode string
	TemplateVars map[string]string
	SourceEvent  string
	Language     string
	// SecurityEmail bypasses email preference (password reset / verification) and
	// treats the default secret template vars (see usecase.DefaultSecretVars) as secret.
	SecurityEmail bool
	// SecretVars names further template vars that must never be stored readable. Rows keep
	// a masked rendering; the real email is stored encrypted until delivery succeeds.
	SecretVars []string
}
