package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPostgres(pool *pgxpool.Pool, q *db.Queries) *PostgresRepository {
	return &PostgresRepository{pool: pool, q: q}
}

func (r *PostgresRepository) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	if r.pool == nil {
		return fn(r.q)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) List(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string, limit, offset int32) ([]model.OrderSummary, error) {
	rows, err := r.q.ListOrders(ctx, db.ListOrdersParams{
		Q:                 textToPgtype(q),
		AccountID:         int64ToPgtype(accountID),
		Status:            textToPgtype(status),
		PaymentStatus:     textToPgtype(paymentStatus),
		FulfillmentStatus: textToPgtype(fulfillmentStatus),
		OrderType:         textToPgtype(orderType),
		LimitCount:        limit,
		OffsetCount:       offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]model.OrderSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.OrderSummary{
			UUID:              row.Uuid.String(),
			OrderNumber:       row.OrderNumber,
			OrderType:         row.OrderType,
			CustomerEmail:     row.CustomerEmail,
			CustomerName:      row.CustomerName,
			CustomerSurname:   row.CustomerSurname,
			CompanyName:       pgtypeToString(row.CompanyName),
			AccountUUID:       row.AccountUuid.String(),
			AccountName:       row.AccountName,
			AccountType:       string(row.AccountType),
			Status:            row.Status,
			FulfillmentStatus: row.FulfillmentStatus,
			GrandTotal:        numericToFloat(row.GrandTotal),
			Currency:          row.Currency,
			PaymentStatus:     row.PaymentStatus,
			TrackingNumber:    row.TrackingNumber,
			ItemsCount:        int(row.ItemsCount),
			CreatedAt:         row.CreatedAt.Time,
		})
	}
	return result, nil
}

func (r *PostgresRepository) Count(ctx context.Context, q *string, accountID *int64, status, paymentStatus, fulfillmentStatus, orderType *string) (int64, error) {
	return r.q.CountOrders(ctx, db.CountOrdersParams{
		Q:                 textToPgtype(q),
		AccountID:         int64ToPgtype(accountID),
		Status:            textToPgtype(status),
		PaymentStatus:     textToPgtype(paymentStatus),
		FulfillmentStatus: textToPgtype(fulfillmentStatus),
		OrderType:         textToPgtype(orderType),
	})
}

func (r *PostgresRepository) GetByUUID(ctx context.Context, id uuid.UUID) (*model.OrderDetail, error) {
	row, err := r.q.GetOrderByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	detail := toOrderDetailFromRow(row)

	items, err := r.ListItems(ctx, row.ID)
	if err == nil {
		detail.Items = items
	}

	assetRequests, err := r.ListAssetRequests(ctx, row.ID)
	if err == nil {
		detail.AssetRequests = assetRequests
	}

	return detail, nil
}

func (r *PostgresRepository) GetByOrderNumber(ctx context.Context, orderNumber string) (*model.OrderDetail, error) {
	row, err := r.q.GetOrderByOrderNumber(ctx, orderNumber)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	detail := toOrderDetailFromOrderNumberRow(row)

	items, err := r.ListItems(ctx, row.ID)
	if err == nil {
		detail.Items = items
	}

	assetRequests, err := r.ListAssetRequests(ctx, row.ID)
	if err == nil {
		detail.AssetRequests = assetRequests
	}

	return detail, nil
}

