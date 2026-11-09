package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	notifModel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	productsRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
)

var (
	ErrNotFound           = errors.New("order not found")
	ErrItemNotFound       = errors.New("order item not found")
	ErrAssetNotFound      = errors.New("asset request not found")
	ErrInvalidToken       = errors.New("invalid or expired asset token")
	ErrEmptyItems         = errors.New("order must contain at least one item")
	ErrForbidden          = errors.New("account role cannot place orders")
	ErrInvalidAccount     = errors.New("target account not found")
	ErrInvalidProduct     = errors.New("invalid or missing product")
	ErrAlreadyProvisioned = errors.New("item is already provisioned")
	ErrNoProvisionedCards = errors.New("order has no provisioned cards")
	ErrOrderClosed        = errors.New("order is cancelled or refunded")
)

type Service struct {
	repo         repository.Repository
	productsRepo productsRepo.Repository
	notifSvc     orderNotifier
	storage      storage.Driver
	frontendURL  string
	routeCache   *routecache.Cache
	search       searchengine.Enqueuer
	refunder     PaymentRefunder
	webhooks     Webhooks
}

func NewService(repo repository.Repository, prodRepo productsRepo.Repository, notifSvc orderNotifier, store storage.Driver, frontendURL string) *Service {
	return &Service{
		repo:         repo,
		productsRepo: prodRepo,
		notifSvc:     notifSvc,
		storage:      store,
		frontendURL:  strings.TrimRight(frontendURL, "/"),
	}
}

// ListForAccount lists the orders of an account (app routes).
func (s *Service) ListForAccount(ctx context.Context, account authctx.Account, q *string, status, paymentStatus, fulfillmentStatus, orderType *string, limit, offset int32) ([]model.OrderSummary, int64, error) {
	accountID := account.ID
	return s.list(ctx, q, &accountID, status, paymentStatus, fulfillmentStatus, orderType, limit, offset)
}

// ListAll lists orders across accounts for the platform panel; accountID optionally filters.
func (s *Service) ListAll(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string, limit, offset int32) ([]model.OrderSummary, int64, error) {
	return s.list(ctx, q, accountID, status, paymentStatus, fulfillmentStatus, orderType, limit, offset)
}

