package providers

import (
	"context"
	"encoding/json"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/realtime"
	"github.com/google/uuid"
	"log/slog"
)

// DeliveryResult is the outcome of a provider send.
type DeliveryResult struct {
	Status            string
	Provider          string
	ProviderReference string
}

// Provider delivers a notification on a channel.
type Provider interface {
	Channel() string
	Deliver(ctx context.Context, n db.Notification, userUUID *uuid.UUID) (DeliveryResult, error)
}

// InappProvider marks in-app rows as delivered (row itself is the inbox).
type InappProvider struct{}

func (InappProvider) Channel() string { return model.ChannelInapp }

func (InappProvider) Deliver(_ context.Context, _ db.Notification, _ *uuid.UUID) (DeliveryResult, error) {
	return DeliveryResult{Status: model.StatusDelivered, Provider: "inapp"}, nil
}

// AttachmentResolver builds an email's attachments from its row at send time, so rows never
// store file contents (TEC-940: the vCard of the contact exchange visitor copy). It returns nil
// for templates it does not handle.
type AttachmentResolver interface {
	Attachments(ctx context.Context, templateCode string, payload map[string]any) ([]mail.Attachment, error)
}

// EmailProvider sends via SMTP. Attachments may be nil.
type EmailProvider struct {
	Mail        mail.Sender
	Attachments AttachmentResolver
}

func (EmailProvider) Channel() string { return model.ChannelEmail }

func (p EmailProvider) Deliver(ctx context.Context, n db.Notification, _ *uuid.UUID) (DeliveryResult, error) {
	to := ""
	if n.Recipient.Valid {
		to = n.Recipient.String
	}
	if to == "" {
		return DeliveryResult{}, ErrNoRecipient
	}
	if p.Mail == nil {
		p.Mail = mail.NoopSender{}
	}
	var files []mail.Attachment
	if p.Attachments != nil && n.TemplateCode.Valid {
		var payload map[string]any
		_ = json.Unmarshal(n.Payload, &payload)
		var err error
		if files, err = p.Attachments.Attachments(ctx, n.TemplateCode.String, payload); err != nil {
			return DeliveryResult{}, err
		}
	}
	if err := p.Mail.Send(ctx, mail.Message{
		To: []string{to}, Subject: n.Title, Body: n.Body, Attachments: files,
	}); err != nil {
		return DeliveryResult{}, err
	}
	return DeliveryResult{Status: model.StatusSent, Provider: "smtp"}, nil
}

// RealtimeProvider publishes to Centrifugo user channel.
type RealtimeProvider struct {
	Pub realtime.Publisher
}

func (RealtimeProvider) Channel() string { return model.ChannelRealtime }

func (p RealtimeProvider) Deliver(ctx context.Context, n db.Notification, userUUID *uuid.UUID) (DeliveryResult, error) {
	if p.Pub == nil {
		p.Pub = realtime.NoopPublisher{}
	}
	channel := ""
	if userUUID != nil {
		channel = "user:" + userUUID.String()
	}
	if channel == "" {
		return DeliveryResult{Status: model.StatusSent, Provider: "centrifugo"}, nil
	}
	notification := map[string]any{
		"uuid": n.Uuid, "channel": n.Channel, "title": n.Title, "body": n.Body,
		"status": n.Status, "priority": n.Priority,
	}
	if n.ActionUrl.Valid && n.ActionUrl.String != "" {
		notification["action_url"] = n.ActionUrl.String
	}
	if n.TemplateCode.Valid && n.TemplateCode.String != "" {
		notification["template_code"] = n.TemplateCode.String
	}
	// Forward the template payload so the live card renders without a follow-up GET.
	if len(n.Payload) > 0 {
		var extra map[string]any
		if err := json.Unmarshal(n.Payload, &extra); err == nil && len(extra) > 0 {
			notification["payload"] = extra
		}
	}
	payload := map[string]any{
		"type":         "notification.created",
		"notification": notification,
	}
	if err := p.Pub.Publish(ctx, channel, payload); err != nil {
		return DeliveryResult{}, err
	}
	return DeliveryResult{Status: model.StatusSent, Provider: "centrifugo"}, nil
}

// NoopProvider logs and marks sms/push as sent.
type NoopProvider struct {
	Name string
	Log  *slog.Logger
}

func (p NoopProvider) Channel() string { return p.Name }

func (p NoopProvider) Deliver(_ context.Context, n db.Notification, _ *uuid.UUID) (DeliveryResult, error) {
	if p.Log != nil {
		p.Log.Info("notification_noop_provider", "channel", p.Name, "uuid", n.Uuid)
	}
	return DeliveryResult{Status: model.StatusSent, Provider: "noop"}, nil
}

// ErrNoRecipient is returned when email has no recipient.
var ErrNoRecipient = errString("notification recipient required")

type errString string

func (e errString) Error() string { return string(e) }