func (r *PostgresRepository) Create(ctx context.Context, order model.CreateOrderInput, orderNumber string, accountID, placedByUserID int64, subtotal, taxTotal, grandTotal float64, currency string) (*model.OrderDetail, error) {
	shippingJSON, _ := json.Marshal(order.ShippingAddress)
	billingJSON, _ := json.Marshal(order.BillingAddress)
	metadataJSON, _ := json.Marshal(order.Metadata)
	if len(order.Metadata) == 0 {
		metadataJSON = []byte("{}")
	}

	if currency == "" {
		currency = "TRY"
	}

	orderType := order.OrderType
	if orderType == "" {
		orderType = "standard"
	}

	var row db.Order
	err := r.withTx(ctx, func(q *db.Queries) error {
		var err error
		row, err = q.CreateOrder(ctx, db.CreateOrderParams{
			AccountID:         accountID,
			OrderNumber:       orderNumber,
			UserID:            placedByUserID,
			Status:            "pending",
			Subtotal:          floatToNumeric(subtotal),
			TaxTotal:          floatToNumeric(taxTotal),
			ShippingTotal:     floatToNumeric(0),
			DiscountTotal:     floatToNumeric(0),
			GrandTotal:        floatToNumeric(grandTotal),
			Currency:          currency,
			PaymentStatus:     "unpaid",
			PaymentProvider:   order.PaymentProvider,
			PaymentReference:  "",
			ShippingAddress:   shippingJSON,
			BillingAddress:    billingJSON,
			CustomerNote:      order.CustomerNote,
			AdminNote:         order.AdminNote,
			OrderType:         orderType,
			FulfillmentStatus: "unfulfilled",
			Metadata:          metadataJSON,
		})
		if err != nil {
			return err
		}
		return recordStatusChange(ctx, q, row.AccountID, row.ID, "", row.Status, &placedByUserID)
	})
	if err != nil {
		return nil, err
	}

	return &model.OrderDetail{
		ID:                row.ID,
		UUID:              row.Uuid.String(),
		OrderNumber:       row.OrderNumber,
		OrderType:         row.OrderType,
		AccountID:         row.AccountID,
		UserID:            row.UserID,
		Status:            row.Status,
		FulfillmentStatus: row.FulfillmentStatus,
		Subtotal:          numericToFloat(row.Subtotal),
		TaxTotal:          numericToFloat(row.TaxTotal),
		ShippingTotal:     numericToFloat(row.ShippingTotal),
		DiscountTotal:     numericToFloat(row.DiscountTotal),
		GrandTotal:        numericToFloat(row.GrandTotal),
		Currency:          row.Currency,
		PaymentStatus:     row.PaymentStatus,
		PaymentProvider:   row.PaymentProvider,
		PaymentReference:  row.PaymentReference,
		ShippingAddress:   order.ShippingAddress,
		BillingAddress:    order.BillingAddress,
		ShippingCarrier:   row.ShippingCarrier,
		TrackingNumber:    row.TrackingNumber,
		TrackingURL:       row.TrackingUrl,
		CustomerNote:      row.CustomerNote,
		AdminNote:         row.AdminNote,
		Metadata:          order.Metadata,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, actorUserID *int64) (*model.OrderDetail, error) {
	found, err := r.changeStatus(ctx, id, actorUserID, func(q *db.Queries) (string, error) {
		row, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{Uuid: id, Status: status})
		return row.Status, err
	})
	if err != nil || !found {
		return nil, err
	}
	return r.GetByUUID(ctx, id)
}

// changeStatus locks the order, runs update and records the transition in
// order_status_history in the same transaction (TEC-944). found is false when the order does not
// exist or is deleted.
func (r *PostgresRepository) changeStatus(ctx context.Context, id uuid.UUID, actorUserID *int64, update func(q *db.Queries) (string, error)) (bool, error) {
	found := false
	err := r.withTx(ctx, func(q *db.Queries) error {
		prev, err := q.LockOrderForStatusChange(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		to, err := update(q)
		if err != nil {
			return err
		}
		found = true
		return recordStatusChange(ctx, q, prev.AccountID, prev.ID, prev.Status, to, actorUserID)
	})
	return found, err
}

// recordStatusChange writes one history row when the status actually changed; from is empty for
// the creation row.
func recordStatusChange(ctx context.Context, q *db.Queries, accountID, orderID int64, from, to string, actorUserID *int64) error {
	if from == to {
		return nil
	}
	params := db.InsertOrderStatusHistoryParams{AccountID: accountID, OrderID: orderID, ToStatus: to}
	if from != "" {
		params.FromStatus = pgtype.Text{String: from, Valid: true}
	}
	if actorUserID != nil {
		params.ChangedByUserID = pgtype.Int8{Int64: *actorUserID, Valid: true}
	}
	if err := q.InsertOrderStatusHistory(ctx, params); err != nil {
		return fmt.Errorf("record order status change: %w", err)
	}
	return nil
}

func (r *PostgresRepository) UpdatePaymentStatus(ctx context.Context, id uuid.UUID, paymentStatus string, provider, reference *string, actorUserID *int64) (*model.OrderDetail, error) {
	found, err := r.changeStatus(ctx, id, actorUserID, func(q *db.Queries) (string, error) {
		row, err := q.UpdateOrderPaymentStatus(ctx, db.UpdateOrderPaymentStatusParams{
			Uuid:             id,
			PaymentStatus:    paymentStatus,
			PaymentProvider:  textToPgtype(provider),
			PaymentReference: textToPgtype(reference),
		})
		if err != nil || paymentStatus != "paid" || row.Status != "pending" {
			return row.Status, err
		}
		// A paid order no longer waits for payment (TEC-943).
		paid, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{Uuid: id, Status: "paid"})
		return paid.Status, err
	})
	if err != nil || !found {
		return nil, err
	}
	return r.GetByUUID(ctx, id)
}

func (r *PostgresRepository) PaymentTarget(ctx context.Context, id uuid.UUID) (*model.PaymentTarget, error) {
	row, err := r.q.GetOrderPaymentAmount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model.PaymentTarget{
		ID: row.ID, UUID: row.Uuid, AccountID: row.AccountID, OrderNumber: row.OrderNumber,
		Status: row.Status, PaymentStatus: row.PaymentStatus, AmountMinor: row.AmountMinor, Currency: row.Currency,
	}, nil
}