func (s *Service) list(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string, limit, offset int32) ([]model.OrderSummary, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	items, err := s.repo.List(ctx, q, accountID, status, paymentStatus, fulfillmentStatus, orderType, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.repo.Count(ctx, q, accountID, status, paymentStatus, fulfillmentStatus, orderType)
	if err != nil {
		return nil, 0, err
	}

	return items, count, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.OrderDetail, error) {
	ord, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	return ord, nil
}

// GetForAccount returns an order of the account; orders of other accounts are not found.
func (s *Service) GetForAccount(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.OrderDetail, error) {
	ord, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if ord.AccountID != account.ID {
		return nil, ErrNotFound
	}
	return ord, nil
}

func (s *Service) GetByOrderNumber(ctx context.Context, orderNum string) (*model.OrderDetail, error) {
	ord, err := s.repo.GetByOrderNumber(ctx, orderNum)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	return ord, nil
}

func (s *Service) GetOverviewKPIs(ctx context.Context) (*model.OrderOverviewKPIs, error) {
	return s.repo.GetOverviewKPIs(ctx)
}

// CreateForAccount places an order for the caller's active account (owner/admin only).
func (s *Service) CreateForAccount(ctx context.Context, account authctx.Account, placedByUserID int64, input model.CreateOrderInput) (*model.OrderDetail, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	return s.indexed(s.create(ctx, input, account.ID, placedByUserID))
}

// CreateOnBehalf places an order from the platform panel for the account named by input
// (account_uuid, company_uuid or user_uuid -> personal account); without a target it uses
// fallbackAccountID (the admin's active account).
func (s *Service) CreateOnBehalf(ctx context.Context, input model.CreateOrderInput, placedByUserID, fallbackAccountID int64) (*model.OrderDetail, error) {
	accountID, err := s.resolveTargetAccount(ctx, input, fallbackAccountID)
	if err != nil {
		return nil, err
	}
	return s.indexed(s.create(ctx, input, accountID, placedByUserID))
}

func (s *Service) resolveTargetAccount(ctx context.Context, input model.CreateOrderInput, fallback int64) (int64, error) {
	lookup := func(raw *string, fn func(context.Context, uuid.UUID) (*int64, error)) (int64, bool, error) {
		if raw == nil || strings.TrimSpace(*raw) == "" {
			return 0, false, nil
		}
		id, err := uuid.Parse(strings.TrimSpace(*raw))
		if err != nil {
			return 0, true, ErrInvalidAccount
		}
		accountID, err := fn(ctx, id)
		if err != nil {
			return 0, true, err
		}
		if accountID == nil {
			return 0, true, ErrInvalidAccount
		}
		return *accountID, true, nil
	}
	// company_uuid is kept for compatibility: company accounts reuse the company uuid.
	for _, candidate := range []struct {
		raw *string
		fn  func(context.Context, uuid.UUID) (*int64, error)
	}{
		{input.AccountUUID, s.repo.GetAccountIDByUUID},
		{input.CompanyUUID, s.repo.GetAccountIDByUUID},
		{input.UserUUID, s.repo.GetPersonalAccountIDByUserUUID},
	} {
		if id, given, err := lookup(candidate.raw, candidate.fn); given {
			return id, err
		}
	}
	return fallback, nil
}

func (s *Service) create(ctx context.Context, input model.CreateOrderInput, accountID, placedByUserID int64) (*model.OrderDetail, error) {
	if len(input.Items) == 0 {
		return nil, ErrEmptyItems
	}

	orderNumber := fmt.Sprintf("ORD-%s-%s", time.Now().Format("20060102"), strings.ToUpper(uuid.New().String()[:6]))

	var subtotal float64
	var taxTotal float64

	type processedItem struct {
		productID      *int64
		variantID      *int64
		productName    string
		variantName    string
		sku            string
		qty            int
		unitPrice      float64
		totalPrice     float64
		customPrint    map[string]any
		recipientName  string
		recipientMail  string
		recipientPhone string
		recipientTitle string
		recipientDept  string
		profileID      *int64
		idempotencyKey string
	}

	var pItems []processedItem

	for _, it := range input.Items {
		prodUUID, err := uuid.Parse(it.ProductUUID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid product uuid %q", ErrInvalidProduct, it.ProductUUID)
		}

		prod, err := s.productsRepo.GetByUUID(ctx, prodUUID)
		if err != nil || prod == nil {
			return nil, fmt.Errorf("%w: product not found %q", ErrInvalidProduct, it.ProductUUID)
		}

		unitPrice := prod.BasePrice
		sku := prod.SKU
		var variantID *int64
		var variantName string

		if it.VariantUUID != nil && *it.VariantUUID != "" {
			vUUID, err := uuid.Parse(*it.VariantUUID)
			if err == nil {
				v, err := s.productsRepo.GetVariantByUUID(ctx, vUUID)
				if err == nil && v != nil {
					unitPrice += v.PriceDelta
					if v.SKU != "" {
						sku = v.SKU
					}
					variantName = v.Name
				}
			}
		}

		qty := it.Quantity
		if qty <= 0 {
			qty = 1
		}
		itemTotal := unitPrice * float64(qty)
		subtotal += itemTotal

		var profileID *int64
		if it.ProfileUUID != nil && *it.ProfileUUID != "" {
			pUUID, err := uuid.Parse(*it.ProfileUUID)
			if err == nil {
				pid, err := s.repo.GetProfileIDByUUID(ctx, pUUID, accountID)
				if err == nil && pid != nil {
					profileID = pid
				}
			}
		}

		recName := it.RecipientName
		if recName == "" && it.CustomPrintData != nil {
			if n, ok := it.CustomPrintData["cardholder_name"].(string); ok {
				recName = n
			}
		}

		recTitle := it.RecipientTitle
		if recTitle == "" && it.CustomPrintData != nil {
			if t, ok := it.CustomPrintData["cardholder_title"].(string); ok {
				recTitle = t
			}
		}

		idempotencyKey := it.IdempotencyKey
		if idempotencyKey == "" {
			idempotencyKey = uuid.New().String()
		}

		pItems = append(pItems, processedItem{
			productID:      &prod.ID,
			variantID:      variantID,
			productName:    prod.Name,
			variantName:    variantName,
			sku:            sku,
			qty:            qty,
			unitPrice:      unitPrice,
			totalPrice:     itemTotal,
			customPrint:    it.CustomPrintData,
			recipientName:  recName,
			recipientMail:  it.RecipientEmail,
			recipientPhone: it.RecipientPhone,
			recipientTitle: recTitle,
			recipientDept:  it.RecipientDepartment,
			profileID:      profileID,
			idempotencyKey: idempotencyKey,
		})
	}

	taxTotal = subtotal * 0.20 // 20% KDV
	grandTotal := subtotal + taxTotal

	if input.OrderType == "" {
		if len(input.Items) > 1 {
			input.OrderType = "bulk"
		} else {
			input.OrderType = "standard"
		}
	}

	order, err := s.repo.Create(ctx, input, orderNumber, accountID, placedByUserID, subtotal, taxTotal, grandTotal, "TRY")
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	for _, pi := range pItems {
		_, _ = s.repo.CreateItem(
			ctx,
			order.ID,
			pi.productID,
			pi.variantID,
			pi.productName,
			pi.variantName,
			pi.sku,
			pi.qty,
			pi.unitPrice,
			pi.totalPrice,
			pi.customPrint,
			pi.recipientName,
			pi.recipientMail,
			pi.recipientPhone,
			pi.recipientTitle,
			pi.recipientDept,
			pi.profileID,
			nil, // card_id initially nil until provisioned
			"pending",
			"unprovisioned",
			"",
			pi.idempotencyKey,
		)
	}

	s.notifyOrderTransitions(ctx, order)

	return s.repo.GetByUUID(ctx, uuid.MustParse(order.UUID))
}

// ProvisionOrder provisions cards and TEC-77 routes for all unprovisioned items in an order
func (s *Service) ProvisionOrder(ctx context.Context, orderUUID uuid.UUID) (*model.OrderDetail, error) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	if isClosedOrder(ord) {
		return nil, ErrOrderClosed
	}

	var provisionedCount int
	var failedCount int

	for _, item := range ord.Items {
		if item.ProvisioningStatus == "provisioned" && item.CardUUID != nil {
			continue
		}

		itemUUID, err := uuid.Parse(item.UUID)
		if err != nil {
			continue
		}

		err = s.provisionSingleItem(ctx, ord, &item, itemUUID)
		if err != nil {
			failedCount++
			_, _ = s.repo.UpdateItemProvisioning(ctx, itemUUID, nil, "failed", err.Error())
		} else {
			provisionedCount++
		}
	}

	// Update order fulfillment status
	if provisionedCount > 0 {
		_, _ = s.repo.UpdateFulfillmentStatus(ctx, orderUUID, "in_production")
	}

	return s.repo.GetByUUID(ctx, orderUUID)
}

