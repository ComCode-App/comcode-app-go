package model

import (
	"time"

	"github.com/google/uuid"
)

type Address struct {
	FullName     string `json:"full_name"`
	Phone        string `json:"phone"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2,omitempty"`
	City         string `json:"city"`
	State        string `json:"state,omitempty"`
	PostalCode   string `json:"postal_code"`
	Country      string `json:"country"`
	TaxID        string `json:"tax_id,omitempty"`
	TaxOffice    string `json:"tax_office,omitempty"`
	CompanyName  string `json:"company_name,omitempty"`
}

type OrderItemSummary struct {
	ID              int64          `json:"-"`
	UUID            string         `json:"uuid"`
	ProductID       *int64         `json:"-"`
	ProductUUID     string         `json:"product_uuid,omitempty"`
	ProductSlug     string         `json:"product_slug,omitempty"`
	ProductName     string         `json:"product_name"`
	VariantName     string         `json:"variant_name,omitempty"`
	SKU             string         `json:"sku"`
	Quantity        int            `json:"quantity"`
	UnitPrice       float64        `json:"unit_price"`
	TotalPrice      float64        `json:"total_price"`
	CustomPrintData map[string]any `json:"custom_print_data,omitempty"`
	ProductImages   []string       `json:"product_images,omitempty"`

	// Recipient Details
	RecipientName       string `json:"recipient_name"`
	RecipientEmail      string `json:"recipient_email"`
	RecipientPhone      string `json:"recipient_phone"`
	RecipientTitle      string `json:"recipient_title"`
	RecipientDepartment string `json:"recipient_department"`

	// Profile Link
	ProfileUUID      *string `json:"profile_uuid,omitempty"`
	ProfileAvatarURL string  `json:"profile_avatar_url,omitempty"`

	// Card Link & Route (TEC-77)
	CardUUID      *string `json:"card_uuid,omitempty"`
	CardSmartCode string  `json:"card_smart_code,omitempty"`
	CardUID       string  `json:"card_uid,omitempty"`
	CardStatus    string  `json:"card_status,omitempty"`
	CardIsActive  bool    `json:"card_is_active"`
	CardIsLocked  bool    `json:"card_is_locked"`
	RoutePath     string  `json:"route_path,omitempty"`
	RouteStatus   string  `json:"route_status,omitempty"`

	// Statuses & Error handling
	FulfillmentStatus  string `json:"fulfillment_status"`
	ProvisioningStatus string `json:"provisioning_status"`
	ProvisioningError  string `json:"provisioning_error,omitempty"`
}

