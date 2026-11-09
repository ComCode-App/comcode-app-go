package repository

import (
	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/google/uuid"
)

type Repository interface {
	ListPendingNotifications(ctx context.Context, accountID, orderID int64) ([]model.NotificationEvent, error)
	MarkNotificationDispatched(ctx context.Context, accountID, orderID, eventID int64) error
	List(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string, limit, offset int32) ([]model.OrderSummary, error)
	Count(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string) (int64, error)
	GetByUUID(ctx context.Context, id uuid.UUID) (*model.OrderDetail, error)
	GetByOrderNumber(ctx context.Context, orderNumber string) (*model.OrderDetail, error)
	Create(ctx context.Context, order model.CreateOrderInput, orderNumber string, accountID, placedByUserID int64, subtotal, taxTotal, grandTotal float64, currency string) (*model.OrderDetail, error)
	// UpdateStatus and UpdateShipping record the transition in order_status_history in the same
	// transaction; actorUserID is nil for system transitions.
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, actorUserID *int64) (*model.OrderDetail, error)
	// UpdatePaymentStatus also moves a pending order to paid (recorded in order_status_history)
	// when paymentStatus is paid.
	UpdatePaymentStatus(ctx context.Context, id uuid.UUID, paymentStatus string, provider, reference *string, actorUserID *int64) (*model.OrderDetail, error)
	// PaymentTarget returns the amount to charge for an order; nil when it does not exist.
	PaymentTarget(ctx context.Context, id uuid.UUID) (*model.PaymentTarget, error)
	// ApplyProviderPayment records a verified provider payment on the order (TEC-943).
	ApplyProviderPayment(ctx context.Context, orderID int64, provider, reference string) (model.ProviderPaymentOutcome, error)
	// ApplyProviderRefund records a succeeded provider refund on the order (TEC-935).
	ApplyProviderRefund(ctx context.Context, orderID int64, full bool, actorUserID *int64) (uuid.UUID, error)
	UpdateFulfillmentStatus(ctx context.Context, id uuid.UUID, fulfillmentStatus string) (*model.OrderDetail, error)
	UpdateShipping(ctx context.Context, id uuid.UUID, carrier, trackingNumber, trackingURL string, markShipped bool, actorUserID *int64) (*model.OrderDetail, error)
	UpdateNotes(ctx context.Context, id uuid.UUID, adminNote, customerNote *string) (*model.OrderDetail, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetOverviewKPIs(ctx context.Context) (*model.OrderOverviewKPIs, error)

	// Items
	ListItems(ctx context.Context, orderID int64) ([]model.OrderItemSummary, error)
	GetItemByUUID(ctx context.Context, itemUUID uuid.UUID) (*model.OrderItemSummary, error)
	CreateItem(ctx context.Context, orderID int64, productID, variantID *int64, productName, variantName, sku string, qty int, unitPrice, totalPrice float64, customPrint map[string]any, recipientName, recipientEmail, recipientPhone, recipientTitle, recipientDept string, profileID, cardID *int64, fulfillmentStatus, provisioningStatus, provisioningError, idempotencyKey string) (*model.OrderItemSummary, error)
	UpdateItemProvisioning(ctx context.Context, itemUUID uuid.UUID, cardID *int64, provisioningStatus, provisioningError string) (*model.OrderItemSummary, error)
	UpdateItemFulfillment(ctx context.Context, itemUUID uuid.UUID, fulfillmentStatus string) (*model.OrderItemSummary, error)

	// Card & Route Provisioning (TEC-77 Integration)
	CreateProvisionedCardAndRoute(ctx context.Context, orderID, orderItemID, accountID, userID int64, profileID *int64, cardName, smartCode, activationCode, cardUID, material, colorHex string) (cardID int64, cardUUID uuid.UUID, routeUUID uuid.UUID, err error)
	GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error)
	GetAccountIDByUUID(ctx context.Context, accountUUID uuid.UUID) (*int64, error)
	GetPersonalAccountIDByUserUUID(ctx context.Context, userUUID uuid.UUID) (*int64, error)
	GetUserIDByUUID(ctx context.Context, userUUID uuid.UUID) (*int64, error)
	ListManifestCards(ctx context.Context, orderUUID uuid.UUID) ([]model.ManifestCard, error)
	RetireCardRoutes(ctx context.Context, orderID int64) ([]model.RetiredCardRoute, error)

	// Asset Requests
	ListAssetRequests(ctx context.Context, orderID int64) ([]model.AssetRequestSummary, error)
	GetAssetRequestByUUID(ctx context.Context, id uuid.UUID) (*model.AssetRequestSummary, error)
	GetAssetRequestByToken(ctx context.Context, token string) (*model.AssetRequestSummary, int64, string, error)
	CreateAssetRequest(ctx context.Context, orderID int64, reqType, title, desc, token string, requestedBy *int64) (*model.AssetRequestSummary, error)
	UpdateAssetRequestStatus(ctx context.Context, id uuid.UUID, status, uploadedFileURL, rejectionReason string) (*model.AssetRequestSummary, error)
}