// ProvisionSingleItem provisions a single item or retries a failed provisioning
func (s *Service) ProvisionSingleItem(ctx context.Context, orderUUID, itemUUID uuid.UUID) (*model.OrderItemSummary, error) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	if isClosedOrder(ord) {
		return nil, ErrOrderClosed
	}

	item, err := s.repo.GetItemByUUID(ctx, itemUUID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}

	if item.ProvisioningStatus == "provisioned" && item.CardUUID != nil {
		return item, nil
	}

	err = s.provisionSingleItem(ctx, ord, item, itemUUID)
	if err != nil {
		_, _ = s.repo.UpdateItemProvisioning(ctx, itemUUID, nil, "failed", err.Error())
		return nil, err
	}

	return s.repo.GetItemByUUID(ctx, itemUUID)
}

func (s *Service) provisionSingleItem(ctx context.Context, ord *model.OrderDetail, item *model.OrderItemSummary, itemUUID uuid.UUID) error {
	smartCode, err := s.generateUniqueSmartCode()
	if err != nil {
		return fmt.Errorf("generate smart code: %w", err)
	}

	activationCode := s.generateActivationCode()
	cardUID := fmt.Sprintf("CC-2026-%s", strings.ToUpper(uuid.New().String()[:8]))

	cardName := item.RecipientName
	if cardName == "" {
		cardName = fmt.Sprintf("NFC Kart - %s", item.ProductName)
	}

	var profileID *int64
	if item.ProfileUUID != nil && *item.ProfileUUID != "" {
		pUUID, err := uuid.Parse(*item.ProfileUUID)
		if err == nil {
			pid, err := s.repo.GetProfileIDByUUID(ctx, pUUID, ord.AccountID)
			if err == nil && pid != nil {
				profileID = pid
			}
		}
	}

	material := "matte_black"
	colorHex := "#18181b"
	if item.VariantName != "" {
		material = strings.ToLower(strings.ReplaceAll(item.VariantName, " ", "_"))
	}

	cardID, _, _, err := s.repo.CreateProvisionedCardAndRoute(
		ctx,
		ord.ID,
		item.ID,
		ord.AccountID,
		ord.UserID,
		profileID,
		cardName,
		smartCode,
		activationCode,
		cardUID,
		material,
		colorHex,
	)
	if err != nil {
		return fmt.Errorf("create card & route: %w", err)
	}

	// Update item with card ID and success
	_, err = s.repo.UpdateItemProvisioning(ctx, itemUUID, &cardID, "provisioned", "")
	return err
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, input model.UpdateOrderStatusInput) (*model.OrderDetail, error) {
	if input.Status == "refunded" {
		if err := s.refundAtProvider(ctx, id); err != nil {
			return nil, err
		}
	}
	ord, err := s.repo.UpdateStatus(ctx, id, input.Status, actorUserID(ctx))
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	s.notifyOrderTransitions(ctx, ord)
	if isClosedOrder(ord) {
		if err := s.retireCardRoutes(ctx, ord.ID); err != nil {
			return nil, err
		}
	}

	if input.Status == "paid" {
		// Auto-provision cards if not already provisioned.
		go func(oUUID uuid.UUID) {
			_, _ = s.ProvisionOrder(context.Background(), oUUID)
		}(id)
	}

	return s.indexed(ord, nil)
}