// ApplyProviderPayment marks the order paid after a verified provider callback. It locks the
// order, so duplicate callbacks and a concurrent manual change serialize; a pending order also
// moves to paid with a system order_status_history row. Run it on a transaction-bound repository
// to commit it together with the payment session.
func (r *PostgresRepository) ApplyProviderPayment(ctx context.Context, orderID int64, provider, reference string) (model.ProviderPaymentOutcome, error) {
	var out model.ProviderPaymentOutcome
	err := r.withTx(ctx, func(q *db.Queries) error {
		prev, err := q.LockOrderForPayment(ctx, orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			out.Result = model.ProviderPaymentOrderClosed
			return nil
		}
		if err != nil {
			return err
		}
		out.OrderUUID = prev.Uuid
		switch {
		case prev.Status == "cancelled" || prev.Status == "refunded" || prev.PaymentStatus == "refunded":
			out.Result = model.ProviderPaymentOrderClosed
			return nil
		case prev.PaymentStatus == "paid" || prev.PaymentStatus == model.PaymentStatusPartiallyRefunded:
			out.Result = model.ProviderPaymentAlreadyPaid
			return nil
		}
		ref := reference
		if _, err := q.UpdateOrderPaymentStatus(ctx, db.UpdateOrderPaymentStatusParams{
			Uuid: prev.Uuid, PaymentStatus: "paid", PaymentProvider: textToPgtype(&provider), PaymentReference: textToPgtype(&ref),
		}); err != nil {
			return err
		}
		out.Result = model.ProviderPaymentApplied
		if prev.Status != "pending" {
			return nil
		}
		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{Uuid: prev.Uuid, Status: "paid"}); err != nil {
			return err
		}
		return recordStatusChange(ctx, q, prev.AccountID, prev.ID, prev.Status, "paid", nil)
	})
	return out, err
}

// ApplyProviderRefund records a succeeded provider refund of the payment that paid the order
// (TEC-935): payment_status becomes partially_refunded, or refunded once everything was returned,
// and a fully refunded order that was not cancelled moves to refunded with an
// order_status_history row. Run it on a transaction-bound repository to commit it together with
// the refund. A deleted order is left alone and returns uuid.Nil.
func (r *PostgresRepository) ApplyProviderRefund(ctx context.Context, orderID int64, full bool, actorUserID *int64) (uuid.UUID, error) {
	var out uuid.UUID
	err := r.withTx(ctx, func(q *db.Queries) error {
		prev, err := q.LockOrderForPayment(ctx, orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = prev.Uuid
		paymentStatus := model.PaymentStatusPartiallyRefunded
		if full {
			paymentStatus = model.PaymentStatusRefunded
		}
		if _, err := q.UpdateOrderPaymentStatus(ctx, db.UpdateOrderPaymentStatusParams{Uuid: prev.Uuid, PaymentStatus: paymentStatus}); err != nil {
			return err
		}
		if !full || prev.Status == "refunded" || prev.Status == "cancelled" {
			return nil
		}
		if _, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{Uuid: prev.Uuid, Status: "refunded"}); err != nil {
			return err
		}
		return recordStatusChange(ctx, q, prev.AccountID, prev.ID, prev.Status, "refunded", actorUserID)
	})
	return out, err
}