type AssetRequestSummary struct {
	UUID            string    `json:"uuid"`
	RequestType     string    `json:"request_type"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	Status          string    `json:"status"`
	Token           string    `json:"token"`
	UploadedFileURL string    `json:"uploaded_file_url"`
	RejectionReason string    `json:"rejection_reason"`
	RequestedByName string    `json:"requested_by_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type OrderSummary struct {
	UUID              string    `json:"uuid"`
	OrderNumber       string    `json:"order_number"`
	OrderType         string    `json:"order_type"`
	CustomerEmail     string    `json:"customer_email"`
	CustomerName      string    `json:"customer_name"`
	CustomerSurname   string    `json:"customer_surname"`
	CompanyName       string    `json:"company_name,omitempty"`
	AccountUUID       string    `json:"account_uuid"`
	AccountName       string    `json:"account_name"`
	AccountType       string    `json:"account_type"`
	Status            string    `json:"status"`
	FulfillmentStatus string    `json:"fulfillment_status"`
	GrandTotal        float64   `json:"grand_total"`
	Currency          string    `json:"currency"`
	PaymentStatus     string    `json:"payment_status"`
	TrackingNumber    string    `json:"tracking_number,omitempty"`
	ItemsCount        int       `json:"items_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type OrderDetail struct {
	ID                int64                 `json:"-"`
	UUID              string                `json:"uuid"`
	OrderNumber       string                `json:"order_number"`
	OrderType         string                `json:"order_type"`
	AccountID         int64                 `json:"-"`
	AccountUUID       string                `json:"account_uuid"`
	AccountName       string                `json:"account_name"`
	AccountType       string                `json:"account_type"`
	UserID            int64                 `json:"-"`
	UserUUID          string                `json:"user_uuid"`
	CustomerEmail     string                `json:"customer_email"`
	CustomerName      string                `json:"customer_name"`
	CustomerSurname   string                `json:"customer_surname"`
	CompanyName       string                `json:"company_name,omitempty"`
	Status            string                `json:"status"`
	FulfillmentStatus string                `json:"fulfillment_status"`
	Subtotal          float64               `json:"subtotal"`
	TaxTotal          float64               `json:"tax_total"`
	ShippingTotal     float64               `json:"shipping_total"`
	DiscountTotal     float64               `json:"discount_total"`
	GrandTotal        float64               `json:"grand_total"`
	Currency          string                `json:"currency"`
	PaymentStatus     string                `json:"payment_status"`
	PaymentProvider   string                `json:"payment_provider"`
	PaymentReference  string                `json:"payment_reference"`
	ShippingAddress   Address               `json:"shipping_address"`
	BillingAddress    Address               `json:"billing_address"`
	ShippingCarrier   string                `json:"shipping_carrier"`
	TrackingNumber    string                `json:"tracking_number"`
	TrackingURL       string                `json:"tracking_url"`
	CustomerNote      string                `json:"customer_note"`
	AdminNote         string                `json:"admin_note"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
	PaidAt            *time.Time            `json:"paid_at,omitempty"`
	ShippedAt         *time.Time            `json:"shipped_at,omitempty"`
	DeliveredAt       *time.Time            `json:"delivered_at,omitempty"`
	CancelledAt       *time.Time            `json:"cancelled_at,omitempty"`
	Items             []OrderItemSummary    `json:"items"`
	AssetRequests     []AssetRequestSummary `json:"asset_requests"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type OrderOverviewKPIs struct {
	TotalOrders         int64   `json:"total_orders"`
	TodayOrders         int64   `json:"today_orders"`
	PendingOrders       int64   `json:"pending_orders"`
	DesignPendingOrders int64   `json:"design_pending_orders"`
	InProductionOrders  int64   `json:"in_production_orders"`
	ShippedOrders       int64   `json:"shipped_orders"`
	TotalRevenue        float64 `json:"total_revenue"`
}

type CreateOrderItemInput struct {
	ProductUUID         string         `json:"product_uuid"`
	VariantUUID         *string        `json:"variant_uuid,omitempty"`
	Quantity            int            `json:"quantity"`
	CustomPrintData     map[string]any `json:"custom_print_data,omitempty"`
	RecipientName       string         `json:"recipient_name,omitempty"`
	RecipientEmail      string         `json:"recipient_email,omitempty"`
	RecipientPhone      string         `json:"recipient_phone,omitempty"`
	RecipientTitle      string         `json:"recipient_title,omitempty"`
	RecipientDepartment string         `json:"recipient_department,omitempty"`
	ProfileUUID         *string        `json:"profile_uuid,omitempty"`
	IdempotencyKey      string         `json:"idempotency_key,omitempty"`
}

type CreateOrderInput struct {
	AccountUUID     *string                `json:"account_uuid,omitempty"`
	UserUUID        *string                `json:"user_uuid,omitempty"`
	CompanyUUID     *string                `json:"company_uuid,omitempty"`
	OrderType       string                 `json:"order_type,omitempty"` // standard or bulk
	Items           []CreateOrderItemInput `json:"items"`
	ShippingAddress Address                `json:"shipping_address"`
	BillingAddress  Address                `json:"billing_address"`
	CustomerNote    string                 `json:"customer_note,omitempty"`
	AdminNote       string                 `json:"admin_note,omitempty"`
	PaymentProvider string                 `json:"payment_provider,omitempty"`
	Metadata        map[string]any         `json:"metadata,omitempty"`
}

type UpdateOrderStatusInput struct {
	Status string `json:"status"`
}

type UpdateOrderPaymentStatusInput struct {
	PaymentStatus    string  `json:"payment_status"`
	PaymentProvider  *string `json:"payment_provider,omitempty"`
	PaymentReference *string `json:"payment_reference,omitempty"`
}

type UpdateOrderFulfillmentStatusInput struct {
	FulfillmentStatus string `json:"fulfillment_status"`
}

type UpdateOrderShippingInput struct {
	ShippingCarrier string `json:"shipping_carrier"`
	TrackingNumber  string `json:"tracking_number"`
	TrackingURL     string `json:"tracking_url,omitempty"`
	MarkAsShipped   bool   `json:"mark_as_shipped"`
}

type UpdateOrderNotesInput struct {
	AdminNote    *string `json:"admin_note,omitempty"`
	CustomerNote *string `json:"customer_note,omitempty"`
}

type BulkUpdateOrderItemsInput struct {
	ItemUUIDs         []string `json:"item_uuids"`
	FulfillmentStatus *string  `json:"fulfillment_status,omitempty"`
}

type ProvisionOrderInput struct {
	AutoReserveRoutes bool `json:"auto_reserve_routes"`
}

type CreateAssetRequestInput struct {
	RequestType string `json:"request_type"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ReviewAssetRequestInput struct {
	Status          string `json:"status"` // "approved" or "rejected"
	RejectionReason string `json:"rejection_reason,omitempty"`
}

type ReviewAssetInput = ReviewAssetRequestInput

// ManifestCard is one provisioned card in an order's production manifest (TEC-581).
type ManifestCard struct {
	OrderNumber string
	CardName    string
	SmartCode   string
	CardUID     string
}

// RetiredCardRoute is a card route retired because its order was cancelled or refunded (TEC-585).
type RetiredCardRoute struct {
	Path     string
	CardUUID uuid.UUID
}

// NotificationEvent is a committed order milestone, scoped to its owning account.
type NotificationEvent struct {
	ID             int64
	Status         string
	Snapshot       map[string]string
	CustomerID     int64
	CustomerName   string
	CustomerLocale string
}

// PaymentTarget is what an online checkout charges for an order (TEC-943). AmountMinor is the
// grand total in kuruş.
type PaymentTarget struct {
	ID            int64
	UUID          uuid.UUID
	AccountID     int64
	OrderNumber   string
	Status        string
	PaymentStatus string
	AmountMinor   int64
	Currency      string
}

// ProviderPaymentResult says what a verified provider payment did to its order.
type ProviderPaymentResult string

const (
	// ProviderPaymentApplied: payment_status became paid (and a pending order became paid).
	ProviderPaymentApplied ProviderPaymentResult = "applied"
	// ProviderPaymentAlreadyPaid: the order was already paid, e.g. by another checkout or by hand.
	ProviderPaymentAlreadyPaid ProviderPaymentResult = "already_paid"
	// ProviderPaymentOrderClosed: the order was cancelled, refunded or deleted before the money
	// arrived; nothing changes and the payment has to be refunded at the provider.
	ProviderPaymentOrderClosed ProviderPaymentResult = "order_closed"
)

// ProviderPaymentOutcome is returned by ApplyProviderPayment.
type ProviderPaymentOutcome struct {
	Result    ProviderPaymentResult
	OrderUUID uuid.UUID
}

// Payment statuses set by provider refunds (TEC-935). A partially refunded order stays open; a
// fully refunded one is closed and its cards are retired.
const (
	PaymentStatusRefunded          = "refunded"
	PaymentStatusPartiallyRefunded = "partially_refunded"
)