// actorUserID is the signed-in user behind a status change, recorded in order_status_history;
// nil for system transitions.
func actorUserID(ctx context.Context) *int64 {
	if p, ok := authctx.PrincipalFrom(ctx); ok && p.UserInternal > 0 {
		id := p.UserInternal
		return &id
	}
	return nil
}

func (s *Service) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, input model.UpdateOrderPaymentStatusInput) (*model.OrderDetail, error) {
	if input.PaymentStatus == model.PaymentStatusRefunded {
		if err := s.refundAtProvider(ctx, id); err != nil {
			return nil, err
		}
	}
	ord, err := s.repo.UpdatePaymentStatus(ctx, id, input.PaymentStatus, input.PaymentProvider, input.PaymentReference, actorUserID(ctx))
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	s.notifyOrderTransitions(ctx, ord)
	if isClosedOrder(ord) {
		if err := s.retireCardRoutes(ctx, ord.ID); err != nil {
			return nil, err
		}
	}

	if input.PaymentStatus == "paid" {
		// When payment succeeds, auto-provision cards & routes in background
		go func(oUUID uuid.UUID) {
			_, _ = s.ProvisionOrder(context.Background(), oUUID)
		}(id)
	}

	return s.indexed(ord, nil)
}

func (s *Service) UpdateFulfillmentStatus(ctx context.Context, id uuid.UUID, input model.UpdateOrderFulfillmentStatusInput) (*model.OrderDetail, error) {
	ord, err := s.repo.UpdateFulfillmentStatus(ctx, id, input.FulfillmentStatus)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}

	s.notifyOrderTransitions(ctx, ord)
	return ord, nil
}

