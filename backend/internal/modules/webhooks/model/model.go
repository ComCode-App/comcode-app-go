// Package model holds the webhook API types and the registry of subscribable events (TEC-603).
package model

import (
	"time"

	"github.com/google/uuid"
)

// Event names an account can subscribe to. Payloads are documented in docs/WEBHOOKS.md.
const (
	EventLeadCreated        = "lead.created"
	EventCardScanned        = "card.scanned"
	EventOrderCreated       = "order.created"
	EventOrderStatusChanged = "order.status_changed"
	EventWebhookTest        = "webhook.test"
)

// EventDefinition describes one subscribable event for the API and the UI.
type EventDefinition struct {
	Name          string `json:"name"`
	DescriptionTR string `json:"description_tr"`
	DescriptionEN string `json:"description_en"`
}

// Registry lists the subscribable events. webhook.test is sent by "send test" only and cannot be
// subscribed to.
var Registry = []EventDefinition{
	{Name: EventLeadCreated, DescriptionTR: "Yeni iletişim talebi ya da kartvizit paylaşımı geldi", DescriptionEN: "A new lead or contact exchange arrived"},
	{Name: EventCardScanned, DescriptionTR: "Bir kart, akıllı link ya da QR tarandı / açıldı", DescriptionEN: "A card, smart link or QR code was scanned or opened"},
	{Name: EventOrderCreated, DescriptionTR: "Yeni kart siparişi oluşturuldu", DescriptionEN: "A new card order was created"},
	{Name: EventOrderStatusChanged, DescriptionTR: "Sipariş durumu değişti (ödendi, kargoda, teslim edildi…)", DescriptionEN: "An order changed status (paid, shipped, delivered…)"},
}

// IsSubscribable reports whether name is in the registry.
func IsSubscribable(name string) bool {
	for _, def := range Registry {
		if def.Name == name {
			return true
		}
	}
	return false
}

// Endpoint is the API view of a webhook endpoint. The secret is never included; SecretHint
// shows its last characters.
type Endpoint struct {
	UUID                uuid.UUID  `json:"uuid"`
	URL                 string     `json:"url"`
	Description         string     `json:"description"`
	Events              []string   `json:"events"`
	IsActive            bool       `json:"is_active"`
	SecretHint          string     `json:"secret_hint"`
	ConsecutiveFailures int32      `json:"consecutive_failures"`
	LastDeliveryAt      *time.Time `json:"last_delivery_at,omitempty"`
	LastSuccessAt       *time.Time `json:"last_success_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// EndpointWithSecret is returned once: on create and on secret rotation.
type EndpointWithSecret struct {
	Endpoint
	Secret string `json:"secret"`
}

// EndpointInput creates or replaces an endpoint.
type EndpointInput struct {
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Events      []string `json:"events"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

// Delivery is one attempt log of an event to an endpoint.
type Delivery struct {
	UUID           uuid.UUID      `json:"uuid"`
	EventName      string         `json:"event_name"`
	EventID        uuid.UUID      `json:"event_id"`
	Payload        map[string]any `json:"payload"`
	Status         string         `json:"status"`
	Attempts       int32          `json:"attempts"`
	MaxAttempts    int32          `json:"max_attempts"`
	NextAttemptAt  *time.Time     `json:"next_attempt_at,omitempty"`
	LastStatusCode *int32         `json:"last_status_code,omitempty"`
	LastError      string         `json:"last_error,omitempty"`
	LastAttemptAt  *time.Time     `json:"last_attempt_at,omitempty"`
	DeliveredAt    *time.Time     `json:"delivered_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// DeliveryPage is a page of deliveries.
type DeliveryPage struct {
	Items  []Delivery `json:"items"`
	Total  int64      `json:"total"`
	Limit  int32      `json:"limit"`
	Offset int32      `json:"offset"`
}

// Envelope is the JSON body POSTed to an endpoint.
type Envelope struct {
	ID         uuid.UUID      `json:"id"`
	Event      string         `json:"event"`
	EventID    uuid.UUID      `json:"event_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Attempt    int32          `json:"attempt"`
	Data       map[string]any `json:"data"`
}