func (r *PostgresRepository) UpdateFulfillmentStatus(ctx context.Context, id uuid.UUID, fulfillmentStatus string) (*model.OrderDetail, error) {
	row, err := r.q.UpdateOrderFulfillmentStatus(ctx, db.UpdateOrderFulfillmentStatusParams{
		Uuid:              id,
		FulfillmentStatus: fulfillmentStatus,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.GetByUUID(ctx, row.Uuid)
}

func (r *PostgresRepository) UpdateShipping(ctx context.Context, id uuid.UUID, carrier, trackingNumber, trackingURL string, markShipped bool, actorUserID *int64) (*model.OrderDetail, error) {
	status := ""
	if markShipped {
		status = "shipped"
	}
	found, err := r.changeStatus(ctx, id, actorUserID, func(q *db.Queries) (string, error) {
		row, err := q.UpdateOrderShipping(ctx, db.UpdateOrderShippingParams{
			Uuid:            id,
			ShippingCarrier: carrier,
			TrackingNumber:  trackingNumber,
			TrackingUrl:     trackingURL,
			NewStatus:       status,
		})
		return row.Status, err
	})
	if err != nil || !found {
		return nil, err
	}
	return r.GetByUUID(ctx, id)
}

func (r *PostgresRepository) UpdateNotes(ctx context.Context, id uuid.UUID, adminNote, customerNote *string) (*model.OrderDetail, error) {
	row, err := r.q.UpdateOrderNotes(ctx, db.UpdateOrderNotesParams{
		Uuid:         id,
		AdminNote:    textToPgtype(adminNote),
		CustomerNote: textToPgtype(customerNote),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return r.GetByUUID(ctx, row.Uuid)
}

func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.q.DeleteOrder(ctx, id)
}

func (r *PostgresRepository) GetOverviewKPIs(ctx context.Context) (*model.OrderOverviewKPIs, error) {
	row, err := r.q.GetOrderStats(ctx)
	if err != nil {
		return nil, err
	}

	rev, _ := row.TotalRevenue.Float64Value()
	revenue := 0.0
	if rev.Valid {
		revenue = rev.Float64
	}

	return &model.OrderOverviewKPIs{
		TotalOrders:         row.TotalOrders,
		TodayOrders:         row.TodayOrders,
		PendingOrders:       row.PendingOrders,
		DesignPendingOrders: row.DesignPendingOrders,
		InProductionOrders:  row.InProductionOrders,
		ShippedOrders:       row.ShippedOrders,
		TotalRevenue:        revenue,
	}, nil
}

// Items
func (r *PostgresRepository) ListItems(ctx context.Context, orderID int64) ([]model.OrderItemSummary, error) {
	rows, err := r.q.ListOrderItemsByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	result := make([]model.OrderItemSummary, 0, len(rows))
	for _, row := range rows {
		var customPrint map[string]any
		if len(row.CustomPrintData) > 0 {
			_ = json.Unmarshal(row.CustomPrintData, &customPrint)
		}
		var images []string
		if len(row.ProductImages) > 0 {
			_ = json.Unmarshal(row.ProductImages, &images)
		}

		var prodID *int64
		if row.ProductID.Valid {
			prodID = &row.ProductID.Int64
		}

		profUUID := pgUUIDToString(row.ProfileUuid)
		cardUUID := pgUUIDToString(row.CardUuid)

		result = append(result, model.OrderItemSummary{
			ID:                  row.ID,
			UUID:                row.Uuid.String(),
			ProductID:           prodID,
			ProductSlug:         pgtypeToString(row.ProductSlug),
			ProductName:         row.ProductName,
			VariantName:         row.VariantName,
			SKU:                 row.Sku,
			Quantity:            int(row.Quantity),
			UnitPrice:           numericToFloat(row.UnitPrice),
			TotalPrice:          numericToFloat(row.TotalPrice),
			CustomPrintData:     customPrint,
			ProductImages:       images,
			RecipientName:       row.RecipientName,
			RecipientEmail:      row.RecipientEmail,
			RecipientPhone:      row.RecipientPhone,
			RecipientTitle:      row.RecipientTitle,
			RecipientDepartment: row.RecipientDepartment,
			ProfileUUID:         profUUID,
			ProfileAvatarURL:    pgtypeToString(row.ProfileAvatarUrl),
			CardUUID:            cardUUID,
			CardSmartCode:       pgtypeToString(row.CardSmartCode),
			CardUID:             pgtypeToString(row.CardUid),
			CardStatus:          pgtypeToString(row.CardStatus),
			CardIsActive:        row.CardIsActive.Bool,
			CardIsLocked:        row.CardIsLocked.Bool,
			RoutePath:           pgtypeToString(row.RoutePath),
			RouteStatus:         pgtypeToString(row.RouteStatus),
			FulfillmentStatus:   row.FulfillmentStatus,
			ProvisioningStatus:  row.ProvisioningStatus,
			ProvisioningError:   row.ProvisioningError,
		})
	}
	return result, nil
}

func (r *PostgresRepository) GetItemByUUID(ctx context.Context, itemUUID uuid.UUID) (*model.OrderItemSummary, error) {
	row, err := r.q.GetOrderItemByUUID(ctx, itemUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var customPrint map[string]any
	if len(row.CustomPrintData) > 0 {
		_ = json.Unmarshal(row.CustomPrintData, &customPrint)
	}
	var images []string
	if len(row.ProductImages) > 0 {
		_ = json.Unmarshal(row.ProductImages, &images)
	}

	var prodID *int64
	if row.ProductID.Valid {
		prodID = &row.ProductID.Int64
	}

	profUUID := pgUUIDToString(row.ProfileUuid)
	cardUUID := pgUUIDToString(row.CardUuid)

	return &model.OrderItemSummary{
		ID:                  row.ID,
		UUID:                row.Uuid.String(),
		ProductID:           prodID,
		ProductSlug:         pgtypeToString(row.ProductSlug),
		ProductName:         row.ProductName,
		VariantName:         row.VariantName,
		SKU:                 row.Sku,
		Quantity:            int(row.Quantity),
		UnitPrice:           numericToFloat(row.UnitPrice),
		TotalPrice:          numericToFloat(row.TotalPrice),
		CustomPrintData:     customPrint,
		ProductImages:       images,
		RecipientName:       row.RecipientName,
		RecipientEmail:      row.RecipientEmail,
		RecipientPhone:      row.RecipientPhone,
		RecipientTitle:      row.RecipientTitle,
		RecipientDepartment: row.RecipientDepartment,
		ProfileUUID:         profUUID,
		ProfileAvatarURL:    pgtypeToString(row.ProfileAvatarUrl),
		CardUUID:            cardUUID,
		CardSmartCode:       pgtypeToString(row.CardSmartCode),
		CardUID:             pgtypeToString(row.CardUid),
		CardStatus:          pgtypeToString(row.CardStatus),
		CardIsActive:        row.CardIsActive.Bool,
		CardIsLocked:        row.CardIsLocked.Bool,
		RoutePath:           pgtypeToString(row.RoutePath),
		RouteStatus:         pgtypeToString(row.RouteStatus),
		FulfillmentStatus:   row.FulfillmentStatus,
		ProvisioningStatus:  row.ProvisioningStatus,
		ProvisioningError:   row.ProvisioningError,
	}, nil
}

func (r *PostgresRepository) CreateItem(ctx context.Context, orderID int64, productID, variantID *int64, productName, variantName, sku string, qty int, unitPrice, totalPrice float64, customPrint map[string]any, recipientName, recipientEmail, recipientPhone, recipientTitle, recipientDept string, profileID, cardID *int64, fulfillmentStatus, provisioningStatus, provisioningError, idempotencyKey string) (*model.OrderItemSummary, error) {
	cpJSON, _ := json.Marshal(customPrint)
	if len(customPrint) == 0 {
		cpJSON = []byte("{}")
	}

	if fulfillmentStatus == "" {
		fulfillmentStatus = "pending"
	}
	if provisioningStatus == "" {
		provisioningStatus = "unprovisioned"
	}

	row, err := r.q.CreateOrderItem(ctx, db.CreateOrderItemParams{
		OrderID:             orderID,
		ProductID:           int64ToPgtype(productID),
		VariantID:           int64ToPgtype(variantID),
		ProductName:         productName,
		VariantName:         variantName,
		Sku:                 sku,
		Quantity:            int32(qty),
		UnitPrice:           floatToNumeric(unitPrice),
		TotalPrice:          floatToNumeric(totalPrice),
		CustomPrintData:     cpJSON,
		RecipientName:       recipientName,
		RecipientEmail:      recipientEmail,
		RecipientPhone:      recipientPhone,
		RecipientTitle:      recipientTitle,
		RecipientDepartment: recipientDept,
		ProfileID:           int64ToPgtype(profileID),
		CardID:              int64ToPgtype(cardID),
		FulfillmentStatus:   fulfillmentStatus,
		ProvisioningStatus:  provisioningStatus,
		ProvisioningError:   provisioningError,
		IdempotencyKey:      idempotencyKey,
	})
	if err != nil {
		return nil, err
	}

	return &model.OrderItemSummary{
		ID:                  row.ID,
		UUID:                row.Uuid.String(),
		ProductName:         row.ProductName,
		VariantName:         row.VariantName,
		SKU:                 row.Sku,
		Quantity:            int(row.Quantity),
		UnitPrice:           numericToFloat(row.UnitPrice),
		TotalPrice:          numericToFloat(row.TotalPrice),
		CustomPrintData:     customPrint,
		RecipientName:       row.RecipientName,
		RecipientEmail:      row.RecipientEmail,
		RecipientPhone:      row.RecipientPhone,
		RecipientTitle:      row.RecipientTitle,
		RecipientDepartment: row.RecipientDepartment,
		FulfillmentStatus:   row.FulfillmentStatus,
		ProvisioningStatus:  row.ProvisioningStatus,
		ProvisioningError:   row.ProvisioningError,
	}, nil
}

func (r *PostgresRepository) UpdateItemProvisioning(ctx context.Context, itemUUID uuid.UUID, cardID *int64, provisioningStatus, provisioningError string) (*model.OrderItemSummary, error) {
	_, err := r.q.UpdateOrderItemProvisioning(ctx, db.UpdateOrderItemProvisioningParams{
		Uuid:               itemUUID,
		CardID:             int64ToPgtype(cardID),
		ProvisioningStatus: provisioningStatus,
		ProvisioningError:  provisioningError,
	})
	if err != nil {
		return nil, err
	}
	return r.GetItemByUUID(ctx, itemUUID)
}

func (r *PostgresRepository) UpdateItemFulfillment(ctx context.Context, itemUUID uuid.UUID, fulfillmentStatus string) (*model.OrderItemSummary, error) {
	_, err := r.q.UpdateOrderItemFulfillment(ctx, db.UpdateOrderItemFulfillmentParams{
		Uuid:              itemUUID,
		FulfillmentStatus: fulfillmentStatus,
	})
	if err != nil {
		return nil, err
	}
	return r.GetItemByUUID(ctx, itemUUID)
}

// Card & Route Provisioning (TEC-77 Integration)
func (r *PostgresRepository) CreateProvisionedCardAndRoute(ctx context.Context, orderID, orderItemID, accountID, userID int64, profileID *int64, cardName, smartCode, activationCode, cardUID, material, colorHex string) (int64, uuid.UUID, uuid.UUID, error) {
	var createdCardID int64
	var createdCardUUID uuid.UUID
	var createdRouteUUID uuid.UUID

	err := r.withTx(ctx, func(q *db.Queries) error {
		// 1. Create NFC Card
		cardRow, err := q.CreateProvisionedCard(ctx, db.CreateProvisionedCardParams{
			AccountID:          accountID,
			UserID:             userID,
			Name:               cardName,
			CardUid:            textToPgtype(&cardUID),
			ActivationCode:     textToPgtype(&activationCode),
			SmartCode:          smartCode,
			TargetType:         "profile",
			ProfileID:          int64ToPgtype(profileID),
			DirectUrl:          "",
			CustomData:         []byte("{}"),
			CardType:           "physical_nfc",
			Material:           material,
			ColorHex:           colorHex,
			IsActive:           false, // Unactivated until scanned/entered
			IsLocked:           false,
			LeadCaptureEnabled: false,
			OrderID:            int64ToPgtype(&orderID),
			OrderItemID:        int64ToPgtype(&orderItemID),
			Status:             "unactivated",
		})
		if err != nil {
			return fmt.Errorf("create provisioned card: %w", err)
		}

		createdCardID = cardRow.ID
		createdCardUUID = cardRow.Uuid

		// 2. Create Reserved Card Route (TEC-77)
		routePath := "c/" + smartCode
		routeRow, err := q.CreateRoute(ctx, db.CreateRouteParams{
			Path:               routePath,
			RouteType:          "card_route",
			Status:             "reserved",
			IsImmutable:        true,
			TargetType:         "profile",
			TargetID:           pgtype.UUID{Valid: false},
			ProfileID:          int64ToPgtype(profileID),
			CardID:             int64ToPgtype(&createdCardID),
			DirectUrl:          "",
			AccountID:          pgtype.Int8{Int64: accountID, Valid: true},
			OwnerUserID:        int64ToPgtype(&userID),
			PresentationConfig: []byte("{}"),
			LegacyMetadata:     []byte("{}"),
		})
		if err != nil {
			return fmt.Errorf("create reserved route for card: %w", err)
		}

		createdRouteUUID = routeRow.Uuid

		return nil
	})

	if err != nil {
		return 0, uuid.Nil, uuid.Nil, err
	}

	return createdCardID, createdCardUUID, createdRouteUUID, nil
}

// GetProfileIDByUUID resolves a profile of the given account; profiles of other accounts are not found.
func (r *PostgresRepository) GetProfileIDByUUID(ctx context.Context, profileUUID uuid.UUID, accountID int64) (*int64, error) {
	p, err := r.q.GetProfileByUUIDForAccount(ctx, db.GetProfileByUUIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p.ID, nil
}

// GetAccountIDByUUID resolves an account (personal or company) by its uuid.
func (r *PostgresRepository) GetAccountIDByUUID(ctx context.Context, accountUUID uuid.UUID) (*int64, error) {
	id, err := r.q.GetAccountIDByUUID(ctx, accountUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// GetPersonalAccountIDByUserUUID resolves a user's personal account.
func (r *PostgresRepository) GetPersonalAccountIDByUserUUID(ctx context.Context, userUUID uuid.UUID) (*int64, error) {
	id, err := r.q.GetPersonalAccountIDByUserUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

func (r *PostgresRepository) GetUserIDByUUID(ctx context.Context, userUUID uuid.UUID) (*int64, error) {
	u, err := r.q.GetUserByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u.ID, nil
}

// Asset Requests
func (r *PostgresRepository) ListAssetRequests(ctx context.Context, orderID int64) ([]model.AssetRequestSummary, error) {
	rows, err := r.q.ListOrderAssetRequests(ctx, orderID)
	if err != nil {
		return nil, err
	}

	result := make([]model.AssetRequestSummary, 0, len(rows))
	for _, row := range rows {
		reqName := ""
		if row.RequestedByName.Valid {
			reqName = row.RequestedByName.String
			if row.RequestedBySurname.Valid {
				reqName += " " + row.RequestedBySurname.String
			}
		}

		result = append(result, model.AssetRequestSummary{
			UUID:            row.Uuid.String(),
			RequestType:     row.RequestType,
			Title:           row.Title,
			Description:     row.Description,
			Status:          row.Status,
			Token:           row.Token,
			UploadedFileURL: row.UploadedFileUrl,
			RejectionReason: row.RejectionReason,
			RequestedByName: reqName,
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		})
	}
	return result, nil
}

func (r *PostgresRepository) GetAssetRequestByUUID(ctx context.Context, id uuid.UUID) (*model.AssetRequestSummary, error) {
	row, err := r.q.GetAssetRequestByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &model.AssetRequestSummary{
		UUID:            row.Uuid.String(),
		RequestType:     row.RequestType,
		Title:           row.Title,
		Description:     row.Description,
		Status:          row.Status,
		Token:           row.Token,
		UploadedFileURL: row.UploadedFileUrl,
		RejectionReason: row.RejectionReason,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) GetAssetRequestByToken(ctx context.Context, token string) (*model.AssetRequestSummary, int64, string, error) {
	row, err := r.q.GetAssetRequestByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, "", nil
		}
		return nil, 0, "", err
	}
	return &model.AssetRequestSummary{
		UUID:            row.Uuid.String(),
		RequestType:     row.RequestType,
		Title:           row.Title,
		Description:     row.Description,
		Status:          row.Status,
		Token:           row.Token,
		UploadedFileURL: row.UploadedFileUrl,
		RejectionReason: row.RejectionReason,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, row.OrderUserID, row.OrderNumber, nil
}

func (r *PostgresRepository) CreateAssetRequest(ctx context.Context, orderID int64, reqType, title, desc, token string, requestedBy *int64) (*model.AssetRequestSummary, error) {
	row, err := r.q.CreateAssetRequest(ctx, db.CreateAssetRequestParams{
		OrderID:     orderID,
		RequestType: reqType,
		Title:       title,
		Description: desc,
		Status:      "requested",
		Token:       token,
		RequestedBy: int64ToPgtype(requestedBy),
	})
	if err != nil {
		return nil, err
	}

	return &model.AssetRequestSummary{
		UUID:            row.Uuid.String(),
		RequestType:     row.RequestType,
		Title:           row.Title,
		Description:     row.Description,
		Status:          row.Status,
		Token:           row.Token,
		UploadedFileURL: row.UploadedFileUrl,
		RejectionReason: row.RejectionReason,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) UpdateAssetRequestStatus(ctx context.Context, id uuid.UUID, status, uploadedFileURL, rejectionReason string) (*model.AssetRequestSummary, error) {
	row, err := r.q.UpdateAssetRequestStatus(ctx, db.UpdateAssetRequestStatusParams{
		Uuid:            id,
		Status:          status,
		UploadedFileUrl: uploadedFileURL,
		RejectionReason: rejectionReason,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &model.AssetRequestSummary{
		UUID:            row.Uuid.String(),
		RequestType:     row.RequestType,
		Title:           row.Title,
		Description:     row.Description,
		Status:          row.Status,
		Token:           row.Token,
		UploadedFileURL: row.UploadedFileUrl,
		RejectionReason: row.RejectionReason,
		CreatedAt:       row.CreatedAt.Time,
		UpdatedAt:       row.UpdatedAt.Time,
	}, nil
}

func toOrderDetailFromRow(row db.GetOrderByUUIDRow) *model.OrderDetail {
	var shippingAddr model.Address
	_ = json.Unmarshal(row.ShippingAddress, &shippingAddr)

	var billingAddr model.Address
	_ = json.Unmarshal(row.BillingAddress, &billingAddr)

	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}

	var paidAt *time.Time
	if row.PaidAt.Valid {
		t := row.PaidAt.Time
		paidAt = &t
	}
	var shippedAt *time.Time
	if row.ShippedAt.Valid {
		t := row.ShippedAt.Time
		shippedAt = &t
	}
	var deliveredAt *time.Time
	if row.DeliveredAt.Valid {
		t := row.DeliveredAt.Time
		deliveredAt = &t
	}
	var cancelledAt *time.Time
	if row.CancelledAt.Valid {
		t := row.CancelledAt.Time
		cancelledAt = &t
	}

	return &model.OrderDetail{
		ID:                row.ID,
		UUID:              row.Uuid.String(),
		OrderNumber:       row.OrderNumber,
		OrderType:         row.OrderType,
		AccountID:         row.AccountID,
		AccountUUID:       row.AccountUuid.String(),
		AccountName:       row.AccountName,
		AccountType:       string(row.AccountType),
		UserID:            row.UserID,
		UserUUID:          row.UserUuid.String(),
		CustomerEmail:     row.CustomerEmail,
		CustomerName:      row.CustomerName,
		CustomerSurname:   row.CustomerSurname,
		CompanyName:       pgtypeToString(row.CompanyName),
		Status:            row.Status,
		FulfillmentStatus: row.FulfillmentStatus,
		Subtotal:          numericToFloat(row.Subtotal),
		TaxTotal:          numericToFloat(row.TaxTotal),
		ShippingTotal:     numericToFloat(row.ShippingTotal),
		DiscountTotal:     numericToFloat(row.DiscountTotal),
		GrandTotal:        numericToFloat(row.GrandTotal),
		Currency:          row.Currency,
		PaymentStatus:     row.PaymentStatus,
		PaymentProvider:   row.PaymentProvider,
		PaymentReference:  row.PaymentReference,
		ShippingAddress:   shippingAddr,
		BillingAddress:    billingAddr,
		ShippingCarrier:   row.ShippingCarrier,
		TrackingNumber:    row.TrackingNumber,
		TrackingURL:       row.TrackingUrl,
		CustomerNote:      row.CustomerNote,
		AdminNote:         row.AdminNote,
		Metadata:          metadata,
		PaidAt:            paidAt,
		ShippedAt:         shippedAt,
		DeliveredAt:       deliveredAt,
		CancelledAt:       cancelledAt,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func toOrderDetailFromOrderNumberRow(row db.GetOrderByOrderNumberRow) *model.OrderDetail {
	var shippingAddr model.Address
	_ = json.Unmarshal(row.ShippingAddress, &shippingAddr)

	var billingAddr model.Address
	_ = json.Unmarshal(row.BillingAddress, &billingAddr)

	var metadata map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &metadata)
	}

	var paidAt *time.Time
	if row.PaidAt.Valid {
		t := row.PaidAt.Time
		paidAt = &t
	}
	var shippedAt *time.Time
	if row.ShippedAt.Valid {
		t := row.ShippedAt.Time
		shippedAt = &t
	}
	var deliveredAt *time.Time
	if row.DeliveredAt.Valid {
		t := row.DeliveredAt.Time
		deliveredAt = &t
	}
	var cancelledAt *time.Time
	if row.CancelledAt.Valid {
		t := row.CancelledAt.Time
		cancelledAt = &t
	}

	return &model.OrderDetail{
		ID:                row.ID,
		UUID:              row.Uuid.String(),
		OrderNumber:       row.OrderNumber,
		OrderType:         row.OrderType,
		AccountID:         row.AccountID,
		AccountUUID:       row.AccountUuid.String(),
		AccountName:       row.AccountName,
		AccountType:       string(row.AccountType),
		UserID:            row.UserID,
		UserUUID:          row.UserUuid.String(),
		CustomerEmail:     row.CustomerEmail,
		CustomerName:      row.CustomerName,
		CustomerSurname:   row.CustomerSurname,
		CompanyName:       pgtypeToString(row.CompanyName),
		Status:            row.Status,
		FulfillmentStatus: row.FulfillmentStatus,
		Subtotal:          numericToFloat(row.Subtotal),
		TaxTotal:          numericToFloat(row.TaxTotal),
		ShippingTotal:     numericToFloat(row.ShippingTotal),
		DiscountTotal:     numericToFloat(row.DiscountTotal),
		GrandTotal:        numericToFloat(row.GrandTotal),
		Currency:          row.Currency,
		PaymentStatus:     row.PaymentStatus,
		PaymentProvider:   row.PaymentProvider,
		PaymentReference:  row.PaymentReference,
		ShippingAddress:   shippingAddr,
		BillingAddress:    billingAddr,
		ShippingCarrier:   row.ShippingCarrier,
		TrackingNumber:    row.TrackingNumber,
		TrackingURL:       row.TrackingUrl,
		CustomerNote:      row.CustomerNote,
		AdminNote:         row.AdminNote,
		Metadata:          metadata,
		PaidAt:            paidAt,
		ShippedAt:         shippedAt,
		DeliveredAt:       deliveredAt,
		CancelledAt:       cancelledAt,
		CreatedAt:         row.CreatedAt.Time,
		UpdatedAt:         row.UpdatedAt.Time,
	}
}

func textToPgtype(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func int64ToPgtype(i *int64) pgtype.Int8 {
	if i == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *i, Valid: true}
}

func pgtypeToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func pgUUIDToString(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0.0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0.0
	}
	return f.Float64
}

func floatToNumeric(f float64) pgtype.Numeric {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return pgtype.Numeric{Valid: false}
	}
	var num pgtype.Numeric
	_ = num.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return num
}

// ListManifestCards returns the provisioned cards of an order in item order (TEC-581).
func (r *PostgresRepository) ListManifestCards(ctx context.Context, orderUUID uuid.UUID) ([]model.ManifestCard, error) {
	rows, err := r.q.ListOrderManifest(ctx, orderUUID)
	if err != nil {
		return nil, err
	}
	out := make([]model.ManifestCard, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.ManifestCard{
			OrderNumber: row.OrderNumber, CardName: row.CardName, SmartCode: row.SmartCode, CardUID: row.CardUid.String,
		})
	}
	return out, nil
}

// RetireCardRoutes retires the routes of every card provisioned for the order (TEC-585).
func (r *PostgresRepository) RetireCardRoutes(ctx context.Context, orderID int64) ([]model.RetiredCardRoute, error) {
	rows, err := r.q.RetireOrderCardRoutes(ctx, pgtype.Int8{Int64: orderID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]model.RetiredCardRoute, 0, len(rows))
	for _, row := range rows {
		out = append(out, model.RetiredCardRoute{Path: row.Path, CardUUID: row.CardUuid})
	}
	return out, nil
}