func (s *Service) BulkUpdateItems(ctx context.Context, orderUUID uuid.UUID, input model.BulkUpdateOrderItemsInput) (*model.OrderDetail, error) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}

	for _, itemUUIDStr := range input.ItemUUIDs {
		iUUID, err := uuid.Parse(itemUUIDStr)
		if err != nil {
			continue
		}
		if input.FulfillmentStatus != nil && *input.FulfillmentStatus != "" {
			_, _ = s.repo.UpdateItemFulfillment(ctx, iUUID, *input.FulfillmentStatus)
		}
	}

	return s.repo.GetByUUID(ctx, orderUUID)
}

func (s *Service) UpdateShipping(ctx context.Context, id uuid.UUID, input model.UpdateOrderShippingInput) (*model.OrderDetail, error) {
	ord, err := s.repo.UpdateShipping(ctx, id, input.ShippingCarrier, input.TrackingNumber, input.TrackingURL, input.MarkAsShipped, actorUserID(ctx))
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}

	s.notifyOrderTransitions(ctx, ord)

	return ord, nil
}

func (s *Service) UpdateNotes(ctx context.Context, id uuid.UUID, input model.UpdateOrderNotesInput) (*model.OrderDetail, error) {
	return s.repo.UpdateNotes(ctx, id, input.AdminNote, input.CustomerNote)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if s.search != nil {
		s.search.EnqueueDelete(ctx, "orders", id.String())
	}
	return nil
}

// Asset Requests
func (s *Service) CreateAssetRequest(ctx context.Context, orderUUID uuid.UUID, input model.CreateAssetRequestInput, requestedBy *int64) (*model.AssetRequestSummary, error) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}

	tokenBytes := make([]byte, 24)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	req, err := s.repo.CreateAssetRequest(ctx, ord.ID, input.RequestType, input.Title, input.Description, token, requestedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset request: %w", err)
	}

	// Update order status to design_pending
	changed, err := s.repo.UpdateStatus(ctx, orderUUID, "design_pending", actorUserID(ctx))
	if err != nil {
		return nil, err
	}
	s.notifyOrderTransitions(ctx, changed)

	// Notify customer
	uploadURL := fmt.Sprintf("/orders/%s/upload?token=%s", ord.OrderNumber, token)
	s.sendOrderNotification(ctx, ord.UserID, "Görsel / Logo Talebi", fmt.Sprintf("%s numaralı kart siparişiniz için '%s' talep edildi. Lütfen yükleme bağlantısını ziyaret edin.", ord.OrderNumber, input.Title), map[string]any{
		"order_number": ord.OrderNumber,
		"action_url":   uploadURL,
		"token":        token,
	})

	return req, nil
}

func (s *Service) GetAssetByToken(ctx context.Context, token string) (*model.AssetRequestSummary, error) {
	req, _, _, err := s.repo.GetAssetRequestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrInvalidToken
	}
	return req, nil
}

func (s *Service) UploadAsset(ctx context.Context, token string, file io.Reader, filename, contentType string, size int64) (*model.AssetRequestSummary, error) {
	req, orderUserID, orderNumber, err := s.repo.GetAssetRequestByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrInvalidToken
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = exts[0]
		} else {
			ext = ".png"
		}
	}
	ext = strings.TrimPrefix(ext, ".")

	key := storage.OrderAssetObjectKey(orderNumber, req.UUID, ext)

	var fileURL string
	if s.storage != nil {
		err = s.storage.Upload(ctx, storage.File{
			Body:        file,
			Size:        size,
			ContentType: contentType,
			Filename:    filename,
		}, key)
		if err != nil {
			return nil, fmt.Errorf("failed to upload asset: %w", err)
		}
		fileURL = s.storage.PublicURL(key)
	} else {
		fileURL = fmt.Sprintf("/api/v1/storage/stream/%s", key)
	}

	reqUUID := uuid.MustParse(req.UUID)
	updatedReq, err := s.repo.UpdateAssetRequestStatus(ctx, reqUUID, "uploaded", fileURL, "")
	if err != nil {
		return nil, err
	}

	// Notify customer upload confirmed
	s.sendOrderNotification(ctx, orderUserID, "Dosyanız Alındı", fmt.Sprintf("%s siparişi için dosyanız başarıyla iletildi ve incelemeye alındı.", orderNumber), map[string]any{
		"order_number": orderNumber,
	})

	return updatedReq, nil
}

func (s *Service) ReviewAsset(ctx context.Context, orderUUID, assetUUID uuid.UUID, input model.ReviewAssetInput) (*model.AssetRequestSummary, error) {
	ord, err := s.repo.GetByUUID(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}

	status := input.Status
	if status != "approved" && status != "rejected" {
		return nil, errors.New("status must be approved or rejected")
	}

	updatedReq, err := s.repo.UpdateAssetRequestStatus(ctx, assetUUID, status, "", input.RejectionReason)
	if err != nil {
		return nil, err
	}

	switch status {
	case "approved":
		changed, err := s.repo.UpdateStatus(ctx, orderUUID, "design_approved", actorUserID(ctx))
		if err != nil {
			return nil, err
		}
		s.notifyOrderTransitions(ctx, changed)
	case "rejected":
		s.sendOrderNotification(ctx, ord.UserID, "Görsel Revizyon Talebi", fmt.Sprintf("%s siparişiniz için yüklenen dosya uygun bulunmadı. Gerekçe: %s", ord.OrderNumber, input.RejectionReason), map[string]any{
			"order_number":     ord.OrderNumber,
			"rejection_reason": input.RejectionReason,
		})
	}

	return updatedReq, nil
}

func (s *Service) generateUniqueSmartCode() (string, error) {
	const charset = "abcdefghjkmnpqrstuvwxyz23456789"
	code := "cc-"
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		code += string(charset[n.Int64()])
	}
	return code, nil
}

func (s *Service) generateActivationCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := "ACT-"
	for i := 0; i < 6; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return fmt.Sprintf("ACT-%s", strings.ToUpper(uuid.New().String()[:6]))
		}
		code += string(charset[n.Int64()])
	}
	return code
}

func (s *Service) sendOrderNotification(ctx context.Context, userID int64, title, body string, payload map[string]any) {
	if s.notifSvc == nil {
		return
	}
	var actionURL *string
	if u, ok := payload["action_url"].(string); ok && u != "" {
		actionURL = &u
	}
	_, _ = s.notifSvc.Enqueue(ctx, notifModel.EnqueueInput{
		UserID:    &userID,
		Channels:  []string{notifModel.ChannelInapp, notifModel.ChannelRealtime, notifModel.ChannelEmail},
		Title:     title,
		Body:      body,
		ActionURL: actionURL,
		Payload:   payload,
	})
}

// PublishCreatedForAccount dispatches committed creation events and indexes an order created
// by a transaction-bound caller. It never publishes a partial or another account's order.
func (s *Service) PublishCreatedForAccount(ctx context.Context, account authctx.Account, id uuid.UUID) {
	ord, err := s.GetForAccount(ctx, id, account)
	if err != nil {
		return
	}
	s.notifyOrderTransitions(ctx, ord)
	_, _ = s.indexed(ord, nil)
}
